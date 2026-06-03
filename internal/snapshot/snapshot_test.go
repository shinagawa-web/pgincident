package snapshot

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/shinagawa-web/pgincident/internal/core"
)

var fixedTime = time.Date(2026, 6, 2, 15, 4, 5, 0, time.UTC)

func fullReport() Report {
	return Report{
		ConnName: "production",
		Snapshot: core.Snapshot{
			CapturedAt: fixedTime,
			PGVersion:  "16.2",
			ServerAddr: "localhost:5432",
			DBStats: core.DBStats{
				ConnectionsActive:  5,
				ConnectionsMax:     100,
				TPS:                42,
				CacheHitRatio:      0.99,
				CheckpointReq:      3,
				AutovacuumWorkers:  1,
				HasStandbys:        true,
				ReplicationLagSecs: 1.5,
			},
			Activities: []core.Activity{
				{PID: 1001, User: "alice", Database: "app", State: "active", Duration: 12*time.Second + 500*time.Millisecond, Application: "psql", Query: "SELECT count(*) FROM orders"},
			},
			Locks: []core.Lock{
				{BlockedPID: 200, BlockingPID: 100, WaitTime: 3 * time.Second, Relation: "public.orders", Mode: "ShareLock", LockType: "relation"},
			},
			IdleInTx: []core.Activity{
				{PID: 3001, User: "bob", Database: "app", Duration: 2 * time.Minute, Application: "app", Query: "BEGIN"},
			},
		},
	}
}

func TestGenerateHeader(t *testing.T) {
	r := fullReport()
	out := Generate(r)

	mustContain(t, out, "# pgincident Snapshot")
	mustContain(t, out, "2026-06-02 15:04:05 UTC")
	mustContain(t, out, "production")
	mustContain(t, out, "localhost:5432")
	mustContain(t, out, "16.2")
}

func TestGenerateSSLLine(t *testing.T) {
	r := fullReport()
	r.Snapshot.SSL = true
	out := Generate(r)
	mustContain(t, out, "**SSL:**")

	r.Snapshot.SSL = false
	out = Generate(r)
	if strings.Contains(out, "**SSL:**") {
		t.Error("expected no SSL line when SSL=false")
	}
}

func TestGenerateDBHealth(t *testing.T) {
	r := fullReport()
	out := Generate(r)

	mustContain(t, out, "## Database Health")
	mustContain(t, out, "5 / 100")
	mustContain(t, out, "42")
	mustContain(t, out, "99.0%")
	mustContain(t, out, "1.5s") // replication lag
}

func TestGenerateDBHealthNoTPS(t *testing.T) {
	r := fullReport()
	r.Snapshot.DBStats.TPS = 0
	out := Generate(r)
	mustContain(t, out, "| TPS | — |")
}

func TestGenerateDBHealthNoStandbys(t *testing.T) {
	r := fullReport()
	r.Snapshot.DBStats.HasStandbys = false
	out := Generate(r)
	mustContain(t, out, "| no |")
}

func TestGenerateDBHealthStandbyNoLag(t *testing.T) {
	r := fullReport()
	r.Snapshot.DBStats.HasStandbys = true
	r.Snapshot.DBStats.ReplicationLagSecs = 0
	out := Generate(r)
	mustContain(t, out, "yes (lag —)")
}

func TestGenerateActivities(t *testing.T) {
	r := fullReport()
	out := Generate(r)

	mustContain(t, out, "## Long-running Queries (1)")
	mustContain(t, out, "1001")
	mustContain(t, out, "alice")
	mustContain(t, out, "SELECT count(*) FROM orders")
}

func TestGenerateActivitiesEmpty(t *testing.T) {
	r := fullReport()
	r.Snapshot.Activities = nil
	out := Generate(r)
	mustContain(t, out, "## Long-running Queries (0)")
	mustContain(t, out, "_none_")
}

func TestGenerateLocks(t *testing.T) {
	r := fullReport()
	out := Generate(r)

	mustContain(t, out, "## Lock Chains (1)")
	mustContain(t, out, "200")
	mustContain(t, out, "100")
	mustContain(t, out, "public.orders")
}

func TestGenerateLocksEmpty(t *testing.T) {
	r := fullReport()
	r.Snapshot.Locks = nil
	out := Generate(r)
	mustContain(t, out, "## Lock Chains (0)")
	mustContain(t, out, "_none_")
}

func TestGenerateIdleInTx(t *testing.T) {
	r := fullReport()
	out := Generate(r)

	mustContain(t, out, "## Idle-in-Transaction (1)")
	mustContain(t, out, "3001")
	mustContain(t, out, "bob")
}

func TestGenerateIdleInTxEmpty(t *testing.T) {
	r := fullReport()
	r.Snapshot.IdleInTx = nil
	out := Generate(r)
	mustContain(t, out, "## Idle-in-Transaction (0)")
	mustContain(t, out, "_none_")
}

func TestGenerateQueryLinkAndAnchor(t *testing.T) {
	r := fullReport()
	out := Generate(r)

	// table cell is a link to the anchor
	mustContain(t, out, "[SELECT count(*) FROM orders](#query-1001)")
	// anchor exists in the Full Query Text section
	mustContain(t, out, `<a id="query-1001"></a>`)
	// full SQL appears in a fenced code block
	mustContain(t, out, "```sql\nSELECT count(*) FROM orders\n```")
}

func TestGenerateIdleInTxLinkAndAnchor(t *testing.T) {
	r := fullReport()
	out := Generate(r)

	mustContain(t, out, "[BEGIN](#query-3001)")
	mustContain(t, out, `<a id="query-3001"></a>`)
	mustContain(t, out, "```sql\nBEGIN\n```")
}

func TestGenerateFullQueryUnmodified(t *testing.T) {
	r := fullReport()
	r.QueryLimit = 10
	r.Snapshot.Activities = []core.Activity{
		{PID: 42, User: "u", Database: "d", State: "active", Query: "SELECT  1\nFROM  foo"},
	}
	out := Generate(r)

	// table shows normalized+truncated link text (limit=10: runes[:9]+"…")
	mustContain(t, out, "[SELECT 1 …](#query-42)")
	// full query section shows original, unmodified SQL
	mustContain(t, out, "```sql\nSELECT  1\nFROM  foo\n```")
}

func TestGenerateQueryTruncation(t *testing.T) {
	r := fullReport()
	r.QueryLimit = 10
	r.Snapshot.Activities = []core.Activity{
		{PID: 1, Query: "SELECT * FROM very_long_table_name WHERE id = 1"},
	}
	out := Generate(r)
	// truncated at 10 runes: runes[:9] = "SELECT * " + "…"
	mustContain(t, out, "SELECT * …")
}

func TestGenerateQueryNoTruncation(t *testing.T) {
	r := fullReport()
	r.QueryLimit = 0 // defaults to 80
	r.Snapshot.Activities = []core.Activity{
		{PID: 1, Query: "SELECT 1"},
	}
	out := Generate(r)
	mustContain(t, out, "SELECT 1")
}

func TestGenerateQueryNormalization(t *testing.T) {
	r := fullReport()
	r.Snapshot.Activities = []core.Activity{
		{PID: 1, Query: "SELECT  1\nFROM  foo"},
	}
	out := Generate(r)
	mustContain(t, out, "SELECT 1 FROM foo")
}

func TestGenerateMDEscape(t *testing.T) {
	r := fullReport()
	r.Snapshot.Activities = []core.Activity{
		{PID: 1, User: "alice|bob", Query: "SELECT 1"},
	}
	out := Generate(r)
	mustContain(t, out, `alice\|bob`)
}

func TestFormatDurationSeconds(t *testing.T) {
	if got := formatDuration(45 * time.Second); got != "45s" {
		t.Errorf("got %q, want 45s", got)
	}
}

func TestFormatDurationMinutes(t *testing.T) {
	if got := formatDuration(3*time.Minute + 20*time.Second); got != "3m20s" {
		t.Errorf("got %q, want 3m20s", got)
	}
}

func TestFormatDurationHours(t *testing.T) {
	if got := formatDuration(2*time.Hour + 5*time.Minute + 3*time.Second); got != "2h05m03s" {
		t.Errorf("got %q, want 2h05m03s", got)
	}
}

func TestWriteFileCreatesFile(t *testing.T) {
	dir := t.TempDir()
	content := "# test"
	ts := time.Date(2026, 6, 2, 15, 4, 5, 0, time.UTC)

	path, err := WriteFile(dir, content, ts)
	if err != nil {
		t.Fatalf("WriteFile error: %v", err)
	}

	expected := filepath.Join(dir, ".pgincident", "snapshot-20260602-150405.md")
	if path != expected {
		t.Errorf("path = %q, want %q", path, expected)
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile error: %v", err)
	}
	if string(got) != content {
		t.Errorf("content = %q, want %q", got, content)
	}
}

func TestWriteFileMkdirError(t *testing.T) {
	// Pass a path to a file (not a dir) as the base dir so MkdirAll fails.
	f, err := os.CreateTemp(t.TempDir(), "notadir")
	if err != nil {
		t.Fatal(err)
	}
	f.Close()
	// dir/.pgincident can't be created because dir is a file path
	_, err = WriteFile(f.Name(), "content", fixedTime)
	if err == nil {
		t.Fatal("expected error when dir is a file, got nil")
	}
}

func TestWriteFileWriteError(t *testing.T) {
	// Create a .pgincident directory that is actually a file — WriteFile will
	// succeed at MkdirAll (the dir already ends with .pgincident), but
	// os.WriteFile will fail because the target path is a directory, not a file.
	base := t.TempDir()
	pgDir := filepath.Join(base, ".pgincident")
	// Create the subdir, then make a directory where the .md file should go.
	if err := os.MkdirAll(pgDir, 0o755); err != nil {
		t.Fatal(err)
	}
	// Pre-create a directory at the path that WriteFile would use.
	mdPath := filepath.Join(pgDir, "snapshot-20260602-150405.md")
	if err := os.MkdirAll(mdPath, 0o755); err != nil {
		t.Fatal(err)
	}
	_, err := WriteFile(base, "content", fixedTime)
	if err == nil {
		t.Fatal("expected error when snapshot path is a directory, got nil")
	}
}

func mustContain(t *testing.T, s, sub string) {
	t.Helper()
	if !strings.Contains(s, sub) {
		t.Errorf("expected %q to contain %q", s, sub)
	}
}
