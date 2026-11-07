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
