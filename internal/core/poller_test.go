package core

import (
	"testing"
	"time"
)

func TestClampInterval(t *testing.T) {
	cases := []struct {
		d    time.Duration
		want time.Duration
	}{
		{50 * time.Millisecond, minInterval},
		{minInterval, minInterval},
		{time.Second, time.Second},
		{5 * time.Second, 5 * time.Second},
	}
	for _, c := range cases {
		got := clampInterval(c.d)
		if got != c.want {
			t.Errorf("clampInterval(%v) = %v, want %v", c.d, got, c.want)
		}
	}
}

func TestPollerDefaults(t *testing.T) {
	p := NewPoller(nil, time.Second)
	if p.LongRunningThreshold != 5*time.Second {
		t.Errorf("LongRunningThreshold = %v, want 5s", p.LongRunningThreshold)
	}
	if p.IdleInTxThreshold != 30*time.Second {
		t.Errorf("IdleInTxThreshold = %v, want 30s", p.IdleInTxThreshold)
	}
}

func TestPollerSetInterval(t *testing.T) {
	p := NewPoller(nil, time.Second)

	p.SetInterval(3 * time.Second)
	if p.Interval() != 3*time.Second {
		t.Errorf("Interval() = %v, want 3s", p.Interval())
	}

	// below minimum should clamp
	p.SetInterval(1 * time.Millisecond)
	if p.Interval() != minInterval {
		t.Errorf("Interval() = %v, want minInterval (%v)", p.Interval(), minInterval)
	}
}
