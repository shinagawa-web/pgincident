package tui

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/shinagawa-web/pgincident/internal/core"
)

func TestRenderTitleBarNoServer(t *testing.T) {
	s := core.Snapshot{}
	out := renderTitleBar(s, time.Second, 80)
	if !strings.Contains(out, "pgincident") {
		t.Errorf("expected pgincident in title bar, got: %q", out)
	}
}

func TestRenderTitleBarNarrowWidth(t *testing.T) {
	s := core.Snapshot{ServerAddr: "localhost:5432", PGVersion: "16.1"}
	out := renderTitleBar(s, time.Second, 1) // very narrow → gap < 1 path
	if out == "" {
		t.Error("expected non-empty output even on narrow width")
	}
}

func TestRenderTitleBarWithServer(t *testing.T) {
	s := core.Snapshot{ServerAddr: "localhost:5432", PGVersion: "16.1"}
	out := renderTitleBar(s, 2*time.Second, 80)
	if !strings.Contains(out, "localhost:5432") {
		t.Errorf("expected server addr in title bar, got: %q", out)
	}
	if !strings.Contains(out, "16.1") {
		t.Errorf("expected PG version in title bar, got: %q", out)
	}
	if !strings.Contains(out, "2.0s") {
		t.Errorf("expected interval in title bar, got: %q", out)
	}
}

func TestRenderStatsBarNoTPS(t *testing.T) {
	s := core.DBStats{ConnectionsActive: 10, ConnectionsMax: 100, TPS: 0, CacheHitRatio: 0.99}
	out := renderStatsBar(s, 80)
	if !strings.Contains(out, "10/100") {
		t.Errorf("expected connections in stats bar, got: %q", out)
	}
	if !strings.Contains(out, "TPS: —") {
		t.Errorf("expected TPS dash in stats bar, got: %q", out)
	}
	if !strings.Contains(out, "99.0%") {
		t.Errorf("expected cache hit in stats bar, got: %q", out)
	}
}

func TestRenderStatsBarWithTPS(t *testing.T) {
	s := core.DBStats{ConnectionsActive: 50, ConnectionsMax: 200, TPS: 1234, CacheHitRatio: 0.995}
	out := renderStatsBar(s, 80)
	if !strings.Contains(out, "1234") {
		t.Errorf("expected TPS value in stats bar, got: %q", out)
	}
}

func TestRenderStatusError(t *testing.T) {
	out := renderStatus(errors.New("connection lost"), "")
	if !strings.Contains(out, "connection lost") {
		t.Errorf("expected error message, got: %q", out)
	}
}

func TestRenderStatusMsg(t *testing.T) {
	out := renderStatus(nil, "interval: 2.0s")
	if !strings.Contains(out, "interval: 2.0s") {
		t.Errorf("expected status message, got: %q", out)
	}
}

func TestRenderStatusEmpty(t *testing.T) {
	out := renderStatus(nil, "")
	if out != "" {
		t.Errorf("expected empty status, got: %q", out)
	}
}

func TestRenderFooter(t *testing.T) {
	out := renderFooter()
	if !strings.Contains(out, "[q]uit") {
		t.Errorf("expected footer content, got: %q", out)
	}
}

func TestSectionTitleActive(t *testing.T) {
	out := sectionTitle("Long-running queries (> 5s)", "[5 active]", true, 80)
	if !strings.Contains(out, "Long-running queries") {
		t.Errorf("expected label in title, got: %q", out)
	}
	if !strings.Contains(out, "[5 active]") {
		t.Errorf("expected badge in title, got: %q", out)
	}
}

func TestSectionTitleInactive(t *testing.T) {
	out := sectionTitle("Locks (waiting)", "[0 waiting]", false, 80)
	if !strings.Contains(out, "Locks") {
		t.Errorf("expected label in title, got: %q", out)
	}
}
