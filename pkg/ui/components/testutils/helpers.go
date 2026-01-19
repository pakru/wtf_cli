package testutils

import (
	tea "charm.land/bubbletea/v2"
)

// Test helpers for creating v2 KeyPressMsg values

// NewKeyPressMsg creates a KeyPressMsg from a key code (for special keys)
func NewKeyPressMsg(code rune) tea.KeyPressMsg {
	return tea.KeyPressMsg(tea.Key{Code: code})
}

// NewTextKeyPressMsg creates a KeyPressMsg for text input
func NewTextKeyPressMsg(text string) tea.KeyPressMsg {
	if len(text) == 0 {
		return tea.KeyPressMsg(tea.Key{})
	}
	r := []rune(text)[0]
	return tea.KeyPressMsg(tea.Key{
		Code: r,
		Text: text,
	})
}

// Common special keys using the new API
var (
	TestKeyUp        = NewKeyPressMsg(tea.KeyUp)
	TestKeyDown      = NewKeyPressMsg(tea.KeyDown)
	TestKeyLeft      = NewKeyPressMsg(tea.KeyLeft)
	TestKeyRight     = NewKeyPressMsg(tea.KeyRight)
	TestKeyEnter     = NewKeyPressMsg(tea.KeyEnter)
	TestKeyTab       = NewKeyPressMsg(tea.KeyTab)
	TestKeyEsc       = NewKeyPressMsg(tea.KeyEscape)
	TestKeyBackspace = NewKeyPressMsg(tea.KeyBackspace)
	TestKeySpace     = NewKeyPressMsg(tea.KeySpace)
	TestKeyHome      = NewKeyPressMsg(tea.KeyHome)
	TestKeyEnd       = NewKeyPressMsg(tea.KeyEnd)
	TestKeyPgUp      = NewKeyPressMsg(tea.KeyPgUp)
	TestKeyPgDown    = NewKeyPressMsg(tea.KeyPgDown)
	TestKeyDelete    = NewKeyPressMsg(tea.KeyDelete)
)

// Ctrl+X keys using modifier
func NewCtrlKeyPressMsg(char rune) tea.KeyPressMsg {
	return tea.KeyPressMsg(tea.Key{
		Code: char,
		Mod:  tea.ModCtrl,
	})
}

// Common ctrl combinations
var (
	TestKeyCtrlC = NewCtrlKeyPressMsg('c')
	TestKeyCtrlD = NewCtrlKeyPressMsg('d')
	TestKeyCtrlW = NewCtrlKeyPressMsg('w')
	TestKeyCtrlZ = NewCtrlKeyPressMsg('z')
	TestKeyCtrlX = NewCtrlKeyPressMsg('x')
)
