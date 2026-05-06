package core

import (
	"errors"
	"testing"
	"time"
)

type mockRows struct {
	data    [][]any
	current int
	err     error
	closed  bool
}

func (m *mockRows) Next() bool {
	m.current++
	return m.current <= len(m.data)
}

func (m *mockRows) Scan(dest ...any) error {
	if m.err != nil {
		return m.err
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

func (m *mockRows) Err() error  { return nil }
func (m *mockRows) Close()      { m.closed = true }

func TestScanActivitiesEmpty(t *testing.T) {
	rows := &mockRows{}
	activities, err := scanActivities(rows)
	if err != nil {
		t.Fatal(err)
	}
	if len(activities) != 0 {
		t.Errorf("got %d activities, want 0", len(activities))
	}
	if !rows.closed {
		t.Error("rows.Close() not called")
	}
}

func TestScanActivitiesMultipleRows(t *testing.T) {
	now := time.Now()
	rows := &mockRows{
		data: [][]any{
			{1234, "alice", "mydb", "active", now, 5 * time.Second, "SELECT 1", "app", "127.0.0.1"},
			{5678, "bob", "mydb", "active", now, 10 * time.Second, "SELECT 2", "app", "127.0.0.1"},
		},
	}
	activities, err := scanActivities(rows)
	if err != nil {
		t.Fatal(err)
	}
	if len(activities) != 2 {
		t.Fatalf("got %d activities, want 2", len(activities))
	}
	if activities[0].PID != 1234 || activities[0].User != "alice" {
		t.Errorf("unexpected first row: %+v", activities[0])
	}
	if activities[1].Duration != 10*time.Second {
		t.Errorf("unexpected duration: %v", activities[1].Duration)
	}
}

func TestScanActivitiesScanError(t *testing.T) {
	now := time.Now()
	rows := &mockRows{
		data: [][]any{
			{1234, "alice", "mydb", "active", now, 5 * time.Second, "SELECT 1", "app", "127.0.0.1"},
		},
		err: errors.New("scan error"),
	}
	_, err := scanActivities(rows)
	if err == nil {
		t.Error("expected scan error, got nil")
	}
}
