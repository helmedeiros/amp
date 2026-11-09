package tui

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/helmedeiros/amp/internal/music"
)

var (
	chill   = music.Source{Kind: music.SourcePlaylist, Name: "Chill"}
	derez   = music.Track{Name: "Derezzed", Artist: "Daft Punk", Album: "TRON"}
	theGrid = music.Track{Name: "The Grid", Artist: "Daft Punk", Album: "TRON"}
)

func TestViewFor(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		status music.Status
		want   queueView
		ok     bool
	}{
		{name: "nothing playing keeps the view", status: music.Status{}, ok: false},
		{name: "amp queue", status: music.Status{Track: derez, Source: music.Source{Kind: music.SourceQueue, Name: "amp queue"}}, want: queueView{}, ok: true},
		{name: "playlist ignores the album", status: music.Status{Track: derez, Source: chill}, want: queueView{src: chill}, ok: true},
		{
			name: "library carries the album", status: music.Status{Track: derez, Source: music.Source{Kind: music.SourceLibrary, Name: "Library"}},
			want: queueView{src: music.Source{Kind: music.SourceLibrary, Name: "Library"}, album: "TRON"}, ok: true,
		},
		{
			name: "streaming carries the album", status: music.Status{Track: derez, Source: music.Source{Kind: music.SourceExternal}},
			want: queueView{src: music.Source{Kind: music.SourceExternal}, album: "TRON"}, ok: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, ok := viewFor(tt.status)

			assert.Equal(t, tt.ok, ok)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestQueueViewTitle(t *testing.T) {
	t.Parallel()

	assert.Empty(t, queueView{}.title(false), "amp's queue has no title")
	assert.Equal(t, "Following Music.app · playlist “Chill”", queueView{src: chill}.title(false))

	stream := queueView{src: music.Source{Kind: music.SourceExternal}, album: "TRON"}
	assert.Equal(t, "Following Music.app · Apple Music · this album’s tracks in your library", stream.title(false))
	assert.Equal(t, "Following Music.app · Apple Music · this album is not in your library", stream.title(true))
}

func newFollowApp(ctrl *stubController) app {
	stream := make(chan music.Status)
	close(stream)
	return newApp(context.Background(), ctrl, stream)
}

func update(t *testing.T, m app, msg tea.Msg) app {
	t.Helper()
	next, cmd := m.Update(msg)
	m = next.(app)
	for _, out := range drainBatch(cmd) {
		if items, ok := out.(tabItemsMsg); ok {
			next, _ = m.Update(items)
			m = next.(app)
		}
	}
	return m
}

func TestAppQueueFollowsPlaybackStartedInMusic(t *testing.T) {
	t.Parallel()

	ctrl := &stubController{
		queue:        []music.Track{{Name: "Gorgon", Artist: "Utsu-P"}},
		sourceTracks: []music.Track{theGrid, derez},
	}
	m := update(t, newFollowApp(ctrl), newFollowApp(ctrl).loadTab(tabQueue)())
	require.Equal(t, []string{"Utsu-P — Gorgon"}, m.items[tabQueue])

	m = update(t, m, statusMsg(music.Status{State: music.Playing, Track: derez, Source: chill}))

	assert.Equal(t, queueView{src: chill}, ctrl.sourceAsked)
	assert.Equal(t, trackLines([]music.Track{theGrid, derez}), m.items[tabQueue])
	assert.Equal(t, 1, m.lists[tabQueue].marker, "the playing track is marked")
	assert.Equal(t, 1, m.lists[tabQueue].Cursor(), "the cursor follows it")
	assert.Contains(t, m.View(), "Following Music.app · playlist “Chill”")
}

func TestAppFollowedQueueFitsTheTerminal(t *testing.T) {
	t.Parallel()

	tracks := make([]music.Track, 40)
	for i := range tracks {
		tracks[i] = music.Track{Name: fmt.Sprintf("Song %d", i), Artist: "Daft Punk", Album: "TRON"}
	}
	ctrl := &stubController{sourceTracks: tracks}
	m := update(t, newFollowApp(ctrl), tea.WindowSizeMsg{Width: 120, Height: 30})

	m = update(t, m, statusMsg(music.Status{State: music.Playing, Track: tracks[20], Source: chill}))

	view := m.View()
	assert.Equal(t, 30, strings.Count(view, "\n")+1, "the frame fills the terminal exactly")
	assert.True(t, strings.HasPrefix(view, stateStyle[music.Playing].Render("PLAYING")), "the header stays on screen")
	assert.Contains(t, view, "Following Music.app")
}

func TestAppQueueReturnsToAmpQueue(t *testing.T) {
	t.Parallel()

	ctrl := &stubController{
		queue:        []music.Track{{Name: "Gorgon", Artist: "Utsu-P"}},
		sourceTracks: []music.Track{derez},
	}
	m := update(t, newFollowApp(ctrl), statusMsg(music.Status{Track: derez, Source: chill}))
	require.True(t, m.queue.following())

	m = update(t, m, statusMsg(music.Status{Track: music.Track{Name: "Gorgon", Artist: "Utsu-P"}, Source: music.Source{Kind: music.SourceQueue, Name: "amp queue"}}))

	assert.False(t, m.queue.following())
	assert.Equal(t, []string{"Utsu-P — Gorgon"}, m.items[tabQueue])
	assert.NotContains(t, m.View(), "Following Music.app")
}

func TestAppQueueKeepsViewWhenPlaybackStops(t *testing.T) {
	t.Parallel()

	ctrl := &stubController{sourceTracks: []music.Track{derez}}
	m := update(t, newFollowApp(ctrl), statusMsg(music.Status{Track: derez, Source: chill}))

	m = update(t, m, statusMsg(music.Status{State: music.Stopped}))

	assert.Equal(t, queueView{src: chill}, m.queue)
	assert.Equal(t, trackLines([]music.Track{derez}), m.items[tabQueue])
}

func TestAppQueueDropsRowsReadForAnOldSource(t *testing.T) {
	t.Parallel()

	m := update(t, newFollowApp(&stubController{}), statusMsg(music.Status{Track: derez, Source: chill}))

	m = update(t, m, tabItemsMsg{tab: tabQueue, items: []string{"stale"}, view: queueView{}})

	assert.NotContains(t, m.items[tabQueue], "stale")
}

func TestAppSourceChangeOffQueueTabDefersLoad(t *testing.T) {
	t.Parallel()

	ctrl := &stubController{sourceTracks: []music.Track{derez}}
	m := newFollowApp(ctrl)
	m.active = tabPlaylists

	next, cmd := m.Update(statusMsg(music.Status{Track: derez, Source: chill}))
	m = next.(app)
	drainBatch(cmd)

	assert.NotContains(t, ctrl.calls, "SourceTracks", "the Queue is not on screen")
	assert.False(t, m.loaded[tabQueue], "it reloads when the user switches to it")
}

func takeOverResult(t *testing.T, cmd tea.Cmd) takeOverMsg {
	t.Helper()
	for _, out := range drainBatch(cmd) {
		if msg, ok := out.(takeOverMsg); ok {
			return msg
		}
	}
	t.Fatal("no takeOverMsg")
	return takeOverMsg{}
}

func TestAppEnterOnFollowedQueueTakesOverFromThatRow(t *testing.T) {
	t.Parallel()

	ctrl := &stubController{sourceTracks: []music.Track{theGrid, derez}, takeOverQueued: 2}
	m := update(t, newFollowApp(ctrl), statusMsg(music.Status{Track: derez, Source: chill}))
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("k")})
	m = next.(app)

	next, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = next.(app)
	assert.True(t, m.working)
	msg := takeOverResult(t, cmd)

	assert.NotContains(t, ctrl.calls, "PlayQueueAt")
	assert.Equal(t, queueView{src: chill}, ctrl.sourceAsked)
	assert.Equal(t, 0, ctrl.playedIdx)
	assert.Zero(t, ctrl.takeOverPos)

	next, _ = m.Update(msg)
	m = next.(app)
	assert.False(t, m.working)
	assert.False(t, m.queue.following())
	assert.Equal(t, tabQueue, m.active)
	assert.Contains(t, m.View(), "took over 2 track(s)")
}

func TestAppTKeyTakesOverAtThePlayingTrackAndPosition(t *testing.T) {
	t.Parallel()

	ctrl := &stubController{sourceTracks: []music.Track{theGrid, derez}, takeOverQueued: 2}
	status := music.Status{Track: derez, Elapsed: 42 * time.Second, Source: music.Source{Kind: music.SourceExternal}}
	m := update(t, newFollowApp(ctrl), statusMsg(status))

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("t")})
	msg := takeOverResult(t, cmd)

	assert.Equal(t, queueView{src: music.Source{Kind: music.SourceExternal}, album: "TRON"}, ctrl.sourceAsked)
	assert.Equal(t, 1, ctrl.playedIdx)
	assert.Equal(t, 42*time.Second, ctrl.takeOverPos)

	next, _ := m.Update(msg)
	assert.Contains(t, next.(app).View(), "this album's tracks in your library")
}

func TestAppTKeyStartsFromTheTopWhenThePlayingTrackIsMissing(t *testing.T) {
	t.Parallel()

	ctrl := &stubController{sourceTracks: []music.Track{theGrid}, takeOverQueued: 1}
	status := music.Status{Track: derez, Elapsed: 42 * time.Second, Source: music.Source{Kind: music.SourceExternal}}
	m := update(t, newFollowApp(ctrl), statusMsg(status))

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("t")})
	takeOverResult(t, cmd)

	assert.Equal(t, 0, ctrl.playedIdx)
	assert.Zero(t, ctrl.takeOverPos)
}

func TestAppTakeOverWithNothingToCopy(t *testing.T) {
	t.Parallel()

	ctrl := &stubController{}
	m := update(t, newFollowApp(ctrl), statusMsg(music.Status{Track: derez, Source: music.Source{Kind: music.SourceExternal}}))

	next, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("t")})

	assert.Nil(t, cmd)
	assert.NotContains(t, ctrl.calls, "TakeOver")
	assert.Contains(t, next.(app).View(), "none of these tracks are in your library")
}

func TestAppTakeOverFailureShowsNotice(t *testing.T) {
	t.Parallel()

	m := update(t, newFollowApp(&stubController{}), statusMsg(music.Status{Track: derez, Source: chill}))

	next, _ := m.Update(takeOverMsg{err: errors.New("boom")})
	assert.Contains(t, next.(app).View(), "take over failed: boom")
	assert.True(t, next.(app).queue.following())

	next, _ = m.Update(takeOverMsg{queued: 0, view: queueView{src: chill}})
	assert.Contains(t, next.(app).View(), "could not be copied")
}

func TestAppTKeyIgnoredOnAmpQueue(t *testing.T) {
	t.Parallel()

	ctrl := &stubController{}
	_, cmd := newTestApp(ctrl).Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("t")})

	assert.Nil(t, cmd)
	assert.Empty(t, ctrl.calls)
}

func TestAppNextAndPreviousKeys(t *testing.T) {
	t.Parallel()

	ctrl := &stubController{}
	m := newTestApp(ctrl)

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("n")})
	cmd()
	_, cmd = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("p")})
	cmd()

	assert.Equal(t, []string{"Next", "Previous"}, ctrl.calls)
}

func TestAppAmpPlayStopsFollowing(t *testing.T) {
	t.Parallel()

	m := update(t, newFollowApp(&stubController{sourceTracks: []music.Track{derez}}), statusMsg(music.Status{Track: derez, Source: chill}))

	next, _ := m.Update(queuePlayedMsg{})

	assert.False(t, next.(app).queue.following())
}
