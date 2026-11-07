package music

import "strings"

// SourceKind says where Music.app is playing from.
type SourceKind int

// Source kinds.
const (
	SourceNone SourceKind = iota
	SourceQueue
	SourcePlaylist
	SourceLibrary
	SourceExternal
)

func (k SourceKind) String() string {
	switch k {
	case SourceQueue:
		return "queue"
	case SourcePlaylist:
		return "playlist"
	case SourceLibrary:
		return "library"
	case SourceExternal:
		return "external"
	default:
		return "none"
	}
}

// ParseSourceKind maps a wire name to a SourceKind, defaulting to SourceNone.
func ParseSourceKind(raw string) SourceKind {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "queue":
		return SourceQueue
	case "playlist":
		return SourcePlaylist
	case "library":
		return SourceLibrary
	case "external":
		return SourceExternal
	default:
		return SourceNone
	}
}

// Source is what the current track is playing from.
type Source struct {
	Kind SourceKind
	Name string
}

// IsExternal reports whether playback was started outside amp.
func (s Source) IsExternal() bool {
	switch s.Kind {
	case SourcePlaylist, SourceLibrary, SourceExternal:
		return true
	default:
		return false
	}
}

// Label is a short human description of the source.
func (s Source) Label() string {
	switch s.Kind {
	case SourceQueue:
		return s.Name
	case SourcePlaylist:
		return "playlist “" + s.Name + "”"
	case SourceLibrary:
		return "library"
	case SourceExternal:
		return "Apple Music"
	default:
		return ""
	}
}
