package core

import (
	"context"
	"fmt"
)

const statsSQL = `
SELECT
    (SELECT count(*) FROM pg_stat_activity WHERE state IS NOT NULL)::int AS connections_active,
    (SELECT setting::int FROM pg_settings WHERE name = 'max_connections')  AS connections_max,
    COALESCE(sum(xact_commit + xact_rollback), 0)::bigint                 AS xact_total,
    COALESCE(sum(blks_hit)::float / NULLIF(sum(blks_hit + blks_read), 0), 0) AS cache_hit_ratio,
    (SELECT checkpoints_req FROM pg_stat_bgwriter)                         AS checkpoint_req,
    (SELECT EXISTS (SELECT 1 FROM pg_stat_replication))                    AS has_standbys,
    COALESCE(
        (SELECT MAX(EXTRACT(EPOCH FROM (
            COALESCE(write_lag, '0') +
            COALESCE(flush_lag, '0') +
            COALESCE(replay_lag, '0')
        ))) FROM pg_stat_replication),
        0
    )::float                                                               AS replication_lag_secs,
    (SELECT COUNT(*)::int FROM pg_stat_activity WHERE query LIKE 'autovacuum:%') AS autovacuum_workers
FROM pg_stat_database
WHERE datname = current_database()`

// Stats returns current database statistics. TPS is always 0 here;
// the Poller computes it as a delta between successive calls.
func (c *Client) Stats(ctx context.Context) (DBStats, error) {
	var s DBStats
	err := c.conn.QueryRow(ctx, statsSQL).Scan(
		&s.ConnectionsActive,
		&s.ConnectionsMax,
		&s.XactTotal,
		&s.CacheHitRatio,
		&s.CheckpointReq,
		&s.HasStandbys,
		&s.ReplicationLagSecs,
		&s.AutovacuumWorkers,
	)
	return s, err
}

// ServerInfo returns the Postgres version string and host:port address.
func (c *Client) ServerInfo(ctx context.Context) (version, addr string, err error) {
	err = c.conn.QueryRow(ctx, "SHOW server_version").Scan(&version)
	if err != nil {
		return
	}
	cfg := c.conn.Config()
	addr = fmt.Sprintf("%s:%d", cfg.Host, cfg.Port)
	return
}
