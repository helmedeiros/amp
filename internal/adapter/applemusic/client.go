package applemusic

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/helmedeiros/amp/internal/music"
	"github.com/helmedeiros/amp/internal/port"
)

const (
	apiBase = "https://api.music.apple.com"
	origin  = "https://music.apple.com"
)

// Client talks to the Apple Music catalog API using the scraped web-player
// developer token plus the user's media-user-token. It implements port.Catalog.
type Client struct {
	hc       *http.Client
	creds    Creds
	baseURL  string
	fetchDev func(context.Context, *http.Client) (string, error)

	mu       sync.Mutex
	devToken string
}

var _ port.Catalog = (*Client)(nil)

// NewClient builds a Client for the given credentials, scraping the developer
// token lazily on first use.
func NewClient(creds Creds) *Client {
	return &Client{
		hc:       &http.Client{Timeout: 15 * time.Second},
		creds:    creds,
		baseURL:  apiBase,
		fetchDev: FetchDeveloperToken,
	}
}

// searchResponse is the slice of the catalog search payload we consume.
type searchResponse struct {
	Results struct {
		Albums struct {
			Data []struct {
				ID         string `json:"id"`
				Attributes struct {
					Name       string `json:"name"`
					ArtistName string `json:"artistName"`
					TrackCount int    `json:"trackCount"`
				} `json:"attributes"`
			} `json:"data"`
		} `json:"albums"`
	} `json:"results"`
}

// ResolveAlbum searches the catalog and returns the ID of the album whose name
// and artist match exactly (case-insensitively), preferring the edition with
// the most tracks. It returns an empty ID when nothing matches.
func (c *Client) ResolveAlbum(ctx context.Context, name, artist string) (string, error) {
	q := url.Values{}
	q.Set("term", strings.TrimSpace(artist+" "+name))
	q.Set("types", "albums")
	q.Set("limit", "25")
	path := fmt.Sprintf("/v1/catalog/%s/search?%s", url.PathEscape(c.creds.Storefront), q.Encode())

	body, err := c.apiGet(ctx, path)
	if err != nil {
		return "", err
	}
	var sr searchResponse
	if err := json.Unmarshal(body, &sr); err != nil {
		return "", fmt.Errorf("apple music: decode search: %w", err)
	}

	bestID, bestTracks := "", -1
	for _, a := range sr.Results.Albums.Data {
		if strings.EqualFold(a.Attributes.Name, name) &&
			strings.EqualFold(a.Attributes.ArtistName, artist) &&
			a.Attributes.TrackCount > bestTracks {
			bestID, bestTracks = a.ID, a.Attributes.TrackCount
		}
	}
	return bestID, nil
}

// artistSearchResponse is the slice of an artist search we consume.
type artistSearchResponse struct {
	Results struct {
		Artists struct {
			Data []struct {
				ID         string `json:"id"`
				Attributes struct {
					Name string `json:"name"`
				} `json:"attributes"`
			} `json:"data"`
		} `json:"artists"`
	} `json:"results"`
}

// albumsResponse is the slice of an artist's albums relationship we consume.
type albumsResponse struct {
	Next string `json:"next"`
	Data []struct {
		ID         string `json:"id"`
		Attributes struct {
			Name          string `json:"name"`
			TrackCount    int    `json:"trackCount"`
			IsSingle      bool   `json:"isSingle"`
			IsCompilation bool   `json:"isCompilation"`
		} `json:"attributes"`
	} `json:"data"`
}

// ArtistAlbums resolves the artist and returns their catalog albums with singles
// excluded and multiple editions collapsed to one entry each (preferring the
// standard edition — the fewest tracks). It leaves finer curation to the caller.
func (c *Client) ArtistAlbums(ctx context.Context, artist string) ([]music.CatalogAlbum, error) {
	body, err := c.apiGet(ctx, fmt.Sprintf("/v1/catalog/%s/search?term=%s&types=artists&limit=5",
		url.PathEscape(c.creds.Storefront), url.QueryEscape(artist)))
	if err != nil {
		return nil, err
	}
	var sr artistSearchResponse
	if err := json.Unmarshal(body, &sr); err != nil {
		return nil, fmt.Errorf("apple music: decode artist search: %w", err)
	}
	var artistID string
	for _, a := range sr.Results.Artists.Data {
		if strings.EqualFold(a.Attributes.Name, artist) {
			artistID = a.ID
			break
		}
	}
	if artistID == "" && len(sr.Results.Artists.Data) > 0 {
		artistID = sr.Results.Artists.Data[0].ID
	}
	if artistID == "" {
		return nil, nil
	}

	// Collapse editions by base name, keeping the fewest-track (standard) one.
	best := map[string]music.CatalogAlbum{}
	path := fmt.Sprintf("/v1/catalog/%s/artists/%s/albums?limit=100", url.PathEscape(c.creds.Storefront), artistID)
	for page := 0; path != "" && page < 4; page++ {
		body, err := c.apiGet(ctx, path)
		if err != nil {
			return nil, err
		}
		var ar albumsResponse
		if err := json.Unmarshal(body, &ar); err != nil {
			return nil, fmt.Errorf("apple music: decode albums: %w", err)
		}
		for _, a := range ar.Data {
			if a.Attributes.IsSingle {
				continue
			}
			key := baseAlbumName(a.Attributes.Name)
			cur, ok := best[key]
			if !ok || a.Attributes.TrackCount < cur.TrackCount {
				best[key] = music.CatalogAlbum{ID: a.ID, Name: a.Attributes.Name, TrackCount: a.Attributes.TrackCount}
			}
		}
		path = ar.Next
	}

	out := make([]music.CatalogAlbum, 0, len(best))
	for _, a := range best {
		out = append(out, a)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

// editionSuffix matches trailing "(… Edition)" / "[… Remaster]" style qualifiers
// so different editions of one album collapse together.
var editionSuffix = regexp.MustCompile(`(?i)\s*[\(\[][^\)\]]*(edition|deluxe|remaster|anniversar|expanded|version|bonus|special|super|mono|stereo|reissue|explicit|remix|legacy|mix)[^\)\]]*[\)\]]`)

func baseAlbumName(name string) string {
	for {
		next := strings.TrimSpace(editionSuffix.ReplaceAllString(name, ""))
		if next == name {
			return strings.ToLower(name)
		}
		name = next
	}
}

// Verify checks that the stored credentials still authenticate, by reading the
// account storefront. A non-nil error means the token is missing/expired.
func (c *Client) Verify(ctx context.Context) error {
	_, err := c.apiGet(ctx, "/v1/me/storefront")
	return err
}

// AddAlbum adds the catalog album to the user's library. Apple returns 202
// Accepted; the tracks appear once iCloud Music Library syncs.
func (c *Client) AddAlbum(ctx context.Context, albumID string) error {
	if strings.TrimSpace(albumID) == "" {
		return fmt.Errorf("apple music: empty album id")
	}
	path := "/v1/me/library?ids[albums]=" + url.QueryEscape(albumID)
	resp, err := c.do(ctx, http.MethodPost, path, true)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusAccepted && resp.StatusCode != http.StatusNoContent {
		return fmt.Errorf("apple music: add album: %s", resp.Status)
	}
	return nil
}

// apiGet performs an authenticated GET and returns the body, re-scraping the
// developer token once on an auth failure.
func (c *Client) apiGet(ctx context.Context, path string) ([]byte, error) {
	resp, err := c.do(ctx, http.MethodGet, path, true)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("apple music: GET %s: %s", path, resp.Status)
	}
	return io.ReadAll(resp.Body)
}

// do issues an authenticated request, refreshing the developer token once if the
// first attempt is rejected as unauthorized (the scraped token may have rotated).
func (c *Client) do(ctx context.Context, method, path string, retry bool) (*http.Response, error) {
	tok, err := c.developerToken(ctx, false)
	if err != nil {
		return nil, err
	}
	resp, err := c.send(ctx, method, path, tok)
	if err != nil {
		return nil, err
	}
	if retry && (resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden) {
		_ = resp.Body.Close()
		if tok, err = c.developerToken(ctx, true); err != nil {
			return nil, err
		}
		return c.send(ctx, method, path, tok)
	}
	return resp, nil
}

func (c *Client) send(ctx context.Context, method, path, devToken string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+devToken)
	req.Header.Set("Music-User-Token", c.creds.MediaUserToken)
	req.Header.Set("Origin", origin)
	req.Header.Set("User-Agent", userAgent)
	return c.hc.Do(req)
}

// developerToken returns the cached scraped token, fetching it when absent or
// when force refreshes a rotated one.
func (c *Client) developerToken(ctx context.Context, force bool) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.devToken != "" && !force {
		return c.devToken, nil
	}
	tok, err := c.fetchDev(ctx, c.hc)
	if err != nil {
		return "", err
	}
	c.devToken = tok
	return tok, nil
}
