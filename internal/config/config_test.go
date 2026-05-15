package config_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/shinagawa-web/pgincident/internal/config"
)

func writeTOML(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	f := filepath.Join(dir, "pgincident.toml")
	if err := os.WriteFile(f, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
	return f
}

func TestLoadFull(t *testing.T) {
	f := writeTOML(t, `
dsn = "postgres://u:p@localhost/db"

[thresholds]
long_running = "10s"
idle_in_transaction = "2m"
`)
	cfg, err := config.Load(f)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.DSN != "postgres://u:p@localhost/db" {
		t.Errorf("DSN = %q, want postgres://u:p@localhost/db", cfg.DSN)
	}
	if got := cfg.Thresholds.LongRunning.TimeDuration(); got != 10*time.Second {
		t.Errorf("LongRunning = %v, want 10s", got)
	}
	if got := cfg.Thresholds.IdleInTx.TimeDuration(); got != 2*time.Minute {
		t.Errorf("IdleInTx = %v, want 2m", got)
	}
}

func TestLoadDefaultThresholds(t *testing.T) {
	f := writeTOML(t, `dsn = "postgres://u:p@localhost/db"`)
	cfg, err := config.Load(f)
	if err != nil {
		t.Fatal(err)
	}
	if got := cfg.Thresholds.LongRunning.TimeDuration(); got != 5*time.Second {
		t.Errorf("LongRunning = %v, want 5s (default)", got)
	}
	if got := cfg.Thresholds.IdleInTx.TimeDuration(); got != 30*time.Second {
		t.Errorf("IdleInTx = %v, want 30s (default)", got)
	}
}

func TestLoadInvalidDuration(t *testing.T) {
	f := writeTOML(t, `
dsn = "postgres://u:p@localhost/db"
[thresholds]
long_running = "bad"
`)
	_, err := config.Load(f)
	if err == nil {
		t.Error("expected error for invalid duration, got nil")
	}
}

func TestLoadNegativeDuration(t *testing.T) {
	f := writeTOML(t, `
dsn = "postgres://u:p@localhost/db"
[thresholds]
long_running = "-5s"
`)
	_, err := config.Load(f)
	if err == nil {
		t.Error("expected error for negative duration, got nil")
	}
}

func TestLoadZeroDuration(t *testing.T) {
	f := writeTOML(t, `
dsn = "postgres://u:p@localhost/db"
[thresholds]
long_running = "0s"
`)
	_, err := config.Load(f)
	if err == nil {
		t.Error("expected error for zero duration, got nil")
	}
}

func TestLoadUnknownKey(t *testing.T) {
	f := writeTOML(t, `
dsn = "postgres://u:p@localhost/db"
[thresholds]
idle_in_transcation = "30s"
`)
	_, err := config.Load(f)
	if err == nil {
		t.Error("expected error for unknown key, got nil")
	}
}

func TestLoadMissingFile(t *testing.T) {
	_, err := config.Load("/nonexistent/path/.pgincident.toml")
	if err == nil {
		t.Error("expected error for missing file, got nil")
	}
}

func TestDurationTimeDuration(t *testing.T) {
	var d config.Duration
	if err := d.UnmarshalText([]byte("1m30s")); err != nil {
		t.Fatal(err)
	}
	if got := d.TimeDuration(); got != 90*time.Second {
		t.Errorf("TimeDuration = %v, want 1m30s", got)
	}
}

func TestDefaultPath(t *testing.T) {
	p, err := config.DefaultPath()
	if err != nil {
		t.Skip("home directory not available")
	}
	if filepath.Base(p) != ".pgincident.toml" {
		t.Errorf("DefaultPath base = %q, want .pgincident.toml", filepath.Base(p))
	}
}

func TestResolvePathExplicit(t *testing.T) {
	got, err := config.ResolvePath("/explicit/path.toml")
	if err != nil {
		t.Fatal(err)
	}
	if got != "/explicit/path.toml" {
		t.Errorf("got %q, want /explicit/path.toml", got)
	}
}

func TestResolvePathCurrentDir(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ".pgincident.toml"), []byte(""), 0600); err != nil {
		t.Fatal(err)
	}
	t.Chdir(dir)
	got, err := config.ResolvePath("")
	if err != nil {
		t.Fatal(err)
	}
	if got != ".pgincident.toml" {
		t.Errorf("got %q, want .pgincident.toml", got)
	}
}

func TestResolvePathFallbackHome(t *testing.T) {
	t.Chdir(t.TempDir())
	t.Setenv("HOME", t.TempDir())
	got, err := config.ResolvePath("")
	if err != nil {
		t.Fatal(err)
	}
	if got == ".pgincident.toml" {
		t.Error("got relative path, want absolute home path")
	}
	if filepath.Base(got) != ".pgincident.toml" {
		t.Errorf("got %q, want path ending in .pgincident.toml", got)
	}
}
