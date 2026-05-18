package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/shinagawa-web/pgincident/internal/core"
)

func (a *App) renderOverview() string {
	div := dimStyle.Render(strings.Repeat("─", a.width))

	parts := []string{
		renderTitleBar(a.snapshot, a.poller.Interval(), a.currentConn, a.width),
		div,
		boldStyle.Render("  DB Health Overview"),
		div,
		"",
		colHeaderStyle.Render(fmt.Sprintf("  %-22s%-22s%s", "Metric", "Value", "Status")),
		dimStyle.Render("  " + strings.Repeat("─", 50)),
		renderOverviewMetrics(a.snapshot.DBStats),
		"",
		div,
		renderOverviewFooter(),
	}
	return strings.Join(parts, "\n")
}

func renderOverviewMetrics(s core.DBStats) string {
	rows := []string{
		overviewRow("Connections", fmtConnections(s), s.ConnectionsStatus()),
		overviewRow("TPS", fmtTPS(s.TPS), core.StatusNormal),
		overviewRow("Cache hit", fmt.Sprintf("%.1f%%", s.CacheHitRatio*100), s.CacheHitStatus()),
		overviewRow("Checkpoints", fmt.Sprintf("req: %d", s.CheckpointReq), s.CheckpointStatus()),
	}
	if s.HasStandbys {
		rows = append(rows, overviewRow("Replication lag", fmt.Sprintf("%.1fs", s.ReplicationLagSecs), s.ReplicationLagStatus()))
	}
	rows = append(rows, overviewRow("Autovacuum", fmt.Sprintf("%d workers", s.AutovacuumWorkers), s.AutovacuumStatus()))
	return strings.Join(rows, "\n")
}

func overviewRow(label, value string, status core.HealthStatus) string {
	badge, style := statusBadge(status)
	return fmt.Sprintf("  %-22s%-22s%s", label, value, style.Render(badge))
}

func statusBadge(s core.HealthStatus) (string, lipgloss.Style) {
	switch s {
	case core.StatusCritical:
		return "CRIT", errorStyle
	case core.StatusWarning:
		return "WARN", warnStyle
	default:
		return "OK", lipgloss.NewStyle()
	}
}

func fmtConnections(s core.DBStats) string {
	return fmt.Sprintf("%d / %d (%s)", s.ConnectionsActive, s.ConnectionsMax, connPct(s.ConnectionsActive, s.ConnectionsMax))
}

func fmtTPS(tps float64) string {
	if tps == 0 {
		return "—"
	}
	return fmt.Sprintf("%.0f", tps)
}

func renderOverviewFooter() string {
	return footerStyle.Render("[o]dashboard  [q]uit  [+/-]interval  [?]help")
}
