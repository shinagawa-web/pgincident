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

type signalResultMsg struct {
	action string
	pid    int
	ok     bool
	err    error
}

type confirmAction int

const (
	noConfirm confirmAction = iota
	confirmKill
	confirmCancel
)

// App is the root Bubble Tea model.
type App struct {
	client    *core.Client
	poller    *core.Poller
	pollCh    chan core.PollResult
	cancel    context.CancelFunc
	snapshot  core.Snapshot
	section   Section
	cursor    [sectionCount]int
	width     int
	height    int
	lastErr   error
	statusMsg string
	confirming confirmAction
	showHelp  bool
	canSignal bool
	quitting  bool
}

func New(client *core.Client, poller *core.Poller, canSignal bool) *App {
	ctx, cancel := context.WithCancel(context.Background())
	ch := make(chan core.PollResult, 1)
	go poller.Run(ctx, ch)
	return &App{
		client:    client,
		poller:    poller,
		pollCh:    ch,
		cancel:    cancel,
		canSignal: canSignal,
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
	case signalResultMsg:
		if msg.err != nil {
			a.statusMsg = fmt.Sprintf("error: %v", msg.err)
		} else if msg.ok {
			a.statusMsg = fmt.Sprintf("%s pid=%d: done", msg.action, msg.pid)
		} else {
			a.statusMsg = fmt.Sprintf("%s pid=%d: backend not found", msg.action, msg.pid)
		}
	case tea.KeyMsg:
		return a.handleKey(msg)
	}
	return a, nil
}

func (a *App) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if a.confirming != noConfirm {
		switch msg.String() {
		case "y", "Y":
			action, pid := a.confirming, a.selectedPID()
			a.confirming = noConfirm
			return a, a.signalCmd(action, pid)
		case "n", "N", "esc":
			a.confirming = noConfirm
		}
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
		d := a.poller.Interval() - 500*time.Millisecond
		if d < 500*time.Millisecond {
			d = 500 * time.Millisecond
		}
		a.poller.SetInterval(d)
		a.statusMsg = fmt.Sprintf("interval: %.1fs", a.poller.Interval().Seconds())
	case "K":
		if !a.canSignal {
			a.statusMsg = "pg_signal_backend required — cannot terminate backends"
		} else if a.selectedPID() > 0 {
			a.confirming = confirmKill
		}
	case "c":
		if !a.canSignal {
			a.statusMsg = "pg_signal_backend required — cannot cancel queries"
		} else if a.selectedPID() > 0 {
			a.confirming = confirmCancel
		}
	}
	return a, nil
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

func (a *App) selectedPID() int {
	switch a.section {
	case SectionActivity:
		if i := a.cursor[SectionActivity]; i < len(a.snapshot.Activities) {
			return a.snapshot.Activities[i].PID
		}
	case SectionLocks:
		if i := a.cursor[SectionLocks]; i < len(a.snapshot.Locks) {
			return a.snapshot.Locks[i].BlockedPID
		}
	case SectionIdle:
		if i := a.cursor[SectionIdle]; i < len(a.snapshot.IdleInTx) {
			return a.snapshot.IdleInTx[i].PID
		}
	}
	return 0
}

func (a *App) signalCmd(action confirmAction, pid int) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		var ok bool
		var err error
		var name string
		switch action {
		case confirmKill:
			ok, err = a.client.TerminateBackend(ctx, pid)
			name = "terminate"
		case confirmCancel:
			ok, err = a.client.CancelBackend(ctx, pid)
			name = "cancel"
		}
		return signalResultMsg{action: name, pid: pid, ok: ok, err: err}
	}
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

	if a.showHelp {
		return a.renderHelp()
	}
	if a.confirming != noConfirm {
		return a.renderConfirmModal()
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
		renderFooter(a.canSignal),
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
		"  K             terminate selected backend\n" +
		"  c             cancel selected query\n" +
		"  + / -         increase / decrease interval\n" +
		"  ?             this help\n\n" +
		dimStyle.Render("press any key to close")
	return lipgloss.Place(a.width, a.height, lipgloss.Center, lipgloss.Center,
		modalStyle.Render(content))
}

func (a *App) renderConfirmModal() string {
	pid := a.selectedPID()
	var verb, query string
	switch a.confirming {
	case confirmKill:
		verb = "Terminate backend"
	case confirmCancel:
		verb = "Cancel query for backend"
	}
	switch a.section {
	case SectionActivity:
		if i := a.cursor[SectionActivity]; i < len(a.snapshot.Activities) {
			query = truncate(a.snapshot.Activities[i].Query, 60)
		}
	case SectionLocks:
		query = "(blocked query)"
	case SectionIdle:
		if i := a.cursor[SectionIdle]; i < len(a.snapshot.IdleInTx) {
			query = truncate(a.snapshot.IdleInTx[i].Query, 60)
		}
	}
	content := fmt.Sprintf("%s pid=%d?\n\n  %s\n\n  [y]es   [n]o",
		verb, pid, dimStyle.Render(query))
	return lipgloss.Place(a.width, a.height, lipgloss.Center, lipgloss.Center,
		modalStyle.Render(content))
}
