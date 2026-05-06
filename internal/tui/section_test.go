package tui

import "testing"

func TestSectionNext(t *testing.T) {
	cases := []struct {
		s    Section
		want Section
	}{
		{SectionActivity, SectionLocks},
		{SectionLocks, SectionIdle},
		{SectionIdle, SectionActivity},
	}
	for _, c := range cases {
		if got := c.s.next(); got != c.want {
			t.Errorf("%v.next() = %v, want %v", c.s, got, c.want)
		}
	}
}

func TestSectionPrev(t *testing.T) {
	cases := []struct {
		s    Section
		want Section
	}{
		{SectionActivity, SectionIdle},
		{SectionLocks, SectionActivity},
		{SectionIdle, SectionLocks},
	}
	for _, c := range cases {
		if got := c.s.prev(); got != c.want {
			t.Errorf("%v.prev() = %v, want %v", c.s, got, c.want)
		}
	}
}
