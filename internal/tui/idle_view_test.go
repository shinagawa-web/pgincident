package tui

import (
	"strings"
	"testing"
	"time"

	"github.com/shinagawa-web/pgincident/internal/core"
)

func TestRenderIdleSectionEmpty(t *testing.T) {
	out := renderIdleSection(nil, 0, true, 5, 80, 30*time.Second)
	if !strings.Contains(out, "Idle in transaction") {
		t.Errorf("expected section title, got: %q", out)
	}
	if !strings.Contains(out, "[0 idle]") {
		t.Errorf("expected zero count badge, got: %q", out)
	}
}

func TestRenderIdleSectionWithData(t *testing.T) {
	idle := []core.Activity{
		{PID: 9999, User: "carol", Duration: 45 * time.Second, Query: "BEGIN"},
	}
	out := renderIdleSection(idle, 0, true, 5, 80, 30*time.Second)
	if !strings.Contains(out, "9999") {
		t.Errorf("expected PID 9999, got: %q", out)
	}
	if !strings.Contains(out, "carol") {
		t.Errorf("expected user carol, got: %q", out)
	}
	if !strings.Contains(out, "▸") {
		t.Errorf("expected cursor on selected row, got: %q", out)
	}
}

func TestRenderIdleSectionInactive(t *testing.T) {
	idle := []core.Activity{
		{PID: 9999, User: "carol", Duration: 45 * time.Second, Query: "BEGIN"},
	}
	out := renderIdleSection(idle, 0, false, 5, 80, 30*time.Second)
	if strings.Contains(out, "▸") {
		t.Errorf("inactive section should not show cursor, got: %q", out)
	}
}

func TestRenderIdleSectionNarrowWidth(t *testing.T) {
	idle := []core.Activity{
		{PID: 1, User: "u", Duration: time.Second, Query: "BEGIN"},
	}
	out := renderIdleSection(idle, 0, true, 3, 20, 30*time.Second)
	if out == "" {
		t.Error("expected non-empty output even on narrow width")
	}
}
