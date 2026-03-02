package ui

import "github.com/charmbracelet/bubbletea"

type keyAction int

const (
	keyNone keyAction = iota
	keyQuit
	keyUp
	keyDown
	keyPageUp
	keyPageDown
	keyTop
	keyBottom
)

func parseKey(msg tea.KeyMsg) keyAction {
	switch msg.String() {
	case "q", "ctrl+c":
		return keyQuit
	case "up", "k":
		return keyUp
	case "down", "j":
		return keyDown
	case "pgup", "b":
		return keyPageUp
	case "pgdown", "f", " ":
		return keyPageDown
	case "home", "g":
		return keyTop
	case "end", "G":
		return keyBottom
	default:
		return keyNone
	}
}
