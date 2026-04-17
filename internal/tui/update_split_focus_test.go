package tui

import (
	"testing"

	tea "charm.land/bubbletea/v2"
)

// tabMsg returns a synthetic Tab key press message for testing.
func tabMsg() tea.KeyPressMsg {
	return tea.KeyPressMsg{Code: tea.KeyTab}
}

func TestUpdateServerSplitKeys_TabTogglesFocus(t *testing.T) {
	m := newTestModel()
	m.serverRunning = true
	m.serverExited = false
	m.splitLogFocused = false

	// Tab while server is running: should toggle splitLogFocused.
	got, _ := m.updateServerSplitKeys(tabMsg())
	if !got.splitLogFocused {
		t.Fatalf("expected splitLogFocused=true after Tab, got false")
	}

	// Tab again: should toggle back.
	got2, _ := got.updateServerSplitKeys(tabMsg())
	if got2.splitLogFocused {
		t.Fatalf("expected splitLogFocused=false after second Tab, got true")
	}
}

func TestUpdateServerSplitKeys_TabAfterExitTogglesFocus(t *testing.T) {
	m := newTestModel()
	m.serverRunning = true
	m.serverExited = true
	m.splitLogFocused = false

	got, _ := m.updateServerSplitKeys(tabMsg())
	if !got.splitLogFocused {
		t.Fatalf("expected splitLogFocused=true after Tab on exited server, got false")
	}
}

func TestUpdateServerSplitKeys_TabAppliesBorderStyles(t *testing.T) {
	m := newTestModel()
	m.serverRunning = true
	m.serverExited = false
	m.splitLogFocused = false

	// Capture border color before focus toggle by checking the tbl focus state.
	// The table should be blurred after Tab (log gets focus).
	got, _ := m.updateServerSplitKeys(tabMsg())
	if got.tbl.Focused() {
		t.Fatalf("expected table to be blurred when log pane is focused after Tab")
	}
	// And tab back: table should be focused again.
	got2, _ := got.updateServerSplitKeys(tabMsg())
	if !got2.tbl.Focused() {
		t.Fatalf("expected table to regain focus after second Tab")
	}
}
