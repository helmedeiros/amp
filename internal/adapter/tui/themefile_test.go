package tui

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/helmedeiros/amp/internal/music"
)

func trueColorRenderer() *lipgloss.Renderer {
	r := lipgloss.NewRenderer(io.Discard)
	r.SetColorProfile(termenv.TrueColor)
	return r
}

func TestParseThemeLayersTheFileOverItsLayout(t *testing.T) {
	t.Parallel()

	th, err := parseTheme(trueColorRenderer(), []byte(`
name = "mint"
layout = "flat"

[colors]
playing = "#7FD39B"
accent = "5"

[glyphs]
cursor = ">"
load_on = "#"
`))
	require.NoError(t, err)

	assert.Equal(t, "mint", th.name)
	assert.IsType(t, flatLayout{}, th.layout)
	assert.Equal(t, glyphs{cursor: ">", marker: "♪", fill: "━", empty: "─", loadOn: "#", loadOff: "░"}, th.glyphs)
	assert.Equal(t, "\x1b[1;38;2;127;211;155mPLAYING\x1b[0m", th.state(music.Playing).Render("PLAYING"), "color replaced, bold kept")
	assert.Equal(t, "\x1b[1;35mQueue\x1b[0m", th.accent.Render("Queue"))
	assert.Equal(t, flatTheme(trueColorRenderer()).state(music.Paused).Render("x"), th.state(music.Paused).Render("x"), "unset colors come from the layout")
}

func TestParseThemeDefaultsToTheFlatLayout(t *testing.T) {
	t.Parallel()

	th, err := parseTheme(trueColorRenderer(), []byte(`name = "plain"`))
	require.NoError(t, err)
	assert.IsType(t, flatLayout{}, th.layout)
}

func TestParseThemeRejectsBadFiles(t *testing.T) {
	t.Parallel()

	cases := map[string]struct{ file, want string }{
		"syntax":        {file: `name = `, want: "toml"},
		"no name":       {file: `layout = "flat"`, want: "name is required"},
		"bad layout":    {file: "name = \"x\"\nlayout = \"vinyl\"", want: `unknown layout "vinyl" (flat)`},
		"typo key":      {file: "name = \"x\"\n[colors]\nplayng = \"2\"", want: "unknown key colors.playng"},
		"bad color":     {file: "name = \"x\"\n[colors]\ndim = \"teal\"", want: `colors.dim: invalid color "teal"`},
		"out of range":  {file: "name = \"x\"\n[colors]\ndim = \"256\"", want: `colors.dim: invalid color "256"`},
		"wide glyph":    {file: "name = \"x\"\n[glyphs]\ncursor = \"=>\"", want: `glyphs.cursor: "=>" must be one column wide`},
		"empty glyph":   {file: "name = \"x\"\n[glyphs]\nfill = \"\"", want: `glyphs.fill: "" must be one column wide`},
		"name with dir": {file: `name = "../x"`, want: `name "../x" may only use letters, digits, - and _`},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			_, err := parseTheme(trueColorRenderer(), []byte(c.file))
			require.Error(t, err)
			assert.Contains(t, err.Error(), c.want)
		})
	}
}

func writeTheme(t *testing.T, dir, file, body string) {
	t.Helper()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "themes"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "themes", file), []byte(body), 0o600))
}

func TestLoadThemesAddsUserFilesAfterTheBuiltins(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeTheme(t, dir, "zeta.toml", `name = "zeta"`)
	writeTheme(t, dir, "alpha.toml", `name = "alpha"`)
	writeTheme(t, dir, "broken.toml", `name = `)
	writeTheme(t, dir, "notes.txt", `not a theme`)

	themes, warnings := loadThemes(trueColorRenderer(), dir)

	assert.Equal(t, []string{"flat", "alpha", "zeta"}, names(themes))
	require.Len(t, warnings, 1)
	assert.Contains(t, warnings[0], "broken.toml")
}

func TestLoadThemesLetsAFileReplaceABuiltin(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeTheme(t, dir, "mine.toml", "name = \"flat\"\n[glyphs]\ncursor = \">\"")

	themes, warnings := loadThemes(trueColorRenderer(), dir)

	assert.Empty(t, warnings)
	assert.Equal(t, []string{"flat"}, names(themes))
	assert.Equal(t, ">", themes[0].glyphs.cursor)
}

func TestLoadThemesWithoutAThemesFolder(t *testing.T) {
	t.Parallel()

	themes, warnings := loadThemes(trueColorRenderer(), filepath.Join(t.TempDir(), "missing"))

	assert.Equal(t, []string{"flat"}, names(themes))
	assert.Empty(t, warnings)
}

func TestResolveThemePicksTheRequestedThenTheRemembered(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeTheme(t, dir, "mint.toml", `name = "mint"`)
	r := trueColorRenderer()

	set, err := resolveThemes(r, Options{ConfigDir: dir})
	require.NoError(t, err)
	assert.Equal(t, "flat", set.themes[set.current].name, "nothing asked or remembered")

	require.NoError(t, saveThemeName(dir, "mint"))
	set, err = resolveThemes(r, Options{ConfigDir: dir})
	require.NoError(t, err)
	assert.Equal(t, "mint", set.themes[set.current].name, "remembered")

	set, err = resolveThemes(r, Options{ConfigDir: dir, Theme: "flat"})
	require.NoError(t, err)
	assert.Equal(t, "flat", set.themes[set.current].name, "the flag wins")

	_, err = resolveThemes(r, Options{ConfigDir: dir, Theme: "vinyl"})
	require.EqualError(t, err, `unknown theme "vinyl" (available: flat, mint)`)

	require.NoError(t, saveThemeName(dir, "gone"))
	set, err = resolveThemes(r, Options{ConfigDir: dir})
	require.NoError(t, err)
	assert.Equal(t, "flat", set.themes[set.current].name, "a remembered theme that no longer exists falls back")
}

func TestResolveThemeReportsBrokenFiles(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeTheme(t, dir, "broken.toml", `name = `)

	set, err := resolveThemes(trueColorRenderer(), Options{ConfigDir: dir})
	require.NoError(t, err)
	assert.Len(t, set.warnings, 1)
}

func themedApp(t *testing.T, set themeSet, saved *[]string) app {
	t.Helper()
	m := goldenApp(music.Status{State: music.Playing, Track: goldenQueue()[1]})
	m.useThemes(set, func(name string) error {
		*saved = append(*saved, name)
		return nil
	})
	return m
}

func TestAppCyclesThemesWithShiftT(t *testing.T) {
	t.Parallel()

	r := trueColorRenderer()
	mint, err := parseTheme(r, []byte("name = \"mint\"\n[glyphs]\nmarker = \"*\""))
	require.NoError(t, err)
	var saved []string
	m := themedApp(t, themeSet{themes: []theme{flatTheme(r), mint}}, &saved)
	assert.Contains(t, m.View(), "T theme")

	next, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("T")})
	m = next.(app)
	assert.Equal(t, "mint", m.theme.name)
	assert.Contains(t, m.View(), "theme: mint")
	assert.Contains(t, m.View(), "\n* ", "the theme's marker glyph is used")
	drainBatchSkippingTicks(cmd)
	assert.Equal(t, []string{"mint"}, saved)

	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("T")})
	assert.Equal(t, "flat", next.(app).theme.name, "wraps around")
}

func TestAppHidesTheThemeKeyWithASingleTheme(t *testing.T) {
	t.Parallel()

	var saved []string
	m := themedApp(t, themeSet{themes: []theme{flatTheme(trueColorRenderer())}}, &saved)
	assert.NotContains(t, m.View(), "T theme")

	next, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("T")})
	assert.Nil(t, cmd)
	assert.Equal(t, "flat", next.(app).theme.name)
}

func TestAppShowsThemeWarningsAndSaveFailures(t *testing.T) {
	t.Parallel()

	r := trueColorRenderer()
	m := goldenApp(music.Status{State: music.Playing, Track: goldenQueue()[1]})
	m.useThemes(themeSet{themes: []theme{flatTheme(r), flatTheme(r)}, warnings: []string{"theme broken.toml: toml: bad"}},
		func(string) error { return errors.New("read-only") })
	assert.Contains(t, m.View(), "theme broken.toml: toml: bad")

	next, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("T")})
	m = next.(app)
	for _, msg := range drainBatchSkippingTicks(cmd) {
		m = apply(m, msg)
	}
	assert.Contains(t, m.View(), "theme not saved: read-only")
}

func TestSaveThemeNameRoundTrips(t *testing.T) {
	t.Parallel()

	dir := filepath.Join(t.TempDir(), "amp")
	require.NoError(t, saveThemeName(dir, "mint"))
	assert.Equal(t, "mint", loadThemeName(dir))
	assert.Empty(t, loadThemeName(t.TempDir()))
}

func drainBatchSkippingTicks(cmd tea.Cmd) []tea.Msg {
	if cmd == nil {
		return nil
	}
	batch, ok := cmd().(tea.BatchMsg)
	if !ok {
		return nil
	}
	var out []tea.Msg
	for _, c := range batch {
		if c == nil {
			continue
		}
		ch := make(chan tea.Msg, 1)
		go func() { ch <- c() }()
		select {
		case msg := <-ch:
			out = append(out, msg)
		case <-time.After(200 * time.Millisecond):
		}
	}
	return out
}
