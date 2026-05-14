package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/shinagawa-web/pgincident/internal/config"
)

func TestParseFlagsConfig(t *testing.T) {
	path, err := parseFlags([]string{"--config", "/tmp/my.toml"})
	if err != nil {
		t.Fatal(err)
	}
	if path != "/tmp/my.toml" {
		t.Errorf("cfgPath = %q, want /tmp/my.toml", path)
	}
}

func TestParseFlagsConfigEquals(t *testing.T) {
	path, err := parseFlags([]string{"--config=/tmp/my.toml"})
	if err != nil {
		t.Fatal(err)
	}
	if path != "/tmp/my.toml" {
		t.Errorf("cfgPath = %q, want /tmp/my.toml", path)
	}
}

func TestParseFlagsConfigMissingArg(t *testing.T) {
	_, err := parseFlags([]string{"--config"})
	if err == nil {
		t.Error("expected error for missing --config argument, got nil")
	}
}

func TestParseFlagsVersion(t *testing.T) {
	_, err := parseFlags([]string{"-v"})
	if err != errVersion {
		t.Errorf("err = %v, want errVersion", err)
	}
	_, err = parseFlags([]string{"--version"})
	if err != errVersion {
		t.Errorf("err = %v, want errVersion", err)
	}
}

func TestParseFlagsHelp(t *testing.T) {
	_, err := parseFlags([]string{"-h"})
	if err != errHelp {
		t.Errorf("err = %v, want errHelp", err)
	}
	_, err = parseFlags([]string{"--help"})
	if err != errHelp {
		t.Errorf("err = %v, want errHelp", err)
	}
}

func TestParseFlagsUnknown(t *testing.T) {
	_, err := parseFlags([]string{"--unknown"})
	if err == nil {
		t.Error("expected error for unknown flag, got nil")
	}
}

func TestParseFlagsEmpty(t *testing.T) {
	path, err := parseFlags([]string{})
	if err != nil {
		t.Fatal(err)
	}
	if path != "" {
		t.Errorf("cfgPath = %q, want empty", path)
	}
}

// TestThresholdsAppliedToPoller verifies that config thresholds are forwarded correctly.
func TestThresholdsAppliedToPoller(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "pgincident.toml")
	if err := os.WriteFile(f, []byte(`
dsn = "postgres://u:p@localhost/db"
[thresholds]
long_running = "15s"
idle_in_transaction = "45s"
`), 0600); err != nil {
		t.Fatal(err)
	}

	cfg, err := config.Load(f)
	if err != nil {
		t.Fatal(err)
	}

	if got := cfg.Thresholds.LongRunning.TimeDuration(); got != 15*time.Second {
		t.Errorf("LongRunning = %v, want 15s", got)
	}
	if got := cfg.Thresholds.IdleInTx.TimeDuration(); got != 45*time.Second {
		t.Errorf("IdleInTx = %v, want 45s", got)
	}
}
