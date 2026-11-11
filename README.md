# amp

A terminal player for **Apple Music** on macOS: control playback and read what's
playing straight from the console — handy for hotkeys, remote shells, or
scripting your music without leaving the keyboard.

`amp` treats **Music.app as the engine** and drives it through AppleScript, so
there's no playback daemon to run and nothing to configure. The CLI binary is
`amp`.

## Install

Requires Go 1.26+ and macOS with Music.app.

```sh
go install github.com/helmedeiros/amp/cmd/amp@latest
```

Or build from a clone:

```sh
git clone https://github.com/helmedeiros/amp.git
cd amp
make build      # produces ./bin/amp
```

## Usage

```sh
amp status            # show what's playing (add --json for scripts)
amp now               # the current track on one line
amp artwork -o cover.jpg   # save the current album art to a file

amp open              # launch Apple Music
amp play              # resume playback
amp play "Discovery"  # play a playlist, album, or search match (Tab-completes names)
amp pause
amp toggle            # play/pause
amp stop
amp next
amp prev

amp vol 60            # set volume to 60%
amp vol +10           # raise by 10
amp vol -10           # lower by 10
amp vol up | down     # ±10
amp mute              # silence, remembering the level
amp unmute            # restore it

amp shuffle           # toggle shuffle
amp shuffle on | off
amp repeat off | one | all

amp search daft punk        # search the library (--limit N, --json)
amp queue                   # show the play queue (--json)
amp queue add daft punk     # append search results without playing (--limit N)
amp queue clear             # empty the queue
amp playlists               # list your playlists (--json)
amp library artists         # distinct artists (--json)
amp library albums          # distinct albums (--json)

amp --version
```

Run `amp --help` for the full command list.

`amp status` colorizes its output when writing to a terminal (the state word is
colored, labels are dimmed). Color is disabled automatically when the output is
piped or redirected, and can be turned off explicitly with `--no-color` or the
`NO_COLOR` environment variable. To keep color under `watch`, use
`watch --color -n1 amp status`.

## Daemon (optional)

`amd` is a background daemon that polls Music.app and, over a Unix socket,
serves the cached status and pushes change events. Run it and `amp status` /
`amp now` are served from the cache almost instantly instead of querying
Music.app each time — handy under `watch`. When the daemon isn't running, amp
falls back to direct access automatically; nothing else changes.

```sh
amd &          # start the daemon (Ctrl-C or kill to stop)
amp status     # now served from the cache (~instant)
```

## TUI

`amp tui` opens a full-screen live interface:

- A now-playing header — state, track, progress bar, volume — that updates live
  from the daemon's event stream (or polls when `amd` isn't running).
- Tabs: **Queue, Playlists, Artists, Albums, Search** — switch with `Tab` /
  `Shift-Tab` or number keys `1`–`5`.
- `j`/`k` move, `Enter` plays the highlighted item, `space` toggles play/pause,
  `n`/`p` skip to the next/previous track, `/` types a search query, `r`
  refreshes, `T` switches theme, `q` quits.
- The Queue follows Music.app. When playback starts outside amp (in Music.app,
  with Siri, a remote, or media keys), the Queue switches to what is playing,
  marks the current track, and says where it comes from. A playlist is shown
  as-is. A song started from the library view, or Apple Music streaming, shows
  the playing album's tracks that are in your library, because Music exposes
  no list for those. Music's "Up Next" can't be read, so it is never shown.
- Take over a followed Queue to control it track by track: `Enter` copies the
  list into amp's queue and plays from the highlighted row; `t` does the same
  from the playing track, keeping its position so the music carries on.
- Library tabs reload when you switch to them, and again whenever the library
  changes while you stay on Playlists, Artists, or Albums (checked every 5
  seconds with a cheap fingerprint, shared through `amd` when it runs; paused
  while you filter), so a playlist created in Music.app appears on its own. The
  selection stays put.

```sh
amd &        # optional: event-driven updates
amp tui
```

### Themes

Two themes ship: `flat`, the default, and `tape`, which draws the player as a
cassette deck while playing (reels that trade tape as the track runs, a counter,
the pressed transport button) and, while paused, lays the Queue out as a J-card
with side A and side B split at half the tape. `T` cycles themes and amp
remembers the last one; `amp tui --theme tape` picks one at start.

Make your own by dropping a TOML file in `~/Library/Application
Support/amp/themes/`. It picks a layout and overrides what it wants; the rest
comes from that layout's theme:

```toml
name = "sony-ef90"
layout = "tape"            # flat or tape

[colors]                   # ANSI 0–255 or #hex
playing = "#7FD39B"
accent = "#2B6CB0"         # label stripe and pressed button
selected = "#F2EEE3"

[glyphs]                   # one column each
cursor = "›"
marker = "♫"
fill = "▓"
empty = "░"

[tape]
brand = "SONY"
type = "EF90 TYPE I (NORMAL)"
minutes = 90               # side B starts past half
```

Color keys: `playing`, `paused`, `stopped`, `track`, `dim`, `accent`,
`selected`, `current`, `notice`. Glyph keys: `cursor`, `marker`, `fill`,
`empty`, `load_on`, `load_off`. A broken file is reported in the TUI and
skipped. See [ADR-0006](docs/adr/0006-tui-themes-layout-and-skin.md).

## Shell completion

`amp` generates completion scripts for bash, zsh, fish, and PowerShell. The zsh
and bash scripts complete commands, flags, and arguments dynamically.

```sh
# zsh — place on your fpath (a directory compinit reads), then restart your shell
amp completion zsh > "${fpath[1]}/_amp"

# bash
amp completion bash | sudo tee /etc/bash_completion.d/amp >/dev/null

# fish
amp completion fish > ~/.config/fish/completions/amp.fish
```

See `amp completion --help` for per-shell details.

## Architecture

`amp` is built with a hexagonal (ports & adapters) architecture in Go:

- **`internal/music`** — pure domain: player state, volume, repeat mode, track,
  status.
- **`internal/port`** — the ports: `Player` and `VolumeStore` (driven),
  `Controller` (driving).
- **`internal/app`** — the application service (use cases).
- **`internal/adapter`** — the edges: `applescript` (the Music.app engine via
  `osascript`), `store` (file-backed volume state), and `cli` (the command
  tree).
- **`cmd/amp`** — wiring.

Architectural decisions are recorded in [`docs/adr/`](docs/adr/). The design
keeps `osascript` behind a single seam, so the logic is covered by a wide base
of fast unit tests with a thin layer of integration tests against the real
binary (`make integration`, requires Music.app).

## Development

```sh
make ci            # format check, vet, lint, race-enabled tests
make test          # tests only
make integration   # osascript integration tests (needs Music.app)
make build         # build ./bin/amp
```

## License

[MIT](LICENSE)
