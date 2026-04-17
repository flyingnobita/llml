package tui

import (
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/flyingnobita/llml/internal/models"
)

// tabMsg returns a synthetic Tab key press message for testing.
func tabMsg() tea.KeyPressMsg {
	return tea.KeyPressMsg{Code: tea.KeyTab}
}

func TestUpdateServerSplitKeys_TabTogglesFocusNoPreview(t *testing.T) {
	m := newTestModel()
	m.server.running = true
	m.server.exited = false
	m.server.splitFocused = false

	// Tab while server is running: should toggle splitLogFocused.
	got, _ := m.updateServerSplitKeys(tabMsg())
	if !got.server.splitFocused {
		t.Fatalf("expected splitLogFocused=true after Tab, got false")
	}

	// Tab again: should toggle back.
	got2, _ := got.updateServerSplitKeys(tabMsg())
	if got2.server.splitFocused {
		t.Fatalf("expected splitLogFocused=false after second Tab, got true")
	}
}

func TestUpdateServerSplitKeys_TabCyclesThreeWay(t *testing.T) {
	m := newTestModel()
	m.table.files = []models.ModelFile{{Path: "/tmp/foo.gguf", Backend: models.BackendLlama}}
	m.server.running = true
	m.server.exited = false
	m.server.splitFocused = false
	m.preview.focused = false

	// Tab 1: Table -> Preview
	got, _ := m.updateServerSplitKeys(tabMsg())
	if !got.preview.focused {
		t.Fatalf("expected preview.focused=true, got false")
	}
	if got.server.splitFocused {
		t.Fatalf("expected server.splitFocused=false, got true")
	}

	// Tab 2: Preview -> Server Log
	got2, _ := got.updateServerSplitKeys(tabMsg())
	if got2.preview.focused {
		t.Fatalf("expected preview.focused=false, got true")
	}
	if !got2.server.splitFocused {
		t.Fatalf("expected server.splitFocused=true, got false")
	}

	// Tab 3: Server Log -> Table
	got3, _ := got2.updateServerSplitKeys(tabMsg())
	if got3.preview.focused || got3.server.splitFocused {
		t.Fatalf("expected both false, back to table")
	}
}

func TestUpdateServerSplitKeys_TabAfterExitTogglesFocus(t *testing.T) {
	m := newTestModel()
	m.server.running = true
	m.server.exited = true
	m.server.splitFocused = false

	got, _ := m.updateServerSplitKeys(tabMsg())
	if !got.server.splitFocused {
		t.Fatalf("expected splitLogFocused=true after Tab on exited server, got false")
	}
}

func TestUpdateServerSplitKeys_TabAppliesBorderStyles(t *testing.T) {
	m := newTestModel()
	m.server.running = true
	m.server.exited = false
	m.server.splitFocused = false

	// Capture border color before focus toggle by checking the tbl focus state.
	// The table should be blurred after Tab (log gets focus).
	got, _ := m.updateServerSplitKeys(tabMsg())
	if got.table.tbl.Focused() {
		t.Fatalf("expected table to be blurred when log pane is focused after Tab")
	}
	// And tab back: table should be focused again.
	got2, _ := got.updateServerSplitKeys(tabMsg())
	if !got2.table.tbl.Focused() {
		t.Fatalf("expected table to regain focus after second Tab")
	}
}
