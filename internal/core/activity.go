package core

import (
	"context"
	"time"
)

const activitySQL = `
SELECT
    pid,
    COALESCE(usename, ''),
    COALESCE(datname, ''),
    COALESCE(state, ''),
    COALESCE(query_start, now()),
    now() - COALESCE(query_start, now()),
    COALESCE(query, ''),
    COALESCE(application_name, ''),
    COALESCE(client_addr::text, '(local)')
FROM pg_stat_activity
WHERE state = 'active'
  AND pid <> pg_backend_pid()
  AND query_start < now() - ($1 * interval '1 second')
ORDER BY query_start`

const idleInTxSQL = `
SELECT
    pid,
    COALESCE(usename, ''),
    COALESCE(datname, ''),
    state,
    xact_start,
    now() - xact_start,
    COALESCE(query, ''),
    COALESCE(application_name, ''),
    COALESCE(client_addr::text, '(local)')
FROM pg_stat_activity
WHERE state = 'idle in transaction'
  AND xact_start < now() - ($1 * interval '1 second')
ORDER BY xact_start`

func scanActivities(rows interface {
	Next() bool
	Scan(...any) error
	Err() error
	Close()
}) ([]Activity, error) {
	defer rows.Close()
	var out []Activity
	for rows.Next() {
		var a Activity
		var dur time.Duration
		if err := rows.Scan(
			&a.PID, &a.User, &a.Database, &a.State,
			&a.QueryStart, &dur,
			&a.Query, &a.Application, &a.Client,
		); err != nil {
			return nil, err
		}
		a.Duration = dur
		out = append(out, a)
	}
	return out, rows.Err()
}

// LongRunning returns active queries running longer than threshold.
func (c *Client) LongRunning(ctx context.Context, threshold time.Duration) ([]Activity, error) {
	rows, err := c.conn.Query(ctx, activitySQL, threshold.Seconds())
	if err != nil {
		return nil, err
	}
	return scanActivities(rows)
}

// IdleInTx returns sessions idle in transaction longer than threshold.
func (c *Client) IdleInTx(ctx context.Context, threshold time.Duration) ([]Activity, error) {
	rows, err := c.conn.Query(ctx, idleInTxSQL, threshold.Seconds())
	if err != nil {
		return nil, err
	}
	return scanActivities(rows)
}
