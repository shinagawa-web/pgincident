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

// oneLine collapses all whitespace runs (including newlines) to a single space.
func oneLine(s string) string {
	return strings.Join(strings.Fields(s), " ")
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

// clauseRe matches major SQL clause keywords preceded by a space.
// Longer alternatives are listed first to avoid partial matches (e.g. LEFT JOIN before JOIN).
var clauseRe = regexp.MustCompile(
	`(?i) (SELECT|UNION ALL|LEFT OUTER JOIN|RIGHT OUTER JOIN|FULL OUTER JOIN|LEFT JOIN|RIGHT JOIN|INNER JOIN|CROSS JOIN|GROUP BY|ORDER BY|FROM|WHERE|JOIN|HAVING|LIMIT|OFFSET|UNION|EXCEPT|INTERSECT|RETURNING)\b`,
)

// formatSQL breaks a single-line SQL string at clause boundaries and word-wraps
// each clause to width. Continuation lines are indented by 2 spaces.
func formatSQL(s string, width int) string {
	s = strings.Join(strings.Fields(s), " ")
	s = clauseRe.ReplaceAllString(s, "\n$1 ")

	var lines []string
	for _, clause := range strings.Split(strings.TrimSpace(s), "\n") {
		clause = strings.TrimSpace(clause)
		if clause == "" {
			continue
		}
		parts := strings.Split(wrapText(clause, width), "\n")
		lines = append(lines, parts[0])
		for _, cont := range parts[1:] {
			lines = append(lines, "  "+cont)
		}
	}
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
