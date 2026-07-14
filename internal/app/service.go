// Package app holds the application use cases. It orchestrates the domain and
// the ports and depends on neither the CLI nor any concrete engine adapter.
package app

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/helmedeiros/amp/internal/music"
	"github.com/helmedeiros/amp/internal/port"
)

// DefaultUnmuteVolume is the level unmute restores when no prior level is known.
const DefaultUnmuteVolume = 50

// Service is the application's entry point for controlling playback. It is the
// single object the driving adapters (CLI, later the TUI) call into.
type Service struct {
	player  port.Player
	volume  port.VolumeStore
	catalog port.Catalog    // optional; nil unless Apple Music auth is configured
	sc      port.SoundCloud // optional; nil unless the import tools are wired
}

// NewService wires the service to a Player and a VolumeStore.
func NewService(player port.Player, volume port.VolumeStore) *Service {
	return &Service{player: player, volume: volume}
}

// EnableCatalog attaches an Apple Music catalog client so that playing a
// partly-in-library album first adds the missing tracks. Optional: without it,
// partial albums are played as-is and only reported.
func (s *Service) EnableCatalog(c port.Catalog) { s.catalog = c }

// EnableSoundCloud attaches a SoundCloud importer used by ImportSoundCloud.
func (s *Service) EnableSoundCloud(sc port.SoundCloud) { s.sc = sc }

// Attribute derives the name/artist/album for an imported track from its
// SoundCloud title: a "Band - Song" title is credited to the band (album = band
// too); anything else is credited to soloArtist.
func Attribute(title, soloArtist string) music.Attribution {
	if i := strings.Index(title, " - "); i >= 0 {
		band, song := strings.TrimSpace(title[:i]), strings.TrimSpace(title[i+len(" - "):])
		if band != "" && song != "" {
			return music.Attribution{Name: song, Artist: band, Album: band}
		}
	}
	return music.Attribution{Name: strings.TrimSpace(title), Artist: soloArtist, Album: soloArtist}
}

// ImportSoundCloud imports the tracks at url into the library and a playlist,
// crediting non-band tracks to soloArtist (falling back to each track's
// uploader). Tracks already in the library are skipped, so it is safe to re-run.
func (s *Service) ImportSoundCloud(ctx context.Context, url, playlist, soloArtist, destDir string) (port.ImportResult, error) {
	if s.sc == nil {
		return port.ImportResult{}, fmt.Errorf("soundcloud import not available")
	}
	if strings.TrimSpace(playlist) == "" {
		playlist = "SoundCloud"
	}
	tracks, err := s.sc.List(ctx, url)
	if err != nil {
		return port.ImportResult{}, err
	}

	var res port.ImportResult
	for _, t := range tracks {
		solo := soloArtist
		if solo == "" {
			solo = t.Uploader
		}
		attr := Attribute(t.Title, solo)

		if exists, err := s.player.TrackExists(ctx, attr.Name, attr.Artist); err == nil && exists {
			res.Skipped++
			continue
		}
		path, err := s.sc.Fetch(ctx, t.URL, destDir, attr)
		if err != nil {
			res.Failed++
			continue
		}
		if err := s.player.AddFile(ctx, path, playlist); err != nil {
			res.Failed++
			continue
		}
		res.Imported = append(res.Imported, attr.Artist+" — "+attr.Name)
	}
	return res, nil
}

// albumSyncTimeout bounds how long a play waits for added tracks to sync into
// the library before falling back to playing what is already there.
// albumSyncPoll is how often the library is re-checked meanwhile. They are vars
// so tests can shrink them.
var (
	albumSyncTimeout = 15 * time.Second
	albumSyncPoll    = 2 * time.Second
)

var _ port.Controller = (*Service)(nil)

// Status reads the current player snapshot.
func (s *Service) Status(ctx context.Context) (music.Status, error) {
	return s.player.Status(ctx)
}

// Open launches the music application.
func (s *Service) Open(ctx context.Context) error { return s.player.Open(ctx) }

// SaveArtwork writes the current track's album artwork to path.
func (s *Service) SaveArtwork(ctx context.Context, path string) error {
	if strings.TrimSpace(path) == "" {
		return fmt.Errorf("artwork: empty path")
	}
	return s.player.SaveArtwork(ctx, path)
}

// PlayQuery resumes playback when query is empty; otherwise it resolves the
// query, in order, to a playlist name, an album name (both matched
// case-insensitively), or a track search, and plays the first that matches.
func (s *Service) PlayQuery(ctx context.Context, query string, limit int) (port.PlayResult, error) {
	q := strings.TrimSpace(query)
	if q == "" {
		return port.PlayResult{Kind: "resume"}, s.player.Play(ctx)
	}

	if pls, err := s.player.Playlists(ctx); err == nil {
		for _, p := range pls {
			if strings.EqualFold(p.Name, q) {
				return port.PlayResult{Kind: "playlist", Label: p.Name}, s.player.PlayPlaylist(ctx, p.Name)
			}
		}
	}

	if albums, err := s.player.Albums(ctx); err == nil {
		for _, a := range albums {
			if strings.EqualFold(a.Name, q) {
				filled := s.fillAlbumIfPartial(ctx, a)
				coverage, err := s.player.PlayAlbum(ctx, a.Name)
				return port.PlayResult{Kind: "album", Label: a.Name, Album: coverage, AlbumFilled: filled}, err
			}
		}
	}

	tracks, err := s.player.Search(ctx, q, limit)
	if err != nil {
		return port.PlayResult{}, fmt.Errorf("play: %w", err)
	}
	if len(tracks) == 0 {
		return port.PlayResult{}, fmt.Errorf("play: nothing matched %q", q)
	}
	if err := s.player.PlaySearch(ctx, q, limit, 0); err != nil {
		return port.PlayResult{}, err
	}

	first := tracks[0]
	label := first.Name
	if first.Artist != "" {
		label = first.Artist + " — " + first.Name
	}
	return port.PlayResult{Kind: "track", Label: label}, nil
}

// fillAlbumIfPartial adds an album's missing tracks via the Apple Music catalog
// when a catalog client is configured and the album is only partly in the
// library, then waits (up to albumSyncTimeout) for the added tracks to sync in.
// It is best-effort. It returns true when an add was attempted (so callers can
// tell "still syncing" apart from "no catalog configured").
func (s *Service) fillAlbumIfPartial(ctx context.Context, a music.Album) bool {
	if s.catalog == nil {
		return false
	}
	cov, err := s.player.AlbumCoverage(ctx, a.Name)
	if err != nil || !cov.Partial() {
		return false
	}

	id, err := s.catalog.ResolveAlbum(ctx, a.Name, a.Artist)
	if err != nil || id == "" {
		return false
	}
	if err := s.catalog.AddAlbum(ctx, id); err != nil {
		return false
	}
	s.awaitAlbumSync(ctx, a.Name, cov.Total)
	return true
}

// awaitAlbumSync polls the library until the album holds want tracks or the
// timeout elapses.
func (s *Service) awaitAlbumSync(ctx context.Context, name string, want int) {
	deadline := time.NewTimer(albumSyncTimeout)
	defer deadline.Stop()
	ticker := time.NewTicker(albumSyncPoll)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-deadline.C:
			return
		case <-ticker.C:
			if cov, err := s.player.AlbumCoverage(ctx, name); err == nil && cov.Queued >= want {
				return
			}
		}
	}
}

// Search returns library tracks matching query, up to limit (<= 0 for all).
// The query must be non-empty once trimmed.
func (s *Service) Search(ctx context.Context, query string, limit int) ([]music.Track, error) {
	q := strings.TrimSpace(query)
	if q == "" {
		return nil, fmt.Errorf("search: empty query")
	}
	return s.player.Search(ctx, q, limit)
}

// PlaySearch plays the search results starting at the given index, queueing the
// rest. The query must be non-empty and start must be within range.
func (s *Service) PlaySearch(ctx context.Context, query string, limit, start int) error {
	q := strings.TrimSpace(query)
	if q == "" {
		return fmt.Errorf("play: empty query")
	}
	if start < 0 {
		return fmt.Errorf("play: negative start index %d", start)
	}
	return s.player.PlaySearch(ctx, q, limit, start)
}

// PlayQueueAt plays the queue starting at the given index.
func (s *Service) PlayQueueAt(ctx context.Context, index int) error {
	if index < 0 {
		return fmt.Errorf("play queue: negative index %d", index)
	}
	return s.player.PlayQueueAt(ctx, index)
}

// Queue returns the tracks currently in the queue.
func (s *Service) Queue(ctx context.Context) ([]music.Track, error) {
	return s.player.Queue(ctx)
}

// QueueAdd appends the search results for a non-empty query to the queue and
// returns how many were added.
func (s *Service) QueueAdd(ctx context.Context, query string, limit int) (int, error) {
	q := strings.TrimSpace(query)
	if q == "" {
		return 0, fmt.Errorf("queue add: empty query")
	}
	return s.player.QueueAdd(ctx, q, limit)
}

// QueueClear empties the queue.
func (s *Service) QueueClear(ctx context.Context) error {
	return s.player.QueueClear(ctx)
}

// Playlists returns the user's playlists.
func (s *Service) Playlists(ctx context.Context) ([]music.Playlist, error) {
	return s.player.Playlists(ctx)
}

// Artists returns the distinct, sorted artist names in the library.
func (s *Service) Artists(ctx context.Context) ([]string, error) {
	return s.player.Artists(ctx)
}

// Albums returns the distinct, sorted albums in the library, each with its
// artist.
func (s *Service) Albums(ctx context.Context) ([]music.Album, error) {
	return s.player.Albums(ctx)
}

// CatalogEnabled reports whether an Apple Music catalog client is configured.
func (s *Service) CatalogEnabled() bool { return s.catalog != nil }

// ArtistCatalogAlbums returns the artist's catalog albums that are not already
// in the library — the candidates for the "add albums from Apple Music" flow.
// It returns an error when no catalog client is configured.
func (s *Service) ArtistCatalogAlbums(ctx context.Context, artist string) ([]music.CatalogAlbum, error) {
	if s.catalog == nil {
		return nil, fmt.Errorf("apple music not configured: run `amp auth apple-music`")
	}
	catalog, err := s.catalog.ArtistAlbums(ctx, artist)
	if err != nil {
		return nil, err
	}

	// Drop the ones already present in the library (matched by base name).
	have := map[string]bool{}
	if lib, err := s.player.Albums(ctx); err == nil {
		for _, a := range lib {
			if strings.EqualFold(a.Artist, artist) || a.Artist == "" {
				have[baseName(a.Name)] = true
			}
		}
	}
	missing := make([]music.CatalogAlbum, 0, len(catalog))
	for _, a := range catalog {
		if !have[baseName(a.Name)] {
			missing = append(missing, a)
		}
	}
	return missing, nil
}

// AddCatalogAlbums adds the given catalog album IDs to the library, returning how
// many succeeded. Tracks appear once iCloud Music Library syncs.
func (s *Service) AddCatalogAlbums(ctx context.Context, ids []string) (int, error) {
	if s.catalog == nil {
		return 0, fmt.Errorf("apple music not configured: run `amp auth apple-music`")
	}
	added := 0
	for _, id := range ids {
		if err := s.catalog.AddAlbum(ctx, id); err != nil {
			return added, err
		}
		added++
	}
	return added, nil
}

// baseName strips edition/remaster qualifiers so album names compare across
// catalog and library.
func baseName(name string) string {
	for {
		next := strings.TrimSpace(editionSuffix.ReplaceAllString(name, ""))
		if next == name {
			return strings.ToLower(name)
		}
		name = next
	}
}

var editionSuffix = regexp.MustCompile(`(?i)\s*[\(\[][^\)\]]*(edition|deluxe|remaster|anniversar|expanded|version|bonus|special|super|mono|stereo|reissue|explicit|remix|legacy|mix)[^\)\]]*[\)\]]`)

// Play resumes or starts playback.
func (s *Service) Play(ctx context.Context) error { return s.player.Play(ctx) }

// Pause halts playback.
func (s *Service) Pause(ctx context.Context) error { return s.player.Pause(ctx) }

// Toggle flips between playing and paused.
func (s *Service) Toggle(ctx context.Context) error { return s.player.TogglePlayPause(ctx) }

// Stop halts playback and unloads the current track.
func (s *Service) Stop(ctx context.Context) error { return s.player.Stop(ctx) }

// Next advances to the next track.
func (s *Service) Next(ctx context.Context) error { return s.player.Next(ctx) }

// Previous returns to the previous track.
func (s *Service) Previous(ctx context.Context) error { return s.player.Previous(ctx) }

// SetVolume sets an absolute volume, clamped to the valid range, and returns
// the level that was applied.
func (s *Service) SetVolume(ctx context.Context, level int) (music.Volume, error) {
	v := music.NewVolume(level)
	if err := s.player.SetVolume(ctx, v); err != nil {
		return 0, fmt.Errorf("set volume: %w", err)
	}
	return v, nil
}

// SetShuffle enables or disables shuffle.
func (s *Service) SetShuffle(ctx context.Context, enabled bool) error {
	return s.player.SetShuffle(ctx, enabled)
}

// ToggleShuffle flips shuffle relative to its current state and returns the new
// value. The current state is read first; if that read fails, no change is made.
func (s *Service) ToggleShuffle(ctx context.Context) (bool, error) {
	status, err := s.player.Status(ctx)
	if err != nil {
		return false, fmt.Errorf("read shuffle: %w", err)
	}

	enabled := !status.Shuffle
	if err := s.player.SetShuffle(ctx, enabled); err != nil {
		return false, fmt.Errorf("set shuffle: %w", err)
	}
	return enabled, nil
}

// SetRepeat sets the repeat mode.
func (s *Service) SetRepeat(ctx context.Context, mode music.RepeatMode) error {
	return s.player.SetRepeat(ctx, mode)
}

// AdjustVolume shifts the current volume by delta, clamped to the valid range,
// and returns the new level. The current volume is read first; if that read
// fails, no change is applied.
func (s *Service) AdjustVolume(ctx context.Context, delta int) (music.Volume, error) {
	status, err := s.player.Status(ctx)
	if err != nil {
		return 0, fmt.Errorf("read volume: %w", err)
	}

	v := status.Volume.Adjust(delta)
	if err := s.player.SetVolume(ctx, v); err != nil {
		return 0, fmt.Errorf("set volume: %w", err)
	}
	return v, nil
}

// Seek moves the player position and returns the new position. Relative and
// percentage seeks read the current status first to resolve the target; the
// result is clamped to the start of the track (and to its end when the
// duration is known).
func (s *Service) Seek(ctx context.Context, mode music.SeekMode, value float64) (time.Duration, error) {
	var (
		pos      float64
		duration float64
	)

	switch mode {
	case music.SeekAbsolute:
		pos = value
	case music.SeekRelative, music.SeekPercent:
		status, err := s.player.Status(ctx)
		if err != nil {
			return 0, fmt.Errorf("read position: %w", err)
		}
		duration = status.Track.Duration.Seconds()
		if mode == music.SeekRelative {
			pos = status.Elapsed.Seconds() + value
		} else {
			pos = duration * value / 100
		}
	}

	if pos < 0 {
		pos = 0
	}
	if duration > 0 && pos > duration {
		pos = duration
	}

	if err := s.player.SetPosition(ctx, pos); err != nil {
		return 0, fmt.Errorf("seek: %w", err)
	}
	return time.Duration(pos * float64(time.Second)), nil
}

// Mute remembers the current volume and sets it to zero. If the volume is
// already zero it does nothing, so the remembered level survives a double mute.
func (s *Service) Mute(ctx context.Context) error {
	status, err := s.player.Status(ctx)
	if err != nil {
		return fmt.Errorf("read volume: %w", err)
	}
	if status.Volume.IsMuted() {
		return nil
	}

	if err := s.volume.Save(status.Volume.Int()); err != nil {
		return fmt.Errorf("remember volume: %w", err)
	}
	if err := s.player.SetVolume(ctx, music.NewVolume(0)); err != nil {
		return fmt.Errorf("mute: %w", err)
	}
	return nil
}

// Unmute restores the remembered volume, falling back to DefaultUnmuteVolume
// when no prior level is known, and returns the level applied.
func (s *Service) Unmute(ctx context.Context) (music.Volume, error) {
	level, ok, err := s.volume.Load()
	if err != nil {
		return 0, fmt.Errorf("recall volume: %w", err)
	}
	if !ok || level <= 0 {
		level = DefaultUnmuteVolume
	}

	v := music.NewVolume(level)
	if err := s.player.SetVolume(ctx, v); err != nil {
		return 0, fmt.Errorf("unmute: %w", err)
	}
	return v, nil
}
