package ui

import (
	tea "charm.land/bubbletea/v2"
)

// Test helpers for creating v2 KeyPressMsg values

// newKeyPressMsg creates a KeyPressMsg from a key code (for special keys)
func newKeyPressMsg(code rune) tea.KeyPressMsg {
	return tea.KeyPressMsg(tea.Key{Code: code})
}

// newTextKeyPressMsg creates a KeyPressMsg for text input
func newTextKeyPressMsg(text string) tea.KeyPressMsg {
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
	testKeyUp        = newKeyPressMsg(tea.KeyUp)
	testKeyDown      = newKeyPressMsg(tea.KeyDown)
	testKeyLeft      = newKeyPressMsg(tea.KeyLeft)
	testKeyRight     = newKeyPressMsg(tea.KeyRight)
	testKeyEnter     = newKeyPressMsg(tea.KeyEnter)
	testKeyTab       = newKeyPressMsg(tea.KeyTab)
	testKeyEsc       = newKeyPressMsg(tea.KeyEscape)
	testKeyBackspace = newKeyPressMsg(tea.KeyBackspace)
	testKeySpace     = newKeyPressMsg(tea.KeySpace)
	testKeyHome      = newKeyPressMsg(tea.KeyHome)
	testKeyEnd       = newKeyPressMsg(tea.KeyEnd)
	testKeyPgUp      = newKeyPressMsg(tea.KeyPgUp)
	testKeyPgDown    = newKeyPressMsg(tea.KeyPgDown)
	testKeyDelete    = newKeyPressMsg(tea.KeyDelete)
)

// Ctrl+X keys using modifier
func newCtrlKeyPressMsg(char rune) tea.KeyPressMsg {
	return tea.KeyPressMsg(tea.Key{
		Code: char,
		Mod:  tea.ModCtrl,
	})
}

// Common ctrl combinations
var (
	testKeyCtrlC = newCtrlKeyPressMsg('c')
	testKeyCtrlD = newCtrlKeyPressMsg('d')
	testKeyCtrlW = newCtrlKeyPressMsg('w')
	testKeyCtrlZ = newCtrlKeyPressMsg('z')
	testKeyCtrlX = newCtrlKeyPressMsg('x')
)
