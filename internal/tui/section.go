package tui

// Section identifies which panel of the dashboard is focused.
type Section int

const (
	SectionActivity Section = iota
	SectionLocks
	SectionIdle
	sectionCount
)

func (s Section) next() Section { return (s + 1) % sectionCount }
func (s Section) prev() Section { return (s + sectionCount - 1) % sectionCount }
