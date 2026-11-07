package applescript

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/helmedeiros/amp/internal/music"
)

const currentPlaylistTracksScript = `
const Music = Application('Music');
const out = [];
try {
  const ts = Music.currentPlaylist.tracks;
  const names = ts.name(), artists = ts.artist(), albums = ts.album(), durs = ts.duration();
  for (let i = 0; i < names.length; i++) {
    out.push({name: names[i], artist: artists[i], album: albums[i], duration: durs[i]});
  }
} catch (e) {}
JSON.stringify(out);
`

func albumTracksScript(album string) string {
	a, _ := json.Marshal(album)
	return fmt.Sprintf(`
const Music = Application('Music');
const out = [];
try {
  const ts = Music.libraryPlaylists[0].tracks.whose({album: %s});
  const names = ts.name(), artists = ts.artist(), albums = ts.album(), durs = ts.duration();
  const discs = ts.discNumber(), nums = ts.trackNumber();
  for (let i = 0; i < names.length; i++) {
    out.push({name: names[i], artist: artists[i], album: albums[i], duration: durs[i], disc: discs[i], num: nums[i]});
  }
  out.sort((x, y) => (x.disc - y.disc) || (x.num - y.num));
} catch (e) {}
JSON.stringify(out);
`, a)
}

// CurrentPlaylistTracks returns the tracks of the playlist Music.app is playing from.
func (p *Player) CurrentPlaylistTracks(ctx context.Context) ([]music.Track, error) {
	out, err := p.run.Run(ctx, javaScript, currentPlaylistTracksScript)
	if err != nil {
		return nil, err
	}
	return parseTracks(out)
}

// AlbumTracks returns the library tracks of the named album in disc and track order.
func (p *Player) AlbumTracks(ctx context.Context, album string) ([]music.Track, error) {
	out, err := p.run.Run(ctx, javaScript, albumTracksScript(album))
	if err != nil {
		return nil, err
	}
	return parseTracks(out)
}

func takeOverScript(src music.Source, album string, start int, position float64) string {
	kind, _ := json.Marshal(src.Kind.String())
	name, _ := json.Marshal(src.Name)
	al, _ := json.Marshal(album)
	qn, _ := json.Marshal(queuePlaylistName)
	return fmt.Sprintf(`
const Music = Application('Music');
const kind = %s, name = %s, album = %s, start = %d, pos = %f, qn = %s;
const out = {queued: 0};
let src = [];
try {
  if (kind === 'playlist' && name !== qn) {
    let p = null;
    try { const c = Music.currentPlaylist; if (c.name() === name) p = c; } catch (e) {}
    if (p === null) p = Music.userPlaylists.byName(name);
    src = p.tracks();
  } else if (kind !== 'playlist' && album !== '') {
    src = Music.libraryPlaylists[0].tracks.whose({album: album})();
    const key = t => { try { return t.discNumber() * 1000 + t.trackNumber(); } catch (e) { return 0; } };
    src.sort((a, b) => key(a) - key(b));
  }
} catch (e) {}
const n = src.length;
if (n > 0) {
  const s = ((start %% n) + n) %% n;
  const rotated = src.slice(s).concat(src.slice(0, s));
  let pl;
  try { pl = Music.userPlaylists.byName(qn); pl.name(); Music.delete(pl.tracks); }
  catch (e) { pl = Music.make({new: 'playlist', withProperties: {name: qn}}); }
  for (const t of rotated) { try { Music.duplicate(t, {to: pl}); out.queued++; } catch (e) {} }
  if (out.queued > 0) {
    pl.play();
    if (pos > 0) { try { Music.playerPosition = pos; } catch (e) {} }
  }
}
JSON.stringify(out);
`, kind, name, al, start, position, qn)
}

// TakeOver copies the playing context into amp's queue and plays it from start.
func (p *Player) TakeOver(ctx context.Context, src music.Source, album string, start int, position float64) (int, error) {
	out, err := p.run.Run(ctx, javaScript, takeOverScript(src, album, start, position))
	if err != nil {
		return 0, err
	}
	var res struct {
		Queued int `json:"queued"`
	}
	if err := json.Unmarshal(out, &res); err != nil {
		return 0, fmt.Errorf("decode take over: %w", err)
	}
	return res.Queued, nil
}
