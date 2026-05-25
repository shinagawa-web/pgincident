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
		"terminating connection",  // SQLSTATE 57P01/57P02: admin or crash shutdown
		"server closed the connection unexpectedly",
	}
	for _, sig := range connSignals {
		if strings.Contains(errStr, sig) {
			return "connection lost"
		}
	}
	return errStr
}
