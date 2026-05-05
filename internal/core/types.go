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
