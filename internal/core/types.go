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
	ConnectionsActive  int
	ConnectionsMax     int
	TPS                float64
	CacheHitRatio      float64
	XactTotal          int64   // raw cumulative value; used for TPS delta, not displayed
	CheckpointReq      int64   // checkpoints triggered by WAL pressure since last reset
	HasStandbys        bool    // true when pg_stat_replication has at least one row
	ReplicationLagSecs float64 // max lag across all standbys in seconds; 0 if fully caught up or no standbys
	AutovacuumWorkers  int     // number of active autovacuum worker processes
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
