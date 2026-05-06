package core

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// mockPgxRows implements pgx.Rows for testing.
type mockPgxRows struct {
	data    [][]any
	current int
	err     error
	scanErr error
	closed  bool
}

func (m *mockPgxRows) Next() bool {
	m.current++
	return m.current <= len(m.data)
}

func (m *mockPgxRows) Scan(dest ...any) error {
	if m.scanErr != nil {
		return m.scanErr
	}
	row := m.data[m.current-1]
	for i, d := range dest {
		switch v := d.(type) {
		case *int:
			*v = row[i].(int)
		case *string:
			*v = row[i].(string)
		case *time.Time:
			*v = row[i].(time.Time)
		case *time.Duration:
			*v = row[i].(time.Duration)
		}
	}
	return nil
}

func (m *mockPgxRows) Err() error                                   { return m.err }
func (m *mockPgxRows) Close()                                       { m.closed = true }
func (m *mockPgxRows) CommandTag() pgconn.CommandTag                { return pgconn.CommandTag{} }
func (m *mockPgxRows) FieldDescriptions() []pgconn.FieldDescription { return nil }
func (m *mockPgxRows) Values() ([]any, error)                       { return nil, nil }
func (m *mockPgxRows) RawValues() [][]byte                          { return nil }
func (m *mockPgxRows) Conn() *pgx.Conn                              { return nil }

// mockPgxRow implements pgx.Row for testing.
type mockPgxRow struct {
	scanFn func(dest ...any) error
}

func (r *mockPgxRow) Scan(dest ...any) error { return r.scanFn(dest...) }

// mockConn implements pgConn for testing.
type mockConn struct {
	queryFn    func(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	queryRowFn func(ctx context.Context, sql string, args ...any) pgx.Row
	config     *pgx.ConnConfig
	closed     bool
}

func (m *mockConn) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	return m.queryFn(ctx, sql, args...)
}

func (m *mockConn) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	return m.queryRowFn(ctx, sql, args...)
}

func (m *mockConn) Close(_ context.Context) error {
	m.closed = true
	return nil
}

func (m *mockConn) Config() *pgx.ConnConfig {
	if m.config != nil {
		return m.config
	}
	return &pgx.ConnConfig{}
}
