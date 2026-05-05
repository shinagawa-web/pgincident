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
    left(COALESCE(query, ''), 200),
    COALESCE(application_name, ''),
    COALESCE(client_addr::text, '(local)')
FROM pg_stat_activity
WHERE state = 'active'
  AND pid <> pg_backend_pid()
  AND query_start < now() - ($1 * interval '1 second')
ORDER BY query_start`

// LongRunning returns active queries running longer than threshold.
func (c *Client) LongRunning(ctx context.Context, threshold time.Duration) ([]Activity, error) {
	rows, err := c.conn.Query(ctx, activitySQL, threshold.Seconds())
	if err != nil {
		return nil, err
	}
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
