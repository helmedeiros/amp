package applemusic

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStatusMessage(t *testing.T) {
	t.Parallel()

	assert.Contains(t, StatusMessage(Creds{}, nil), "not connected")

	ok := StatusMessage(Creds{MediaUserToken: "t", Storefront: "de"}, nil)
	assert.Contains(t, ok, "connected ✓")
	assert.Contains(t, ok, "de")

	bad := StatusMessage(Creds{MediaUserToken: "t", Storefront: "de"}, errors.New("401"))
	assert.Contains(t, bad, "rejected")
}

func TestCredsRoundTripAndPermissions(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "nested", "apple-music.json")
	want := Creds{MediaUserToken: "secret-token", Storefront: "de"}

	require.NoError(t, SaveCreds(path, want))

	got, err := LoadCreds(path)
	require.NoError(t, err)
	assert.Equal(t, want, got)

	info, err := os.Stat(path)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o600), info.Mode().Perm(), "token file is owner-only")
}

func TestLoadCredsMissingIsNotAnError(t *testing.T) {
	t.Parallel()

	got, err := LoadCreds(filepath.Join(t.TempDir(), "absent.json"))
	require.NoError(t, err)
	assert.False(t, got.Valid())
}

func TestCredsValid(t *testing.T) {
	t.Parallel()

	assert.True(t, Creds{MediaUserToken: "t", Storefront: "us"}.Valid())
	assert.False(t, Creds{MediaUserToken: "t"}.Valid(), "storefront required")
	assert.False(t, Creds{Storefront: "us"}.Valid(), "token required")
}
