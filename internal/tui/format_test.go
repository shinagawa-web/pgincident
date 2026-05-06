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
		{0, "00:00:00.00"},
		{5 * time.Second, "00:00:05.00"},
		{90 * time.Second, "00:01:30.00"},
		{2*time.Hour + 14*time.Minute + 32*time.Second, "02:14:32.00"},
		{500 * time.Millisecond, "00:00:00.50"},
		// boundary: must not produce "60.00" due to floating-point rounding
		{119999 * time.Millisecond, "00:01:59.99"},
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
