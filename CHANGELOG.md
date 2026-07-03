# Changelog

All notable changes to this project are documented here. The format is based on
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and this project
adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [0.2.0] - 2026-07-03

### Added

- **`amd` daemon:** polls Music.app, caches the latest status, and serves it
  plus change events to clients over a Unix socket (newline-delimited JSON:
  `status`, `ping`, `subscribe`). `subscribe` streams a message on every change.
- `amp status` and `amp now` are served from the daemon's cache when it is
  running (~instant vs ~0.5s direct), falling back to direct AppleScript
  automatically when it is not. Other commands are unaffected.

## [0.1.0] - 2026-07-02

First release: the project is rebuilt from an iTunes shell wrapper into **amp**,
a Go CLI for Apple Music, with a hexagonal architecture (domain / ports /
application / adapters) and a wide unit-test base over a single osascript seam.

### Added

- **Playback:** `play [query]` (resume, or smart-play a playlist, album, or
  track search), `pause`, `toggle`, `stop`, `next`, `prev`, `seek`
  (`<seconds|mm:ss|+n|-n|n%>`).
- **Now playing:** `status` (labeled layout, progress bar, percentage,
  TTY-gated color, `--json`), `now` (one line).
- **Volume:** `vol <n|+n|-n|up|down>`, `mute`, `unmute` (remembers the prior
  level).
- **Modes:** `shuffle [on|off|toggle]`, `repeat <off|one|all>`.
- **Library:** `search <query>` (interactive picker on a terminal; picking a
  track plays it and queues the rest), `queue` / `queue add` / `queue clear`,
  `playlists`, `library artists`, `library albums`.
- **App:** `open` (launch Music), `--version`.
- Live shell completion of playlist/album names for `play`.
- Shell completion scripts via `amp completion <shell>`.

### Notes

- Apple Music is driven entirely through AppleScript (`osascript`); no private
  frameworks. The play queue is emulated with a managed `amp queue` playlist.
- Removed the legacy iTunes-era shell scripts (superseded by the Go CLI; still
  available in git history).

[0.2.0]: https://github.com/helmedeiros/amp/releases/tag/v0.2.0
[0.1.0]: https://github.com/helmedeiros/amp/releases/tag/v0.1.0
