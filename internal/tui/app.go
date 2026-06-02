package tui

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/shinagawa-web/pgincident/internal/core"
	"github.com/shinagawa-web/pgincident/internal/snapshot"
	"github.com/shinagawa-web/pgincident/internal/version"
)

const (
	minWidth  = 80
	minHeight = 24
)

// ConnectionPreset holds a named DSN for the connection selector.
type ConnectionPreset struct {
	Name string
	DSN  string
}

// ReconnectFn connects to the given DSN and returns a new Poller.
type ReconnectFn func(ctx context.Context, dsn string) (*core.Poller, error)

type snapshotMsg struct {
	core.PollResult
	gen int
}

type connectionSwitchedMsg struct {
	poller  *core.Poller
	name    string
	autoGen int
}

type reconnectErrMsg struct {
	err  error
	name string
	dsn  string
}

type snapshotExportMsg struct {
	path string
	err  error
}

type autoReconnectFailedMsg struct {
	gen      int
	delay    time.Duration
	deadline time.Time
	dsn      string
	name     string
	attempt  int
}

// Screen identifies which top-level screen is active.
type Screen int

const (
	ScreenOverview  Screen = iota // default startup screen
	ScreenDashboard               // incident dashboard (Level 2)
)

// App is the root Bubble Tea model.
type App struct {
	poller           *core.Poller
	pollCh           chan core.PollResult
	cancel           context.CancelFunc
	snapshot         core.Snapshot
	screen           Screen
	section          Section
	cursor           [sectionCount]int
	width            int
	height           int
	lastErr          error
	statusMsg        string
	showHelp         bool
	showDetail       bool
	detailItem       *core.Activity
	detailScroll     int
	detailLines      []string // cached formatted lines; valid when detailLinesItem == detailItem && detailLinesWidth == width
	detailLinesItem  *core.Activity
	detailLinesWidth int
	quitting         bool

	// connection switching
	connList         []ConnectionPreset
	currentConn      string
	reconnectFn      ReconnectFn
	showConnSelector bool
	connCursor       int
	gen                  int
	pollerDone           <-chan struct{} // closed when the current poller goroutine exits
	reconnectMaxDuration time.Duration  // max wall-clock time to retry before giving up
	autoReconnecting     bool
	autoReconnectGen     int
}

func New(poller *core.Poller, currentConn string, connList []ConnectionPreset, reconnectFn ReconnectFn) *App {
	ctx, cancel := context.WithCancel(context.Background())
	ch := make(chan core.PollResult, 1)
	done := make(chan struct{})
	go func() {
		defer close(done)
		poller.Run(ctx, ch)
		close(ch)
	}()
	return &App{
		poller:               poller,
		pollCh:               ch,
		cancel:               cancel,
		currentConn:          currentConn,
		connList:             connList,
		reconnectFn:          reconnectFn,
		pollerDone:           done,
		screen:               ScreenOverview,
		reconnectMaxDuration: 10 * time.Minute,
	}
}

func (a *App) Init() tea.Cmd {
	return waitForSnapshot(a.pollCh, a.gen)
}

func waitForSnapshot(ch <-chan core.PollResult, gen int) tea.Cmd {
	return func() tea.Msg {
		r, ok := <-ch
		if !ok {
			return nil // channel closed: poller stopped, nothing to dispatch
		}
		return snapshotMsg{PollResult: r, gen: gen}
	}
}

func (a *App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		a.width, a.height = msg.Width, msg.Height
		if a.showDetail && a.detailItem != nil {
			a.detailScroll = a.clampScroll(a.detailScroll)
		}
	case snapshotMsg:
		if msg.gen != a.gen {
			return a, nil
		}
		if msg.Err != nil {
			if sanitizeConnError(msg.Err.Error()) == "connection lost" && !a.autoReconnecting && a.reconnectFn != nil {
				preset := a.currentPreset()
				if preset != nil {
					a.autoReconnecting = true
					a.snapshot = core.Snapshot{}
					a.statusMsg = fmt.Sprintf("reconnecting to %s — connection lost", preset.Name)
					a.lastErr = nil
					return a, tea.Batch(
						waitForSnapshot(a.pollCh, a.gen),
						a.beginAutoReconnect(preset.DSN, preset.Name),
					)
				} else {
					a.lastErr = errors.New("connection lost")
				}
			} else if sanitizeConnError(msg.Err.Error()) != "connection lost" {
				a.lastErr = msg.Err
			}
		} else {
			a.snapshot = msg.Snapshot
			a.lastErr = nil
			if a.autoReconnecting {
				a.autoReconnectGen++
				a.statusMsg = fmt.Sprintf("connected: %s", a.currentConn)
			}
			a.autoReconnecting = false
		}
		return a, waitForSnapshot(a.pollCh, a.gen)
	case connectionSwitchedMsg:
		if msg.autoGen != 0 && msg.autoGen != a.autoReconnectGen {
			return a, nil
		}
		a.autoReconnectGen++
		a.cancel()
		ctx, cancel := context.WithCancel(context.Background())
		ch := make(chan core.PollResult, 1)
		done := make(chan struct{})
		go func() {
			defer close(done)
			msg.poller.Run(ctx, ch)
			close(ch)
		}()
		a.gen++
		a.poller = msg.poller
		a.pollCh = ch
		a.cancel = cancel
		a.pollerDone = done
		a.currentConn = msg.name
		a.showConnSelector = false
		a.autoReconnecting = false
		a.statusMsg = fmt.Sprintf("connected: %s", msg.name)
		return a, waitForSnapshot(a.pollCh, a.gen)
	case reconnectErrMsg:
		if msg.dsn != "" {
			a.currentConn = msg.name
			a.showConnSelector = false
			a.autoReconnecting = true
			a.snapshot = core.Snapshot{}
			a.statusMsg = fmt.Sprintf("reconnecting to %s — connection lost", msg.name)
			a.lastErr = nil
			return a, a.beginAutoReconnect(msg.dsn, msg.name)
		}
		a.lastErr = msg.err
		a.showConnSelector = false
		return a, nil
	case autoReconnectFailedMsg:
		if msg.gen != a.autoReconnectGen {
			return a, nil
		}
		if time.Now().After(msg.deadline) {
			a.autoReconnecting = false
			a.lastErr = errors.New("connection lost")
			a.statusMsg = ""
			a.autoReconnectGen++
			return a, nil
		}
		a.statusMsg = fmt.Sprintf("reconnecting to %s — attempt %d, retrying in %s", msg.name, msg.attempt, msg.delay.Round(time.Second))
		fn := a.reconnectFn
		gen := msg.gen
		deadline := msg.deadline
		dsn := msg.dsn
		name := msg.name
		nextAttempt := msg.attempt + 1
		nextDelay := backoffNext(msg.delay)
		oldCancel := a.cancel
		oldDone := a.pollerDone
		return a, tea.Tick(msg.delay, func(_ time.Time) tea.Msg {
			oldCancel()
			<-oldDone
			p, err := fn(context.Background(), dsn)
			if err != nil {
				return autoReconnectFailedMsg{gen: gen, delay: nextDelay, deadline: deadline, dsn: dsn, name: name, attempt: nextAttempt}
			}
			return connectionSwitchedMsg{poller: p, name: name, autoGen: gen}
		})
	case snapshotExportMsg:
		if msg.err != nil {
			a.lastErr = msg.err
		} else {
			a.statusMsg = fmt.Sprintf("saved: %s", msg.path)
		}
	case tea.KeyMsg:
		return a.handleKey(msg)
	}
	return a, nil
}

func (a *App) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if a.showConnSelector {
		return a.handleConnSelectorKey(msg)
	}
	if a.showDetail {
		if a.detailItem == nil {
			a.showDetail = false
			a.detailScroll = 0
			return a, nil
		}
		sqlRows := a.height - 4
		total := len(a.getDetailLines())
		switch msg.String() {
		case "up", "k":
			if a.detailScroll > 0 {
				a.detailScroll--
			}
			return a, nil
		case "down", "j":
			if total > sqlRows {
				a.detailScroll = a.clampScroll(a.detailScroll + 1)
			}
			return a, nil
		}
		a.showDetail = false
		a.detailItem = nil
		a.detailScroll = 0
		return a, nil
	}
	if a.showHelp {
		a.showHelp = false
		return a, nil
	}

	switch msg.String() {
	case "q", "ctrl+c":
		a.quitting = true
		a.cancel()
		return a, tea.Quit
	case "?":
		a.showHelp = true
	case "c":
		if len(a.connList) > 1 {
			a.showConnSelector = true
			for i, p := range a.connList {
				if p.Name == a.currentConn {
					a.connCursor = i
					break
				}
			}
		}
	case "o":
		if a.screen == ScreenOverview {
			a.screen = ScreenDashboard
		} else {
			a.screen = ScreenOverview
		}
	case "tab":
		a.section = a.section.next()
		a.statusMsg = ""
	case "shift+tab":
		a.section = a.section.prev()
		a.statusMsg = ""
	case "up", "k":
		a.moveCursor(-1)
	case "down", "j":
		a.moveCursor(1)
	case "+":
		a.poller.SetInterval(a.poller.Interval() + 500*time.Millisecond)
		a.statusMsg = fmt.Sprintf("interval: %.1fs", a.poller.Interval().Seconds())
	case "-":
		a.poller.SetInterval(a.poller.Interval() - 500*time.Millisecond)
		a.statusMsg = fmt.Sprintf("interval: %.1fs", a.poller.Interval().Seconds())
	case "enter":
		if a.screen == ScreenDashboard {
			if act := a.selectedActivity(); act != nil {
				a.showDetail = true
				a.detailItem = act
				a.detailScroll = 0
				a.detailLines = nil
			}
		}
	case "s":
		if !a.snapshot.CapturedAt.IsZero() {
			snap := a.snapshot
			conn := a.currentConn
			return a, func() tea.Msg {
				return writeSnapshot(snap, conn)
			}
		}
	}
	return a, nil
}

func (a *App) handleConnSelectorKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "up", "k":
		if a.connCursor > 0 {
			a.connCursor--
		}
	case "down", "j":
		if a.connCursor < len(a.connList)-1 {
			a.connCursor++
		}
	case "enter":
		selected := a.connList[a.connCursor]
		if selected.Name == a.currentConn {
			a.showConnSelector = false
			return a, nil
		}
		return a, a.doReconnect(selected)
	case "esc", "c", "q":
		a.showConnSelector = false
	}
	return a, nil
}

func (a *App) doReconnect(preset ConnectionPreset) tea.Cmd {
	fn := a.reconnectFn
	oldCancel := a.cancel
	oldDone := a.pollerDone
	return func() tea.Msg {
		// Stop the old poller and wait for its goroutine to exit before
		// calling reconnectFn, which will close the old DB client. This
		// prevents a data race where the old poller is still mid-query
		// when the client is closed.
		oldCancel()
		<-oldDone
		ctx := context.Background()
		p, err := fn(ctx, preset.DSN)
		if err != nil {
			return reconnectErrMsg{err: err, name: preset.Name, dsn: preset.DSN}
		}
		return connectionSwitchedMsg{poller: p, name: preset.Name}
	}
}

func (a *App) currentPreset() *ConnectionPreset {
	for i := range a.connList {
		if a.connList[i].Name == a.currentConn {
			return &a.connList[i]
		}
	}
	return nil
}

func (a *App) beginAutoReconnect(dsn, name string) tea.Cmd {
	a.autoReconnectGen++
	gen := a.autoReconnectGen
	deadline := time.Now().Add(a.reconnectMaxDuration)
	fn := a.reconnectFn
	oldCancel := a.cancel
	oldDone := a.pollerDone
	return func() tea.Msg {
		oldCancel()
		<-oldDone
		p, err := fn(context.Background(), dsn)
		if err != nil {
			return autoReconnectFailedMsg{gen: gen, delay: time.Second, deadline: deadline, dsn: dsn, name: name, attempt: 1}
		}
		return connectionSwitchedMsg{poller: p, name: name, autoGen: gen}
	}
}

func backoffNext(d time.Duration) time.Duration {
	next := d * 2
	if next > 30*time.Second {
		return 30 * time.Second
	}
	return next
}

func (a *App) selectedActivity() *core.Activity {
	switch a.section {
	case SectionActivity:
		if i := a.cursor[SectionActivity]; i < len(a.snapshot.Activities) {
			return &a.snapshot.Activities[i]
		}
	case SectionIdle:
		if i := a.cursor[SectionIdle]; i < len(a.snapshot.IdleInTx) {
			return &a.snapshot.IdleInTx[i]
		}
	}
	return nil
}

func (a *App) moveCursor(delta int) {
	n := a.sectionLen()
	if n == 0 {
		return
	}
	c := a.cursor[a.section] + delta
	if c < 0 {
		c = 0
	} else if c >= n {
		c = n - 1
	}
	a.cursor[a.section] = c
}

func (a *App) sectionLen() int {
	switch a.section {
	case SectionActivity:
		return len(a.snapshot.Activities)
	case SectionLocks:
		return len(a.snapshot.Locks)
	case SectionIdle:
		return len(a.snapshot.IdleInTx)
	}
	return 0
}

// sectionDataRows returns the number of data rows each section can display.
func (a *App) sectionDataRows() int {
	// fixed rows: titleBar(1) + statsBar(1) + 4×divider(4) + statusBar(1) + footer(1) = 8
	// per section overhead: sectionTitle(1) + colHeader(1) = 2 → 3 sections = 6
	rows := (a.height - 8 - 6) / 3
	if rows < 3 {
		return 3
	}
	return rows
}

func (a *App) View() string {
	if a.quitting {
		return ""
	}
	if a.width == 0 {
		return "loading…"
	}
	if a.width < minWidth || a.height < minHeight {
		msg := fmt.Sprintf("Terminal too small (%d×%d).\nResize to at least %d×%d.", a.width, a.height, minWidth, minHeight)
		return lipgloss.Place(a.width, a.height, lipgloss.Center, lipgloss.Center,
			warnStyle.Render(msg))
	}

	if a.showConnSelector {
		return a.renderConnSelector()
	}
	if a.showDetail && a.detailItem != nil {
		return a.renderDetail()
	}
	if a.showHelp {
		return a.renderHelp()
	}

	if a.screen == ScreenOverview {
		return a.renderOverview()
	}

	div := dimStyle.Render(strings.Repeat("─", a.width))
	dr := a.sectionDataRows()

	parts := []string{
		renderTitleBar(a.snapshot, a.poller.Interval(), a.currentConn, a.width),
		renderStatsBar(a.snapshot.DBStats, a.width),
		div,
		renderActivitySection(a.snapshot.Activities, a.cursor[SectionActivity], a.section == SectionActivity, dr, a.width, a.poller.LongRunningThreshold),
		div,
		renderLocksSection(a.snapshot.Locks, a.cursor[SectionLocks], a.section == SectionLocks, dr, a.width),
		div,
		renderIdleSection(a.snapshot.IdleInTx, a.cursor[SectionIdle], a.section == SectionIdle, dr, a.width, a.poller.IdleInTxThreshold),
		div,
		renderStatus(a.lastErr, a.statusMsg),
		renderFooter(len(a.connList) > 1),
	}

	return strings.Join(parts, "\n")
}

func (a *App) getDetailLines() []string {
	if a.detailItem == nil {
		return nil
	}
	if a.detailLines == nil || a.detailLinesItem != a.detailItem || a.detailLinesWidth != a.width {
		a.detailLines = strings.Split(formatSQL(a.detailItem.Query, a.width), "\n")
		a.detailLinesItem = a.detailItem
		a.detailLinesWidth = a.width
	}
	return a.detailLines
}

func (a *App) clampScroll(offset int) int {
	if a.detailItem == nil {
		return 0
	}
	sqlRows := a.height - 4
	total := len(a.getDetailLines())
	maxScroll := total - sqlRows
	if maxScroll <= 0 {
		return 0
	}
	if offset > maxScroll {
		return maxScroll
	}
	if offset < 0 {
		return 0
	}
	return offset
}

func (a *App) renderHelp() string {
	connLine := ""
	if len(a.connList) > 1 {
		connLine = "  c             connection selector\n"
	}
	content := boldStyle.Render("pgincident v"+version.Version) + "\n\n" +
		"  q / Ctrl-C    quit\n" +
		"  o             overview / dashboard toggle\n" +
		"  Tab           next section\n" +
		"  Shift-Tab     previous section\n" +
		"  ↑ / k         cursor up\n" +
		"  ↓ / j         cursor down\n" +
		"  Enter         query detail overlay (dashboard only)\n" +
		"  + / -         increase / decrease interval\n" +
		"  s             export snapshot to Markdown\n" +
		connLine +
		"  ?             this help\n\n" +
		dimStyle.Render("press any key to close")
	return lipgloss.Place(a.width, a.height, lipgloss.Center, lipgloss.Center,
		modalStyle.Render(content))
}

func (a *App) renderDetail() string {
	act := a.detailItem

	sep := dimStyle.Render(strings.Repeat("─", a.width))

	// title + sep + sep + footer = 4 fixed rows; SQL fills the rest.
	sqlRows := a.height - 4

	rawLines := a.getDetailLines()
	sqlLines := make([]string, len(rawLines))
	for i, line := range rawLines {
		sqlLines[i] = highlightSQL(line)
	}
	total := len(sqlLines)

	// Compute the visible window without mutating detailScroll.
	start := a.detailScroll
	if start > total {
		start = total
	}
	end := start + sqlRows
	if end > total {
		end = total
	}
	visible := append([]string{}, sqlLines[start:end]...)
	for len(visible) < sqlRows {
		visible = append(visible, "")
	}

	var footer string
	if total > sqlRows {
		footer = footerStyle.Render("[↑/↓/k/j] scroll · [any other key] close")
	} else {
		footer = footerStyle.Render("[any key] close")
	}

	parts := []string{
		boldStyle.Render(fmt.Sprintf("Query Detail — PID %d", act.PID)),
		sep,
		strings.Join(visible, "\n"),
		sep,
		footer,
	}
	return strings.Join(parts, "\n")
}

func (a *App) renderConnSelector() string {
	var lines []string
	lines = append(lines, boldStyle.Render("Select Connection"))
	lines = append(lines, "")
	for i, p := range a.connList {
		marker := "  "
		line := p.Name
		if p.Name == a.currentConn {
			line += dimStyle.Render(" (current)")
		}
		if i == a.connCursor {
			marker = "▶ "
			line = activeTitleStyle.Render(line)
		}
		lines = append(lines, marker+line)
	}
	lines = append(lines, "")
	lines = append(lines, dimStyle.Render("[↑↓/jk] move  [Enter] connect  [Esc/c/q] cancel"))

	content := strings.Join(lines, "\n")
	return lipgloss.Place(a.width, a.height, lipgloss.Center, lipgloss.Center,
		modalStyle.Render(content))
}

var userHomeDir = os.UserHomeDir

func writeSnapshot(s core.Snapshot, conn string) snapshotExportMsg {
	r := snapshot.Report{Snapshot: s, ConnName: conn}
	content := snapshot.Generate(r)
	dir, err := userHomeDir()
	if err != nil {
		return snapshotExportMsg{err: err}
	}
	path, err := snapshot.WriteFile(dir, content, s.CapturedAt)
	if err != nil {
		return snapshotExportMsg{err: err}
	}
	return snapshotExportMsg{path: path}
}
