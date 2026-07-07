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
	playingStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("2"))
	noticeStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("3"))
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
		keys   []string // per-row filter keys (lowercased; may include artists)
		err    error
	}
	actionDoneMsg  struct{}
	queuePlayedMsg struct {
		notice string // persistent one-line warning (e.g. album still partial)
		flash  string // transient success line (e.g. album completed)
	}
	actionErrMsg     struct{ err error }
	searchResultsMsg struct {
		items []string
		err   error
	}
	// workTickMsg animates the loading bar while a play/fill is in flight.
	workTickMsg struct{}
	// flashClearMsg clears the transient success line after a short delay.
	flashClearMsg struct{}
)

// workTick schedules the next animation frame of the loading bar.
func workTick() tea.Cmd {
	return tea.Tick(110*time.Millisecond, func(time.Time) tea.Msg { return workTickMsg{} })
}

// flashDuration is how long the "album complete" confirmation stays up.
const flashDuration = 4 * time.Second

func clearFlashAfter() tea.Cmd {
	return tea.Tick(flashDuration, func(time.Time) tea.Msg { return flashClearMsg{} })
}

// app is the root TUI model: a live header plus tabbed list views.
type app struct {
	ctx    context.Context
	ctrl   port.Controller
	stream <-chan music.Status

	status    music.Status
	hasStatus bool

	active tabID
	lists  []list
	items  [][]string // per-tab full display lines (source for filtering)
	values [][]string // per-tab action targets (playlist/album/artist names)
	keys   [][]string // per-tab, per-row filter keys (lowercased, artist-aware)
	loaded []bool

	// filtering narrows the active tab's list locally as the user types.
	// viewMap maps each visible row back to its index in the full items slice
	// (nil = identity) so a play on a filtered row still hits the right track.
	filtering   bool
	filterQuery string
	viewMap     []int

	searchQuery   string
	searchEditing bool

	// notice is a one-line warning shown below the list (e.g. that only part of an
	// album was in the library). Cleared on the next tab switch.
	notice string
	// flash is a transient success line (e.g. an album was completed); it clears
	// itself after flashDuration.
	flash string
	// working marks that a play/fill is in flight, driving an animated bar.
	working   bool
	workFrame int

	// playingKey is the filter key of the track the status stream last reported
	// as playing. It lets the Queue follow the track: when the key changes the
	// cursor jumps to the matching row.
	playingKey string

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
		lists:  lists,
		items:  make([][]string, len(tabNames)),
		values: make([][]string, len(tabNames)),
		keys:   make([][]string, len(tabNames)),
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
		items, values, keys, err := fetchTab(ctx, ctrl, tab)
		return tabItemsMsg{tab: tab, items: items, values: values, keys: keys, err: err}
	}
}

// fetchTab returns the display lines, the per-row action targets (playlist/
// album/artist names; nil for the Queue, which acts by index), and the per-row
// filter keys. A key is what `/` matches against and may carry more than the
// visible line — e.g. a playlist's artists — so filtering by an artist finds
// playlists that merely contain them.
func fetchTab(ctx context.Context, ctrl port.Controller, tab tabID) (items, values, keys []string, err error) {
	switch tab {
	case tabQueue:
		ts, err := ctrl.Queue(ctx)
		return trackLines(ts), nil, trackKeys(ts), err
	case tabPlaylists:
		ps, err := ctrl.Playlists(ctx)
		return playlistLines(ps), playlistNames(ps), playlistKeys(ps), err
	case tabArtists:
		names, err := ctrl.Artists(ctx)
		return names, names, lowerAll(names), err
	case tabAlbums:
		al, err := ctrl.Albums(ctx)
		return albumLines(al), albumNames(al), albumKeys(al), err
	default:
		return nil, nil, nil, nil
	}
}

func (m app) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case statusMsg:
		m.status, m.hasStatus = music.Status(msg), true
		m.syncQueue(false)
		return m, waitForStatus(m.stream)

	case streamClosedMsg:
		return m, nil // header stops updating; the TUI stays usable

	case tabItemsMsg:
		if msg.err == nil {
			m.items[msg.tab] = msg.items
			m.values[msg.tab] = msg.values
			m.keys[msg.tab] = msg.keys
			m.loaded[msg.tab] = true
			m.refreshView(msg.tab)
			if msg.tab == tabQueue {
				m.syncQueue(true)
			}
		}
		return m, nil

	case searchResultsMsg:
		if msg.err == nil {
			m.items[tabSearch] = msg.items
			m.lists[tabSearch].SetItems(msg.items)
		}
		return m, nil

	case queuePlayedMsg:
		// A play set the queue to the chosen context; show it.
		m.working = false
		m.active = tabQueue
		m.clearFilter()
		m.notice = msg.notice
		m.flash = msg.flash
		m.loaded[tabQueue] = false
		cmds := []tea.Cmd{m.loadTab(tabQueue)}
		if m.flash != "" {
			cmds = append(cmds, clearFlashAfter())
		}
		return m, tea.Batch(cmds...)

	case workTickMsg:
		if !m.working {
			return m, nil // stop animating once the play resolves
		}
		m.workFrame++
		return m, workTick()

	case flashClearMsg:
		m.flash = ""
		return m, nil

	case actionDoneMsg:
		return m, nil

	case actionErrMsg:
		m.working = false
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
	if m.filtering {
		return m.handleFilterInput(msg)
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
		} else {
			m.filtering = true
			m.filterQuery = ""
			m.applyView()
		}
	case "esc":
		if m.active != tabSearch && m.filterQuery != "" {
			m.clearFilter()
		}
	case "j", "down":
		m.lists[m.active].MoveDown()
	case "k", "up":
		m.lists[m.active].MoveUp()
	case "enter":
		cmd := m.playSelection()
		if cmd != nil && m.active == tabAlbums {
			// Album plays may pause to fill missing tracks from Apple Music; show
			// an animated bar until the play resolves.
			m.working, m.workFrame, m.flash = true, 0, ""
			return m, tea.Batch(cmd, workTick())
		}
		return m, cmd
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

// handleFilterInput narrows the active tab's list as the user types. Enter
// keeps the narrowed list and returns to navigation (so j/k/enter act on the
// filtered rows); Esc clears the filter and restores the full list.
func (m app) handleFilterInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEnter:
		m.filtering = false
		return m, nil
	case tea.KeyEsc:
		m.clearFilter()
		return m, nil
	case tea.KeyCtrlC:
		m.quitting = true
		return m, tea.Quit
	case tea.KeyBackspace:
		if r := []rune(m.filterQuery); len(r) > 0 {
			m.filterQuery = string(r[:len(r)-1])
		}
		m.applyView()
	case tea.KeySpace:
		m.filterQuery += " "
		m.applyView()
	case tea.KeyRunes:
		m.filterQuery += string(msg.Runes)
		m.applyView()
	}
	return m, nil
}

// clearFilter drops the active filter and restores the full list.
func (m *app) clearFilter() {
	m.filtering = false
	m.filterQuery = ""
	m.viewMap = nil
	m.applyView()
}

// refreshView updates a tab's visible list from its full items, applying the
// active filter when it is the tab on screen.
func (m *app) refreshView(tab tabID) {
	if tab == m.active {
		m.applyView()
		return
	}
	m.lists[tab].SetItems(m.items[tab])
}

// applyView sets the active tab's visible rows from its full items, narrowing to
// the filter query (case-insensitive substring) when one is set. viewMap records
// the original index of each visible row so a play maps back correctly. The
// Search tab is never filtered locally (it searches the library instead).
func (m *app) applyView() {
	tab := m.active
	full := m.items[tab]
	q := strings.ToLower(strings.TrimSpace(m.filterQuery))
	if q == "" || tab == tabSearch {
		m.viewMap = nil
		m.lists[tab].SetItems(full)
		return
	}

	keys := m.keys[tab]
	shown := make([]string, 0, len(full))
	vm := make([]int, 0, len(full))
	for i, it := range full {
		key := strings.ToLower(it)
		if i < len(keys) {
			key = keys[i] // artist-aware key (already lowercased)
		}
		if strings.Contains(key, q) {
			shown = append(shown, it)
			vm = append(vm, i)
		}
	}
	m.viewMap = vm
	m.lists[tab].SetItems(shown)
	m.lists[tab].Top()
	if tab == tabQueue {
		m.lists[tabQueue].SetMarker(m.playingQueueRow())
	}
}

// playingQueueRow returns the visible Queue row of the currently playing track,
// or -1 when nothing is playing or the track is not in the queue. It honours the
// active filter's view mapping so the marker lands on the right visible row.
func (m app) playingQueueRow() int {
	if !m.hasStatus || !m.status.HasTrack() {
		return -1
	}
	want := trackKey(m.status.Track)
	full := -1
	for i, k := range m.keys[tabQueue] {
		if k == want {
			full = i
			break
		}
	}
	if full < 0 {
		return -1
	}
	if m.active != tabQueue || m.viewMap == nil {
		return full
	}
	for row, orig := range m.viewMap {
		if orig == full {
			return row
		}
	}
	return -1 // playing track is filtered out of view
}

// syncQueue refreshes the Queue's playing marker from the current status. When
// focus is set, or the playing track has just changed, it also moves the cursor
// onto that row so the Queue follows what is playing. An in-progress filter edit
// is left alone so typing does not fight the cursor.
func (m *app) syncQueue(focus bool) {
	row := m.playingQueueRow()
	m.lists[tabQueue].SetMarker(row)

	key := ""
	if m.hasStatus && m.status.HasTrack() {
		key = trackKey(m.status.Track)
	}
	if key != m.playingKey {
		focus = true // the playing track changed; jump to it
	}
	m.playingKey = key

	if focus && row >= 0 && m.active == tabQueue && !m.filtering {
		m.lists[tabQueue].MoveTo(row)
	}
}

// orig maps a visible row index to its index in the full item slice.
func (m app) orig(i int) int {
	if m.viewMap == nil || i < 0 || i >= len(m.viewMap) {
		return i
	}
	return m.viewMap[i]
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
	idx := m.orig(l.Cursor())

	switch m.active {
	case tabQueue:
		return queueCmd(func() error { return m.ctrl.PlayQueueAt(m.ctx, idx) })
	case tabSearch:
		q := m.searchQuery
		return queueCmd(func() error { return m.ctrl.PlaySearch(m.ctx, q, searchLimit, idx) })
	}

	vals := m.values[m.active]
	if idx >= len(vals) {
		return nil
	}
	name := vals[idx]
	ctx, ctrl := m.ctx, m.ctrl
	return func() tea.Msg {
		res, err := ctrl.PlayQuery(ctx, name, searchLimit)
		if err != nil {
			return actionErrMsg{err: err}
		}
		notice, flash := albumFeedback(res)
		return queuePlayedMsg{notice: notice, flash: flash}
	}
}

// albumFeedback turns an album play result into user feedback: a transient
// success flash when the album is complete, or a persistent warning when it is
// still partial (distinguishing "still syncing" from "not configured").
func albumFeedback(res port.PlayResult) (notice, flash string) {
	if res.Kind != "album" || res.Album.Total == 0 {
		return "", ""
	}
	c := res.Album
	switch {
	case !c.Partial():
		return "", fmt.Sprintf("✓ %s — all %d tracks in your library", res.Label, c.Total)
	case res.AlbumFilled:
		return fmt.Sprintf("%d of %d tracks — the rest are still syncing from Apple Music; press r to reload", c.Queued, c.Total), ""
	default:
		return fmt.Sprintf("only %d of %d tracks of %q are in your library — run: amp auth apple-music, or add it in Music.app", c.Queued, c.Total, res.Label), ""
	}
}

// actionCmd runs an action that does not change the queue (e.g. play/pause).
func actionCmd(f func() error) tea.Cmd {
	return func() tea.Msg {
		if err := f(); err != nil {
			return actionErrMsg{err: err}
		}
		return actionDoneMsg{}
	}
}

// queueCmd runs a play that sets the queue to the selected context; on success
// the app moves to the Queue tab and refreshes it.
func queueCmd(f func() error) tea.Cmd {
	return func() tea.Msg {
		if err := f(); err != nil {
			return actionErrMsg{err: err}
		}
		return queuePlayedMsg{}
	}
}

func (m app) switchTab(to tabID) (tea.Model, tea.Cmd) {
	m.active = to
	m.filtering = false
	m.filterQuery = ""
	m.viewMap = nil
	m.notice = ""
	if to == tabSearch {
		// Land in navigation mode so number keys still switch tabs; press / to
		// start typing a query.
		return m, nil
	}
	if !m.loaded[to] {
		return m, m.loadTab(to)
	}
	m.lists[to].SetItems(m.items[to]) // clear any lingering filter view
	if to == tabQueue {
		m.syncQueue(true)
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
	} else if m.filtering || m.filterQuery != "" {
		b.WriteString(renderFilterPrompt(m.filterQuery, m.filtering))
		b.WriteString("\n\n")
	}
	b.WriteString(m.lists[m.active].View())
	b.WriteString("\n\n")
	switch {
	case m.working:
		b.WriteString(playingStyle.Render(loadBar(m.workFrame, 24)) + "  " +
			dimStyle.Render("completing album via Apple Music…"))
		b.WriteString("\n\n")
	case m.flash != "":
		b.WriteString(playingStyle.Render(m.flash))
		b.WriteString("\n\n")
	case m.notice != "":
		b.WriteString(noticeStyle.Render(m.notice))
		b.WriteString("\n\n")
	}
	b.WriteString(dimStyle.Render(m.hint()))
	return b.String()
}

func (m app) hint() string {
	if m.active == tabSearch && m.searchEditing {
		return "type a query · enter search · esc cancel"
	}
	if m.filtering {
		return "type to filter · enter keep · esc clear"
	}
	if m.filterQuery != "" {
		return "filtered · esc clear · j/k move · enter play · / refine"
	}
	find := "filter"
	if m.active == tabSearch {
		find = "search"
	}
	base := "tab/1-5 switch · j/k move · / " + find + " · enter play · space pause"
	if m.active != tabSearch {
		base += " · r reload"
	}
	return base + " · q quit"
}

func renderFilterPrompt(query string, editing bool) string {
	if editing {
		return "filter: " + query + "▌"
	}
	return "filter: " + query + dimStyle.Render("  (esc clear)")
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

// playlistKeys builds each playlist's filter key from its name plus the artists
// it contains, so filtering by an artist surfaces playlists that include them.
func playlistKeys(ps []music.Playlist) []string {
	keys := make([]string, len(ps))
	for i, p := range ps {
		keys[i] = strings.ToLower(p.Name + " " + strings.Join(p.Artists, " "))
	}
	return keys
}

// albumLines renders "Artist — Album" (just the album when the artist is
// unknown) so mixed or same-named albums are distinguishable.
func albumLines(albums []music.Album) []string {
	lines := make([]string, len(albums))
	for i, a := range albums {
		if a.Artist == "" {
			lines[i] = a.Name
			continue
		}
		lines[i] = a.Artist + " — " + a.Name
	}
	return lines
}

// albumNames returns the album names used as play targets.
func albumNames(albums []music.Album) []string {
	names := make([]string, len(albums))
	for i, a := range albums {
		names[i] = a.Name
	}
	return names
}

// albumKeys builds each album's filter key from its name and artist.
func albumKeys(albums []music.Album) []string {
	keys := make([]string, len(albums))
	for i, a := range albums {
		keys[i] = strings.ToLower(a.Name + " " + a.Artist)
	}
	return keys
}

// trackKeys builds each track's filter key from its artist, album, and title.
func trackKeys(tracks []music.Track) []string {
	keys := make([]string, len(tracks))
	for i, t := range tracks {
		keys[i] = trackKey(t)
	}
	return keys
}

// trackKey is the case-insensitive artist+album+title key used both to filter
// the queue and to locate the currently playing track within it.
func trackKey(t music.Track) string {
	return strings.ToLower(t.Artist + " " + t.Album + " " + t.Name)
}

// lowerAll returns a lowercased copy of each string, for use as filter keys.
func lowerAll(ss []string) []string {
	keys := make([]string, len(ss))
	for i, s := range ss {
		keys[i] = strings.ToLower(s)
	}
	return keys
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

// loadBar renders an indeterminate progress bar: a lit window that slides back
// and forth across a track, animated by frame, so ongoing work reads as motion.
func loadBar(frame, width int) string {
	if width < 8 {
		width = 8
	}
	const win = 6
	span := 2 * (width - win) // one full there-and-back cycle
	p := frame % span
	pos := p
	if p > width-win {
		pos = span - p // reflect on the return trip
	}
	var b strings.Builder
	for i := 0; i < width; i++ {
		if i >= pos && i < pos+win {
			b.WriteString("▓")
		} else {
			b.WriteString("░")
		}
	}
	return b.String()
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
