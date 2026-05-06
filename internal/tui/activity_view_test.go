package tui

import (
	"strings"
	"testing"
	"time"

	"github.com/shinagawa-web/pgincident/internal/core"
)

func TestScrollOffset(t *testing.T) {
	cases := []struct {
		cursor, maxRows, want int
	}{
		{0, 5, 0},
		{4, 5, 0},
		{5, 5, 1},
		{9, 5, 5},
	}
	for _, c := range cases {
		got := scrollOffset(c.cursor, c.maxRows)
		if got != c.want {
			t.Errorf("scrollOffset(%d, %d) = %d, want %d", c.cursor, c.maxRows, got, c.want)
		}
	}
}

func TestPadSection(t *testing.T) {
	lines := []string{"a", "b"}
	out := padSection(lines, 5)
	parts := strings.Split(out, "\n")
	if len(parts) != 5 {
		t.Errorf("padSection: got %d lines, want 5", len(parts))
	}
}

func TestPadSectionAlreadyFull(t *testing.T) {
	lines := []string{"a", "b", "c"}
	out := padSection(lines, 3)
	parts := strings.Split(out, "\n")
	if len(parts) != 3 {
		t.Errorf("padSection: got %d lines, want 3", len(parts))
	}
}

func TestRenderActivitySectionEmpty(t *testing.T) {
	out := renderActivitySection(nil, 0, true, 5, 80)
	if !strings.Contains(out, "Long-running queries") {
		t.Errorf("expected section title, got: %q", out)
	}
	if !strings.Contains(out, "[0 active]") {
		t.Errorf("expected zero count badge, got: %q", out)
	}
}

func TestRenderActivitySectionWithData(t *testing.T) {
	activities := []core.Activity{
		{PID: 1234, User: "alice", State: "active", Duration: 6 * time.Second, Query: "SELECT 1"},
		{PID: 5678, User: "bob", State: "active", Duration: 12 * time.Second, Query: "UPDATE t SET x=1"},
	}
	out := renderActivitySection(activities, 0, true, 5, 80)
	if !strings.Contains(out, "1234") {
		t.Errorf("expected PID 1234, got: %q", out)
	}
	if !strings.Contains(out, "alice") {
		t.Errorf("expected user alice, got: %q", out)
	}
	if !strings.Contains(out, "▸") {
		t.Errorf("expected cursor on selected row, got: %q", out)
	}
}

func TestRenderActivitySectionInactive(t *testing.T) {
	activities := []core.Activity{
		{PID: 1234, User: "alice", State: "active", Duration: 6 * time.Second, Query: "SELECT 1"},
	}
	out := renderActivitySection(activities, 0, false, 5, 80)
	if strings.Contains(out, "▸") {
		t.Errorf("inactive section should not show cursor, got: %q", out)
	}
}

func TestRenderActivitySectionNarrowWidth(t *testing.T) {
	activities := []core.Activity{
		{PID: 1, User: "u", State: "active", Duration: time.Second, Query: "SELECT 1"},
	}
	// width too narrow: colQuery falls back to minimum 10
	out := renderActivitySection(activities, 0, true, 3, 20)
	if out == "" {
		t.Error("expected non-empty output even on narrow width")
	}
}
