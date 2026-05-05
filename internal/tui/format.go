package tui

import (
	"fmt"
	"strings"
	"time"
)

func formatDuration(d time.Duration) string {
	h := int(d.Hours())
	m := int(d.Minutes()) % 60
	s := d.Seconds() - float64(int(d.Minutes()))*60
	return fmt.Sprintf("%02d:%02d:%05.2f", h, m, s)
}

// truncate cuts s to at most n runes (including the ellipsis if truncated).
func truncate(s string, n int) string {
	runes := []rune(s)
	if len(runes) <= n {
		return s
	}
	if n <= 1 {
		return string(runes[:n])
	}
	return string(runes[:n-1]) + "…"
}

// padRight pads s to exactly n runes (truncating with "…" if needed).
func padRight(s string, n int) string {
	runes := []rune(s)
	switch {
	case len(runes) == n:
		return s
	case len(runes) > n:
		if n > 1 {
			return string(runes[:n-1]) + "…"
		}
		return string(runes[:n])
	default:
		return s + strings.Repeat(" ", n-len(runes))
	}
}

func connPct(active, max int) string {
	if max == 0 {
		return "—"
	}
	return fmt.Sprintf("%d%%", active*100/max)
}
