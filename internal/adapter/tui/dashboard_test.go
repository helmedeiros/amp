package tui

import (
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"

	"github.com/helmedeiros/amp/internal/music"
)

func playingStatus() music.Status {
	return music.Status{
		State:   music.Playing,
		Volume:  music.NewVolume(60),
		Shuffle: true,
		Repeat:  music.RepeatAll,
		Elapsed: 117 * time.Second,
		Track:   music.Track{Name: "Gorgon", Artist: "Utsu-P", Album: "Unknown Album", Duration: 255 * time.Second},
	}
}

func TestDashboardShowsStatus(t *testing.T) {
	t.Parallel()

	m := newDashboard(nil)
	next, _ := m.Update(statusMsg(playingStatus()))
	view := next.(dashboard).View()

	assert.Contains(t, view, "PLAYING")
	assert.Contains(t, view, "Utsu-P — Gorgon")
	assert.Contains(t, view, "Unknown Album")
	assert.Contains(t, view, "01:57")
	assert.Contains(t, view, "04:15")
	assert.Contains(t, view, "━")
	assert.Contains(t, view, "vol 60%")
}

func TestDashboardConnectingBeforeStatus(t *testing.T) {
	t.Parallel()

	assert.Contains(t, newDashboard(nil).View(), "connecting")
}

func TestDashboardQuitsOnKey(t *testing.T) {
	t.Parallel()

	_, cmd := newDashboard(nil).Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})
	assert.NotNil(t, cmd, "q should issue a command (quit)")
}

func TestDashboardQuitsWhenStreamCloses(t *testing.T) {
	t.Parallel()

	next, cmd := newDashboard(nil).Update(streamClosedMsg{})
	assert.True(t, next.(dashboard).quitting)
	assert.NotNil(t, cmd)
}
