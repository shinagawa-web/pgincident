package core

import (
	"context"
	"sync/atomic"
	"time"
)

const minInterval = 100 * time.Millisecond

// Querier is the database interface used by Poller.
type Querier interface {
	ServerInfo(ctx context.Context) (string, string, error)
	LongRunning(ctx context.Context, threshold time.Duration) ([]Activity, error)
	Locks(ctx context.Context) ([]Lock, error)
	IdleInTx(ctx context.Context, threshold time.Duration) ([]Activity, error)
	Stats(ctx context.Context) (DBStats, error)
}

// PollResult is what the Poller sends on each tick.
type PollResult struct {
	Snapshot Snapshot
	Err      error
}

// Poller runs a background loop that captures Snapshots at a configurable interval.
type Poller struct {
	client   Querier
	interval atomic.Int64 // nanoseconds; read/written atomically

	LongRunningThreshold time.Duration
	IdleInTxThreshold    time.Duration

	// cached on first capture
	pgVersion  string
	serverAddr string

	// TPS state
	prevXactTotal  int64
	prevCapturedAt time.Time
}

func clampInterval(d time.Duration) time.Duration {
	if d < minInterval {
		return minInterval
	}
	return d
}

func NewPoller(client Querier, interval time.Duration) *Poller {
	p := &Poller{
		client:               client,
		LongRunningThreshold: 5 * time.Second,
		IdleInTxThreshold:    30 * time.Second,
	}
	p.interval.Store(int64(clampInterval(interval)))
	return p
}

func (p *Poller) SetInterval(d time.Duration) {
	p.interval.Store(int64(clampInterval(d)))
}

func (p *Poller) Interval() time.Duration {
	return time.Duration(p.interval.Load())
}

// Run captures snapshots in a loop and sends each result to out.
// Returns when ctx is cancelled.
func (p *Poller) Run(ctx context.Context, out chan<- PollResult) {
	timer := time.NewTimer(0) // fire immediately on first iteration
	defer timer.Stop()

	for {
		select {
		case <-timer.C:
		case <-ctx.Done():
			return
		}

		s, err := p.capture(ctx)
		select {
		case out <- PollResult{s, err}:
		case <-ctx.Done():
			return
		}

		timer.Reset(p.Interval())
	}
}

func (p *Poller) capture(ctx context.Context) (Snapshot, error) {
	now := time.Now()

	if p.pgVersion == "" {
		v, a, err := p.client.ServerInfo(ctx)
		if err != nil {
			return Snapshot{}, err
		}
		p.pgVersion = v
		p.serverAddr = a
	}

	activities, err := p.client.LongRunning(ctx, p.LongRunningThreshold)
	if err != nil {
		return Snapshot{}, err
	}

	locks, err := p.client.Locks(ctx)
	if err != nil {
		return Snapshot{}, err
	}

	idleInTx, err := p.client.IdleInTx(ctx, p.IdleInTxThreshold)
	if err != nil {
		return Snapshot{}, err
	}

	stats, err := p.client.Stats(ctx)
	if err != nil {
		return Snapshot{}, err
	}

	if !p.prevCapturedAt.IsZero() {
		elapsed := now.Sub(p.prevCapturedAt).Seconds()
		if elapsed > 0 && stats.XactTotal >= p.prevXactTotal {
			stats.TPS = float64(stats.XactTotal-p.prevXactTotal) / elapsed
		}
		// XactTotal < prev means counter reset (server restart / pg_stat_reset);
		// skip TPS this tick and re-baseline.
	}
	p.prevXactTotal = stats.XactTotal
	p.prevCapturedAt = now

	return Snapshot{
		CapturedAt: now,
		PGVersion:  p.pgVersion,
		ServerAddr: p.serverAddr,
		DBStats:    stats,
		Activities: activities,
		Locks:      locks,
		IdleInTx:   idleInTx,
	}, nil
}
