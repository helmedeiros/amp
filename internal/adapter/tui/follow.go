package tui

import "github.com/helmedeiros/amp/internal/music"

type queueView struct {
	src   music.Source
	album string
}

func (v queueView) following() bool {
	return v.src.IsExternal()
}

func viewFor(s music.Status) (v queueView, ok bool) {
	switch s.Source.Kind {
	case music.SourceNone:
		return queueView{}, false
	case music.SourceQueue:
		return queueView{}, true
	case music.SourcePlaylist:
		return queueView{src: s.Source}, true
	default:
		return queueView{src: s.Source, album: s.Track.Album}, true
	}
}

func (v queueView) title(empty bool) string {
	if !v.following() {
		return ""
	}
	t := "Following Music.app · " + v.src.Label()
	if v.src.Kind != music.SourcePlaylist {
		if empty {
			return t + " · this album is not in your library"
		}
		t += " · this album’s tracks in your library"
	}
	return t
}
