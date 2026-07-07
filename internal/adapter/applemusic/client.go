package applemusic

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

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
