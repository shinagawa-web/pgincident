package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/shinagawa-web/pgincident/internal/core"
)

func renderTitleBar(s core.Snapshot, interval time.Duration, connName string, width int) string {
	var left string
	if connName != "" {
		left = boldStyle.Render(connName)
		if s.ServerAddr != "" {
			left += dimStyle.Render(fmt.Sprintf("  %s  PG %s", s.ServerAddr, s.PGVersion))
		}
	} else if s.ServerAddr != "" {
		left = dimStyle.Render(fmt.Sprintf("%s  PG %s", s.ServerAddr, s.PGVersion))
	}
	if s.SSL {
		left += "  " + sslBadgeStyle.Render("SSL")
	}
	right := dimStyle.Render(fmt.Sprintf("interval: %.1fs", interval.Seconds()))
	gap := width - lipgloss.Width(left) - lipgloss.Width(right)
	if gap < 1 {
		gap = 1
	}
	return left + strings.Repeat(" ", gap) + right
}

func renderStatsBar(s core.DBStats, width int) string {
	tps := "—"
	if s.TPS > 0 {
		tps = fmt.Sprintf("%.0f", s.TPS)
	}
	return fmt.Sprintf("Connections: %d/%d (%s)   TPS: %s   Cache hit: %.1f%%",
		s.ConnectionsActive, s.ConnectionsMax,
		connPct(s.ConnectionsActive, s.ConnectionsMax),
		tps,
		s.CacheHitRatio*100,
	)
}

func renderStatus(err error, msg string) string {
	if err != nil {
		return errorStyle.Render("⚠  " + err.Error())
	}
	if msg != "" {
		return warnStyle.Render(msg)
	}
	return ""
}

func renderFooter(multiConn bool) string {
	s := "[q]uit  [Tab]section  [↑↓/jk]cursor  [o]overview  [Enter]detail  [+/-]interval"
	if multiConn {
		s += "  [c]connections"
	}
	s += "  [?]help"
	return footerStyle.Render(s)
}

// sectionTitle renders a title row with the count badge right-aligned.
func sectionTitle(label string, badge string, active bool, width int) string {
	style := inactiveTitleStyle
	marker := "  "
	if active {
		style = activeTitleStyle
		marker = "▶ "
	}
	title := marker + style.Render(label)
	titleW := lipgloss.Width(title)
	badgeStr := dimStyle.Render(badge)
	badgeW := lipgloss.Width(badgeStr)
	gap := width - titleW - badgeW
	if gap < 1 {
		gap = 1
	}
	return title + strings.Repeat(" ", gap) + badgeStr
}
