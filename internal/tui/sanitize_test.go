package tui

import "testing"

func TestU2_ConnClosed(t *testing.T) {
	if got := sanitizeConnError("conn closed"); got != "connection lost" {
		t.Errorf("sanitizeConnError(%q) = %q, want %q", "conn closed", got, "connection lost")
	}
}

func TestU3_ConnClosedWithPrefix(t *testing.T) {
	in := "failed to deallocate cached statement(s): conn closed"
	if got := sanitizeConnError(in); got != "connection lost" {
		t.Errorf("sanitizeConnError(%q) = %q, want %q", in, got, "connection lost")
	}
}

func TestU4_BrokenPipe(t *testing.T) {
	if got := sanitizeConnError("broken pipe"); got != "connection lost" {
		t.Errorf("sanitizeConnError(%q) = %q, want %q", "broken pipe", got, "connection lost")
	}
}

func TestU5_ConnectionResetByPeer(t *testing.T) {
	if got := sanitizeConnError("connection reset by peer"); got != "connection lost" {
		t.Errorf("sanitizeConnError(%q) = %q, want %q", "connection reset by peer", got, "connection lost")
	}
}

func TestU6_EOF(t *testing.T) {
	if got := sanitizeConnError("EOF"); got != "connection lost" {
		t.Errorf("sanitizeConnError(%q) = %q, want %q", "EOF", got, "connection lost")
	}
}

func TestU8_AdminShutdown(t *testing.T) {
	in := "FATAL: terminating connection due to administrator command (SQLSTATE 57P01)"
	if got := sanitizeConnError(in); got != "connection lost" {
		t.Errorf("sanitizeConnError(%q) = %q, want %q", in, got, "connection lost")
	}
}

func TestU9_ServerClosedUnexpectedly(t *testing.T) {
	in := "server closed the connection unexpectedly"
	if got := sanitizeConnError(in); got != "connection lost" {
		t.Errorf("sanitizeConnError(%q) = %q, want %q", in, got, "connection lost")
	}
}

func TestU7_OtherErrorPassthrough(t *testing.T) {
	cases := []string{
		"permission denied for table pg_stat_activity",
		"role \"pg_monitor\" does not exist",
		"syntax error at or near \"SELECT\"",
	}
	for _, in := range cases {
		got := sanitizeConnError(in)
		if got != in {
			t.Errorf("sanitizeConnError(%q) = %q, want unchanged", in, got)
		}
	}
}
