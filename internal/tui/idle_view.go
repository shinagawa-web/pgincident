package tui

import (
	"fmt"

	"github.com/shinagawa-web/pgincident/internal/core"
)

const colIdleTime = 13

func renderIdleSection(idle []core.Activity, cursor int, active bool, maxRows, width int) string {
	colQuery := width - cursorPrefix - colPID - colUser - colIdleTime
	if colQuery < 10 {
		colQuery = 10
	}

	lines := []string{
		sectionTitle("Idle in transaction (> 30s)", fmt.Sprintf("[%d idle]", len(idle)), active, width),
		"  " + colHeaderStyle.Render(
			padRight("PID", colPID)+
				padRight("USER", colUser)+
				padRight("IDLE TIME", colIdleTime)+
				"LAST QUERY",
		),
	}

	start := scrollOffset(cursor, maxRows)
	for i := start; i < len(idle) && i < start+maxRows; i++ {
		a := idle[i]
		row := padRight(fmt.Sprintf("%d", a.PID), colPID) +
			padRight(a.User, colUser) +
			padRight(formatDuration(a.Duration), colIdleTime) +
			truncate(oneLine(a.Query), colQuery)
		if active && i == cursor {
			lines = append(lines, selectedRowStyle.Render("▸ "+row))
		} else {
			lines = append(lines, "  "+row)
		}
	}

	return padSection(lines, 2+maxRows)
}
