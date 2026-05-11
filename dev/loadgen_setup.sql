-- Idempotent setup for the loadgen simulator.
-- Run as the postgres superuser via: docker compose exec -T postgres psql -U postgres
-- This is called automatically by `make dev-load`.

CREATE TABLE IF NOT EXISTS loadgen_accounts (
    id         bigint      PRIMARY KEY,
    balance    numeric(12,2) NOT NULL DEFAULT 0,
    touched_at timestamptz   NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS loadgen_lock_rows (
    id  int  PRIMARY KEY,
    val text
);

-- Seed accounts (10 k rows give the cache-hit ratio room to move).
INSERT INTO loadgen_accounts (id, balance)
SELECT i, (random() * 10000)::numeric(12,2)
FROM   generate_series(1, 10000) AS g(i)
ON CONFLICT DO NOTHING;

INSERT INTO loadgen_lock_rows VALUES (1, 'lock target A'), (2, 'lock target B')
ON CONFLICT DO NOTHING;

-- pgincident_dev has pg_monitor only — grant DML so the simulator can run.
GRANT SELECT, UPDATE ON loadgen_accounts  TO pgincident_dev;
GRANT SELECT, UPDATE ON loadgen_lock_rows TO pgincident_dev;
