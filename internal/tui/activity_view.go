package tui

import (
	"fmt"
	"strings"

	"github.com/shinagawa-web/pgincident/internal/core"
)

const (
	colPID      = 8
	colUser     = 16
	colDuration = 16
	colState    = 14
	cursorPrefix = 2 // "▸ " or "  "
)

func renderActivitySection(activities []core.Activity, cursor int, active bool, maxRows, width int) string {
	colQuery := width - cursorPrefix - colPID - colUser - colDuration - colState
	if colQuery < 10 {
		colQuery = 10
	}

	lines := []string{
		sectionTitle("Long-running queries (> 5s)", fmt.Sprintf("[%d active]", len(activities)), active, width),
		"  " + colHeaderStyle.Render(
			padRight("PID", colPID)+
				padRight("USER", colUser)+
				padRight("DURATION", colDuration)+
				padRight("STATE", colState)+
				"QUERY",
		),
	}

	start := scrollOffset(cursor, maxRows)
	for i := start; i < len(activities) && i < start+maxRows; i++ {
		a := activities[i]
		row := padRight(fmt.Sprintf("%d", a.PID), colPID) +
			padRight(a.User, colUser) +
			padRight(formatDuration(a.Duration), colDuration) +
			padRight(a.State, colState) +
			truncate(a.Query, colQuery)
		if active && i == cursor {
			lines = append(lines, selectedRowStyle.Render("▸ "+row))
		} else {
			lines = append(lines, "  "+row)
		}
	}

	return padSection(lines, 2+maxRows)
}

func scrollOffset(cursor, maxRows int) int {
	if cursor >= maxRows {
		return cursor - maxRows + 1
	}
	return 0
}

func padSection(lines []string, total int) string {
	for len(lines) < total {
		lines = append(lines, "")
	}
	return strings.Join(lines, "\n")
}
