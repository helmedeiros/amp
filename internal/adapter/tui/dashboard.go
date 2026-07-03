package tui

import (
	"context"
	"fmt"
	"math"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/helmedeiros/amp/internal/music"
)

const dashBarWidth = 40

var (
	stateStyle = map[music.PlayerState]lipgloss.Style{
		music.Playing: lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("2")),
		music.Paused:  lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("3")),
		music.Stopped: lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("8")),
	}
	trackStyle = lipgloss.NewStyle().Bold(true)
	dimStyle   = lipgloss.NewStyle().Faint(true)
)

// statusMsg carries a new status snapshot into the model.
type statusMsg music.Status

// streamClosedMsg signals the status stream ended (e.g. the daemon stopped).
type streamClosedMsg struct{}

// dashboard is the walking-skeleton TUI: a live now-playing header driven by a
// status channel.
type dashboard struct {
	stream    <-chan music.Status
	status    music.Status
	hasStatus bool
	width     int
	quitting  bool
}

func newDashboard(stream <-chan music.Status) dashboard {
	return dashboard{stream: stream, width: 80}
}

func (m dashboard) Init() tea.Cmd { return waitForStatus(m.stream) }

// waitForStatus blocks for the next status and turns it into a message,
// re-armed after each receive to keep listening.
func waitForStatus(stream <-chan music.Status) tea.Cmd {
	return func() tea.Msg {
		s, ok := <-stream
		if !ok {
			return streamClosedMsg{}
		}
		return statusMsg(s)
	}
}

func (m dashboard) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case statusMsg:
		m.status, m.hasStatus = music.Status(msg), true
		return m, waitForStatus(m.stream)
	case streamClosedMsg:
		m.quitting = true
		return m, tea.Quit
	case tea.WindowSizeMsg:
		m.width = msg.Width
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "esc", "ctrl+c":
			m.quitting = true
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m dashboard) View() string {
	if m.quitting {
		return ""
	}
	if !m.hasStatus {
		return dimStyle.Render("connecting…") + "\n"
	}

	s := m.status
	var b strings.Builder

	state := stateStyle[s.State].Render(strings.ToUpper(s.State.String()))
	fmt.Fprintf(&b, "%s\n", state)

	if s.HasTrack() {
		fmt.Fprintf(&b, "%s\n", trackStyle.Render(artistTitle(s.Track)))
		if s.Track.Album != "" {
			fmt.Fprintf(&b, "%s\n", dimStyle.Render(s.Track.Album))
		}
		fmt.Fprintf(&b, "%s %s %s  %s\n",
			clock(s.Elapsed), bar(s.Progress(), dashBarWidth), clock(s.Track.Duration),
			dimStyle.Render(fmt.Sprintf("%d%%", int(math.Round(s.Progress()*100)))),
		)
	}

	fmt.Fprintf(&b, "%s", dimStyle.Render(fmt.Sprintf("vol %d%%", s.Volume.Int())))
	if s.Shuffle {
		b.WriteString(dimStyle.Render("  shuffle"))
	}
	if s.Repeat != music.RepeatOff {
		b.WriteString(dimStyle.Render("  repeat " + s.Repeat.String()))
	}
	b.WriteString("\n\n" + dimStyle.Render("q quit"))

	return b.String()
}

func artistTitle(t music.Track) string {
	if t.Artist == "" {
		return t.Name
	}
	return t.Artist + " — " + t.Name
}

func clock(d time.Duration) string {
	total := int(d.Round(time.Second).Seconds())
	return fmt.Sprintf("%02d:%02d", total/60, total%60)
}

func bar(fraction float64, width int) string {
	if width <= 0 {
		return ""
	}
	fraction = math.Max(0, math.Min(1, fraction))
	filled := int(math.Round(fraction * float64(width)))
	return strings.Repeat("━", filled) + strings.Repeat("─", width-filled)
}

// RunDashboard runs the live dashboard until the user quits, the stream ends,
// or ctx is cancelled.
func RunDashboard(ctx context.Context, stream <-chan music.Status) error {
	p := tea.NewProgram(newDashboard(stream), tea.WithAltScreen(), tea.WithContext(ctx))
	_, err := p.Run()
	return err
}
