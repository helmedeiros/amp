package tui

import (
	"github.com/charmbracelet/lipgloss"

	"github.com/helmedeiros/amp/internal/music"
)

type glyphs struct {
	cursor, marker  string
	fill, empty     string
	loadOn, loadOff string
}

type theme struct {
	layout   layout
	glyphs   glyphs
	states   map[music.PlayerState]lipgloss.Style
	track    lipgloss.Style
	dim      lipgloss.Style
	accent   lipgloss.Style
	selected lipgloss.Style
	current  lipgloss.Style
	notice   lipgloss.Style
}

func flatTheme(r *lipgloss.Renderer) theme {
	color := func(c string) lipgloss.Style { return r.NewStyle().Foreground(lipgloss.Color(c)) }
	return theme{
		layout: flatLayout{},
		glyphs: glyphs{cursor: "▶", marker: "♪", fill: "━", empty: "─", loadOn: "▓", loadOff: "░"},
		states: map[music.PlayerState]lipgloss.Style{
			music.Playing: color("2").Bold(true),
			music.Paused:  color("3").Bold(true),
			music.Stopped: color("8").Bold(true),
		},
		track:    r.NewStyle().Bold(true),
		dim:      r.NewStyle().Faint(true),
		accent:   color("6").Bold(true),
		selected: color("6").Bold(true),
		current:  color("2"),
		notice:   color("3"),
	}
}

func (t theme) state(s music.PlayerState) lipgloss.Style { return t.states[s] }
