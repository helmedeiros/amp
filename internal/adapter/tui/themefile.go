package tui

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/charmbracelet/lipgloss"

	"github.com/helmedeiros/amp/internal/music"
)

// Options picks the starting theme and where themes and the last choice live.
type Options struct {
	Theme     string
	ConfigDir string
}

type themeSet struct {
	themes   []theme
	current  int
	warnings []string
}

type themeFile struct {
	Name   string `toml:"name"`
	Layout string `toml:"layout"`
	Colors struct {
		Playing  string `toml:"playing"`
		Paused   string `toml:"paused"`
		Stopped  string `toml:"stopped"`
		Track    string `toml:"track"`
		Dim      string `toml:"dim"`
		Accent   string `toml:"accent"`
		Selected string `toml:"selected"`
		Current  string `toml:"current"`
		Notice   string `toml:"notice"`
	} `toml:"colors"`
	Glyphs struct {
		Cursor  *string `toml:"cursor"`
		Marker  *string `toml:"marker"`
		Fill    *string `toml:"fill"`
		Empty   *string `toml:"empty"`
		LoadOn  *string `toml:"load_on"`
		LoadOff *string `toml:"load_off"`
	} `toml:"glyphs"`
	Tape struct {
		Brand   *string `toml:"brand"`
		Kind    *string `toml:"type"`
		Minutes *int    `toml:"minutes"`
	} `toml:"tape"`
}

var (
	layouts   = map[string]func(*lipgloss.Renderer) theme{"flat": flatTheme, "tape": tapeTheme}
	themeName = regexp.MustCompile(`^[A-Za-z0-9_-]+$`)
	hexColor  = regexp.MustCompile(`^#([0-9A-Fa-f]{3}|[0-9A-Fa-f]{6})$`)
)

func builtinThemes(r *lipgloss.Renderer) []theme {
	return []theme{flatTheme(r), tapeTheme(r)}
}

func layoutNames() []string {
	out := make([]string, 0, len(layouts))
	for name := range layouts {
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}

func parseTheme(r *lipgloss.Renderer, data []byte) (theme, error) {
	var f themeFile
	meta, err := toml.Decode(string(data), &f)
	if err != nil {
		return theme{}, err
	}
	if extra := meta.Undecoded(); len(extra) > 0 {
		return theme{}, fmt.Errorf("unknown key %s", extra[0])
	}
	if f.Name == "" {
		return theme{}, errors.New("name is required")
	}
	if !themeName.MatchString(f.Name) {
		return theme{}, fmt.Errorf("name %q may only use letters, digits, - and _", f.Name)
	}
	if f.Layout == "" {
		f.Layout = "flat"
	}
	base, ok := layouts[f.Layout]
	if !ok {
		return theme{}, fmt.Errorf("unknown layout %q (%s)", f.Layout, strings.Join(layoutNames(), ", "))
	}
	t := base(r)
	t.name = f.Name
	t.states = map[music.PlayerState]lipgloss.Style{
		music.Playing: t.states[music.Playing],
		music.Paused:  t.states[music.Paused],
		music.Stopped: t.states[music.Stopped],
	}

	colors := []struct {
		key   string
		value string
		apply func(lipgloss.Color)
	}{
		{"playing", f.Colors.Playing, func(c lipgloss.Color) { t.states[music.Playing] = t.states[music.Playing].Foreground(c) }},
		{"paused", f.Colors.Paused, func(c lipgloss.Color) { t.states[music.Paused] = t.states[music.Paused].Foreground(c) }},
		{"stopped", f.Colors.Stopped, func(c lipgloss.Color) { t.states[music.Stopped] = t.states[music.Stopped].Foreground(c) }},
		{"track", f.Colors.Track, func(c lipgloss.Color) { t.track = t.track.Foreground(c) }},
		{"dim", f.Colors.Dim, func(c lipgloss.Color) { t.dim = t.dim.Foreground(c) }},
		{"accent", f.Colors.Accent, func(c lipgloss.Color) { t.accent = t.accent.Foreground(c) }},
		{"selected", f.Colors.Selected, func(c lipgloss.Color) { t.selected = t.selected.Foreground(c) }},
		{"current", f.Colors.Current, func(c lipgloss.Color) { t.current = t.current.Foreground(c) }},
		{"notice", f.Colors.Notice, func(c lipgloss.Color) { t.notice = t.notice.Foreground(c) }},
	}
	for _, c := range colors {
		if c.value == "" {
			continue
		}
		if !validColor(c.value) {
			return theme{}, fmt.Errorf("colors.%s: invalid color %q", c.key, c.value)
		}
		c.apply(lipgloss.Color(c.value))
	}

	glyphs := []struct {
		key   string
		value *string
		dst   *string
	}{
		{"cursor", f.Glyphs.Cursor, &t.glyphs.cursor},
		{"marker", f.Glyphs.Marker, &t.glyphs.marker},
		{"fill", f.Glyphs.Fill, &t.glyphs.fill},
		{"empty", f.Glyphs.Empty, &t.glyphs.empty},
		{"load_on", f.Glyphs.LoadOn, &t.glyphs.loadOn},
		{"load_off", f.Glyphs.LoadOff, &t.glyphs.loadOff},
	}
	for _, g := range glyphs {
		if g.value == nil {
			continue
		}
		if lipgloss.Width(*g.value) != 1 {
			return theme{}, fmt.Errorf("glyphs.%s: %q must be one column wide", g.key, *g.value)
		}
		*g.dst = *g.value
	}
	if err := applyTape(&t, f, f.Layout); err != nil {
		return theme{}, err
	}
	return t, nil
}

func applyTape(t *theme, f themeFile, layout string) error {
	tp := f.Tape
	if tp.Brand == nil && tp.Kind == nil && tp.Minutes == nil {
		return nil
	}
	if layout != "tape" {
		return errors.New(`[tape] needs layout = "tape"`)
	}
	labels := []struct {
		key   string
		value *string
		limit int
		dst   *string
	}{
		{"brand", tp.Brand, 20, &t.tape.brand},
		{"type", tp.Kind, 30, &t.tape.kind},
	}
	for _, l := range labels {
		if l.value == nil {
			continue
		}
		if lipgloss.Width(*l.value) > l.limit {
			return fmt.Errorf("tape.%s: longer than %d columns", l.key, l.limit)
		}
		*l.dst = *l.value
	}
	if tp.Minutes != nil {
		if *tp.Minutes < 10 || *tp.Minutes > 180 {
			return fmt.Errorf("tape.minutes: %d is not between 10 and 180", *tp.Minutes)
		}
		t.tape.minutes = *tp.Minutes
	}
	return nil
}

func validColor(s string) bool {
	if hexColor.MatchString(s) {
		return true
	}
	n, err := strconv.Atoi(s)
	return err == nil && n >= 0 && n <= 255
}

func loadThemes(r *lipgloss.Renderer, dir string) ([]theme, []string) {
	themes := builtinThemes(r)
	paths, _ := filepath.Glob(filepath.Join(dir, "themes", "*.toml"))
	sort.Strings(paths)
	var user []theme
	var warnings []string
	for _, p := range paths {
		data, err := os.ReadFile(p)
		if err == nil {
			var t theme
			if t, err = parseTheme(r, data); err == nil {
				user = append(user, t)
				continue
			}
		}
		warnings = append(warnings, fmt.Sprintf("theme %s: %v", filepath.Base(p), err))
	}
	sort.SliceStable(user, func(i, j int) bool { return user[i].name < user[j].name })
	for _, u := range user {
		if i := indexOfTheme(themes, u.name); i >= 0 {
			themes[i] = u
			continue
		}
		themes = append(themes, u)
	}
	return themes, warnings
}

func indexOfTheme(themes []theme, name string) int {
	for i, t := range themes {
		if t.name == name {
			return i
		}
	}
	return -1
}

func resolveThemes(r *lipgloss.Renderer, opts Options) (themeSet, error) {
	themes, warnings := loadThemes(r, opts.ConfigDir)
	set := themeSet{themes: themes, warnings: warnings}
	if opts.Theme != "" {
		i := indexOfTheme(themes, opts.Theme)
		if i < 0 {
			return themeSet{}, fmt.Errorf("unknown theme %q (available: %s)", opts.Theme, strings.Join(names(themes), ", "))
		}
		set.current = i
		return set, nil
	}
	if i := indexOfTheme(themes, loadThemeName(opts.ConfigDir)); i >= 0 {
		set.current = i
	}
	return set, nil
}

func names(themes []theme) []string {
	out := make([]string, len(themes))
	for i, t := range themes {
		out[i] = t.name
	}
	return out
}

func themeNamePath(dir string) string { return filepath.Join(dir, "theme") }

func loadThemeName(dir string) string {
	data, err := os.ReadFile(themeNamePath(dir))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

func saveThemeName(dir, name string) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	return os.WriteFile(themeNamePath(dir), []byte(name+"\n"), 0o600)
}

// DefaultConfigDir is where amp keeps TUI themes and the last theme used.
func DefaultConfigDir() string {
	dir, err := os.UserConfigDir()
	if err != nil {
		return ".amp"
	}
	return filepath.Join(dir, "amp")
}
