package snapshot

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/shinagawa-web/pgincident/internal/core"
)

// Report is the input to Generate.
type Report struct {
	Snapshot   core.Snapshot
	ConnName   string
	QueryLimit int // max chars per query cell; 0 means no truncation
}

// Generate renders r as a Markdown document and returns it as a string.
func Generate(r Report) string {
	s := r.Snapshot
	limit := r.QueryLimit
	if limit == 0 {
		limit = 80
	}

	var b strings.Builder

	fmt.Fprintf(&b, "# pgincident Snapshot\n\n")
	fmt.Fprintf(&b, "**Captured:**    %s\n", s.CapturedAt.UTC().Format("2006-01-02 15:04:05 UTC"))
	fmt.Fprintf(&b, "**Connection:**  %s\n", r.ConnName)
	fmt.Fprintf(&b, "**Server:**      %s\n", s.ServerAddr)
	fmt.Fprintf(&b, "**PostgreSQL:**  %s\n", s.PGVersion)
	if s.SSL {
		fmt.Fprintf(&b, "**SSL:**         yes\n")
	}
	fmt.Fprintf(&b, "\n---\n\n")

	// Database Health
	st := s.DBStats
	tps := "—"
	if st.TPS > 0 {
		tps = fmt.Sprintf("%.0f", st.TPS)
	}
	fmt.Fprintf(&b, "## Database Health\n\n")
	fmt.Fprintf(&b, "| Metric | Value |\n")
	fmt.Fprintf(&b, "| --- | --- |\n")
	fmt.Fprintf(&b, "| Connections | %d / %d |\n", st.ConnectionsActive, st.ConnectionsMax)
	fmt.Fprintf(&b, "| TPS | %s |\n", tps)
	fmt.Fprintf(&b, "| Cache hit ratio | %.1f%% |\n", st.CacheHitRatio*100)
	fmt.Fprintf(&b, "| Checkpoint pressure | %d |\n", st.CheckpointReq)
	fmt.Fprintf(&b, "| Autovacuum workers | %d |\n", st.AutovacuumWorkers)
	hasStandbys := "no"
	if st.HasStandbys {
		lag := "—"
		if st.ReplicationLagSecs > 0 {
			lag = fmt.Sprintf("%.1fs", st.ReplicationLagSecs)
		}
		hasStandbys = fmt.Sprintf("yes (lag %s)", lag)
	}
	fmt.Fprintf(&b, "| Standbys | %s |\n", hasStandbys)
	fmt.Fprintf(&b, "\n")

	// Long-running Queries
	fmt.Fprintf(&b, "## Long-running Queries (%d)\n\n", len(s.Activities))
	if len(s.Activities) == 0 {
		fmt.Fprintf(&b, "_none_\n\n")
	} else {
		fmt.Fprintf(&b, "| PID | User | Database | State | Duration | Application | Query |\n")
		fmt.Fprintf(&b, "| --- | --- | --- | --- | --- | --- | --- |\n")
		for _, a := range s.Activities {
			cell := mdEscape(truncate(normalizeQuery(a.Query), limit))
			fmt.Fprintf(&b, "| %d | %s | %s | %s | %s | %s | [%s](#query-%d) |\n",
				a.PID,
				mdEscape(a.User),
				mdEscape(a.Database),
				mdEscape(a.State),
				formatDuration(a.Duration),
				mdEscape(a.Application),
				cell,
				a.PID,
			)
		}
		fmt.Fprintf(&b, "\n### Full Query Text\n\n")
		for _, a := range s.Activities {
			fmt.Fprintf(&b, "<a id=\"query-%d\"></a>\n", a.PID)
			fmt.Fprintf(&b, "**PID %d** — %s @ %s | %s | %s\n\n", a.PID, a.User, a.Database, a.State, formatDuration(a.Duration))
			fmt.Fprintf(&b, "```sql\n%s\n```\n\n", strings.TrimSpace(a.Query))
		}
	}

	// Lock Chains
	fmt.Fprintf(&b, "## Lock Chains (%d)\n\n", len(s.Locks))
	if len(s.Locks) == 0 {
		fmt.Fprintf(&b, "_none_\n\n")
	} else {
		fmt.Fprintf(&b, "| Blocked PID | Blocking PID | Wait | Relation | Mode | Lock type |\n")
		fmt.Fprintf(&b, "| --- | --- | --- | --- | --- | --- |\n")
		for _, l := range s.Locks {
			fmt.Fprintf(&b, "| %d | %d | %s | %s | %s | %s |\n",
				l.BlockedPID,
				l.BlockingPID,
				formatDuration(l.WaitTime),
				mdEscape(l.Relation),
				mdEscape(l.Mode),
				mdEscape(l.LockType),
			)
		}
		fmt.Fprintf(&b, "\n")
	}

	// Idle-in-Transaction
	fmt.Fprintf(&b, "## Idle-in-Transaction (%d)\n\n", len(s.IdleInTx))
	if len(s.IdleInTx) == 0 {
		fmt.Fprintf(&b, "_none_\n\n")
	} else {
		fmt.Fprintf(&b, "| PID | User | Database | Duration | Application | Query |\n")
		fmt.Fprintf(&b, "| --- | --- | --- | --- | --- | --- |\n")
		for _, a := range s.IdleInTx {
			cell := mdEscape(truncate(normalizeQuery(a.Query), limit))
			fmt.Fprintf(&b, "| %d | %s | %s | %s | %s | [%s](#query-%d) |\n",
				a.PID,
				mdEscape(a.User),
				mdEscape(a.Database),
				formatDuration(a.Duration),
				mdEscape(a.Application),
				cell,
				a.PID,
			)
		}
		fmt.Fprintf(&b, "\n### Full Query Text\n\n")
		for _, a := range s.IdleInTx {
			fmt.Fprintf(&b, "<a id=\"query-%d\"></a>\n", a.PID)
			fmt.Fprintf(&b, "**PID %d** — %s @ %s | %s\n\n", a.PID, a.User, a.Database, formatDuration(a.Duration))
			fmt.Fprintf(&b, "```sql\n%s\n```\n\n", strings.TrimSpace(a.Query))
		}
	}

	return b.String()
}

// WriteFile writes content to dir/.pgincident/snapshot-<timestamp>.md.
// It returns the file path on success.
func WriteFile(dir string, content string, t time.Time) (string, error) {
	dirPath := dir + "/.pgincident"
	if err := os.MkdirAll(dirPath, 0o755); err != nil {
		return "", err
	}
	name := fmt.Sprintf("%s/snapshot-%s.md", dirPath, t.UTC().Format("20060102-150405"))
	if err := os.WriteFile(name, []byte(content), 0o644); err != nil {
		return "", err
	}
	return name, nil
}

// normalizeQuery collapses newlines and consecutive spaces for table display.
func normalizeQuery(q string) string {
	q = strings.ReplaceAll(q, "\n", " ")
	q = strings.ReplaceAll(q, "\r", "")
	for strings.Contains(q, "  ") {
		q = strings.ReplaceAll(q, "  ", " ")
	}
	return strings.TrimSpace(q)
}

// truncate shortens s to at most n runes, appending "…" when cut.
func truncate(s string, n int) string {
	runes := []rune(s)
	if len(runes) <= n {
		return s
	}
	return string(runes[:n-1]) + "…"
}

// mdEscape escapes pipe characters so they don't break Markdown tables.
func mdEscape(s string) string {
	return strings.ReplaceAll(s, "|", "\\|")
}

func formatDuration(d time.Duration) string {
	d = d.Round(time.Second)
	h := int(d.Hours())
	m := int(d.Minutes()) % 60
	sec := int(d.Seconds()) % 60
	if h > 0 {
		return fmt.Sprintf("%dh%02dm%02ds", h, m, sec)
	}
	if m > 0 {
		return fmt.Sprintf("%dm%02ds", m, sec)
	}
	return fmt.Sprintf("%ds", sec)
}
