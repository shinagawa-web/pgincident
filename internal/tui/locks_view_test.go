package tui

import (
	"strings"
	"testing"
	"time"

	"github.com/shinagawa-web/pgincident/internal/core"
)

func TestRenderLocksSectionEmpty(t *testing.T) {
	out := renderLocksSection(nil, 0, true, 5, 80)
	if !strings.Contains(out, "Locks (waiting)") {
		t.Errorf("expected section title, got: %q", out)
	}
	if !strings.Contains(out, "[0 waiting]") {
		t.Errorf("expected zero count badge, got: %q", out)
	}
}

func TestRenderLocksSectionWithData(t *testing.T) {
	locks := []core.Lock{
		{BlockedPID: 100, BlockingPID: 200, WaitTime: 5 * time.Second, Relation: "public.users", Mode: "ShareLock"},
	}
	out := renderLocksSection(locks, 0, true, 5, 80)
	if !strings.Contains(out, "100") {
		t.Errorf("expected blocked PID, got: %q", out)
	}
	if !strings.Contains(out, "200") {
		t.Errorf("expected blocking PID, got: %q", out)
	}
	if !strings.Contains(out, "▸") {
		t.Errorf("expected cursor on selected row, got: %q", out)
	}
}

func TestRenderLocksSectionInactive(t *testing.T) {
	locks := []core.Lock{
		{BlockedPID: 100, BlockingPID: 200, WaitTime: time.Second, Relation: "t", Mode: "ShareLock"},
	}
	out := renderLocksSection(locks, 0, false, 5, 80)
	if strings.Contains(out, "▸") {
		t.Errorf("inactive section should not show cursor, got: %q", out)
	}
}

func TestRenderLocksSectionNarrowWidth(t *testing.T) {
	locks := []core.Lock{
		{BlockedPID: 1, BlockingPID: 2, WaitTime: time.Second, Relation: "t", Mode: "ShareLock"},
	}
	out := renderLocksSection(locks, 0, true, 3, 20)
	if out == "" {
		t.Error("expected non-empty output even on narrow width")
	}
}
