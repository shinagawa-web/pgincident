package tui

import (
	"fmt"

	"github.com/shinagawa-web/pgincident/internal/core"
)

const (
	colBlocked  = 8  // "BLOCKED"(7) + 1
	colBlocking = 9  // "BLOCKING"(8) + 1
	colWait     = 13 // "WAIT TIME"(9) + 4
	colRelation = 20
)

func renderLocksSection(locks []core.Lock, cursor int, active bool, maxRows, width int) string {
	colMode := width - cursorPrefix - colBlocked - colBlocking - colWait - colRelation
	if colMode < 10 {
		colMode = 10
	}

	lines := []string{
		sectionTitle("Locks (waiting)", fmt.Sprintf("[%d waiting]", len(locks)), active, width),
		"  " + colHeaderStyle.Render(
			padRight("BLOCKED", colBlocked)+
				padRight("BLOCKING", colBlocking)+
				padRight("WAIT TIME", colWait)+
				padRight("RELATION", colRelation)+
				"MODE",
		),
	}

	start := scrollOffset(cursor, maxRows)
	for i := start; i < len(locks) && i < start+maxRows; i++ {
		l := locks[i]
		row := padRight(fmt.Sprintf("%d", l.BlockedPID), colBlocked) +
			padRight(fmt.Sprintf("%d", l.BlockingPID), colBlocking) +
			padRight(formatDuration(l.WaitTime), colWait) +
			padRight(l.Relation, colRelation) +
			truncate(l.Mode, colMode)
		if active && i == cursor {
			lines = append(lines, selectedRowStyle.Render("▸ "+row))
		} else {
			lines = append(lines, "  "+row)
		}
	}

	return padSection(lines, 2+maxRows)
}
