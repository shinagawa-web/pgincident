package tui

import "strings"

// sanitizeConnError maps pgx-level connection error strings to "connection lost".
// Returns errStr unchanged for any other error.
func sanitizeConnError(errStr string) string {
	connSignals := []string{
		"conn closed",
		"broken pipe",
		"connection reset by peer",
		"EOF",
	}
	for _, sig := range connSignals {
		if strings.Contains(errStr, sig) {
			return "connection lost"
		}
	}
	return errStr
}
