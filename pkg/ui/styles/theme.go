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

// Dialog styles.
var (
	// DialogButtonStyle is the inactive/unfocused button.
	DialogButtonStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#E7E3EE")).
				Background(lipgloss.Color("#3F3D49")).
				Padding(0, 2)

	// DialogActiveButtonStyle is the focused button (pink/magenta).
	DialogActiveButtonStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#FFFFFF")).
				Background(lipgloss.Color("#F35AED")).
				Padding(0, 2).
				Underline(true).
				Bold(true)

	// DialogTitleStyle is the leading title text in modal dialogs.
	DialogTitleStyle = lipgloss.NewStyle().
				Foreground(ColorAccent)

	// DialogTitleFillStyle is the slash fill after dialog titles.
	DialogTitleFillStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#D946EF"))

	// DialogMetaKeyStyle renders metadata keys such as Tool, Path, and Desc.
	DialogMetaKeyStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#928EA0")).
				Bold(true)

	// DialogMetaValueStyle renders metadata values.
	DialogMetaValueStyle = lipgloss.NewStyle().
				Foreground(ColorText)

	// DialogContentPanelStyle frames the command/argument preview.
	DialogContentPanelStyle = lipgloss.NewStyle().
				Foreground(ColorText).
				Background(lipgloss.Color("#3D3B46")).
				Padding(1, 2)

	// DialogHelpStyle constrains compact key help under dialog buttons.
	DialogHelpStyle = lipgloss.NewStyle().
			Width(0)

	// DialogHelpKeyStyle renders the key tokens in compact dialog help.
	DialogHelpKeyStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#8A8796")).
				Bold(true)

	// DialogHelpTextStyle renders the action descriptions in compact dialog help.
	DialogHelpTextStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#676371")).
				Bold(true)

	// DialogHelpSeparatorStyle renders separators between compact help items.
	DialogHelpSeparatorStyle = lipgloss.NewStyle().
					Foreground(lipgloss.Color("#5C5866")).
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

	// CommandStyle for executable commands rendered in chat output.
	CommandStyle = lipgloss.NewStyle().
			Foreground(ColorTextBright).
			Underline(true)

	// CommandActiveStyle for the currently selected command line.
	CommandActiveStyle = lipgloss.NewStyle().
				Foreground(ColorTextBright).
				Background(ColorBorderMuted).
				Underline(true)

	// Chat role-label styles distinguish speakers in the sidebar transcript.
	ChatUserLabelStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("39")). // blue
				Bold(true)
	ChatAssistantLabelStyle = lipgloss.NewStyle().
				Foreground(ColorAccent). // accent purple
				Bold(true)
	ChatToolLabelStyle = lipgloss.NewStyle().
				Foreground(ColorWarning). // orange
				Bold(true)
	ChatErrorLabelStyle = lipgloss.NewStyle().
				Foreground(ColorError). // red
				Bold(true)

	// ChatSeparatorStyle renders the rule between chat turns in dark gray.
	ChatSeparatorStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("240")) // dark gray
)

// ChatLabel renders a chat role label (e.g. "You:", "Assistant:") in the
// speaker's color. Unknown roles fall back to normal text styling.
func ChatLabel(role, text string) string {
	switch role {
	case "user":
		return ChatUserLabelStyle.Render(text)
	case "assistant":
		return ChatAssistantLabelStyle.Render(text)
	case "tool":
		return ChatToolLabelStyle.Render(text)
	case "error":
		return ChatErrorLabelStyle.Render(text)
	}
	return TextStyle.Render(text)
}

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

	// WelcomeURLStyle for clickable URLs (blue + underline = universal "link" signal)
	WelcomeURLStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("111")).
			Underline(true)

	// WelcomeCommandStyle for copy-paste shell commands
	WelcomeCommandStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("156")).
				Bold(true)

	// WelcomeUpdateVersionStyle for the new version number (green = success)
	WelcomeUpdateVersionStyle = lipgloss.NewStyle().
					Foreground(lipgloss.Color("114")).
					Bold(true)
)

// Full-screen panel styles
var (
	// FullScreenBoxStyle for fullscreen application panels
	FullScreenBoxStyle = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(ColorBorder)
)
