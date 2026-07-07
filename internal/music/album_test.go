package music_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/helmedeiros/amp/internal/music"
)

func TestAlbumCoveragePartial(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		cov     music.AlbumCoverage
		partial bool
	}{
		{"missing tracks", music.AlbumCoverage{Queued: 2, Total: 11}, true},
		{"complete album", music.AlbumCoverage{Queued: 11, Total: 11}, false},
		{"unknown total", music.AlbumCoverage{Queued: 2, Total: 0}, false},
		{"nothing queued", music.AlbumCoverage{Queued: 0, Total: 11}, false},
		{"stale metadata claims fewer", music.AlbumCoverage{Queued: 12, Total: 11}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tc.partial, tc.cov.Partial())
		})
	}
}
