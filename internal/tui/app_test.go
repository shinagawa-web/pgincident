package tui

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/shinagawa-web/pgincident/internal/core"
)

func newTestApp() *App {
	_, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	close(done) // pre-closed: simulates a stopped poller goroutine
	return &App{
		poller:     core.NewPoller(nil, time.Second),
		pollCh:     make(chan core.PollResult, 1),
		cancel:     cancel,
		width:      100,
		height:     30,
		screen:     ScreenDashboard, // most tests target Dashboard behavior
		pollerDone: done,
	}
}

// --- waitForSnapshot ---

func TestWaitForSnapshot(t *testing.T) {
	ch := make(chan core.PollResult, 1)
	ch <- core.PollResult{Snapshot: core.Snapshot{PGVersion: "16.1"}}
	msg := waitForSnapshot(ch, 0)()
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
	msg := waitForSnapshot(ch, 0)()
	r, ok := msg.(snapshotMsg)
	if !ok {
		t.Fatal("expected snapshotMsg")
	}
	if r.Err == nil {
		t.Error("expected error in snapshotMsg")
	}
}

func TestWaitForSnapshotChannelClosed(t *testing.T) {
	ch := make(chan core.PollResult)
	close(ch)
	msg := waitForSnapshot(ch, 0)()
	if msg != nil {
		t.Errorf("expected nil when channel closed, got %T", msg)
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
	if !strings.Contains(v, "interval:") {
		t.Errorf("expected interval in title bar, got: %q", v)
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
	model, cmd := app.Update(snapshotMsg{PollResult: core.PollResult{Snapshot: snap}})
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
	model, _ := app.Update(snapshotMsg{PollResult: core.PollResult{Err: errors.New("db down")}})
	a := model.(*App)
	if a.lastErr == nil {
		t.Error("expected lastErr to be set")
	}
}

func TestUpdateSnapshotStaleGen(t *testing.T) {
	app := newTestApp()
	app.gen = 2
	snap := core.Snapshot{PGVersion: "stale"}
	model, cmd := app.Update(snapshotMsg{PollResult: core.PollResult{Snapshot: snap}, gen: 1})
	a := model.(*App)
	if a.snapshot.PGVersion == "stale" {
		t.Error("stale snapshotMsg should be dropped")
	}
	if cmd != nil {
		t.Error("expected nil cmd for stale snapshotMsg")
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
	app.detailScroll = 2
	model, _ := app.handleKey(key("x"))
	a := model.(*App)
	if a.showDetail {
		t.Error("expected showDetail=false after any key when detail is open")
	}
	if a.detailItem != nil {
		t.Error("expected detailItem=nil after closing detail")
	}
	if a.detailScroll != 0 {
		t.Errorf("expected detailScroll=0 after closing detail, got %d", a.detailScroll)
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

// --- screen transitions ---

func TestHandleKeyOFromOverview(t *testing.T) {
	app := newTestApp()
	app.screen = ScreenOverview
	model, _ := app.handleKey(key("o"))
	a := model.(*App)
	if a.screen != ScreenDashboard {
		t.Errorf("screen = %v, want ScreenDashboard after o from Overview", a.screen)
	}
}

func TestHandleKeyOFromDashboard(t *testing.T) {
	app := newTestApp()
	app.screen = ScreenDashboard
	model, _ := app.handleKey(key("o"))
	a := model.(*App)
	if a.screen != ScreenOverview {
		t.Errorf("screen = %v, want ScreenOverview after o from Dashboard", a.screen)
	}
}

func TestHandleKeyEnterNoOpInOverview(t *testing.T) {
	app := newTestApp()
	app.screen = ScreenOverview
	app.snapshot.Activities = []core.Activity{{PID: 1, Query: "SELECT 1"}}
	app.section = SectionActivity
	model, _ := app.handleKey(tea.KeyMsg{Type: tea.KeyEnter})
	a := model.(*App)
	if a.showDetail {
		t.Error("Enter in Overview should not open detail overlay")
	}
}

func TestViewOverview(t *testing.T) {
	app := newTestApp()
	app.screen = ScreenOverview
	v := app.View()
	if !strings.Contains(v, "DB Health Overview") {
		t.Errorf("expected overview heading in view, got: %q", v)
	}
	if !strings.Contains(v, "Connections") {
		t.Errorf("expected Connections metric in overview, got: %q", v)
	}
	if !strings.Contains(v, "[o]dashboard") {
		t.Errorf("expected overview footer hint, got: %q", v)
	}
	if strings.Contains(v, "[c]connections") {
		t.Errorf("unexpected [c]connections in overview footer for single conn, got: %q", v)
	}
}

func TestViewOverviewFooterMultiConn(t *testing.T) {
	app := newMultiConnApp()
	app.screen = ScreenOverview
	v := app.View()
	if !strings.Contains(v, "[c]connections") {
		t.Errorf("expected [c]connections in overview footer for multi-conn, got: %q", v)
	}
}

func TestViewOverviewNoReplicationRowWhenNoStandbys(t *testing.T) {
	app := newTestApp()
	app.screen = ScreenOverview
	app.snapshot.DBStats.HasStandbys = false
	v := app.View()
	if strings.Contains(v, "Replication lag") {
		t.Error("expected Replication lag row to be hidden when no standbys")
	}
}

func TestViewOverviewReplicationRowVisibleWhenStandbysExist(t *testing.T) {
	app := newTestApp()
	app.screen = ScreenOverview
	app.snapshot.DBStats.HasStandbys = true
	app.snapshot.DBStats.ReplicationLagSecs = 0 // caught up but standbys exist
	v := app.View()
	if !strings.Contains(v, "Replication lag") {
		t.Error("expected Replication lag row to be visible when standbys exist even if lag=0")
	}
}

func TestViewOverviewTPSNonZero(t *testing.T) {
	app := newTestApp()
	app.screen = ScreenOverview
	app.snapshot.DBStats.TPS = 1234
	v := app.View()
	if !strings.Contains(v, "1234") {
		t.Errorf("expected TPS value in overview, got: %q", v)
	}
}

func TestViewOverviewStatusBadges(t *testing.T) {
	app := newTestApp()
	app.screen = ScreenOverview
	// connections > 90% → CRIT; cache hit < 95% → CRIT; autovacuum > 5 → CRIT
	app.snapshot.DBStats.ConnectionsActive = 95
	app.snapshot.DBStats.ConnectionsMax = 100
	app.snapshot.DBStats.CacheHitRatio = 0.94
	app.snapshot.DBStats.AutovacuumWorkers = 6
	v := app.View()
	if !strings.Contains(v, "CRIT") {
		t.Errorf("expected CRIT badge in overview for critical metrics, got: %q", v)
	}
	// checkpoint req > 0 → WARN
	app.snapshot.DBStats.ConnectionsActive = 10
	app.snapshot.DBStats.CacheHitRatio = 0.999
	app.snapshot.DBStats.AutovacuumWorkers = 0
	app.snapshot.DBStats.CheckpointReq = 5
	v = app.View()
	if !strings.Contains(v, "WARN") {
		t.Errorf("expected WARN badge in overview for warning metrics, got: %q", v)
	}
}

func TestNewDefaultsToOverview(t *testing.T) {
	poller := core.NewPoller(&mockQuerier{}, time.Second)
	app := New(poller, "default", []ConnectionPreset{{Name: "default", DSN: "postgres://x"}}, nil)
	if app.screen != ScreenOverview {
		t.Errorf("New() screen = %v, want ScreenOverview", app.screen)
	}
	app.cancel()
}

// --- detail scroll ---

// overflowQuery is long enough that at width=100, height=24 (sqlRows=20) the overlay overflows.
const overflowQuery = "WITH paused AS (SELECT pg_sleep(60)) SELECT a.pid, a.usename, a.application_name, a.client_addr, a.client_hostname, a.client_port, a.backend_start, a.xact_start, a.query_start, a.state_change, a.wait_event_type, a.wait_event, a.state, a.backend_xid, a.backend_xmin, a.query, a.backend_type, a.leader_pid, l.locktype, l.database, l.relation::regclass AS locked_table, l.page, l.tuple, l.virtualxid, l.transactionid, l.classid, l.objid, l.objsubid, l.virtualtransaction, l.pid AS lock_pid, l.mode, l.granted, l.fastpath, s.seq_scan, s.seq_tup_read, s.idx_scan, s.idx_tup_fetch, s.n_tup_ins, s.n_tup_upd, s.n_tup_del, s.n_tup_hot_upd, s.n_live_tup, s.n_dead_tup, s.n_mod_since_analyze, s.last_vacuum, s.last_autovacuum, s.last_analyze, s.last_autoanalyze, ui.indexrelname, ui.idx_scan AS idx_idx_scan, ui.idx_tup_read, ui.idx_tup_fetch AS idx_tup_fetch2, bg.checkpoints_timed, bg.checkpoints_req, bg.checkpoint_write_time, bg.checkpoint_sync_time, bg.buffers_checkpoint, bg.buffers_clean, bg.maxwritten_clean, bg.buffers_backend, bg.buffers_backend_fsync, bg.buffers_alloc, bg.stats_reset AS bg_stats_reset, r.usesysid AS repl_usesysid, r.usename AS repl_usename, r.application_name AS repl_app_name, r.client_addr AS repl_client_addr, r.state AS repl_state, r.sent_lsn, r.write_lsn, r.flush_lsn, r.replay_lsn, r.write_lag, r.flush_lag, r.replay_lag, r.sync_priority, r.sync_state, ssl.ssl, ssl.version AS ssl_version, ssl.cipher AS ssl_cipher, ssl.bits AS ssl_bits, now() - a.query_start AS query_duration, now() - a.xact_start AS xact_duration, now() - a.state_change AS time_in_state, now() - a.backend_start AS connection_age FROM paused, pg_stat_activity a JOIN pg_locks l ON l.pid = a.pid AND l.granted = false JOIN pg_stat_user_tables s ON s.relid = l.relation JOIN pg_stat_bgwriter bg ON TRUE LEFT JOIN pg_stat_replication r ON r.pid = a.pid LEFT JOIN pg_stat_ssl ssl ON ssl.pid = a.pid LEFT JOIN pg_stat_user_indexes ui ON ui.relid = s.relid LEFT JOIN pg_stat_wal_receiver wr ON TRUE LEFT JOIN pg_stat_archiver arch ON TRUE WHERE a.state != 'idle' AND a.pid != pg_backend_pid() ORDER BY query_duration DESC LIMIT 50"

func detailApp() *App {
	app := newTestApp() // width=100
	app.height = 24     // sqlRows = 20; overflowQuery overflows at this height
	act := core.Activity{PID: 5000, Query: overflowQuery}
	app.showDetail = true
	app.detailItem = &act
	return app
}

func TestDetailScrollDown(t *testing.T) {
	app := detailApp()
	model, _ := app.handleKey(key("j"))
	a := model.(*App)
	if a.detailScroll != 1 {
		t.Errorf("detailScroll = %d, want 1 after j", a.detailScroll)
	}
}

func TestDetailScrollDownViaArrow(t *testing.T) {
	app := detailApp()
	model, _ := app.handleKey(tea.KeyMsg{Type: tea.KeyDown})
	a := model.(*App)
	if a.detailScroll != 1 {
		t.Errorf("detailScroll = %d, want 1 after Down", a.detailScroll)
	}
}

func TestDetailScrollBoundUpper(t *testing.T) {
	app := detailApp()
	lines := strings.Split(formatSQL(overflowQuery, app.width), "\n")
	sqlRows := app.height - 4
	maxScroll := len(lines) - sqlRows
	app.detailScroll = maxScroll - 1
	model, _ := app.handleKey(key("j"))
	model, _ = model.(*App).handleKey(key("j"))
	a := model.(*App)
	if a.detailScroll != maxScroll {
		t.Errorf("detailScroll = %d, want %d (clamped at max)", a.detailScroll, maxScroll)
	}
}

func TestDetailScrollUp(t *testing.T) {
	app := detailApp()
	app.detailScroll = 5
	model, _ := app.handleKey(key("k"))
	a := model.(*App)
	if a.detailScroll != 4 {
		t.Errorf("detailScroll = %d, want 4 after k", a.detailScroll)
	}
}

func TestDetailScrollUpViaArrow(t *testing.T) {
	app := detailApp()
	app.detailScroll = 5
	model, _ := app.handleKey(tea.KeyMsg{Type: tea.KeyUp})
	a := model.(*App)
	if a.detailScroll != 4 {
		t.Errorf("detailScroll = %d, want 4 after Up", a.detailScroll)
	}
}

func TestDetailScrollBoundLower(t *testing.T) {
	app := detailApp()
	// Already at 0; pressing up should stay at 0.
	model, _ := app.handleKey(key("k"))
	a := model.(*App)
	if a.detailScroll != 0 {
		t.Errorf("detailScroll = %d, want 0 (clamped at min)", a.detailScroll)
	}
}

func TestHandleKeyDetailNilItem(t *testing.T) {
	app := newTestApp()
	app.showDetail = true
	app.detailItem = nil
	model, _ := app.handleKey(key("j"))
	a := model.(*App)
	if a.showDetail {
		t.Error("showDetail should be false after keypress with nil detailItem")
	}
}

func TestGetDetailLinesNilItem(t *testing.T) {
	app := newTestApp()
	if lines := app.getDetailLines(); lines != nil {
		t.Errorf("getDetailLines() with nil detailItem = %v, want nil", lines)
	}
}

func TestDetailScrollResetOnOpen(t *testing.T) {
	app := newTestApp()
	app.height = 24
	app.snapshot.Activities = []core.Activity{{PID: 5001, Query: overflowQuery}}
	app.section = SectionActivity
	app.detailScroll = 99 // stale scroll from a previous session
	model, _ := app.handleKey(tea.KeyMsg{Type: tea.KeyEnter})
	a := model.(*App)
	if a.detailScroll != 0 {
		t.Errorf("detailScroll = %d, want 0 after opening overlay", a.detailScroll)
	}
}

func TestViewDetailScrollFooter(t *testing.T) {
	app := detailApp()
	v := app.View()
	if !strings.Contains(v, "[↑/↓/k/j] scroll") {
		t.Errorf("expected scroll footer when SQL overflows, got: %q", v)
	}
}

func TestViewDetailNoScrollFooter(t *testing.T) {
	app := newTestApp()
	act := core.Activity{PID: 6000, Query: "SELECT 1"}
	app.showDetail = true
	app.detailItem = &act
	v := app.View()
	if !strings.Contains(v, "[any key] close") {
		t.Errorf("expected simple footer for short SQL, got: %q", v)
	}
	if strings.Contains(v, "[↑/↓/k/j] scroll") {
		t.Errorf("unexpected scroll footer for short SQL, got: %q", v)
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
func (m *mockQuerier) Locks(_ context.Context) ([]core.Lock, error) { return nil, nil }
func (m *mockQuerier) IdleInTx(_ context.Context, _ time.Duration) ([]core.Activity, error) {
	return nil, nil
}
func (m *mockQuerier) Stats(_ context.Context) (core.DBStats, error) {
	return core.DBStats{}, nil
}

func TestNew(t *testing.T) {
	poller := core.NewPoller(&mockQuerier{}, time.Second)
	presets := []ConnectionPreset{{Name: "primary", DSN: "postgres://x"}}
	app := New(poller, "primary", presets, nil)
	if app.poller == nil {
		t.Error("expected poller to be set")
	}
	if app.pollCh == nil {
		t.Error("expected pollCh to be set")
	}
	if app.currentConn != "primary" {
		t.Errorf("currentConn = %q, want primary", app.currentConn)
	}
	if len(app.connList) != 1 {
		t.Errorf("len(connList) = %d, want 1", len(app.connList))
	}
	if app.pollerDone == nil {
		t.Error("expected pollerDone to be set")
	}
	app.cancel()
	<-app.pollerDone // confirm goroutine exits after cancel
}

// --- clampScroll ---

func TestClampScrollNilItem(t *testing.T) {
	app := newTestApp()
	if got := app.clampScroll(5); got != 0 {
		t.Errorf("clampScroll with nil detailItem = %d, want 0", got)
	}
}

func TestClampScrollNoOverflow(t *testing.T) {
	app := newTestApp()
	act := core.Activity{PID: 1, Query: "SELECT 1"}
	app.detailItem = &act
	// 1 formatted line; sqlRows = height(30)-4 = 26 → maxScroll ≤ 0
	if got := app.clampScroll(5); got != 0 {
		t.Errorf("clampScroll with no overflow = %d, want 0", got)
	}
}

func TestClampScrollNegativeOffset(t *testing.T) {
	app := detailApp()
	if got := app.clampScroll(-1); got != 0 {
		t.Errorf("clampScroll(-1) = %d, want 0", got)
	}
}

func TestUpdateWindowSizeDetailOpen(t *testing.T) {
	app := detailApp()
	app.detailScroll = 5
	// Resize to a very tall terminal: sqlRows = 50-4 = 46 > 31 lines → maxScroll=0 → scroll clamped.
	model, _ := app.Update(tea.WindowSizeMsg{Width: 100, Height: 50})
	a := model.(*App)
	if a.detailScroll != 0 {
		t.Errorf("detailScroll = %d, want 0 after resize clamp", a.detailScroll)
	}
}

// countSQLLines returns the number of lines between the two separator lines in the detail view.
func countSQLLines(view string) int {
	lines := strings.Split(view, "\n")
	sep := 0
	count := 0
	for _, l := range lines {
		if strings.HasPrefix(l, "────") {
			sep++
			continue
		}
		if sep == 1 {
			count++
		}
		if sep == 2 {
			break
		}
	}
	return count
}

// TestDetailSQLLinesFilledByHeight verifies that the SQL area always contains exactly
// height-4 lines (padded with blank lines when the query is shorter than the window).
// Heights below minHeight (24) show a "too small" message and are not tested here.
func TestDetailSQLLinesFilledByHeight(t *testing.T) {
	for _, h := range []int{24, 30, 40} {
		app := newTestApp()
		app.width = 100
		app.height = h
		act := core.Activity{PID: 1, Query: overflowQuery}
		app.showDetail = true
		app.detailItem = &act
		got := countSQLLines(app.View())
		want := h - 4
		if got != want {
			t.Errorf("height=%d: SQL lines = %d, want %d", h, got, want)
		}
	}
}

// TestDetailResizeWidthNarrowShowsScrollFooter verifies that narrowing the terminal
// while the detail overlay is open triggers overflow and shows the scroll footer.
// At width=200 height=30 the query fits (23 lines < sqlRows=26).
// After resize to width=100 the query expands to 31 lines > 26 → overflow.
func TestDetailResizeWidthNarrowShowsScrollFooter(t *testing.T) {
	app := newTestApp()
	app.width = 200
	app.height = 30
	act := core.Activity{PID: 1, Query: overflowQuery}
	app.showDetail = true
	app.detailItem = &act

	if strings.Contains(app.View(), "scroll") {
		t.Fatal("expected no scroll footer at width=200 height=30 before resize")
	}

	model, _ := app.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	a := model.(*App)
	if !strings.Contains(a.View(), "scroll") {
		t.Error("expected scroll footer after resize to width=100 height=30")
	}
}

// TestDetailResizeHeightExpandHidesScrollFooter verifies that expanding the terminal
// while scrolling is active removes the scroll footer once the query fits.
// At height=24 the query overflows (31 lines > sqlRows=20).
// After resize to height=36 the query fits (31 lines ≤ sqlRows=32).
func TestDetailResizeHeightExpandHidesScrollFooter(t *testing.T) {
	app := detailApp() // width=100, height=24 → overflow
	if !strings.Contains(app.View(), "scroll") {
		t.Fatal("expected scroll footer at height=24 before resize")
	}

	model, _ := app.Update(tea.WindowSizeMsg{Width: 100, Height: 36})
	a := model.(*App)
	v := a.View()
	if strings.Contains(v, "[↑/↓/k/j] scroll") {
		t.Error("expected scroll footer to disappear after resize to height=36")
	}
	if !strings.Contains(v, "[any key] close") {
		t.Error("expected [any key] close footer after resize to height=36")
	}
}

// TestDetailResizeHeightShrinkShowsScrollFooter verifies that shrinking the terminal
// causes the scroll footer to appear when the query no longer fits.
// At width=150 height=30 the query formats to 26 lines = sqlRows=26 → no overflow.
// After resize to height=24 the query overflows (26 lines > sqlRows=20).
func TestDetailResizeHeightShrinkShowsScrollFooter(t *testing.T) {
	app := newTestApp()
	app.width = 150
	app.height = 30
	act := core.Activity{PID: 1, Query: overflowQuery}
	app.showDetail = true
	app.detailItem = &act

	if strings.Contains(app.View(), "scroll") {
		t.Fatal("expected no scroll footer at width=150 height=30 before resize")
	}

	model, _ := app.Update(tea.WindowSizeMsg{Width: 150, Height: 24})
	a := model.(*App)
	if !strings.Contains(a.View(), "scroll") {
		t.Error("expected scroll footer after resize to height=24")
	}
}

func TestRenderDetailStartBeyondTotal(t *testing.T) {
	app := newTestApp()
	act := core.Activity{PID: 7000, Query: "SELECT 1"}
	app.showDetail = true
	app.detailItem = &act
	app.detailScroll = 999 // beyond total; start should clamp to total
	v := app.View()
	if !strings.Contains(v, "7000") {
		t.Errorf("expected PID in detail view with out-of-range scroll, got: %q", v)
	}
}

// --- connection selector ---

func newMultiConnApp() *App {
	app := newTestApp()
	app.connList = []ConnectionPreset{
		{Name: "primary", DSN: "postgres://p@localhost/db"},
		{Name: "replica", DSN: "postgres://r@localhost/db"},
	}
	app.currentConn = "primary"
	app.reconnectFn = nil
	return app
}

func TestHandleKeyCOpensSelector(t *testing.T) {
	app := newMultiConnApp()
	model, _ := app.handleKey(key("c"))
	a := model.(*App)
	if !a.showConnSelector {
		t.Error("expected showConnSelector=true after c key")
	}
	if a.connCursor != 0 {
		t.Errorf("connCursor = %d, want 0 (index of primary)", a.connCursor)
	}
}

func TestHandleKeyCNoOpSingleConn(t *testing.T) {
	app := newTestApp()
	app.connList = []ConnectionPreset{{Name: "only", DSN: "postgres://x"}}
	app.currentConn = "only"
	model, _ := app.handleKey(key("c"))
	a := model.(*App)
	if a.showConnSelector {
		t.Error("expected showConnSelector=false when only one connection")
	}
}

func TestConnSelectorUp(t *testing.T) {
	app := newMultiConnApp()
	app.showConnSelector = true
	app.connCursor = 1
	model, _ := app.handleKey(tea.KeyMsg{Type: tea.KeyUp})
	a := model.(*App)
	if a.connCursor != 0 {
		t.Errorf("connCursor = %d, want 0 after Up", a.connCursor)
	}
}

func TestConnSelectorDown(t *testing.T) {
	app := newMultiConnApp()
	app.showConnSelector = true
	app.connCursor = 0
	model, _ := app.handleKey(key("j"))
	a := model.(*App)
	if a.connCursor != 1 {
		t.Errorf("connCursor = %d, want 1 after j", a.connCursor)
	}
}

func TestConnSelectorUpAtTop(t *testing.T) {
	app := newMultiConnApp()
	app.showConnSelector = true
	app.connCursor = 0
	model, _ := app.handleKey(key("k"))
	a := model.(*App)
	if a.connCursor != 0 {
		t.Errorf("connCursor = %d, want 0 (already at top)", a.connCursor)
	}
}

func TestConnSelectorDownAtBottom(t *testing.T) {
	app := newMultiConnApp()
	app.showConnSelector = true
	app.connCursor = 1 // already at last
	model, _ := app.handleKey(key("j"))
	a := model.(*App)
	if a.connCursor != 1 {
		t.Errorf("connCursor = %d, want 1 (already at bottom)", a.connCursor)
	}
}

func TestConnSelectorEscCloses(t *testing.T) {
	app := newMultiConnApp()
	app.showConnSelector = true
	model, _ := app.handleKey(tea.KeyMsg{Type: tea.KeyEsc})
	a := model.(*App)
	if a.showConnSelector {
		t.Error("expected showConnSelector=false after Esc")
	}
}

func TestConnSelectorCCloses(t *testing.T) {
	app := newMultiConnApp()
	app.showConnSelector = true
	model, _ := app.handleKey(key("c"))
	a := model.(*App)
	if a.showConnSelector {
		t.Error("expected showConnSelector=false after c in selector")
	}
}

func TestConnSelectorQCloses(t *testing.T) {
	app := newMultiConnApp()
	app.showConnSelector = true
	model, _ := app.handleKey(key("q"))
	a := model.(*App)
	if a.showConnSelector {
		t.Error("expected showConnSelector=false after q in selector")
	}
}

func TestConnSelectorEnterSameConn(t *testing.T) {
	app := newMultiConnApp()
	app.showConnSelector = true
	app.connCursor = 0 // primary is current
	model, cmd := app.handleKey(tea.KeyMsg{Type: tea.KeyEnter})
	a := model.(*App)
	if a.showConnSelector {
		t.Error("expected showConnSelector=false after Enter on current conn")
	}
	if cmd != nil {
		t.Error("expected nil cmd when selecting already-current connection")
	}
}

func TestConnSelectorEnterDifferentConn(t *testing.T) {
	app := newMultiConnApp()
	app.showConnSelector = true
	app.connCursor = 1 // replica
	reconnected := false
	app.reconnectFn = func(_ context.Context, dsn string) (*core.Poller, error) {
		reconnected = true
		p := core.NewPoller(&mockQuerier{}, time.Second)
		return p, nil
	}
	_, cmd := app.handleKey(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("expected non-nil cmd for switching to different connection")
	}
	msg := cmd()
	if !reconnected {
		t.Error("expected reconnectFn to be called")
	}
	switched, ok := msg.(connectionSwitchedMsg)
	if !ok {
		t.Fatalf("expected connectionSwitchedMsg, got %T", msg)
	}
	if switched.name != "replica" {
		t.Errorf("switched.name = %q, want replica", switched.name)
	}
}

func TestConnSelectorReconnectError(t *testing.T) {
	app := newMultiConnApp()
	app.showConnSelector = true
	app.connCursor = 1 // replica
	app.reconnectFn = func(_ context.Context, _ string) (*core.Poller, error) {
		return nil, fmt.Errorf("connection refused")
	}
	_, cmd := app.handleKey(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("expected non-nil cmd")
	}
	msg := cmd()
	errMsg, ok := msg.(reconnectErrMsg)
	if !ok {
		t.Fatalf("expected reconnectErrMsg, got %T", msg)
	}
	if errMsg.err == nil || errMsg.err.Error() != "connection refused" {
		t.Errorf("err = %v, want connection refused", errMsg.err)
	}
}

func TestUpdateConnectionSwitched(t *testing.T) {
	app := newTestApp()
	newPoller := core.NewPoller(&mockQuerier{}, 2*time.Second)
	model, cmd := app.Update(connectionSwitchedMsg{poller: newPoller, name: "replica"})
	a := model.(*App)
	t.Cleanup(func() {
		a.cancel()
		<-a.pollerDone // confirm goroutine exits after cancel
	})
	if a.currentConn != "replica" {
		t.Errorf("currentConn = %q, want replica", a.currentConn)
	}
	if a.showConnSelector {
		t.Error("expected showConnSelector=false after switch")
	}
	if a.gen != 1 {
		t.Errorf("gen = %d, want 1 after switch", a.gen)
	}
	if cmd == nil {
		t.Error("expected waitForSnapshot cmd after switch")
	}
}

func TestUpdateReconnectErr(t *testing.T) {
	app := newTestApp()
	model, cmd := app.Update(reconnectErrMsg{err: fmt.Errorf("timeout")})
	a := model.(*App)
	if a.lastErr == nil || a.lastErr.Error() != "timeout" {
		t.Errorf("lastErr = %v, want timeout", a.lastErr)
	}
	if a.showConnSelector {
		t.Error("expected showConnSelector=false on reconnect error")
	}
	if cmd != nil {
		t.Error("expected nil cmd on reconnect error")
	}
}

func TestViewConnSelector(t *testing.T) {
	app := newMultiConnApp()
	app.showConnSelector = true
	v := app.View()
	if !strings.Contains(v, "Select Connection") {
		t.Errorf("expected Select Connection in view, got: %q", v)
	}
	if !strings.Contains(v, "primary") {
		t.Errorf("expected primary in selector view, got: %q", v)
	}
	if !strings.Contains(v, "replica") {
		t.Errorf("expected replica in selector view, got: %q", v)
	}
}

func TestRenderHelpWithConnections(t *testing.T) {
	app := newMultiConnApp()
	app.showHelp = true
	v := app.View()
	if !strings.Contains(v, "connection selector") {
		t.Errorf("expected connection selector hint in help when multiConn, got: %q", v)
	}
}

func TestRenderHelpWithoutConnections(t *testing.T) {
	app := newTestApp()
	app.connList = []ConnectionPreset{{Name: "only", DSN: "postgres://x"}}
	app.showHelp = true
	v := app.View()
	if strings.Contains(v, "connection selector") {
		t.Errorf("unexpected connection selector hint in help when single conn, got: %q", v)
	}
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

// --- auto-reconnect unit tests ---

func TestUpdateConnectionSwitchedStale(t *testing.T) {
	app := newTestApp()
	app.autoReconnectGen = 2
	newPoller := core.NewPoller(&mockQuerier{}, time.Second)
	model, cmd := app.Update(connectionSwitchedMsg{poller: newPoller, name: "replica", autoGen: 1})
	a := model.(*App)
	if a.currentConn == "replica" {
		t.Error("stale connectionSwitchedMsg should be ignored")
	}
	if cmd != nil {
		t.Error("expected nil cmd for stale connectionSwitchedMsg")
	}
}

func TestUpdateAutoReconnectFailedStale(t *testing.T) {
	app := newTestApp()
	app.autoReconnectGen = 2
	_, cmd := app.Update(autoReconnectFailedMsg{gen: 1, delay: time.Second, deadline: time.Now().Add(time.Minute)})
	if cmd != nil {
		t.Error("expected nil cmd for stale autoReconnectFailedMsg")
	}
}

func TestAutoReconnectFailedRetrySucceeds(t *testing.T) {
	app := newTestApp()
	app.reconnectFn = func(_ context.Context, _ string) (*core.Poller, error) {
		return core.NewPoller(&mockQuerier{}, time.Second), nil
	}
	app.autoReconnectGen = 1
	_, cmd := app.Update(autoReconnectFailedMsg{
		gen:      1,
		delay:    time.Millisecond,
		deadline: time.Now().Add(10 * time.Minute),
		dsn:      "postgres://p",
		name:     "primary",
	})
	if cmd == nil {
		t.Fatal("expected non-nil cmd")
	}
	msg := cmd()
	switched, ok := msg.(connectionSwitchedMsg)
	if !ok {
		t.Fatalf("expected connectionSwitchedMsg, got %T", msg)
	}
	if switched.name != "primary" || switched.autoGen != 1 {
		t.Errorf("switched = {name:%q autoGen:%d}, want {primary 1}", switched.name, switched.autoGen)
	}
}

func TestCurrentPresetNil(t *testing.T) {
	app := newTestApp()
	app.connList = []ConnectionPreset{{Name: "primary", DSN: "postgres://p"}}
	app.currentConn = "nonexistent"
	if app.currentPreset() != nil {
		t.Error("expected nil for unknown currentConn")
	}
}

func TestBackoffNextCap(t *testing.T) {
	if got := backoffNext(20 * time.Second); got != 30*time.Second {
		t.Errorf("backoffNext(20s) = %v, want 30s", got)
	}
}
