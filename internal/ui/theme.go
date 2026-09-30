package ui

import "github.com/charmbracelet/lipgloss"

// The palette is deliberately small: one accent, two greys, and three status
// colours. Adaptive colours let the same build look right on a light or dark
// terminal, and lipgloss drops the escapes entirely when the terminal cannot
// render them or NO_COLOR is set.
var (
	colAccent = lipgloss.AdaptiveColor{Light: "#5B3DF5", Dark: "#A78BFA"}
	colMuted  = lipgloss.AdaptiveColor{Light: "#6B7280", Dark: "#8B90A5"}
	colFaint  = lipgloss.AdaptiveColor{Light: "#9CA3AF", Dark: "#565B6E"}
	colBorder = lipgloss.AdaptiveColor{Light: "#D1D5DB", Dark: "#3A3F52"}
	colOK     = lipgloss.AdaptiveColor{Light: "#047857", Dark: "#4ADE80"}
	colWarn   = lipgloss.AdaptiveColor{Light: "#B45309", Dark: "#FBBF24"}
	colErr    = lipgloss.AdaptiveColor{Light: "#B91C1C", Dark: "#F87171"}
)

// Rendering styles. The st prefix keeps them short at the call sites; they are
// package private on purpose, nothing outside the UI should reach for them.
var (
	stTitle    = lipgloss.NewStyle().Bold(true).Foreground(colAccent)
	stSubtitle = lipgloss.NewStyle().Foreground(colMuted)
	stBorder   = lipgloss.NewStyle().Foreground(colBorder)

	stLabel    = lipgloss.NewStyle().Foreground(colMuted)
	stLabelHot = lipgloss.NewStyle().Bold(true).Foreground(colAccent)
	stValue    = lipgloss.NewStyle()
	stValueHot = lipgloss.NewStyle().Bold(true)
	stFaint    = lipgloss.NewStyle().Foreground(colFaint)

	stOK   = lipgloss.NewStyle().Foreground(colOK)
	stWarn = lipgloss.NewStyle().Foreground(colWarn)
	stErr  = lipgloss.NewStyle().Foreground(colErr)

	stMarker   = lipgloss.NewStyle().Foreground(colAccent)
	stSelected = lipgloss.NewStyle().Bold(true)
	stKeyCap   = lipgloss.NewStyle().Bold(true).Foreground(colAccent)
	stKeyDesc  = lipgloss.NewStyle().Foreground(colFaint)
	stHint     = lipgloss.NewStyle().Foreground(colFaint)

	stPanelTitle = lipgloss.NewStyle().Bold(true).Foreground(colAccent)
)
