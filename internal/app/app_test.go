package app

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/shinagawa-web/pgincident/internal/core"
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

type mockClient struct{}

func (m *mockClient) ServerInfo(_ context.Context) (string, string, error) { return "", "", nil }
func (m *mockClient) LongRunning(_ context.Context, _ time.Duration) ([]core.Activity, error) {
	return nil, nil
}
func (m *mockClient) Locks(_ context.Context) ([]core.Lock, error)  { return nil, nil }
func (m *mockClient) IdleInTx(_ context.Context, _ time.Duration) ([]core.Activity, error) {
	return nil, nil
}
func (m *mockClient) Stats(_ context.Context) (core.DBStats, error) { return core.DBStats{}, nil }
func (m *mockClient) Close(_ context.Context)                        {}

func withMockConnect(t *testing.T, fn func(ctx context.Context, dsn string) (dbClient, error)) {
	t.Helper()
	orig := connectFn
	connectFn = fn
	t.Cleanup(func() { connectFn = orig })
}

func withMockRun(t *testing.T, fn func(tea.Model) error) {
	t.Helper()
	orig := runFn
	runFn = fn
	t.Cleanup(func() { runFn = orig })
}

func TestRunConfigLoadError(t *testing.T) {
	err := Run("/nonexistent/pgincident.toml")
	if err == nil {
		t.Error("expected error for nonexistent config file")
	}
}

func TestRunNoDSN(t *testing.T) {
	f := writeTOML(t, `# no dsn`)
	err := Run(f)
	if err == nil || !strings.Contains(err.Error(), "no DSN") {
		t.Errorf("err = %v, want DSN error", err)
	}
}

func TestRunConnectError(t *testing.T) {
	withMockConnect(t, func(_ context.Context, _ string) (dbClient, error) {
		return nil, fmt.Errorf("connect failed")
	})
	f := writeTOML(t, `dsn = "postgres://u:p@localhost/db"`)
	err := Run(f)
	if err == nil || !strings.Contains(err.Error(), "connect failed") {
		t.Errorf("err = %v, want connect error", err)
	}
}

func TestRunSuccess(t *testing.T) {
	withMockConnect(t, func(_ context.Context, _ string) (dbClient, error) {
		return &mockClient{}, nil
	})
	withMockRun(t, func(_ tea.Model) error { return nil })
	f := writeTOML(t, `dsn = "postgres://u:p@localhost/db"`)
	if err := Run(f); err != nil {
		t.Fatal(err)
	}
}

func TestRunProgramError(t *testing.T) {
	withMockConnect(t, func(_ context.Context, _ string) (dbClient, error) {
		return &mockClient{}, nil
	})
	withMockRun(t, func(_ tea.Model) error { return fmt.Errorf("tui error") })
	f := writeTOML(t, `dsn = "postgres://u:p@localhost/db"`)
	err := Run(f)
	if err == nil || err.Error() != "tui error" {
		t.Errorf("err = %v, want tui error", err)
	}
}

func TestRunEmptyCfgPath(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	err := Run("")
	if err == nil {
		t.Error("expected error with no config file at default path")
	}
}

func TestMainVersion(t *testing.T) {
	var out bytes.Buffer
	code := Main([]string{"--version"}, "1.2.3", &out, &bytes.Buffer{})
	if code != 0 {
		t.Errorf("exit code = %d, want 0", code)
	}
	if !strings.Contains(out.String(), "1.2.3") {
		t.Errorf("stdout = %q, want version string", out.String())
	}
}

func TestMainHelp(t *testing.T) {
	var out bytes.Buffer
	code := Main([]string{"--help"}, "", &out, &bytes.Buffer{})
	if code != 0 {
		t.Errorf("exit code = %d, want 0", code)
	}
	if !strings.Contains(out.String(), "Usage:") {
		t.Errorf("stdout = %q, want help text", out.String())
	}
}

func TestMainFlagError(t *testing.T) {
	var errBuf bytes.Buffer
	code := Main([]string{"--unknown"}, "", &bytes.Buffer{}, &errBuf)
	if code != 1 {
		t.Errorf("exit code = %d, want 1", code)
	}
	if !strings.Contains(errBuf.String(), "error:") {
		t.Errorf("stderr = %q, want error message", errBuf.String())
	}
}

func TestMainRunError(t *testing.T) {
	withMockConnect(t, func(_ context.Context, _ string) (dbClient, error) {
		return nil, fmt.Errorf("db down")
	})
	f := writeTOML(t, `dsn = "postgres://u:p@localhost/db"`)
	var errBuf bytes.Buffer
	code := Main([]string{"--config", f}, "", &bytes.Buffer{}, &errBuf)
	if code != 1 {
		t.Errorf("exit code = %d, want 1", code)
	}
	if !strings.Contains(errBuf.String(), "error:") {
		t.Errorf("stderr = %q, want error message", errBuf.String())
	}
}

func TestMainSuccess(t *testing.T) {
	withMockConnect(t, func(_ context.Context, _ string) (dbClient, error) {
		return &mockClient{}, nil
	})
	withMockRun(t, func(_ tea.Model) error { return nil })
	f := writeTOML(t, `dsn = "postgres://u:p@localhost/db"`)
	code := Main([]string{"--config", f}, "", &bytes.Buffer{}, &bytes.Buffer{})
	if code != 0 {
		t.Errorf("exit code = %d, want 0", code)
	}
}
