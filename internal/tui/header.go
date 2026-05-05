package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/shinagawa-web/pgincident/internal/core"
	"github.com/shinagawa-web/pgincident/internal/version"
)

func renderTitleBar(s core.Snapshot, interval time.Duration, width int) string {
	left := boldStyle.Render("pgincident") + " v" + version.Version
	right := ""
	if s.ServerAddr != "" {
		right = dimStyle.Render(fmt.Sprintf("connected: %s (PG %s)  interval: %.1fs",
			s.ServerAddr, s.PGVersion, interval.Seconds()))
	}
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

func renderFooter(canSignal bool) string {
	base := "[q]uit  [Tab]section  [↑↓/jk]cursor  [+/-]interval  [r]efresh  [?]help"
	if canSignal {
		base = "[q]uit  [Tab]section  [↑↓/jk]cursor  [K]ill  [c]ancel  [+/-]interval  [r]efresh  [?]help"
	}
	return footerStyle.Render(base)
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
