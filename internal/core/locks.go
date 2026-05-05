package core

import (
	"context"
	"time"
)

const locksSQL = `
SELECT
    blocked.pid,
    blocking.pid,
    now() - blocked.query_start,
    COALESCE(blocked_locks.relation::regclass::text, ''),
    blocked_locks.mode,
    blocked_locks.locktype
FROM pg_locks blocked_locks
JOIN pg_stat_activity blocked
    ON blocked.pid = blocked_locks.pid
JOIN pg_locks blocking_locks
    ON  blocking_locks.locktype        = blocked_locks.locktype
    AND blocking_locks.relation        IS NOT DISTINCT FROM blocked_locks.relation
    AND blocking_locks.page            IS NOT DISTINCT FROM blocked_locks.page
    AND blocking_locks.tuple           IS NOT DISTINCT FROM blocked_locks.tuple
    AND blocking_locks.transactionid   IS NOT DISTINCT FROM blocked_locks.transactionid
    AND blocking_locks.classid         IS NOT DISTINCT FROM blocked_locks.classid
    AND blocking_locks.objid           IS NOT DISTINCT FROM blocked_locks.objid
    AND blocking_locks.objsubid        IS NOT DISTINCT FROM blocked_locks.objsubid
    AND blocking_locks.pid            <> blocked_locks.pid
JOIN pg_stat_activity blocking
    ON blocking.pid = blocking_locks.pid
WHERE NOT blocked_locks.granted
ORDER BY blocked.query_start`

// Locks returns all blocked-blocking lock pairs.
func (c *Client) Locks(ctx context.Context) ([]Lock, error) {
	rows, err := c.conn.Query(ctx, locksSQL)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Lock
	for rows.Next() {
		var l Lock
		var wait time.Duration
		if err := rows.Scan(
			&l.BlockedPID, &l.BlockingPID, &wait,
			&l.Relation, &l.Mode, &l.LockType,
		); err != nil {
			return nil, err
		}
		l.WaitTime = wait
		out = append(out, l)
	}
	return out, rows.Err()
}
