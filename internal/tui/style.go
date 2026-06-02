package tui

import "github.com/charmbracelet/lipgloss"

var (
	boldStyle = lipgloss.NewStyle().Bold(true)
	dimStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))

	activeTitleStyle   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("39"))
	inactiveTitleStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("252"))

	selectedRowStyle = lipgloss.NewStyle().
				Background(lipgloss.Color("237")).
				Foreground(lipgloss.Color("255"))

	colHeaderStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("243"))

	errorStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("196")).Bold(true)
	warnStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("214"))
	footerStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))

	modalStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("214")).
			Padding(1, 3)

	sqlKeywordStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("75")).Bold(true)

	sslBadgeStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("76")).Bold(true)
)
