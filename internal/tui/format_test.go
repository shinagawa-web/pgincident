package tui

import (
	"testing"
	"time"
)

func TestFormatDuration(t *testing.T) {
	cases := []struct {
		d    time.Duration
		want string
	}{
		{0, "00:00:00"},
		{5 * time.Second, "00:00:05"},
		{90 * time.Second, "00:01:30"},
		{2*time.Hour + 14*time.Minute + 32*time.Second, "02:14:32"},
		{500 * time.Millisecond, "00:00:00"},
		{119999 * time.Millisecond, "00:01:59"},
	}
	for _, c := range cases {
		got := formatDuration(c.d)
		if got != c.want {
			t.Errorf("formatDuration(%v) = %q, want %q", c.d, got, c.want)
		}
	}
}

func TestTruncate(t *testing.T) {
	cases := []struct {
		s    string
		n    int
		want string
	}{
		{"hello", 10, "hello"},
		{"hello", 5, "hello"},
		{"hello", 4, "hel…"},
		{"hello", 1, "h"},
		{"hello", 0, ""},
		{"日本語テスト", 4, "日本語…"},
		{"日本語テスト", 6, "日本語テスト"},
	}
	for _, c := range cases {
		got := truncate(c.s, c.n)
		if got != c.want {
			t.Errorf("truncate(%q, %d) = %q, want %q", c.s, c.n, got, c.want)
		}
	}
}

func TestPadRight(t *testing.T) {
	cases := []struct {
		s    string
		n    int
		want string
	}{
		{"hi", 5, "hi   "},
		{"hello", 5, "hello"},
		{"toolong", 4, "too…"},
		{"toolong", 1, "t"},
		{"日本語", 5, "日本語  "},
		{"日本語テスト", 4, "日本語…"},
	}
	for _, c := range cases {
		got := padRight(c.s, c.n)
		if got != c.want {
			t.Errorf("padRight(%q, %d) = %q, want %q", c.s, c.n, got, c.want)
		}
	}
}

func TestOneLine(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"SELECT 1", "SELECT 1"},
		{"SELECT\n1", "SELECT 1"},
		{"WITH paused AS (SELECT pg_sleep(60))\nSELECT id FROM t", "WITH paused AS (SELECT pg_sleep(60)) SELECT id FROM t"},
		{"  multiple   spaces  ", "multiple spaces"},
		{"", ""},
	}
	for _, c := range cases {
		got := oneLine(c.in)
		if got != c.want {
			t.Errorf("oneLine(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestWrapText(t *testing.T) {
	cases := []struct {
		s     string
		width int
		want  string
	}{
		{"hello world", 20, "hello world"},
		{"hello world", 5, "hello\nworld"},
		{"one two three", 7, "one two\nthree"},
		{"", 10, ""},
		{"single", 6, "single"},
		{"a b c", 1, "a\nb\nc"},
		{"hello world", 0, "hello world"},
	}
	for _, c := range cases {
		got := wrapText(c.s, c.width)
		if got != c.want {
			t.Errorf("wrapText(%q, %d) = %q, want %q", c.s, c.width, got, c.want)
		}
	}
}

func TestHighlightSQL(t *testing.T) {
	// With NO_COLOR=1 (set by TestMain), lipgloss returns plain text.
	cases := []struct {
		in   string
		want string
	}{
		{"SELECT 1", "SELECT 1"},
		{"select count(*) from users", "SELECT COUNT(*) FROM users"},
		{"WHERE id = 1 AND status = 'active'", "WHERE id = 1 AND status = 'active'"},
		{"no keywords here", "no keywords here"},
		{"", ""},
	}
	for _, c := range cases {
		got := highlightSQL(c.in)
		if got != c.want {
			t.Errorf("highlightSQL(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestFormatSQL(t *testing.T) {
	cases := []struct {
		name  string
		s     string
		width int
		want  string
	}{
		{
			name:  "clause breaks",
			s:     "SELECT id FROM users WHERE status = 'active' ORDER BY id LIMIT 10",
			width: 80,
			want:  "SELECT id\nFROM users\nWHERE status = 'active'\nORDER BY id\nLIMIT 10",
		},
		{
			name:  "join variants",
			s:     "SELECT a.id FROM a LEFT JOIN b ON b.a_id = a.id INNER JOIN c ON c.id = b.c_id",
			width: 80,
			want:  "SELECT a.id\nFROM a\nLEFT JOIN b ON b.a_id = a.id\nINNER JOIN c ON c.id = b.c_id",
		},
		{
			name:  "long clause wraps with indent",
			s:     "SELECT very_long_column_one, very_long_column_two, very_long_column_three FROM t",
			width: 40,
			want:  "SELECT very_long_column_one,\n  very_long_column_two,\n  very_long_column_three\nFROM t",
		},
		{
			name:  "normalises whitespace",
			s:     "SELECT  id   FROM   users",
			width: 80,
			want:  "SELECT id\nFROM users",
		},
		{
			name:  "no clause keywords",
			s:     "pg_sleep(10)",
			width: 80,
			want:  "pg_sleep(10)",
		},
		{
			name:  "empty string",
			s:     "",
			width: 80,
			want:  "",
		},
		{
			name:  "CTE: (SELECT breaks after (",
			s:     "WITH paused AS (SELECT pg_sleep(60)) SELECT id FROM t WHERE id = 1",
			width: 80,
			want:  "WITH paused AS (\nSELECT pg_sleep(60))\nSELECT id\nFROM t\nWHERE id = 1",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := formatSQL(c.s, c.width)
			if got != c.want {
				t.Errorf("formatSQL(%q, %d)\ngot:  %q\nwant: %q", c.s, c.width, got, c.want)
			}
		})
	}
}

func TestConnPct(t *testing.T) {
	cases := []struct {
		active, max int
		want        string
	}{
		{0, 0, "—"},
		{100, 200, "50%"},
		{142, 200, "71%"},
		{200, 200, "100%"},
	}
	for _, c := range cases {
		got := connPct(c.active, c.max)
		if got != c.want {
			t.Errorf("connPct(%d, %d) = %q, want %q", c.active, c.max, got, c.want)
		}
	}
}
