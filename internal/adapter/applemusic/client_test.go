package applemusic

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// testClient builds a Client pointed at srv with a stubbed developer token.
func testClient(srv *httptest.Server, creds Creds) *Client {
	return &Client{
		hc:       srv.Client(),
		creds:    creds,
		baseURL:  srv.URL,
		fetchDev: func(context.Context, *http.Client) (string, error) { return "dev-tok", nil },
	}
}

func TestResolveAlbumPicksExactNameAndArtist(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "Bearer dev-tok", r.Header.Get("Authorization"))
		assert.Equal(t, "mut", r.Header.Get("Music-User-Token"))
		assert.Contains(t, r.URL.Path, "/v1/catalog/de/search")
		_, _ = w.Write([]byte(`{"results":{"albums":{"data":[
			{"id":"deluxe","attributes":{"name":"Franz Ferdinand (Deluxe)","artistName":"Franz Ferdinand","trackCount":20}},
			{"id":"live","attributes":{"name":"Franz Ferdinand","artistName":"A Tribute Band","trackCount":11}},
			{"id":"real","attributes":{"name":"Franz Ferdinand","artistName":"Franz Ferdinand","trackCount":11}}
		]}}}`))
	}))
	defer srv.Close()

	c := testClient(srv, Creds{MediaUserToken: "mut", Storefront: "de"})
	id, err := c.ResolveAlbum(context.Background(), "Franz Ferdinand", "Franz Ferdinand")

	require.NoError(t, err)
	assert.Equal(t, "real", id, "exact name+artist wins over a deluxe edition or tribute")
}

func TestResolveAlbumNoMatchReturnsEmpty(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"results":{}}`))
	}))
	defer srv.Close()

	c := testClient(srv, Creds{Storefront: "us"})
	id, err := c.ResolveAlbum(context.Background(), "Nope", "Nobody")

	require.NoError(t, err)
	assert.Empty(t, id)
}

func TestAddAlbumSendsPostAndAcceptsAccepted(t *testing.T) {
	t.Parallel()

	var gotMethod, gotQuery, gotOrigin string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod, gotQuery, gotOrigin = r.Method, r.URL.RawQuery, r.Header.Get("Origin")
		w.WriteHeader(http.StatusAccepted)
	}))
	defer srv.Close()

	c := testClient(srv, Creds{MediaUserToken: "mut", Storefront: "de"})
	err := c.AddAlbum(context.Background(), "315843479")

	require.NoError(t, err)
	assert.Equal(t, http.MethodPost, gotMethod)
	assert.Contains(t, gotQuery, "ids[albums]=315843479")
	assert.Equal(t, origin, gotOrigin)
}

func TestAddAlbumErrorsOnRejection(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer srv.Close()

	c := testClient(srv, Creds{MediaUserToken: "mut", Storefront: "de"})
	// Forbidden triggers one dev-token refresh then a second forbidden -> error.
	err := c.AddAlbum(context.Background(), "1")
	require.Error(t, err)
}

func TestDoRefreshesDeveloperTokenOnUnauthorized(t *testing.T) {
	t.Parallel()

	var authSeen []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authSeen = append(authSeen, r.Header.Get("Authorization"))
		if len(authSeen) == 1 {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.WriteHeader(http.StatusAccepted)
	}))
	defer srv.Close()

	calls := 0
	c := &Client{
		hc:      srv.Client(),
		creds:   Creds{MediaUserToken: "mut", Storefront: "de"},
		baseURL: srv.URL,
		fetchDev: func(context.Context, *http.Client) (string, error) {
			calls++
			return "tok-v" + string(rune('0'+calls)), nil
		},
	}
	require.NoError(t, c.AddAlbum(context.Background(), "1"))
	assert.Equal(t, 2, calls, "a 401 forces a fresh developer-token scrape")
	assert.Equal(t, []string{"Bearer tok-v1", "Bearer tok-v2"}, authSeen)
}
