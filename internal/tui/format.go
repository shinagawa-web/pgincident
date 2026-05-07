package tui

import (
	"fmt"
	"regexp"
	"strings"
	"time"
)

func formatDuration(d time.Duration) string {
	total := int64(d / time.Second)
	h := total / 3600
	m := (total % 3600) / 60
	sec := total % 60
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

// wrapText wraps s to at most width runes per line, breaking on whitespace.
func wrapText(s string, width int) string {
	if width <= 0 {
		return s
	}
	words := strings.Fields(s)
	if len(words) == 0 {
		return ""
	}
	var lines []string
	line := words[0]
	for _, w := range words[1:] {
		if len([]rune(line))+1+len([]rune(w)) <= width {
			line += " " + w
		} else {
			lines = append(lines, line)
			line = w
		}
	}
	lines = append(lines, line)
	return strings.Join(lines, "\n")
}

var sqlKeywordRe = regexp.MustCompile(
	`(?i)\b(SELECT|DISTINCT|FROM|WHERE|JOIN|LEFT|RIGHT|INNER|OUTER|CROSS|FULL|ON|AND|OR|NOT|IN|EXISTS|HAVING|LIMIT|OFFSET|UNION|ALL|EXCEPT|INTERSECT|INSERT|INTO|UPDATE|DELETE|SET|VALUES|AS|WITH|CASE|WHEN|THEN|ELSE|END|NULL|IS|LIKE|ILIKE|BETWEEN|ASC|DESC|COUNT|SUM|AVG|MIN|MAX|COALESCE|NULLIF|CAST|INTERVAL|GROUP|ORDER|BY|RETURNING|USING|LATERAL|RECURSIVE|TRUE|FALSE)\b`,
)

func highlightSQL(line string) string {
	return sqlKeywordRe.ReplaceAllStringFunc(line, func(kw string) string {
		return sqlKeywordStyle.Render(strings.ToUpper(kw))
	})
}

func connPct(active, max int) string {
	if max == 0 {
		return "—"
	}
	return fmt.Sprintf("%d%%", active*100/max)
}
