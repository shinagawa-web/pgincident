package core

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
)

// pgConn is the subset of *pgx.Conn methods used by Client.
// *pgx.Conn directly satisfies this interface.
type pgConn interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Close(ctx context.Context) error
	Config() *pgx.ConnConfig
}

// connectFn can be overridden in tests.
var connectFn = func(ctx context.Context, dsn string) (pgConn, error) {
	return pgx.Connect(ctx, dsn)
}

// Client wraps a single pgx connection.
type Client struct {
	conn pgConn
}

// Connect opens a connection and verifies pg_monitor membership.
func Connect(ctx context.Context, dsn string) (*Client, error) {
	conn, err := connectFn(ctx, dsn)
	if err != nil {
		return nil, fmt.Errorf("connect: %w", err)
	}
	return newClient(ctx, conn)
}

func newClient(ctx context.Context, conn pgConn) (*Client, error) {
	var ok bool
	err := conn.QueryRow(ctx,
		"SELECT pg_has_role(current_user, 'pg_monitor', 'MEMBER')",
	).Scan(&ok)
	if err != nil {
		conn.Close(ctx)
		return nil, fmt.Errorf("pg_monitor check: %w", err)
	}
	if !ok {
		conn.Close(ctx)
		return nil, fmt.Errorf(
			"user %q is not a member of pg_monitor — grant it with: GRANT pg_monitor TO <user>",
			connUser(conn),
		)
	}
	return &Client{conn: conn}, nil
}

func (c *Client) Close(ctx context.Context) {
	c.conn.Close(ctx)
}

func connUser(conn pgConn) string {
	return conn.Config().User
}
