package tui

import (
	"os"
	"path/filepath"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/flyingnobita/llml/internal/profiles"
)

// Export panel tests: edge cases.
// Split out of export_view_test.go, which had grown past 1900 lines.

func TestToggleGroup_InvalidCursorNoop(t *testing.T) {
	m := New()
	m.export.open = true
	m.export.focus = exportFocusList
	m.export.items = []exportProfileItem{
		{kind: exportItemHeader, modelKey: "/models/A.gguf", modelDisplay: "A", checked: false},
		{kind: exportItemProfile, modelKey: "/models/A.gguf", modelDisplay: "A", backend: "llama", profileName: "p1", checked: false},
	}
	// Set filteredIndices so real cursor is -1.
	m.export.filteredIndices = []int{0, 1}
	m.export.cursor = 5 // out of bounds

	// Should not panic.
	m = m.toggleGroup(true)
	// Items should be unchanged.
	if m.export.items[0].checked {
		t.Error("items should be unchanged when cursor is invalid")
	}
}

func TestToggleGroup_ProfileCursorNoop(t *testing.T) {
	m := New()
	m.export.open = true
	m.export.focus = exportFocusList
	m.export.items = []exportProfileItem{
		{kind: exportItemHeader, modelKey: "/models/A.gguf", modelDisplay: "A", checked: false},
		{kind: exportItemProfile, modelKey: "/models/A.gguf", modelDisplay: "A", backend: "llama", profileName: "p1", checked: false},
	}
	m.export.cursor = 1 // profile, not header

	// Should be a no-op (guard returns when not header).
	m = m.toggleGroup(true)
	if m.export.items[1].checked {
		t.Error("profile should not be changed by toggleGroup on non-header")
	}
}

// --- Remaining edge-case tests ---

func TestOpenExportView_DefaultBackendForEmpty(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	llmlDir := testLlmlDir(t, dir)
	paramsPath := filepath.Join(llmlDir, "model-params.json")

	// Profile with empty backend should default to "llama".
	data := []byte(`{
	  "version": 3,
	  "models": {
	    "/models/Test.gguf": {
	      "profiles": [
	        {"name": "nobackend", "backend": "", "env": [], "args": ["--ctx-size", "4096"]}
	      ],
	      "activeIndex": 0
	    }
	  }
	}`)
	if err := os.WriteFile(paramsPath, data, 0o644); err != nil {
		t.Fatal(err)
	}

	m := New()
	m = m.openExportView()

	if !m.export.open {
		t.Fatal("export should be open")
	}
	// Should have header + 1 profile.
	if len(m.export.items) != 2 {
		t.Fatalf("len(items) = %d, want 2", len(m.export.items))
	}
	if m.export.items[1].backend != "llama" {
		t.Errorf("empty backend should default to llama, got %q", m.export.items[1].backend)
	}
}

func TestRebuildExportFilter_CursorClamping(t *testing.T) {
	m := New()
	m.export.open = true
	m.export.focus = exportFocusList
	m.export.items = []exportProfileItem{
		{kind: exportItemProfile, modelDisplay: "A", backend: "llama", profileName: "p1", checked: false},
		{kind: exportItemProfile, modelDisplay: "B", backend: "vllm", profileName: "p2", checked: false},
		{kind: exportItemProfile, modelDisplay: "C", backend: "ollama", profileName: "p3", checked: false},
	}
	m.export.cursor = 5 // beyond filtered indices

	m.export.focus = exportFocusFilter
	m.export.filterInput.SetValue("p1")
	m = m.rebuildExportFilter()

	// Cursor should be clamped to 0 (only 1 visible item).
	if m.export.cursor != 0 {
		t.Errorf("cursor = %d, want 0 (clamped to max filtered index)", m.export.cursor)
	}
}

func TestUpdateExportKey_FilterFocusDefaultKey(t *testing.T) {
	m := New()
	m.export.open = true
	m.export.focus = exportFocusFilter
	m.export.items = []exportProfileItem{
		{kind: exportItemProfile, modelDisplay: "A", backend: "llama", profileName: "p1", checked: false},
	}
	m.export.filterInput.SetValue("")

	// Send a regular character while filter has focus.
	msg := tea.KeyPressMsg{Code: 'g', Text: "g"}
	tm, _ := m.updateExportKey(msg)
	m2 := asModel(tm)

	if m2.export.focus != exportFocusFilter {
		t.Error("focus should stay on filter")
	}
}

func TestUpdateExportKey_PathFocusDefaultKey(t *testing.T) {
	m := New()
	m.export.open = true
	m.export.focus = exportFocusPath
	m.export.items = []exportProfileItem{
		{kind: exportItemProfile, modelDisplay: "A", backend: "llama", profileName: "p1", checked: false},
	}
	m.export.outputPath = "/tmp/test.toml"
	m.export.pathInput.SetValue("/tmp/test.toml")

	// Send a regular character while path has focus.
	msg := tea.KeyPressMsg{Code: 'x', Text: "x"}
	tm, _ := m.updateExportKey(msg)
	m2 := asModel(tm)

	if m2.export.focus != exportFocusPath {
		t.Error("focus should stay on path")
	}
}

func TestUpdateExportKey_ListUnhandledKey(t *testing.T) {
	m := New()
	m.export.open = true
	m.export.focus = exportFocusList
	m.export.items = []exportProfileItem{
		{kind: exportItemProfile, modelDisplay: "A", backend: "llama", profileName: "p1", checked: false},
	}
	m.export.cursor = 0

	// Send a key that's not handled by any case in the list section.
	msg := tea.KeyPressMsg{Code: 'x', Text: "x"}
	tm, cmd := m.updateExportKey(msg)
	m2 := asModel(tm)

	if cmd != nil {
		t.Error("unhandled key should return nil cmd")
	}
	if !m2.export.open {
		t.Error("unhandled key should not close export")
	}
}

func TestUpdateCollisionKey_DefaultKey(t *testing.T) {
	m := New()
	m.collision.open = true
	m.collision.dest = "/tmp/export.toml"
	m.collision.suffixPath = "/tmp/export-2.toml"

	// Send an unrecognized key.
	msg := tea.KeyPressMsg{Code: 'x', Text: "x"}
	tm, _ := m.updateCollisionKey(msg)
	m2 := asModel(tm)

	if !m2.collision.open {
		t.Error("unhandled key in collision modal should not close it")
	}
}

func TestFinishExport_WriteError(t *testing.T) {
	m := New()
	m.export.open = true

	// Build reasonable profiles to export.
	m.export.items = []exportProfileItem{
		{kind: exportItemProfile, modelDisplay: "A", backend: "llama", profileName: "p1", checked: true,
			pp: profiles.PortableProfile{Name: "p1", Backend: "llama", ModelHint: "A"}},
	}

	// Use a path that can't be written: a directory.
	badDir := t.TempDir()
	badPath := filepath.Join(badDir, "nonexistent", "export.toml")

	tm, _ := m.finishExport(badPath, false)
	m2 := asModel(tm)

	// Should not panic; should flash an error via lastRunNote.
	if m2.lastRunNote == "" {
		t.Error("expected error note when WritePortable fails")
	}
}

func TestExportModalBlock_LastRunNote(t *testing.T) {
	m := New()
	m.layout.width = 100
	m.layout.height = 40
	m.layout.bodyInnerW = 80
	m.export.open = true
	m.export.focus = exportFocusList
	m.export.items = []exportProfileItem{
		{kind: exportItemProfile, modelDisplay: "A", backend: "llama", profileName: "p1", checked: true},
	}
	m.export.cursor = 0
	m.export.outputPath = "/tmp/test.toml"
	m.export.pathInput.SetValue("/tmp/test.toml")
	m.lastRunNote = "Test status message"
	m.lastRunNoteSuccess = true

	block := m.exportModalBlock()
	if !contains(block, "Test status message") {
		t.Error("block should include lastRunNote when set")
	}
}

// Test the secondary sort key (same backend, different names).
func TestOpenExportView_SameBackendSortByName(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	llmlDir := testLlmlDir(t, dir)
	paramsPath := filepath.Join(llmlDir, "model-params.json")

	data := []byte(`{
	  "version": 3,
	  "models": {
	    "/models/Test.gguf": {
	      "profiles": [
	        {"name": "z-profile", "backend": "llama", "env": [], "args": ["--ctx-size", "4096"]},
	        {"name": "a-profile", "backend": "llama", "env": [], "args": ["--n-gpu-layers", "80"]}
	      ],
	      "activeIndex": 0
	    }
	  }
	}`)
	if err := os.WriteFile(paramsPath, data, 0o644); err != nil {
		t.Fatal(err)
	}

	m := New()
	m = m.openExportView()

	if len(m.export.items) != 3 {
		t.Fatalf("len(items) = %d, want 3", len(m.export.items))
	}
	// First profile should be "a-profile" (alphabetically first).
	if m.export.items[1].profileName != "a-profile" {
		t.Errorf("items[1].profileName = %q, want a-profile", m.export.items[1].profileName)
	}
	if m.export.items[2].profileName != "z-profile" {
		t.Errorf("items[2].profileName = %q, want z-profile", m.export.items[2].profileName)
	}
}

func TestExportKeyPathFocusTextUpdate(t *testing.T) {
	m := New()
	m.export.open = true
	m.export.focus = exportFocusPath
	m.export.items = []exportProfileItem{
		{kind: exportItemProfile, modelDisplay: "A", backend: "llama", profileName: "p1", checked: false},
	}
	m.export.outputPath = "/tmp/test.toml"
	m.export.pathInput.SetValue("/tmp/test.toml")
	m.export.pathInput.Focus()
	m.export.pathInput.CursorEnd()

	// Type '2' to append to the path.
	msg := tea.KeyPressMsg{Code: '2', Text: "2"}
	tm, _ := m.updateExportKey(msg)
	m2 := asModel(tm)

	if m2.export.outputPath != "/tmp/test.toml2" {
		t.Errorf("outputPath = %q, want /tmp/test.toml2", m2.export.outputPath)
	}
}

func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
