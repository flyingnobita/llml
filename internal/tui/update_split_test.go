package tui

import (
	"testing"

	tea "charm.land/bubbletea/v2"
)

func TestIsCtrlC(t *testing.T) {
	if !isCtrlC(tea.KeyPressMsg(tea.Key{Text: "ctrl+c"})) {
		t.Fatal("string ctrl+c")
	}
	if !isCtrlC(tea.KeyPressMsg(tea.Key{Code: 'c', Text: "c", Mod: tea.ModCtrl})) {
		t.Fatal("mod+code")
	}
	if isCtrlC(tea.KeyPressMsg(tea.Key{Code: 'x', Text: "x", Mod: tea.ModCtrl})) {
		t.Fatal("ctrl+x should not match")
	}
}

func TestIsTabKey(t *testing.T) {
	if !isTabKey(tea.KeyPressMsg(tea.Key{Code: tea.KeyTab})) {
		t.Fatal("KeyTab")
	}
	if !isTabKey(tea.KeyPressMsg(tea.Key{Text: "tab"})) {
		t.Fatal("string tab")
	}
	if isTabKey(tea.KeyPressMsg(tea.Key{Text: "a", Code: 'a'})) {
		t.Fatal("a should not match")
	}
}

func TestIsEscapeKey(t *testing.T) {
	if !isEscapeKey(tea.KeyPressMsg(tea.Key{Code: tea.KeyEscape})) {
		t.Fatal("KeyEscape")
	}
	if !isEscapeKey(tea.KeyPressMsg(tea.Key{Text: "esc"})) {
		t.Fatal("string esc")
	}
	if isEscapeKey(tea.KeyPressMsg(tea.Key{Text: "a", Code: 'a'})) {
		t.Fatal("a should not match")
	}
}
