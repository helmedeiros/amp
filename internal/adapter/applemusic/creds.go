package applemusic

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Creds are the stored Apple Music credentials: the user's media-user-token
// (valid ~180 days) and their storefront (e.g. "us", "de"). The developer token
// is not stored — it is scraped fresh from the web player on each run.
type Creds struct {
	MediaUserToken string `json:"media_user_token"`
	Storefront     string `json:"storefront"`
}

// Valid reports whether the credentials carry the fields needed to call the API.
func (c Creds) Valid() bool {
	return strings.TrimSpace(c.MediaUserToken) != "" && strings.TrimSpace(c.Storefront) != ""
}

// LoadCreds reads credentials from path. A missing file yields zero Creds and no
// error, so callers can treat "not configured" as an ordinary state.
func LoadCreds(path string) (Creds, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return Creds{}, nil
		}
		return Creds{}, fmt.Errorf("apple music: read creds: %w", err)
	}
	var c Creds
	if err := json.Unmarshal(data, &c); err != nil {
		return Creds{}, fmt.Errorf("apple music: decode creds: %w", err)
	}
	return c, nil
}

// SaveCreds writes credentials to path with owner-only permissions, since the
// token grants read/write access to the user's whole library.
func SaveCreds(path string, c Creds) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("apple music: create config dir: %w", err)
	}
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return fmt.Errorf("apple music: write creds: %w", err)
	}
	return nil
}

// storefrontResponse is the slice of GET /v1/me/storefront we consume.
type storefrontResponse struct {
	Data []struct {
		ID string `json:"id"`
	} `json:"data"`
}

// Authenticate validates a media-user-token by scraping a developer token and
// reading the account's storefront, returning ready-to-store credentials. It
// fails when the token is rejected, so `amp auth apple-music` can report a bad
// paste rather than saving credentials that never work.
func Authenticate(ctx context.Context, mediaUserToken string) (Creds, error) {
	mediaUserToken = strings.TrimSpace(mediaUserToken)
	if mediaUserToken == "" {
		return Creds{}, fmt.Errorf("apple music: empty media-user-token")
	}

	// Storefront is unknown until we ask; use a temporary client keyed on the
	// catalog default and read /v1/me/storefront, which returns the user's own.
	c := NewClient(Creds{MediaUserToken: mediaUserToken, Storefront: "us"})
	c.hc = &http.Client{Timeout: 15 * time.Second}

	body, err := c.apiGet(ctx, "/v1/me/storefront")
	if err != nil {
		return Creds{}, fmt.Errorf("apple music: token rejected (%w)", err)
	}
	var sr storefrontResponse
	if err := json.Unmarshal(body, &sr); err != nil || len(sr.Data) == 0 {
		return Creds{}, fmt.Errorf("apple music: could not read storefront")
	}
	return Creds{MediaUserToken: mediaUserToken, Storefront: sr.Data[0].ID}, nil
}

// StatusMessage renders a one-line connection summary from stored credentials
// and the result of a token check (tokenErr is the error from Client.Verify, or
// nil when the token still works). It is pure so it can be unit-tested.
func StatusMessage(creds Creds, tokenErr error) string {
	switch {
	case !creds.Valid():
		return "Apple Music: not connected. Run `amp auth apple-music` to connect."
	case tokenErr != nil:
		return fmt.Sprintf("Apple Music: connected (storefront %s) but the token was rejected — re-run `amp auth apple-music`.", creds.Storefront)
	default:
		return fmt.Sprintf("Apple Music: connected ✓ (storefront %s).", creds.Storefront)
	}
}

// CredsPath returns where credentials are stored, under the user config dir.
func CredsPath() string {
	dir, err := os.UserConfigDir()
	if err != nil {
		return ".amp-apple-music.json"
	}
	return filepath.Join(dir, "amp", "apple-music.json")
}
