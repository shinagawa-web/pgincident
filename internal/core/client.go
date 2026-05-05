package core

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
)

// Client wraps a single pgx connection.
type Client struct {
	conn *pgx.Conn
}

// Connect opens a connection and verifies pg_monitor membership.
func Connect(ctx context.Context, dsn string) (*Client, error) {
	conn, err := pgx.Connect(ctx, dsn)
	if err != nil {
		return nil, fmt.Errorf("connect: %w", err)
	}

	var ok bool
	err = conn.QueryRow(ctx,
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

func connUser(conn *pgx.Conn) string {
	return conn.Config().User
}
