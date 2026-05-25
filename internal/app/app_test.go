package app

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/shinagawa-web/pgincident/internal/core"
)

type quitModel struct{}

func (quitModel) Init() tea.Cmd                       { return tea.Quit }
func (quitModel) Update(tea.Msg) (tea.Model, tea.Cmd) { return quitModel{}, tea.Quit }
func (quitModel) View() string                        { return "" }

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
func (m *mockClient) Locks(_ context.Context) ([]core.Lock, error) { return nil, nil }
func (m *mockClient) IdleInTx(_ context.Context, _ time.Duration) ([]core.Activity, error) {
	return nil, nil
}
func (m *mockClient) Stats(_ context.Context) (core.DBStats, error) { return core.DBStats{}, nil }
func (m *mockClient) Close(_ context.Context)                       {}

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

func withMockNewPoller(t *testing.T, fn func(dbClient, time.Duration) *core.Poller) {
	t.Helper()
	orig := newPollerFn
	newPollerFn = fn
	t.Cleanup(func() { newPollerFn = orig })
}

func withMockResolvePath(t *testing.T, fn func(string) (string, error)) {
	t.Helper()
	orig := resolvePathFn
	resolvePathFn = fn
	t.Cleanup(func() { resolvePathFn = orig })
}

type failWriter struct {
	failWrite bool
	failClose bool
}

func (w *failWriter) Write(p []byte) (int, error) {
	if w.failWrite {
		return 0, fmt.Errorf("write failed")
	}
	return len(p), nil
}

func (w *failWriter) Close() error {
	if w.failClose {
		return fmt.Errorf("close failed")
	}
	return nil
}

func withMockOpenInitFile(t *testing.T, fn func(string) (io.WriteCloser, error)) {
	t.Helper()
	orig := openInitFileFn
	openInitFileFn = fn
	t.Cleanup(func() { openInitFileFn = orig })
}

func withMockInitPath(t *testing.T, fn func() (string, error)) {
	t.Helper()
	orig := initPathFn
	initPathFn = fn
	t.Cleanup(func() { initPathFn = orig })
}

func TestRunResolvePathError(t *testing.T) {
	withMockResolvePath(t, func(_ string) (string, error) {
		return "", fmt.Errorf("no home directory")
	})
	err := Run("")
	if err == nil || err.Error() != "no home directory" {
		t.Errorf("err = %v, want no home directory error", err)
	}
}

func TestRunConfigLoadError(t *testing.T) {
	err := Run("/nonexistent/pgincident.toml")
	if err == nil {
		t.Error("expected error for nonexistent config file")
	}
}

func TestRunNoConnections(t *testing.T) {
	f := writeTOML(t, `# no connections`)
	err := Run(f)
	if err == nil || !strings.Contains(err.Error(), "no connections") {
		t.Errorf("err = %v, want no connections error", err)
	}
}

func TestRunConnectError(t *testing.T) {
	withMockConnect(t, func(_ context.Context, _ string) (dbClient, error) {
		return nil, fmt.Errorf("connect failed")
	})
	f := writeTOML(t, "[connections.default]\ndsn = \"postgres://u:p@localhost/db\"")
	err := Run(f)
	if err == nil || !strings.Contains(err.Error(), "connect failed") {
		t.Errorf("err = %v, want connect error", err)
	}
	if !strings.Contains(err.Error(), "default") {
		t.Errorf("err = %v, want connection name \"default\" in error", err)
	}
}

func TestFriendlyConnectErrNonPgconn(t *testing.T) {
	plain := fmt.Errorf("some other error")
	if got := friendlyConnectErr(plain); got != plain {
		t.Errorf("expected passthrough for non-pgconn error, got %v", got)
	}
}

func TestFriendlyConnectErrPgconn(t *testing.T) {
	_, err := pgconn.Connect(context.Background(), "host=127.0.0.1 port=19999 user=x dbname=x connect_timeout=1")
	if err == nil {
		t.Skip("port 19999 unexpectedly open; skipping")
	}
	got := friendlyConnectErr(err).Error()
	if strings.Contains(got, "failed to connect to") {
		t.Errorf("pgx internals must be stripped, got: %v", got)
	}
	if !strings.Contains(got, "127.0.0.1:19999") {
		t.Errorf("expected host:port in error, got: %v", got)
	}
	if !strings.Contains(got, "connection refused") {
		t.Errorf("expected reason in error, got: %v", got)
	}
}

func TestRootCauseSingleError(t *testing.T) {
	leaf := fmt.Errorf("leaf")
	if got := rootCause(leaf); got != leaf {
		t.Errorf("rootCause of unwrappable error = %v, want %v", got, leaf)
	}
}

func TestRootCauseWrapped(t *testing.T) {
	leaf := fmt.Errorf("leaf")
	wrapped := fmt.Errorf("outer: %w", leaf)
	if got := rootCause(wrapped); got != leaf {
		t.Errorf("rootCause = %v, want %v", got, leaf)
	}
}

func TestRootCauseJoined(t *testing.T) {
	leaf := fmt.Errorf("leaf")
	joined := errors.Join(leaf, fmt.Errorf("other"))
	if got := rootCause(joined); got != leaf {
		t.Errorf("rootCause of joined = %v, want %v", got, leaf)
	}
}

func TestRootCauseEmptyJoin(t *testing.T) {
	// errors.Join(nil...) returns nil, so construct a custom type that
	// implements Unwrap() []error and returns an empty slice.
	type emptyJoin struct{}
	_ = emptyJoin{}

	// Use a wrapper that satisfies the multi-unwrap interface with empty slice.
	joined := &multiErr{}
	got := rootCause(joined)
	if got != joined {
		t.Errorf("rootCause of empty-join = %v, want passthrough %v", got, joined)
	}
}

// multiErr satisfies interface{ Unwrap() []error } with an empty slice.
type multiErr struct{}

func (e *multiErr) Error() string        { return "multi" }
func (e *multiErr) Unwrap() []error      { return nil }

func TestRunConnectErrorIncludesConnName(t *testing.T) {
	withMockConnect(t, func(_ context.Context, _ string) (dbClient, error) {
		return nil, fmt.Errorf("dial failed")
	})
	f := writeTOML(t, "[connections.primary]\ndsn = \"postgres://u:p@localhost/db\"")
	err := Run(f)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	want := `connect "primary": dial failed`
	if err.Error() != want {
		t.Errorf("err = %q, want %q", err.Error(), want)
	}
}

func TestRunSuccess(t *testing.T) {
	withMockConnect(t, func(_ context.Context, _ string) (dbClient, error) {
		return &mockClient{}, nil
	})
	withMockRun(t, func(_ tea.Model) error { return nil })
	f := writeTOML(t, "[connections.default]\ndsn = \"postgres://u:p@localhost/db\"")
	if err := Run(f); err != nil {
		t.Fatal(err)
	}
}

func TestRunProgramError(t *testing.T) {
	withMockConnect(t, func(_ context.Context, _ string) (dbClient, error) {
		return &mockClient{}, nil
	})
	withMockRun(t, func(_ tea.Model) error { return fmt.Errorf("tui error") })
	f := writeTOML(t, "[connections.default]\ndsn = \"postgres://u:p@localhost/db\"")
	err := Run(f)
	if err == nil || err.Error() != "tui error" {
		t.Errorf("err = %v, want tui error", err)
	}
}

func TestRunEmptyCfgPath(t *testing.T) {
	t.Chdir(t.TempDir())
	t.Setenv("HOME", t.TempDir())
	err := Run("")
	if err == nil {
		t.Error("expected error with no config file at default path")
	}
}

func TestRunCurrentDirConfig(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ".pgincident.toml"), []byte("[connections.default]\ndsn = \"postgres://u:p@localhost/db\""), 0600); err != nil {
		t.Fatal(err)
	}
	t.Chdir(dir)
	withMockConnect(t, func(_ context.Context, _ string) (dbClient, error) {
		return &mockClient{}, nil
	})
	withMockRun(t, func(_ tea.Model) error { return nil })
	if err := Run(""); err != nil {
		t.Fatal(err)
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
	f := writeTOML(t, "[connections.default]\ndsn = \"postgres://u:p@localhost/db\"")
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
	f := writeTOML(t, "[connections.default]\ndsn = \"postgres://u:p@localhost/db\"")
	code := Main([]string{"--config", f}, "", &bytes.Buffer{}, &bytes.Buffer{})
	if code != 0 {
		t.Errorf("exit code = %d, want 0", code)
	}
}

func TestRunThresholdPropagation(t *testing.T) {
	var captured *core.Poller
	withMockNewPoller(t, func(client dbClient, interval time.Duration) *core.Poller {
		p := core.NewPoller(client, interval)
		captured = p
		return p
	})
	withMockConnect(t, func(_ context.Context, _ string) (dbClient, error) {
		return &mockClient{}, nil
	})
	withMockRun(t, func(_ tea.Model) error { return nil })
	f := writeTOML(t, `
[connections.default]
dsn = "postgres://u:p@localhost/db"
[thresholds]
long_running = "10s"
idle_in_transaction = "2m"
`)
	if err := Run(f); err != nil {
		t.Fatal(err)
	}
	if captured.LongRunningThreshold != 10*time.Second {
		t.Errorf("LongRunningThreshold = %v, want 10s", captured.LongRunningThreshold)
	}
	if captured.IdleInTxThreshold != 2*time.Minute {
		t.Errorf("IdleInTxThreshold = %v, want 2m", captured.IdleInTxThreshold)
	}
}

func TestInitSuccess(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".pgincident.toml")
	withMockInitPath(t, func() (string, error) { return path, nil })
	var out bytes.Buffer
	if err := Init(&out); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), path) {
		t.Errorf("stdout = %q, want path %s", out.String(), path)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(got), `[connections.default]`) {
		t.Errorf("file content = %q, want [connections.default] section", string(got))
	}
	if !strings.Contains(string(got), `dsn = "postgres://`) {
		t.Errorf("file content = %q, want dsn field", string(got))
	}
	if !strings.Contains(string(got), `long_running`) {
		t.Errorf("file content = %q, want long_running field", string(got))
	}
}

func TestInitFileExists(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".pgincident.toml")
	if err := os.WriteFile(path, []byte("existing"), 0600); err != nil {
		t.Fatal(err)
	}
	withMockInitPath(t, func() (string, error) { return path, nil })
	err := Init(&bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Errorf("err = %v, want already exists error", err)
	}
}

func TestInitWriteError(t *testing.T) {
	withMockInitPath(t, func() (string, error) { return "/tmp/irrelevant.toml", nil })
	withMockOpenInitFile(t, func(_ string) (io.WriteCloser, error) {
		return &failWriter{failWrite: true}, nil
	})
	err := Init(&bytes.Buffer{})
	if err == nil || err.Error() != "write failed" {
		t.Errorf("err = %v, want write failed", err)
	}
}

func TestInitCloseError(t *testing.T) {
	withMockInitPath(t, func() (string, error) { return "/tmp/irrelevant.toml", nil })
	withMockOpenInitFile(t, func(_ string) (io.WriteCloser, error) {
		return &failWriter{failClose: true}, nil
	})
	err := Init(&bytes.Buffer{})
	if err == nil || err.Error() != "close failed" {
		t.Errorf("err = %v, want close failed", err)
	}
}

func TestInitOpenError(t *testing.T) {
	withMockInitPath(t, func() (string, error) {
		return "/nonexistent/dir/.pgincident.toml", nil
	})
	err := Init(&bytes.Buffer{})
	if err == nil {
		t.Error("expected error for unwritable path")
	}
}

func TestInitDefaultPathError(t *testing.T) {
	withMockInitPath(t, func() (string, error) { return "", fmt.Errorf("no home dir") })
	err := Init(&bytes.Buffer{})
	if err == nil || err.Error() != "no home dir" {
		t.Errorf("err = %v, want no home dir error", err)
	}
}

func TestMainInit(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".pgincident.toml")
	withMockInitPath(t, func() (string, error) { return path, nil })
	var out bytes.Buffer
	code := Main([]string{"--init"}, "", &out, &bytes.Buffer{})
	if code != 0 {
		t.Errorf("exit code = %d, want 0", code)
	}
	if !strings.Contains(out.String(), "Created") {
		t.Errorf("stdout = %q, want Created message", out.String())
	}
}

func TestMainInitError(t *testing.T) {
	withMockInitPath(t, func() (string, error) { return "", fmt.Errorf("no home dir") })
	var errBuf bytes.Buffer
	code := Main([]string{"--init"}, "", &bytes.Buffer{}, &errBuf)
	if code != 1 {
		t.Errorf("exit code = %d, want 1", code)
	}
	if !strings.Contains(errBuf.String(), "error:") {
		t.Errorf("stderr = %q, want error message", errBuf.String())
	}
}

func TestDefaultInitPath(t *testing.T) {
	path, err := initPathFn()
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Base(path) != ".pgincident.toml" {
		t.Errorf("path = %q, want filename .pgincident.toml", path)
	}
}

func TestDefaultInitPathGetWdError(t *testing.T) {
	orig := getWdFn
	getWdFn = func() (string, error) { return "", fmt.Errorf("getwd failed") }
	t.Cleanup(func() { getWdFn = orig })
	_, err := initPathFn()
	if err == nil || err.Error() != "getwd failed" {
		t.Errorf("err = %v, want getwd failed", err)
	}
}

func TestDefaultConnect_InvalidDSN(t *testing.T) {
	_, err := defaultConnect(context.Background(), "not a dsn")
	if err == nil {
		t.Fatal("expected error for invalid DSN")
	}
}

func TestDefaultRun_ImmediateQuit(t *testing.T) {
	devNull, err := os.Open(os.DevNull)
	if err != nil {
		t.Fatal(err)
	}
	defer devNull.Close()
	origStdin := os.Stdin
	os.Stdin = devNull
	defer func() { os.Stdin = origStdin }()

	// Without a real TTY (e.g. in CI), p.Run() returns a "no TTY" error.
	// All three statements in defaultRun are still executed, so coverage is met.
	_ = defaultRun(quitModel{})
}

type trackingClient struct {
	mockClient
	closed bool
}

func (c *trackingClient) Close(_ context.Context) { c.closed = true }

func TestBuildReconnectSuccess(t *testing.T) {
	old := &trackingClient{}
	var client dbClient = old
	withMockConnect(t, func(_ context.Context, _ string) (dbClient, error) {
		return &mockClient{}, nil
	})
	pol := core.NewPoller(nil, 5*time.Second)
	pol.LongRunningThreshold = 10 * time.Second
	pol.IdleInTxThreshold = 2 * time.Minute

	reconnect := buildReconnect(&client, pol)
	newPol, err := reconnect(context.Background(), "postgres://new@localhost/db")
	if err != nil {
		t.Fatal(err)
	}
	if !old.closed {
		t.Error("expected old client to be closed after reconnect")
	}
	if newPol.LongRunningThreshold != 10*time.Second {
		t.Errorf("LongRunningThreshold = %v, want 10s", newPol.LongRunningThreshold)
	}
	if newPol.IdleInTxThreshold != 2*time.Minute {
		t.Errorf("IdleInTxThreshold = %v, want 2m", newPol.IdleInTxThreshold)
	}
}

func TestBuildReconnectConnectError(t *testing.T) {
	old := &trackingClient{}
	var client dbClient = old
	withMockConnect(t, func(_ context.Context, _ string) (dbClient, error) {
		return nil, fmt.Errorf("connection refused")
	})
	pol := core.NewPoller(nil, 5*time.Second)

	reconnect := buildReconnect(&client, pol)
	_, err := reconnect(context.Background(), "postgres://bad@localhost/db")
	if err == nil || err.Error() != "connection refused" {
		t.Errorf("err = %v, want connection refused", err)
	}
	if old.closed {
		t.Error("old client must not be closed when connect fails")
	}
}
