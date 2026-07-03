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
	"github.com/helmedeiros/amp/internal/port"
)

const headerBarWidth = 40

var (
	stateStyle = map[music.PlayerState]lipgloss.Style{
		music.Playing: lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("2")),
		music.Paused:  lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("3")),
		music.Stopped: lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("8")),
	}
	trackStyle     = lipgloss.NewStyle().Bold(true)
	dimStyle       = lipgloss.NewStyle().Faint(true)
	activeTabStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("6"))
)

// tabID identifies a content tab.
type tabID int

const (
	tabQueue tabID = iota
	tabPlaylists
	tabArtists
	tabAlbums
)

var tabNames = []string{"Queue", "Playlists", "Artists", "Albums"}

type (
	statusMsg       music.Status
	streamClosedMsg struct{}
	tabItemsMsg     struct {
		tab   tabID
		items []string
		err   error
	}
)

// app is the root TUI model: a live header plus tabbed list views.
type app struct {
	ctx    context.Context
	ctrl   port.Controller
	stream <-chan music.Status

	status    music.Status
	hasStatus bool

	active tabID
	lists  []list
	loaded []bool

	width, height int
	quitting      bool
}

func newApp(ctx context.Context, ctrl port.Controller, stream <-chan music.Status) app {
	lists := make([]list, len(tabNames))
	for i := range lists {
		lists[i] = newList()
	}
	return app{
		ctx: ctx, ctrl: ctrl, stream: stream,
		lists: lists, loaded: make([]bool, len(tabNames)),
		width: 80, height: 24,
	}
}

func (m app) Init() tea.Cmd {
	return tea.Batch(waitForStatus(m.stream), m.loadTab(m.active))
}

func waitForStatus(stream <-chan music.Status) tea.Cmd {
	return func() tea.Msg {
		s, ok := <-stream
		if !ok {
			return streamClosedMsg{}
		}
		return statusMsg(s)
	}
}

func (m app) loadTab(tab tabID) tea.Cmd {
	ctx, ctrl := m.ctx, m.ctrl
	return func() tea.Msg {
		items, err := fetchTab(ctx, ctrl, tab)
		return tabItemsMsg{tab: tab, items: items, err: err}
	}
}

func fetchTab(ctx context.Context, ctrl port.Controller, tab tabID) ([]string, error) {
	switch tab {
	case tabQueue:
		ts, err := ctrl.Queue(ctx)
		return trackLines(ts), err
	case tabPlaylists:
		ps, err := ctrl.Playlists(ctx)
		return playlistLines(ps), err
	case tabArtists:
		return ctrl.Artists(ctx)
	case tabAlbums:
		return ctrl.Albums(ctx)
	default:
		return nil, nil
	}
}

func (m app) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case statusMsg:
		m.status, m.hasStatus = music.Status(msg), true
		return m, waitForStatus(m.stream)

	case streamClosedMsg:
		return m, nil // header stops updating; the TUI stays usable

	case tabItemsMsg:
		if msg.err == nil {
			m.lists[msg.tab].SetItems(msg.items)
			m.loaded[msg.tab] = true
		}
		return m, nil

	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.resizeLists()
		return m, nil

	case tea.KeyMsg:
		return m.handleKey(msg)
	}
	return m, nil
}

func (m app) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "ctrl+c":
		m.quitting = true
		return m, tea.Quit
	case "tab", "l", "right":
		return m.switchTab((m.active + 1) % tabID(len(tabNames)))
	case "shift+tab", "h", "left":
		return m.switchTab((m.active - 1 + tabID(len(tabNames))) % tabID(len(tabNames)))
	case "1", "2", "3", "4":
		return m.switchTab(tabID(msg.String()[0] - '1'))
	case "j", "down":
		m.lists[m.active].MoveDown()
	case "k", "up":
		m.lists[m.active].MoveUp()
	case "r":
		m.loaded[m.active] = false
		return m, m.loadTab(m.active)
	}
	return m, nil
}

func (m app) switchTab(to tabID) (tea.Model, tea.Cmd) {
	m.active = to
	if !m.loaded[to] {
		return m, m.loadTab(to)
	}
	return m, nil
}

func (m *app) resizeLists() {
	// Leave room for the header (~4 lines), the tab bar, and the footer.
	h := m.height - 8
	if h < 3 {
		h = 3
	}
	for i := range m.lists {
		m.lists[i].SetHeight(h)
	}
}

func (m app) View() string {
	if m.quitting {
		return ""
	}

	var b strings.Builder
	b.WriteString(renderHeader(m.status, m.hasStatus))
	b.WriteString("\n\n")
	b.WriteString(renderTabBar(m.active))
	b.WriteString("\n\n")
	b.WriteString(m.lists[m.active].View())
	b.WriteString("\n\n")
	b.WriteString(dimStyle.Render("tab/1-4 switch · j/k move · r refresh · q quit"))
	return b.String()
}

func renderTabBar(active tabID) string {
	parts := make([]string, len(tabNames))
	for i, name := range tabNames {
		label := fmt.Sprintf("%d %s", i+1, name)
		if tabID(i) == active {
			label = activeTabStyle.Render(label)
		} else {
			label = dimStyle.Render(label)
		}
		parts[i] = label
	}
	return strings.Join(parts, "   ")
}

func renderHeader(s music.Status, hasStatus bool) string {
	if !hasStatus {
		return dimStyle.Render("connecting…")
	}

	var b strings.Builder
	b.WriteString(stateStyle[s.State].Render(strings.ToUpper(s.State.String())))
	if s.HasTrack() {
		fmt.Fprintf(&b, "\n%s", trackStyle.Render(artistTitle(s.Track)))
		if s.Track.Album != "" {
			fmt.Fprintf(&b, "\n%s", dimStyle.Render(s.Track.Album))
		}
		fmt.Fprintf(&b, "\n%s %s %s  %s",
			clock(s.Elapsed), bar(s.Progress(), headerBarWidth), clock(s.Track.Duration),
			dimStyle.Render(fmt.Sprintf("%d%%", int(math.Round(s.Progress()*100)))),
		)
	}
	fmt.Fprintf(&b, "\n%s", dimStyle.Render(fmt.Sprintf("vol %d%%", s.Volume.Int())))
	if s.Shuffle {
		b.WriteString(dimStyle.Render("  shuffle"))
	}
	if s.Repeat != music.RepeatOff {
		b.WriteString(dimStyle.Render("  repeat " + s.Repeat.String()))
	}
	return b.String()
}

func trackLines(tracks []music.Track) []string {
	lines := make([]string, len(tracks))
	for i, t := range tracks {
		line := artistTitle(t)
		if t.Album != "" {
			line += " (" + t.Album + ")"
		}
		if t.Duration > 0 {
			line += "  " + clock(t.Duration)
		}
		lines[i] = line
	}
	return lines
}

func playlistLines(ps []music.Playlist) []string {
	lines := make([]string, len(ps))
	for i, p := range ps {
		lines[i] = fmt.Sprintf("%s  (%d)", p.Name, p.Count)
	}
	return lines
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

// RunApp runs the tabbed TUI until the user quits or ctx is cancelled.
func RunApp(ctx context.Context, ctrl port.Controller, stream <-chan music.Status) error {
	p := tea.NewProgram(newApp(ctx, ctrl, stream), tea.WithAltScreen(), tea.WithContext(ctx))
	_, err := p.Run()
	return err
}
