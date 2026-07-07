// Package applemusic is the driven adapter for the Apple Music catalog HTTP
// API. It reuses the Apple Music web player's own developer token (scraped from
// music.apple.com) together with a user-supplied media-user-token, so amp can
// add catalog albums to the library without an Apple Developer account. This is
// not a sanctioned integration: the scraped token can change without notice.
package applemusic

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"regexp"
)

const webPlayerHome = "https://music.apple.com/us/browse"

// bundleRef matches the main web-player JS bundle referenced by the home page.
var bundleRef = regexp.MustCompile(`/assets/index[^"']*\.js`)

// jwtPattern matches a JWT (the developer token is an ES256 JWT embedded in the
// bundle).
var jwtPattern = regexp.MustCompile(`eyJ[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+`)

// FetchDeveloperToken scrapes the Apple Music web player's developer token by
// loading the home page, finding its JS bundle, and extracting the JWT. The
// token is Apple's own web-player token; it rotates, so callers should fetch it
// fresh rather than persist it for long.
func FetchDeveloperToken(ctx context.Context, hc *http.Client) (string, error) {
	home, err := get(ctx, hc, webPlayerHome)
	if err != nil {
		return "", fmt.Errorf("apple music: load web player: %w", err)
	}
	ref := bundleRef.Find(home)
	if ref == nil {
		return "", fmt.Errorf("apple music: no JS bundle on web player page")
	}

	bundle, err := get(ctx, hc, "https://music.apple.com"+string(ref))
	if err != nil {
		return "", fmt.Errorf("apple music: load web player bundle: %w", err)
	}
	tok := jwtPattern.Find(bundle)
	if tok == nil {
		return "", fmt.Errorf("apple music: no developer token in web player bundle")
	}
	return string(tok), nil
}

// get fetches url with a browser-like User-Agent and returns the body.
func get(ctx context.Context, hc *http.Client, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", userAgent)
	resp, err := hc.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GET %s: %s", url, resp.Status)
	}
	return io.ReadAll(resp.Body)
}

const userAgent = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.0 Safari/605.1.15"
