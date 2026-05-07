package tui

import (
	"fmt"
	"strings"
	"time"
)

func formatDuration(d time.Duration) string {
	s := int(d.Seconds())
	h := s / 3600
	m := (s % 3600) / 60
	sec := s % 60
	return fmt.Sprintf("%02d:%02d:%02d", h, m, sec)
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
