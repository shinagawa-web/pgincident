package tui

import (
	"context"
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/shinagawa-web/pgincident/internal/core"
	"github.com/shinagawa-web/pgincident/internal/version"
)

type snapshotMsg core.PollResult

// App is the root Bubble Tea model.
type App struct {
	poller     *core.Poller
	pollCh     chan core.PollResult
	cancel     context.CancelFunc
	snapshot   core.Snapshot
	section    Section
	cursor     [sectionCount]int
	width      int
	height     int
	lastErr    error
	statusMsg  string
	showHelp   bool
	showDetail bool
	detailItem *core.Activity
	quitting   bool
}

func New(poller *core.Poller) *App {
	ctx, cancel := context.WithCancel(context.Background())
	ch := make(chan core.PollResult, 1)
	go poller.Run(ctx, ch)
	return &App{
		poller: poller,
		pollCh: ch,
		cancel: cancel,
	}
}

func (a *App) Init() tea.Cmd {
	return waitForSnapshot(a.pollCh)
}

func waitForSnapshot(ch <-chan core.PollResult) tea.Cmd {
	return func() tea.Msg { return snapshotMsg(<-ch) }
}

func (a *App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		a.width, a.height = msg.Width, msg.Height
	case snapshotMsg:
		if msg.Err != nil {
			a.lastErr = msg.Err
		} else {
			a.snapshot = msg.Snapshot
			a.lastErr = nil
		}
		return a, waitForSnapshot(a.pollCh)
	case tea.KeyMsg:
		return a.handleKey(msg)
	}
	return a, nil
}

func (a *App) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if a.showDetail {
		a.showDetail = false
		a.detailItem = nil
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
		if act := a.selectedActivity(); act != nil {
			a.showDetail = true
			a.detailItem = act
		}
	}
	return a, nil
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
	if a.width < 80 || a.height < 24 {
		msg := fmt.Sprintf("Terminal too small (%d×%d).\nResize to at least 80×24.", a.width, a.height)
		return lipgloss.Place(a.width, a.height, lipgloss.Center, lipgloss.Center,
			warnStyle.Render(msg))
	}

	if a.showDetail && a.detailItem != nil {
		return a.renderDetail()
	}
	if a.showHelp {
		return a.renderHelp()
	}

	div := dimStyle.Render(strings.Repeat("─", a.width))
	dr := a.sectionDataRows()

	parts := []string{
		renderTitleBar(a.snapshot, a.poller.Interval(), a.width),
		renderStatsBar(a.snapshot.DBStats, a.width),
		div,
		renderActivitySection(a.snapshot.Activities, a.cursor[SectionActivity], a.section == SectionActivity, dr, a.width),
		div,
		renderLocksSection(a.snapshot.Locks, a.cursor[SectionLocks], a.section == SectionLocks, dr, a.width),
		div,
		renderIdleSection(a.snapshot.IdleInTx, a.cursor[SectionIdle], a.section == SectionIdle, dr, a.width),
		div,
		renderStatus(a.lastErr, a.statusMsg),
		renderFooter(),
	}

	return strings.Join(parts, "\n")
}

func (a *App) renderHelp() string {
	content := boldStyle.Render("pgincident v"+version.Version) + "\n\n" +
		"  q / Ctrl-C    quit\n" +
		"  Tab           next section\n" +
		"  Shift-Tab     previous section\n" +
		"  ↑ / k         cursor up\n" +
		"  ↓ / j         cursor down\n" +
		"  Enter         query detail overlay\n" +
		"  + / -         increase / decrease interval\n" +
		"  ?             this help\n\n" +
		dimStyle.Render("press any key to close")
	return lipgloss.Place(a.width, a.height, lipgloss.Center, lipgloss.Center,
		modalStyle.Render(content))
}

func (a *App) renderDetail() string {
	act := a.detailItem

	// sqlWidth: inner content width for the SQL block.
	// modalStyle adds Padding(1,3)=6 and Border=2, so subtract 8 from modal outer width.
	modalWidth := a.width * 2 / 3
	if modalWidth > 80 {
		modalWidth = 80
	}
	if modalWidth < 40 {
		modalWidth = 40
	}
	sqlWidth := modalWidth - 8

	header := boldStyle.Render(fmt.Sprintf("Query Detail — PID %d", act.PID))
	meta := fmt.Sprintf("  User:      %s\n  Duration:  %s\n  State:     %s",
		act.User, formatDuration(act.Duration), act.State)
	sep := dimStyle.Render(strings.Repeat("─", sqlWidth))
	sql := wrapText(act.Query, sqlWidth)
	dismiss := dimStyle.Render("press any key to close")

	content := header + "\n\n" + meta + "\n\n" + sep + "\n" + sql + "\n" + sep + "\n\n" + dismiss
	return lipgloss.Place(a.width, a.height, lipgloss.Center, lipgloss.Center,
		modalStyle.Render(content))
}
