package tui

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/shinagawa-web/pgincident/internal/core"
)

func newTestApp() *App {
	_, cancel := context.WithCancel(context.Background())
	return &App{
		poller: core.NewPoller(nil, time.Second),
		pollCh: make(chan core.PollResult, 1),
		cancel: cancel,
		width:  100,
		height: 30,
	}
}

// --- waitForSnapshot ---

func TestWaitForSnapshot(t *testing.T) {
	ch := make(chan core.PollResult, 1)
	ch <- core.PollResult{Snapshot: core.Snapshot{PGVersion: "16.1"}}
	msg := waitForSnapshot(ch)()
	r, ok := msg.(snapshotMsg)
	if !ok {
		t.Fatal("expected snapshotMsg")
	}
	if r.Snapshot.PGVersion != "16.1" {
		t.Errorf("PGVersion = %q, want 16.1", r.Snapshot.PGVersion)
	}
}

func TestWaitForSnapshotError(t *testing.T) {
	ch := make(chan core.PollResult, 1)
	ch <- core.PollResult{Err: errors.New("db down")}
	msg := waitForSnapshot(ch)()
	r, ok := msg.(snapshotMsg)
	if !ok {
		t.Fatal("expected snapshotMsg")
	}
	if r.Err == nil {
		t.Error("expected error in snapshotMsg")
	}
}

// --- sectionLen ---

func TestSectionLen(t *testing.T) {
	app := newTestApp()
	app.snapshot = core.Snapshot{
		Activities: []core.Activity{{}, {}},
		Locks:      []core.Lock{{}},
		IdleInTx:   []core.Activity{{}, {}, {}},
	}
	app.section = SectionActivity
	if app.sectionLen() != 2 {
		t.Errorf("Activity sectionLen = %d, want 2", app.sectionLen())
	}
	app.section = SectionLocks
	if app.sectionLen() != 1 {
		t.Errorf("Locks sectionLen = %d, want 1", app.sectionLen())
	}
	app.section = SectionIdle
	if app.sectionLen() != 3 {
		t.Errorf("Idle sectionLen = %d, want 3", app.sectionLen())
	}
}

func TestSectionLenEmpty(t *testing.T) {
	app := newTestApp()
	app.section = SectionActivity
	if app.sectionLen() != 0 {
		t.Errorf("empty sectionLen = %d, want 0", app.sectionLen())
	}
}

// --- sectionDataRows ---

func TestSectionDataRows(t *testing.T) {
	app := newTestApp()
	app.height = 30
	rows := app.sectionDataRows()
	if rows < 3 {
		t.Errorf("sectionDataRows = %d, want >= 3", rows)
	}
}

func TestSectionDataRowsMinimum(t *testing.T) {
	app := newTestApp()
	app.height = 10 // very small
	rows := app.sectionDataRows()
	if rows != 3 {
		t.Errorf("sectionDataRows with small height = %d, want 3 (minimum)", rows)
	}
}

// --- moveCursor ---

func TestMoveCursorDown(t *testing.T) {
	app := newTestApp()
	app.snapshot.Activities = []core.Activity{{}, {}, {}}
	app.section = SectionActivity
	app.moveCursor(1)
	if app.cursor[SectionActivity] != 1 {
		t.Errorf("cursor = %d, want 1", app.cursor[SectionActivity])
	}
}

func TestMoveCursorBoundLower(t *testing.T) {
	app := newTestApp()
	app.snapshot.Activities = []core.Activity{{}, {}}
	app.section = SectionActivity
	app.moveCursor(-1)
	if app.cursor[SectionActivity] != 0 {
		t.Errorf("cursor should not go below 0, got %d", app.cursor[SectionActivity])
	}
}

func TestMoveCursorBoundUpper(t *testing.T) {
	app := newTestApp()
	app.snapshot.Activities = []core.Activity{{}, {}}
	app.section = SectionActivity
	app.moveCursor(100)
	if app.cursor[SectionActivity] != 1 {
		t.Errorf("cursor should not exceed len-1, got %d", app.cursor[SectionActivity])
	}
}

func TestMoveCursorEmpty(t *testing.T) {
	app := newTestApp()
	app.section = SectionActivity
	app.moveCursor(1) // should not panic
}

// --- View ---

func TestViewQuitting(t *testing.T) {
	app := newTestApp()
	app.quitting = true
	if app.View() != "" {
		t.Error("expected empty string when quitting")
	}
}

func TestViewLoading(t *testing.T) {
	app := newTestApp()
	app.width = 0
	if app.View() != "loading…" {
		t.Errorf("expected loading message, got: %q", app.View())
	}
}

func TestViewTooSmall(t *testing.T) {
	app := newTestApp()
	app.width = 79
	app.height = 24
	v := app.View()
	if !strings.Contains(v, "too small") {
		t.Errorf("expected too small message, got: %q", v)
	}
}

func TestViewNormal(t *testing.T) {
	app := newTestApp()
	v := app.View()
	if !strings.Contains(v, "pgincident") {
		t.Errorf("expected pgincident in view, got: %q", v)
	}
	if !strings.Contains(v, "Long-running queries") {
		t.Errorf("expected activity section in view, got: %q", v)
	}
}

func TestViewHelp(t *testing.T) {
	app := newTestApp()
	app.showHelp = true
	v := app.View()
	if !strings.Contains(v, "quit") {
		t.Errorf("expected help content, got: %q", v)
	}
}

// --- Update ---

func TestUpdateWindowSize(t *testing.T) {
	app := newTestApp()
	model, _ := app.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	a := model.(*App)
	if a.width != 120 || a.height != 40 {
		t.Errorf("size = %dx%d, want 120x40", a.width, a.height)
	}
}

func TestUpdateSnapshot(t *testing.T) {
	app := newTestApp()
	snap := core.Snapshot{PGVersion: "16.1"}
	model, cmd := app.Update(snapshotMsg{Snapshot: snap})
	a := model.(*App)
	if a.snapshot.PGVersion != "16.1" {
		t.Errorf("PGVersion = %q, want 16.1", a.snapshot.PGVersion)
	}
	if cmd == nil {
		t.Error("expected next waitForSnapshot cmd")
	}
}

func TestUpdateSnapshotError(t *testing.T) {
	app := newTestApp()
	model, _ := app.Update(snapshotMsg{Err: errors.New("db down")})
	a := model.(*App)
	if a.lastErr == nil {
		t.Error("expected lastErr to be set")
	}
}

func TestUpdateUnknownMsg(t *testing.T) {
	app := newTestApp()
	model, cmd := app.Update("unknown")
	if model != app {
		t.Error("expected same model for unknown message")
	}
	if cmd != nil {
		t.Error("expected nil cmd for unknown message")
	}
}

func TestUpdateKeyMsg(t *testing.T) {
	app := newTestApp()
	model, _ := app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("?")})
	a := model.(*App)
	if !a.showHelp {
		t.Error("expected showHelp=true after ? key via Update")
	}
}

func TestSectionLenDefault(t *testing.T) {
	app := newTestApp()
	app.section = sectionCount // invalid section → default branch
	if app.sectionLen() != 0 {
		t.Errorf("sectionLen for invalid section = %d, want 0", app.sectionLen())
	}
}

// --- handleKey ---

func key(s string) tea.KeyMsg {
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
}

func TestHandleKeyQuit(t *testing.T) {
	app := newTestApp()
	model, cmd := app.handleKey(key("q"))
	a := model.(*App)
	if !a.quitting {
		t.Error("expected quitting after q")
	}
	if cmd == nil {
		t.Error("expected tea.Quit cmd")
	}
}

func TestHandleKeyCtrlC(t *testing.T) {
	app := newTestApp()
	model, _ := app.handleKey(tea.KeyMsg{Type: tea.KeyCtrlC})
	a := model.(*App)
	if !a.quitting {
		t.Error("expected quitting after ctrl+c")
	}
}

func TestHandleKeyHelp(t *testing.T) {
	app := newTestApp()
	model, _ := app.handleKey(key("?"))
	a := model.(*App)
	if !a.showHelp {
		t.Error("expected showHelp after ?")
	}
}

func TestHandleKeyHelpClose(t *testing.T) {
	app := newTestApp()
	app.showHelp = true
	model, _ := app.handleKey(key("x"))
	a := model.(*App)
	if a.showHelp {
		t.Error("expected showHelp=false after any key when help is open")
	}
}

func TestHandleKeyTab(t *testing.T) {
	app := newTestApp()
	app.section = SectionActivity
	model, _ := app.handleKey(tea.KeyMsg{Type: tea.KeyTab})
	a := model.(*App)
	if a.section != SectionLocks {
		t.Errorf("section = %v, want SectionLocks after Tab", a.section)
	}
}

func TestHandleKeyShiftTab(t *testing.T) {
	app := newTestApp()
	app.section = SectionLocks
	model, _ := app.handleKey(tea.KeyMsg{Type: tea.KeyShiftTab})
	a := model.(*App)
	if a.section != SectionActivity {
		t.Errorf("section = %v, want SectionActivity after Shift+Tab", a.section)
	}
}

func TestHandleKeyUp(t *testing.T) {
	app := newTestApp()
	app.snapshot.Activities = []core.Activity{{}, {}}
	app.section = SectionActivity
	app.cursor[SectionActivity] = 1
	model, _ := app.handleKey(tea.KeyMsg{Type: tea.KeyUp})
	a := model.(*App)
	if a.cursor[SectionActivity] != 0 {
		t.Errorf("cursor = %d, want 0 after Up", a.cursor[SectionActivity])
	}
}

func TestHandleKeyDown(t *testing.T) {
	app := newTestApp()
	app.snapshot.Activities = []core.Activity{{}, {}}
	app.section = SectionActivity
	model, _ := app.handleKey(key("j"))
	a := model.(*App)
	if a.cursor[SectionActivity] != 1 {
		t.Errorf("cursor = %d, want 1 after j", a.cursor[SectionActivity])
	}
}

func TestHandleKeyK(t *testing.T) {
	app := newTestApp()
	app.snapshot.Activities = []core.Activity{{}, {}}
	app.section = SectionActivity
	app.cursor[SectionActivity] = 1
	model, _ := app.handleKey(key("k"))
	a := model.(*App)
	if a.cursor[SectionActivity] != 0 {
		t.Errorf("cursor = %d, want 0 after k", a.cursor[SectionActivity])
	}
}

func TestHandleKeyIntervalIncrease(t *testing.T) {
	app := newTestApp()
	app.poller = core.NewPoller(nil, time.Second)
	before := app.poller.Interval()
	model, _ := app.handleKey(key("+"))
	a := model.(*App)
	if a.poller.Interval() <= before {
		t.Errorf("interval should increase after +, got %v", a.poller.Interval())
	}
	if a.statusMsg == "" {
		t.Error("expected statusMsg after interval change")
	}
}

func TestHandleKeyIntervalDecrease(t *testing.T) {
	app := newTestApp()
	app.poller = core.NewPoller(nil, 3*time.Second)
	before := app.poller.Interval()
	model, _ := app.handleKey(key("-"))
	a := model.(*App)
	if a.poller.Interval() >= before {
		t.Errorf("interval should decrease after -, got %v", a.poller.Interval())
	}
}

func TestHandleKeyIntervalDecreaseClamp(t *testing.T) {
	app := newTestApp()
	app.poller = core.NewPoller(nil, 1*time.Second) // already at minimum
	model, _ := app.handleKey(key("-"))
	a := model.(*App)
	if a.poller.Interval() != 1*time.Second {
		t.Errorf("interval should stay at 1s when already at minimum, got %v", a.poller.Interval())
	}
}

// --- selectedActivity ---

func TestSelectedActivityActivity(t *testing.T) {
	app := newTestApp()
	app.snapshot.Activities = []core.Activity{{PID: 42, Query: "SELECT 1"}}
	app.section = SectionActivity
	got := app.selectedActivity()
	if got == nil || got.PID != 42 {
		t.Errorf("selectedActivity = %v, want PID 42", got)
	}
}

func TestSelectedActivityIdle(t *testing.T) {
	app := newTestApp()
	app.snapshot.IdleInTx = []core.Activity{{PID: 99, Query: "SELECT 2"}}
	app.section = SectionIdle
	got := app.selectedActivity()
	if got == nil || got.PID != 99 {
		t.Errorf("selectedActivity = %v, want PID 99", got)
	}
}

func TestSelectedActivityLocks(t *testing.T) {
	app := newTestApp()
	app.snapshot.Locks = []core.Lock{{BlockedPID: 1, BlockingPID: 2}}
	app.section = SectionLocks
	if app.selectedActivity() != nil {
		t.Error("selectedActivity should return nil for Locks section")
	}
}

func TestSelectedActivityEmpty(t *testing.T) {
	app := newTestApp()
	app.section = SectionActivity
	if app.selectedActivity() != nil {
		t.Error("selectedActivity should return nil when slice is empty")
	}
}

// --- Enter key / detail overlay ---

func TestHandleKeyEnterOpensDetail(t *testing.T) {
	app := newTestApp()
	app.snapshot.Activities = []core.Activity{{PID: 1001, Query: "SELECT count(*) FROM orders"}}
	app.section = SectionActivity
	model, _ := app.handleKey(tea.KeyMsg{Type: tea.KeyEnter})
	a := model.(*App)
	if !a.showDetail {
		t.Error("expected showDetail=true after Enter on Activity row")
	}
	if a.detailItem == nil || a.detailItem.PID != 1001 {
		t.Errorf("detailItem PID = %v, want 1001", a.detailItem)
	}
}

func TestHandleKeyEnterIdleOpensDetail(t *testing.T) {
	app := newTestApp()
	app.snapshot.IdleInTx = []core.Activity{{PID: 2002, Query: "UPDATE jobs SET status='done'"}}
	app.section = SectionIdle
	model, _ := app.handleKey(tea.KeyMsg{Type: tea.KeyEnter})
	a := model.(*App)
	if !a.showDetail {
		t.Error("expected showDetail=true after Enter on Idle row")
	}
	if a.detailItem == nil || a.detailItem.PID != 2002 {
		t.Errorf("detailItem PID = %v, want 2002", a.detailItem)
	}
}

func TestHandleKeyEnterLocksNoOp(t *testing.T) {
	app := newTestApp()
	app.snapshot.Locks = []core.Lock{{BlockedPID: 1, BlockingPID: 2}}
	app.section = SectionLocks
	model, _ := app.handleKey(tea.KeyMsg{Type: tea.KeyEnter})
	a := model.(*App)
	if a.showDetail {
		t.Error("expected showDetail=false after Enter on Locks row")
	}
}

func TestHandleKeyEnterEmptyNoOp(t *testing.T) {
	app := newTestApp()
	app.section = SectionActivity
	model, _ := app.handleKey(tea.KeyMsg{Type: tea.KeyEnter})
	a := model.(*App)
	if a.showDetail {
		t.Error("expected showDetail=false after Enter on empty section")
	}
}

func TestDetailCloseOnAnyKey(t *testing.T) {
	app := newTestApp()
	act := core.Activity{PID: 1001, Query: "SELECT 1"}
	app.showDetail = true
	app.detailItem = &act
	model, _ := app.handleKey(key("x"))
	a := model.(*App)
	if a.showDetail {
		t.Error("expected showDetail=false after any key when detail is open")
	}
	if a.detailItem != nil {
		t.Error("expected detailItem=nil after closing detail")
	}
}

func TestViewDetail(t *testing.T) {
	app := newTestApp()
	act := core.Activity{PID: 1001, User: "alice", Query: "SELECT count(*) FROM orders", State: "active"}
	app.showDetail = true
	app.detailItem = &act
	v := app.View()
	if !strings.Contains(v, "1001") {
		t.Errorf("expected PID 1001 in detail view, got: %q", v)
	}
	if !strings.Contains(v, "FROM orders") {
		t.Errorf("expected full SQL in detail view, got: %q", v)
	}
	if !strings.Contains(v, "[any key] close") {
		t.Errorf("expected dismiss hint in detail view, got: %q", v)
	}
}

func TestViewDetailWideTerminal(t *testing.T) {
	app := newTestApp()
	app.width = 150
	act := core.Activity{PID: 2001, User: "bob", Query: "SELECT 1", State: "active"}
	app.showDetail = true
	app.detailItem = &act
	v := app.View()
	if !strings.Contains(v, "2001") {
		t.Errorf("expected PID 2001 in detail view on wide terminal, got: %q", v)
	}
}

func TestViewDetailSQLFillsHeight(t *testing.T) {
	app := newTestApp()
	app.height = 24
	longSQL := strings.Repeat("SELECT id FROM orders WHERE status = 'pending' AND ", 5) + "TRUE"
	act := core.Activity{PID: 3001, Query: longSQL, State: "active"}
	app.showDetail = true
	app.detailItem = &act
	lines := strings.Split(app.View(), "\n")
	if len(lines) != 24 {
		t.Errorf("detail view should have exactly 24 lines, got %d", len(lines))
	}
}

// --- New ---

type mockQuerier struct{}

func (m *mockQuerier) ServerInfo(_ context.Context) (string, string, error) {
	return "16.1", "localhost:5432", nil
}
func (m *mockQuerier) LongRunning(_ context.Context, _ time.Duration) ([]core.Activity, error) {
	return nil, nil
}
func (m *mockQuerier) Locks(_ context.Context) ([]core.Lock, error)  { return nil, nil }
func (m *mockQuerier) IdleInTx(_ context.Context, _ time.Duration) ([]core.Activity, error) {
	return nil, nil
}
func (m *mockQuerier) Stats(_ context.Context) (core.DBStats, error) {
	return core.DBStats{}, nil
}

func TestNew(t *testing.T) {
	poller := core.NewPoller(&mockQuerier{}, time.Second)
	app := New(poller)
	if app.poller == nil {
		t.Error("expected poller to be set")
	}
	if app.pollCh == nil {
		t.Error("expected pollCh to be set")
	}
	app.cancel()
}

// --- Init ---

func TestInit(t *testing.T) {
	app := newTestApp()
	app.pollCh <- core.PollResult{Snapshot: core.Snapshot{PGVersion: "16.1"}}
	cmd := app.Init()
	if cmd == nil {
		t.Fatal("expected non-nil cmd from Init")
	}
	msg := cmd()
	if _, ok := msg.(snapshotMsg); !ok {
		t.Errorf("expected snapshotMsg from Init cmd, got %T", msg)
	}
}
