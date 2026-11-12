# Changelog

All notable changes to this project are documented here. The format is based on
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and this project
adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [0.10.0] - 2026-11-12

### Added

- **TUI themes: `flat` and `tape`.** `flat` is the look so far. `tape` draws
  the player as a cassette deck while playing: reels that trade tape as the
  track runs and turn only while playing, a tape counter, volume LEDs, and the
  current state's transport button pressed. While paused, the deck shrinks to
  a cassette spine and the Queue becomes a J-card, with side A and side B split
  where the next track would run past half the tape (45 minutes on a C90).
  Below 64 columns it falls back to the flat header.
- **TUI: `T` cycles themes, and amp remembers the last one.** `amp tui --theme
  <name>` picks one at start.
- **Theme files.** Drop a TOML file in `~/Library/Application
  Support/amp/themes/` to make your own: pick a layout and override colors,
  glyphs, and the cassette's brand, type and length. Unknown keys, bad colors
  and glyphs wider than one column are rejected; a broken file is reported in
  the TUI and skipped. See the README and ADR-0006.

## [0.9.1] - 2026-11-09

### Fixed

- **TUI: the player header stays on screen.** The list height assumed a fixed
  amount of chrome, so a playing track's header (and, on the Queue, the
  "Following Music.app" title) pushed the frame past the terminal and scrolled
  the header off the top. Lists now fit whatever is drawn around them.

## [0.9.0] - 2026-11-07

### Added

- **TUI: the Queue follows Music.app.** When playback starts outside amp (in
  Music.app, with Siri, a remote, or media keys), the Queue switches to what is
  playing, marks the current track, and names the source. A playlist is shown
  as-is; a song started from the library view, or Apple Music streaming, shows
  the playing album's tracks that are in your library. Music's "Up Next" can't
  be read, so it is never shown.
- **TUI: take over playback started elsewhere.** On a followed Queue, `Enter`
  copies the list into amp's queue and plays from the highlighted row; `t` does
  the same from the playing track and keeps its position.
- **TUI: `n` / `p` skip to the next / previous track.**
- **`amp status` shows where a track plays from.** A `from` row (amp queue,
  playlist, library, or Apple Music), and a `source` object in `--json`. `amd`
  forwards it; restart a running `amd` to pick it up.

## [0.8.0] - 2026-10-29

### Changed

- **TUI: far lighter library refresh.** 0.7.0 reloaded the visible tab every
  15 seconds, which ran the full playlists script (~3.6s with ~100 playlists)
  each time. The TUI now checks a cheap library fingerprint (playlist names plus
  the library track total, ~0.1s) every 5 seconds and reloads the tab only when
  it changed. A change seen while you filter or a play is in flight stays
  pending instead of being lost.

### Added

- **`amd` serves the library fingerprint.** The daemon polls it every 5 seconds
  and answers a new `library` command from its cache, so any number of TUIs
  share one poll. Without `amd` running, amp reads it directly as before.

## [0.7.0] - 2026-10-29

### Added

- **TUI: library tabs stay fresh.** Playlists, Artists, and Albums reload every
  15 seconds while you stay on them (paused while you filter, pick albums, or a
  play is in flight), so a playlist created in Music.app appears without
  pressing `r`.

### Fixed

- **TUI: new playlists didn't appear until `r`.** Tabs were fetched once and
  cached; switching to a tab now shows the cached list and refetches it.
- **TUI: reloads no longer move your selection.** The cursor follows the
  selected row, and an active filter is kept, when a tab reloads.

## [0.6.1] - 2026-10-14

### Fixed

- Dropped a dangling `itunesScript` symlink left behind when the iTunes-era
  shell scripts were removed in 0.1.0.

### Changed

- CI installs golangci-lint v2.13.2 from the module proxy, matching a current
  local toolchain, and the test fakes are formatted to that gofumpt.

## [0.6.0] - 2026-07-14

### Added

- **Apple Music connection (`amp auth apple-music`).** Connect amp to your Apple
  Music account (stores a `media-user-token`) so it can reach the catalog;
  `amp auth apple-music status` verifies the token still works. The TUI header
  shows `Apple Music ✓` when connected.
- **Complete partly-owned albums automatically.** Playing an album that
  streaming only partly added to your library first pulls the missing tracks
  from the Apple Music catalog, waits for iCloud to sync, then plays the full
  album. Without a connection it plays what's there and warns.
- **Add an artist's missing albums.** On the Artists or Albums tab, press `a` to
  fetch an artist's Apple Music discography (singles excluded, editions
  collapsed) and pick which not-yet-in-library albums to add.
- **Import SoundCloud uploads (`amp soundcloud import <url>`).** Download your
  own SoundCloud tracks (via yt-dlp + ffmpeg), tag them (a `Band - Song` title
  is credited to the band; others to `--solo-artist`), and add them to your
  library and a playlist. Idempotent — re-run to pick up new uploads.
- **TUI: follow the playing track.** The Queue marks the current track and moves
  to it as playback advances. Album plays show a loading bar and a self-clearing
  confirmation. The footer documents `r reload` and `a add albums`.

### Changed

- `play <album>` now reports when an album is only partly in your library
  (`only N of M tracks…`).

## [0.5.0] - 2026-07-05

### Added

- **TUI: filter within a tab.** On the Queue, Playlists, Artists, and Albums
  tabs, press `/` to filter the loaded list live (case-insensitive substring).
  `Enter` keeps the narrowed list and returns to navigation (`j`/`k` move,
  `Enter` plays — the highlighted row maps back to its real position); `Esc`
  clears the filter. The Search tab keeps its library-wide search.
- **Filter by artist across tabs.** The filter matches an artist-aware key, not
  just the visible text: on **Albums** the artist is shown (`Artist — Album`, or
  `Various Artists` for mixed albums) and matched; on **Playlists**, typing an
  artist surfaces every playlist that contains a track by them, even when the
  playlist name doesn't mention it.

### Changed

- `library albums` now prints `Artist — Album` (and `--json` emits
  `{"name","artist"}` objects) so same-named or mixed albums are
  distinguishable.

## [0.4.0] - 2026-07-05

### Changed

- Every play now sets the queue to the selected context. Playing a **playlist**,
  **album**, **artist**, or **search** result loads those tracks into the
  `amp queue` (a fast bulk copy — ~0.65s for 1000+ tracks) and plays from the
  top, so the queue always reflects what's playing. In the TUI, any play jumps
  to the **Queue** tab and refreshes it, making the queue the single window into
  the current context.

## [0.3.1] - 2026-07-05

### Fixed

- TUI: the Search tab no longer traps number keys. Switching to it lands in
  navigation mode (so `1`–`5` and `Tab` still switch tabs); press `/` to start
  typing a query and `Esc` to stop.

## [0.3.0] - 2026-07-05

### Added

- **`amp tui`** — a full-screen terminal UI (Bubble Tea): a live now-playing
  header driven by the daemon's event stream, and tabs for **Queue, Playlists,
  Artists, Albums, and Search**. `j`/`k` move, `Enter` plays the highlighted
  item (a queue track from that position, or a playlist/album/artist/search
  result), `space` toggles play/pause, `/` searches, `q` quits.

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

[0.10.0]: https://github.com/helmedeiros/amp/releases/tag/v0.10.0
[0.9.1]: https://github.com/helmedeiros/amp/releases/tag/v0.9.1
[0.9.0]: https://github.com/helmedeiros/amp/releases/tag/v0.9.0
[0.8.0]: https://github.com/helmedeiros/amp/releases/tag/v0.8.0
[0.7.0]: https://github.com/helmedeiros/amp/releases/tag/v0.7.0
[0.6.1]: https://github.com/helmedeiros/amp/releases/tag/v0.6.1
[0.6.0]: https://github.com/helmedeiros/amp/releases/tag/v0.6.0
[0.5.0]: https://github.com/helmedeiros/amp/releases/tag/v0.5.0
[0.4.0]: https://github.com/helmedeiros/amp/releases/tag/v0.4.0
[0.3.1]: https://github.com/helmedeiros/amp/releases/tag/v0.3.1
[0.3.0]: https://github.com/helmedeiros/amp/releases/tag/v0.3.0
[0.2.0]: https://github.com/helmedeiros/amp/releases/tag/v0.2.0
[0.1.0]: https://github.com/helmedeiros/amp/releases/tag/v0.1.0
