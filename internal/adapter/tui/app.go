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
	tabSearch
)

var tabNames = []string{"Queue", "Playlists", "Artists", "Albums", "Search"}

const searchLimit = 50

type (
	statusMsg       music.Status
	streamClosedMsg struct{}
	tabItemsMsg     struct {
		tab    tabID
		items  []string
		values []string
		err    error
	}
	actionDoneMsg    struct{}
	actionErrMsg     struct{ err error }
	searchResultsMsg struct {
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
	values [][]string // per-tab action targets (playlist/album/artist names)
	loaded []bool

	searchQuery   string
	searchEditing bool

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
		lists: lists, values: make([][]string, len(tabNames)),
		loaded: make([]bool, len(tabNames)),
		width:  80, height: 24,
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
		items, values, err := fetchTab(ctx, ctrl, tab)
		return tabItemsMsg{tab: tab, items: items, values: values, err: err}
	}
}

// fetchTab returns the display lines and, where applicable, the per-row action
// targets (playlist/album/artist names). The Queue acts by index, so it has no
// values.
func fetchTab(ctx context.Context, ctrl port.Controller, tab tabID) (items, values []string, err error) {
	switch tab {
	case tabQueue:
		ts, err := ctrl.Queue(ctx)
		return trackLines(ts), nil, err
	case tabPlaylists:
		ps, err := ctrl.Playlists(ctx)
		return playlistLines(ps), playlistNames(ps), err
	case tabArtists:
		names, err := ctrl.Artists(ctx)
		return names, names, err
	case tabAlbums:
		names, err := ctrl.Albums(ctx)
		return names, names, err
	default:
		return nil, nil, nil
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
			m.values[msg.tab] = msg.values
			m.loaded[msg.tab] = true
		}
		return m, nil

	case searchResultsMsg:
		if msg.err == nil {
			m.lists[tabSearch].SetItems(msg.items)
		}
		return m, nil

	case actionDoneMsg:
		return m, m.loadTab(m.active) // reflect any queue/order change

	case actionErrMsg:
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
	if m.active == tabSearch && m.searchEditing {
		return m.handleSearchInput(msg)
	}

	n := tabID(len(tabNames))
	switch msg.String() {
	case "q", "ctrl+c":
		m.quitting = true
		return m, tea.Quit
	case "tab", "l", "right":
		return m.switchTab((m.active + 1) % n)
	case "shift+tab", "h", "left":
		return m.switchTab((m.active - 1 + n) % n)
	case "1", "2", "3", "4", "5":
		return m.switchTab(tabID(msg.String()[0] - '1'))
	case "/":
		if m.active == tabSearch {
			m.searchEditing = true
		}
	case "j", "down":
		m.lists[m.active].MoveDown()
	case "k", "up":
		m.lists[m.active].MoveUp()
	case "enter":
		return m, m.playSelection()
	case " ":
		return m, actionCmd(func() error { return m.ctrl.Toggle(m.ctx) })
	case "r":
		if m.active != tabSearch {
			m.loaded[m.active] = false
			return m, m.loadTab(m.active)
		}
	}
	return m, nil
}

// handleSearchInput edits the query while the Search tab is in edit mode.
func (m app) handleSearchInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEnter:
		m.searchEditing = false
		return m, m.runSearch()
	case tea.KeyEsc:
		m.searchEditing = false
	case tea.KeyCtrlC:
		m.quitting = true
		return m, tea.Quit
	case tea.KeyBackspace:
		if r := []rune(m.searchQuery); len(r) > 0 {
			m.searchQuery = string(r[:len(r)-1])
		}
	case tea.KeySpace:
		m.searchQuery += " "
	case tea.KeyRunes:
		m.searchQuery += string(msg.Runes)
	}
	return m, nil
}

func (m app) runSearch() tea.Cmd {
	ctx, ctrl, q := m.ctx, m.ctrl, strings.TrimSpace(m.searchQuery)
	return func() tea.Msg {
		if q == "" {
			return searchResultsMsg{}
		}
		tracks, err := ctrl.Search(ctx, q, searchLimit)
		return searchResultsMsg{items: trackLines(tracks), err: err}
	}
}

// playSelection plays the highlighted row: a Queue track by index, or a
// playlist/album/artist by name (routed through the smart play resolver).
func (m app) playSelection() tea.Cmd {
	l := &m.lists[m.active]
	if l.Len() == 0 {
		return nil
	}
	idx := l.Cursor()

	switch m.active {
	case tabQueue:
		return actionCmd(func() error { return m.ctrl.PlayQueueAt(m.ctx, idx) })
	case tabSearch:
		q := m.searchQuery
		return actionCmd(func() error { return m.ctrl.PlaySearch(m.ctx, q, searchLimit, idx) })
	}

	vals := m.values[m.active]
	if idx >= len(vals) {
		return nil
	}
	name := vals[idx]
	return actionCmd(func() error {
		_, err := m.ctrl.PlayQuery(m.ctx, name, searchLimit)
		return err
	})
}

func actionCmd(f func() error) tea.Cmd {
	return func() tea.Msg {
		if err := f(); err != nil {
			return actionErrMsg{err: err}
		}
		return actionDoneMsg{}
	}
}

func (m app) switchTab(to tabID) (tea.Model, tea.Cmd) {
	m.active = to
	if to == tabSearch {
		if m.lists[tabSearch].Len() == 0 {
			m.searchEditing = true
		}
		return m, nil
	}
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
	if m.active == tabSearch {
		b.WriteString(renderSearchPrompt(m.searchQuery, m.searchEditing))
		b.WriteString("\n\n")
	}
	b.WriteString(m.lists[m.active].View())
	b.WriteString("\n\n")
	b.WriteString(dimStyle.Render("tab/1-5 switch · j/k move · / search · enter play · space pause · q quit"))
	return b.String()
}

func renderSearchPrompt(query string, editing bool) string {
	if editing {
		return "search: " + query + "▌"
	}
	if query == "" {
		return dimStyle.Render("search: press / to type a query")
	}
	return "search: " + query
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

func playlistNames(ps []music.Playlist) []string {
	names := make([]string, len(ps))
	for i, p := range ps {
		names[i] = p.Name
	}
	return names
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
