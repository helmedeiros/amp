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
