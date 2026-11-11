# ADR-0006: TUI themes as a built-in layout plus a skin file

- Status: accepted
- Date: 2026-11-11

## Context

The TUI had one look, set by package-level lipgloss styles. We want a second
one, `tape`, that draws the player as a cassette deck while playing and the
Queue as a J-card while paused, and we want people to be able to make and share
their own variations.

The tape look changes structure, not only colors: the header becomes a boxed
deck with reels and a transport row, and the Queue becomes two columns. A file
format that only sets colors cannot express that.

## Decision

A theme has two parts:

- **Layout** (Go code): draws the header and the list. Two ship today, `flat`
  (the original screen) and `tape`. A layout falls back to `flat` when the
  terminal is too narrow for it (64 columns for `tape`).
- **Skin** (data): colors, single-column glyphs, and for `tape` the brand,
  type and length printed on the cassette.

Built-in themes are `flat` and `tape`. A theme file is TOML in
`<config>/amp/themes/*.toml` (`~/Library/Application Support/amp/themes` on
macOS). It names a layout and overrides parts of its skin; anything left out
comes from the layout's built-in theme. A file whose name matches a built-in
replaces it.

Files are validated strictly: unknown keys, invalid colors, glyphs wider than
one column, and `[tape]` on a non-tape layout are errors. A broken file is
reported in the TUI's notice line and skipped; it never stops the TUI.

`amp tui --theme <name>` picks the theme (an unknown name is an error that
lists the available ones), `T` cycles through them, and the last one used is
stored in `<config>/amp/theme`.

## Consequences

- Sharing a theme is sharing one small text file; no code runs from it.
- New structural looks still need Go code and a release. Accepted: layouts are
  few and worth testing; skins are many and safe.
- The tape J-card depends on track durations, so the Queue now keeps its
  tracks alongside the display lines.
- Rendering is pinned by golden files, plain and ANSI, so a style change shows
  up as a reviewed diff.

## Alternatives considered

- **Colors-only themes.** Simple, but cannot draw a cassette.
- **A template language for layouts.** Fully user-defined screens, at the cost
  of a templating surface, layout logic in data that cannot be tested here, and
  far easier breakage on narrow terminals. Rejected for now.
- **JSON instead of TOML.** No new dependency, but worse to edit by hand and
  no comments. TOML costs one small, well-known module.
