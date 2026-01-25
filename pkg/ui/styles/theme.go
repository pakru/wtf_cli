// Package styles provides a centralized theme and style system for the wtf_cli UI.
// This enables consistent styling across all UI components and easy theming.
package styles

import (
	"charm.land/lipgloss/v2"
)

// Color palette - ANSI 256 colors used throughout the application
var (
	// Primary accent color (purple)
	ColorAccent = lipgloss.Color("141")

	// Text colors
	ColorText       = lipgloss.Color("252") // Primary text
	ColorTextMuted  = lipgloss.Color("245") // Secondary/muted text
	ColorTextBright = lipgloss.Color("15")  // Bright/highlighted text

	// Semantic colors
	ColorError   = lipgloss.Color("196") // Error messages
	ColorWarning = lipgloss.Color("214") // Warning/edit mode
	ColorSuccess = lipgloss.Color("42")  // Success messages

	// Code/syntax colors
	ColorCode        = lipgloss.Color("213") // Code text
	ColorCodeBg      = lipgloss.Color("235") // Code background
	ColorPlaceholder = lipgloss.Color("240") // Placeholder text

	// Border colors
	ColorBorder      = lipgloss.Color("141") // Default border (matches accent)
	ColorBorderMuted = lipgloss.Color("62")  // Muted border
)

// Common reusable styles

// Panel/Box styles
var (
	// BoxStyle is the default rounded box for overlays and panels
	BoxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(ColorBorder).
			Padding(1, 2)

	// BoxStyleCompact has less padding
	BoxStyleCompact = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(ColorBorder).
			Padding(0, 2)
)

// Text styles
var (
	// TitleStyle for panel/section titles
	TitleStyle = lipgloss.NewStyle().
			Foreground(ColorAccent).
			Bold(true)

	// TextStyle for normal text
	TextStyle = lipgloss.NewStyle().
			Foreground(ColorText)

	// TextMutedStyle for secondary/helper text
	TextMutedStyle = lipgloss.NewStyle().
			Foreground(ColorTextMuted).
			Italic(true)

	// TextBoldStyle for emphasized text
	TextBoldStyle = lipgloss.NewStyle().
			Foreground(ColorText).
			Bold(true)
)

// Selection and highlighting
var (
	// SelectedStyle for highlighted/selected items
	SelectedStyle = lipgloss.NewStyle().
			Foreground(ColorTextBright).
			Background(ColorAccent).
			Bold(true)

	// SelectedDescStyle for selected item descriptions
	SelectedDescStyle = lipgloss.NewStyle().
				Foreground(ColorTextBright).
				Bold(true)
)

// Input and form styles
var (
	// LabelStyle for form labels
	LabelStyle = lipgloss.NewStyle().
			Foreground(ColorTextMuted).
			Width(20)

	// ValueStyle for form values
	ValueStyle = lipgloss.NewStyle().
			Foreground(ColorText)

	// EditStyle for edit mode indicators
	EditStyle = lipgloss.NewStyle().
			Foreground(ColorWarning).
			Bold(true)

	// FilterStyle for filter/search input
	FilterStyle = lipgloss.NewStyle().
			Foreground(ColorWarning).
			Bold(true)

	// PlaceholderStyle for placeholder text
	PlaceholderStyle = lipgloss.NewStyle().
				Foreground(ColorPlaceholder).
				Italic(true)
)

// Feedback styles
var (
	// ErrorStyle for error messages
	ErrorStyle = lipgloss.NewStyle().
			Foreground(ColorError)

	// FooterStyle for footer/help text
	FooterStyle = lipgloss.NewStyle().
			Foreground(ColorTextMuted).
			Italic(true)
)

// Code styles
var (
	// CodeStyle for code blocks
	CodeStyle = lipgloss.NewStyle().
		Foreground(ColorCode).
		Background(ColorCodeBg)
)

// Status bar styles
var (
	// StatusBarStyle is the default status bar style (purple theme)
	StatusBarStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FAFAFA")).
			Background(lipgloss.Color("#7D56F4")).
			Padding(0, 1).
			Bold(true)

	// StatusBarStyleCyan is the cyan theme variant
	StatusBarStyleCyan = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#FAFAFA")).
				Background(lipgloss.Color("#00B8D4")).
				Padding(0, 1).
				Bold(true)

	// StatusBarStyleDark is the dark theme variant
	StatusBarStyleDark = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#D0D0D0")).
				Background(lipgloss.Color("#3C3C3C")).
				Padding(0, 1)
)

// Welcome message styles
var (
	// WelcomeBorderStyle for welcome box borders
	WelcomeBorderStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("99"))

	// WelcomeTitleStyle for welcome message title
	WelcomeTitleStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("219")).
				Bold(true)

	// WelcomeKeyStyle for keyboard shortcut keys
	WelcomeKeyStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("222")).
			Bold(true)

	// WelcomeHeaderStyle for section headers
	WelcomeHeaderStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("248"))

	// WelcomeVersionStyle for version info (dimmed)
	WelcomeVersionStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("244"))
)

// Full-screen panel styles
var (
	// FullScreenBoxStyle for fullscreen application panels
	FullScreenBoxStyle = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(ColorBorder)
)
