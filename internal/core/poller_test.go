package core

import (
	"context"
	"errors"
	"testing"
	"time"
)

// mockQuerier implements Querier for unit tests.
type mockQuerier struct {
	serverInfo  func(ctx context.Context) (string, string, error)
	longRunning func(ctx context.Context, threshold time.Duration) ([]Activity, error)
	locks       func(ctx context.Context) ([]Lock, error)
	idleInTx    func(ctx context.Context, threshold time.Duration) ([]Activity, error)
	stats       func(ctx context.Context) (DBStats, error)
}

func (m *mockQuerier) ServerInfo(ctx context.Context) (string, string, error) {
	return m.serverInfo(ctx)
}
func (m *mockQuerier) LongRunning(ctx context.Context, t time.Duration) ([]Activity, error) {
	return m.longRunning(ctx, t)
}
func (m *mockQuerier) Locks(ctx context.Context) ([]Lock, error) { return m.locks(ctx) }
func (m *mockQuerier) IdleInTx(ctx context.Context, t time.Duration) ([]Activity, error) {
	return m.idleInTx(ctx, t)
}
func (m *mockQuerier) Stats(ctx context.Context) (DBStats, error) { return m.stats(ctx) }

func defaultMock() *mockQuerier {
	return &mockQuerier{
		serverInfo:  func(_ context.Context) (string, string, error) { return "16.1", "localhost:5432", nil },
		longRunning: func(_ context.Context, _ time.Duration) ([]Activity, error) { return nil, nil },
		locks:       func(_ context.Context) ([]Lock, error) { return nil, nil },
		idleInTx:    func(_ context.Context, _ time.Duration) ([]Activity, error) { return nil, nil },
		stats:       func(_ context.Context) (DBStats, error) { return DBStats{XactTotal: 100}, nil },
	}
}

func TestClampInterval(t *testing.T) {
	cases := []struct {
		d    time.Duration
		want time.Duration
	}{
		{50 * time.Millisecond, minInterval},
		{minInterval, minInterval},
		{time.Second, time.Second},
		{5 * time.Second, 5 * time.Second},
	}
	for _, c := range cases {
		got := clampInterval(c.d)
		if got != c.want {
			t.Errorf("clampInterval(%v) = %v, want %v", c.d, got, c.want)
		}
	}
}

func TestPollerDefaults(t *testing.T) {
	p := NewPoller(nil, time.Second)
	if p.LongRunningThreshold != 5*time.Second {
		t.Errorf("LongRunningThreshold = %v, want 5s", p.LongRunningThreshold)
	}
	if p.IdleInTxThreshold != 30*time.Second {
		t.Errorf("IdleInTxThreshold = %v, want 30s", p.IdleInTxThreshold)
	}
}

func TestPollerSetInterval(t *testing.T) {
	p := NewPoller(nil, time.Second)

	p.SetInterval(3 * time.Second)
	if p.Interval() != 3*time.Second {
		t.Errorf("Interval() = %v, want 3s", p.Interval())
	}

	p.SetInterval(1 * time.Millisecond)
	if p.Interval() != minInterval {
		t.Errorf("Interval() = %v, want minInterval (%v)", p.Interval(), minInterval)
	}
}

func TestCaptureNormal(t *testing.T) {
	p := NewPoller(defaultMock(), time.Second)
	s, err := p.capture(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if s.PGVersion != "16.1" {
		t.Errorf("PGVersion = %q, want 16.1", s.PGVersion)
	}
	if s.ServerAddr != "localhost:5432" {
		t.Errorf("ServerAddr = %q, want localhost:5432", s.ServerAddr)
	}
	// first capture: no TPS yet
	if s.DBStats.TPS != 0 {
		t.Errorf("TPS on first capture = %v, want 0", s.DBStats.TPS)
	}
}

func TestCaptureTPSDelta(t *testing.T) {
	xact := int64(1000)
	mock := defaultMock()
	mock.stats = func(_ context.Context) (DBStats, error) {
		xact += 500
		return DBStats{XactTotal: xact}, nil
	}

	p := NewPoller(mock, time.Second)
	p.prevCapturedAt = time.Now().Add(-time.Second)
	p.prevXactTotal = 1000

	s, err := p.capture(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if s.DBStats.TPS <= 0 {
		t.Errorf("TPS = %v, want > 0", s.DBStats.TPS)
	}
}

func TestCaptureTPSBackwardCounter(t *testing.T) {
	mock := defaultMock()
	mock.stats = func(_ context.Context) (DBStats, error) {
		return DBStats{XactTotal: 10}, nil // less than prevXactTotal
	}

	p := NewPoller(mock, time.Second)
	p.prevCapturedAt = time.Now().Add(-time.Second)
	p.prevXactTotal = 1000

	s, err := p.capture(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if s.DBStats.TPS != 0 {
		t.Errorf("TPS after counter reset = %v, want 0", s.DBStats.TPS)
	}
}

func TestCaptureServerInfoError(t *testing.T) {
	mock := defaultMock()
	mock.serverInfo = func(_ context.Context) (string, string, error) {
		return "", "", errors.New("connection lost")
	}

	p := NewPoller(mock, time.Second)
	_, err := p.capture(context.Background())
	if err == nil {
		t.Error("expected error from ServerInfo, got nil")
	}
}

func TestCaptureServerInfoCached(t *testing.T) {
	calls := 0
	mock := defaultMock()
	mock.serverInfo = func(_ context.Context) (string, string, error) {
		calls++
		return "16.1", "localhost:5432", nil
	}

	p := NewPoller(mock, time.Second)
	p.capture(context.Background())
	p.capture(context.Background())

	if calls != 1 {
		t.Errorf("ServerInfo called %d times, want 1 (should be cached)", calls)
	}
}

func TestPollerRun(t *testing.T) {
	p := NewPoller(defaultMock(), 50*time.Millisecond)
	ctx, cancel := context.WithCancel(context.Background())
	ch := make(chan PollResult, 1)

	go p.Run(ctx, ch)

	result := <-ch
	if result.Err != nil {
		t.Errorf("unexpected error: %v", result.Err)
	}
	if result.Snapshot.PGVersion != "16.1" {
		t.Errorf("PGVersion = %q, want 16.1", result.Snapshot.PGVersion)
	}
	cancel()
}

func TestPollerRunError(t *testing.T) {
	mock := defaultMock()
	mock.longRunning = func(_ context.Context, _ time.Duration) ([]Activity, error) {
		return nil, errors.New("db error")
	}

	p := NewPoller(mock, 50*time.Millisecond)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ch := make(chan PollResult, 1)

	go p.Run(ctx, ch)

	result := <-ch
	if result.Err == nil {
		t.Error("expected error, got nil")
	}
}

func TestPollerRunCancelDuringSend(t *testing.T) {
	p := NewPoller(defaultMock(), 50*time.Millisecond)
	ctx, cancel := context.WithCancel(context.Background())
	ch := make(chan PollResult) // unbuffered — blocks until receiver is ready

	go p.Run(ctx, ch)
	cancel() // cancel before any receiver reads, forcing ctx.Done() path

	// Drain if anything was already queued before cancel took effect
	select {
	case <-ch:
	default:
	}
}

func TestCaptureLongRunningError(t *testing.T) {
	mock := defaultMock()
	mock.longRunning = func(_ context.Context, _ time.Duration) ([]Activity, error) {
		return nil, errors.New("long running error")
	}
	p := NewPoller(mock, time.Second)
	_, err := p.capture(context.Background())
	if err == nil {
		t.Error("expected LongRunning error")
	}
}

func TestCaptureLocksError(t *testing.T) {
	mock := defaultMock()
	mock.locks = func(_ context.Context) ([]Lock, error) {
		return nil, errors.New("locks error")
	}
	p := NewPoller(mock, time.Second)
	_, err := p.capture(context.Background())
	if err == nil {
		t.Error("expected Locks error")
	}
}

func TestCaptureIdleInTxError(t *testing.T) {
	mock := defaultMock()
	mock.idleInTx = func(_ context.Context, _ time.Duration) ([]Activity, error) {
		return nil, errors.New("idleInTx error")
	}
	p := NewPoller(mock, time.Second)
	_, err := p.capture(context.Background())
	if err == nil {
		t.Error("expected IdleInTx error")
	}
}

func TestCaptureStatsError(t *testing.T) {
	mock := defaultMock()
	mock.stats = func(_ context.Context) (DBStats, error) {
		return DBStats{}, errors.New("stats error")
	}
	p := NewPoller(mock, time.Second)
	_, err := p.capture(context.Background())
	if err == nil {
		t.Error("expected Stats error")
	}
}
