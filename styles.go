package main

import (
	"github.com/charmbracelet/lipgloss"
)

// Layout constants
const (
	StepsWidthPct      = 25
	MinStepsWidth      = 25
	NarrowModeWidth    = 60  // Only activate on very narrow terminals
	PanelBorderPadding = 4
	MinPanelHeight     = 5
	MinStepsHeight     = 10
	ScrollLines        = 3
	PopupMaxHeight     = 30
	OutputPanelPct     = 70  // Output takes 70% of vertical space
	DiffPanelPct       = 30  // Diff takes 30% of vertical space
)

// Color palette
var (
	neonCyan    = lipgloss.Color("51")
	neonMagenta = lipgloss.Color("201")
	neonGreen   = lipgloss.Color("46")
	neonYellow  = lipgloss.Color("226")
	neonRed     = lipgloss.Color("196")
	darkBg      = lipgloss.Color("235")
	darkerBg    = lipgloss.Color("233")
	mutedGray   = lipgloss.Color("240")
)

// Base styles
var (
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(neonCyan).
			Background(darkerBg).
			Padding(0, 1).
			MarginBottom(1)

	statusBarStyle = lipgloss.NewStyle().
			Foreground(neonCyan).
			Background(darkBg).
			Padding(0, 1).
			Bold(true)

	progressBarStyle = lipgloss.NewStyle().
				Foreground(neonMagenta).
				Bold(true)

	panelStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(neonCyan).
			Padding(0, 1)

	statsStyle = lipgloss.NewStyle().
			Foreground(neonMagenta).
			Italic(true)
	
	// Inline text styles
	boldCyanStyle = lipgloss.NewStyle().
			Foreground(neonCyan).
			Bold(true)
	
	cyanStyle = lipgloss.NewStyle().
			Foreground(neonCyan)
	
	yellowStyle = lipgloss.NewStyle().
			Foreground(neonYellow)
	
	greenStyle = lipgloss.NewStyle().
			Foreground(neonGreen)
	
	magentaBoldStyle = lipgloss.NewStyle().
			Foreground(neonMagenta).
			Bold(true)
	
	cyanFaintStyle = lipgloss.NewStyle().
			Foreground(neonCyan).
			Faint(true)
)

// GetStepStatusStyle returns the style for a step based on its status
func GetStepStatusStyle(status StepStatus) lipgloss.Style {
	switch status {
	case StatusPending:
		return lipgloss.NewStyle().Foreground(mutedGray).Faint(true)
	case StatusRunning:
		return lipgloss.NewStyle().Foreground(neonYellow).Bold(true).Blink(true)
	case StatusCompleted:
		return lipgloss.NewStyle().Foreground(neonGreen).Bold(true)
	case StatusFailed:
		return lipgloss.NewStyle().Foreground(neonRed).Bold(true)
	default:
		return lipgloss.NewStyle()
	}
}

// GetStepIcon returns the icon for a step based on its status
func GetStepIcon(status StepStatus) string {
	switch status {
	case StatusPending:
		return "○"
	case StatusRunning:
		return "⚙"
	case StatusCompleted:
		return "✓"
	case StatusFailed:
		return "✗"
	default:
		return "?"
	}
}

// PanelTitleStyle returns the style for panel titles
func PanelTitleStyle() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(neonCyan).Bold(true)
}

// DividerStyle returns the style for dividers
func DividerStyle() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(neonCyan).Faint(true)
}
