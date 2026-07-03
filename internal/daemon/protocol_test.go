package daemon_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/helmedeiros/amp/internal/daemon"
	"github.com/helmedeiros/amp/internal/music"
)

func TestStatusDTORoundTrip(t *testing.T) {
	t.Parallel()

	want := music.Status{
		State:   music.Playing,
		Volume:  music.NewVolume(60),
		Shuffle: true,
		Repeat:  music.RepeatAll,
		Elapsed: 117 * time.Second,
		Track: music.Track{
			Name:     "Gorgon",
			Artist:   "Utsu-P",
			Album:    "Unknown Album",
			Duration: 255 * time.Second,
		},
	}

	got := daemon.ToStatusDTO(want).ToStatus()

	assert.Equal(t, want, got)
}

func TestStatusDTONoTrack(t *testing.T) {
	t.Parallel()

	dto := daemon.ToStatusDTO(music.Status{State: music.Stopped, Volume: music.NewVolume(30)})

	assert.Nil(t, dto.Track)
	assert.False(t, dto.ToStatus().HasTrack())
}
