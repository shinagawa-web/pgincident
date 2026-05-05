package core

import "time"

// Activity represents one row from pg_stat_activity.
type Activity struct {
	PID         int
	User        string
	Database    string
	State       string
	QueryStart  time.Time
	Duration    time.Duration
	Query       string
	Application string
	Client      string
}

// Lock represents one blocked-blocking relationship from pg_locks.
type Lock struct {
	BlockedPID  int
	BlockingPID int
	WaitTime    time.Duration
	Relation    string
	Mode        string
	LockType    string
}

// DBStats holds the header metrics.
type DBStats struct {
	ConnectionsActive int
	ConnectionsMax    int
	TPS               float64
	CacheHitRatio     float64
	XactTotal         int64 // raw cumulative value; used for TPS delta, not displayed
}

// Snapshot is a single point-in-time capture of all dashboard data.
type Snapshot struct {
	CapturedAt time.Time
	PGVersion  string
	ServerAddr string
	DBStats    DBStats
	Activities []Activity // long-running active queries
	Locks      []Lock     // waiting locks
	IdleInTx   []Activity // idle in transaction sessions
}
