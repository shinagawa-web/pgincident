-- Dev user with pg_monitor (no superuser)
CREATE USER pgincident_dev WITH PASSWORD 'pgincident_dev';
GRANT pg_monitor TO pgincident_dev;

-- Table used by seed.sql for lock scenarios
CREATE TABLE IF NOT EXISTS seed_target (id int primary key, val text);
INSERT INTO seed_target VALUES (1, 'row for lock testing') ON CONFLICT DO NOTHING;

-- Loadgen tables are created by dev/loadgen_setup.sql, which make dev-up runs
-- automatically after the container is ready.
