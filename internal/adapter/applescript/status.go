package applescript

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/helmedeiros/amp/internal/music"
)

// ErrNotRunning is returned when Music.app is not running and therefore has no
// status to report.
var ErrNotRunning = errors.New("music: application is not running")

// statusScript is a single JavaScript-for-Automation program that reads the
// whole player snapshot in one osascript invocation and prints it as JSON.
// Reading everything at once avoids the per-field osascript latency the legacy
// shell scripts paid (see ADR-0004).
const statusScript = `
const Music = Application('Music');
const out = { running: Music.running() };
if (out.running) {
  out.state = Music.playerState();
  out.volume = Music.soundVolume();
  out.shuffle = Music.shuffleEnabled();
  out.repeat = Music.songRepeat();
  // The current track can be absent even when not strictly stopped (e.g. right
  // after a stop), in which case reading it throws -1728. Treat that as "no
  // track" rather than failing the whole status read.
  try {
    out.elapsed = Music.playerPosition();
    const t = Music.currentTrack;
    out.track = { name: t.name(), artist: t.artist(), album: t.album(), duration: t.duration() };
  } catch (e) {
    out.track = null;
  }
  try {
    const p = Music.currentPlaylist;
    out.source = { class: p.class(), name: p.name() };
  } catch (e) {
    out.source = null;
  }
}
JSON.stringify(out);
`

// statusDTO mirrors the JSON emitted by statusScript.
type statusDTO struct {
	Running bool       `json:"running"`
	State   string     `json:"state"`
	Volume  int        `json:"volume"`
	Shuffle bool       `json:"shuffle"`
	Repeat  string     `json:"repeat"`
	Elapsed float64    `json:"elapsed"`
	Track   trackDTO   `json:"track"`
	Source  *sourceDTO `json:"source"`
}

type sourceDTO struct {
	Class string `json:"class"`
	Name  string `json:"name"`
}

func toSource(src *sourceDTO, hasTrack bool) music.Source {
	switch {
	case !hasTrack:
		return music.Source{}
	case src == nil:
		return music.Source{Kind: music.SourceExternal}
	case src.Name == queuePlaylistName:
		return music.Source{Kind: music.SourceQueue, Name: src.Name}
	case src.Class == "libraryPlaylist":
		return music.Source{Kind: music.SourceLibrary, Name: src.Name}
	default:
		return music.Source{Kind: music.SourcePlaylist, Name: src.Name}
	}
}

// parseStatus maps the JSON status payload onto the domain Status. It returns
// ErrNotRunning when Music.app is not running.
func parseStatus(raw []byte) (music.Status, error) {
	var dto statusDTO
	if err := json.Unmarshal(raw, &dto); err != nil {
		return music.Status{}, fmt.Errorf("decode status: %w", err)
	}

	if !dto.Running {
		return music.Status{}, ErrNotRunning
	}

	state, err := music.ParsePlayerState(dto.State)
	if err != nil {
		return music.Status{}, err
	}

	repeat, err := music.ParseRepeatMode(dto.Repeat)
	if err != nil {
		return music.Status{}, err
	}

	track := dto.Track.toTrack()
	return music.Status{
		State:   state,
		Volume:  music.NewVolume(dto.Volume),
		Shuffle: dto.Shuffle,
		Repeat:  repeat,
		Elapsed: seconds(dto.Elapsed),
		Track:   track,
		Source:  toSource(dto.Source, !track.IsZero()),
	}, nil
}

// seconds converts a floating-point seconds value (as Music.app reports
// positions and durations) into a time.Duration.
func seconds(s float64) time.Duration {
	return time.Duration(s * float64(time.Second))
}
