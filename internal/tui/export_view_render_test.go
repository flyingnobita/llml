package tui

import (
	"fmt"
	"testing"
)

// Export panel tests: rendering.
// Split out of export_view_test.go, which had grown past 1900 lines.

func TestExportModalBlock_FilteredNoMatches(t *testing.T) {
	m := New()
	m.layout.width = 100
	m.layout.height = 40
	m.layout.bodyInnerW = 80
	m.export.open = true
	m.export.focus = exportFocusList
	m.export.items = []exportProfileItem{
		{kind: exportItemProfile, modelDisplay: "A", backend: "llama", profileName: "p1", checked: false},
	}
	m.export.cursor = 0
	m.export.outputPath = "/tmp/test.toml"
	m.export.pathInput.SetValue("/tmp/test.toml")

	// Activate filter with no matches.
	m.export.focus = exportFocusFilter
	m.export.filterInput.SetValue("zzz")
	m = m.rebuildExportFilter()

	block := m.exportModalBlock()
	if !contains(block, FooterExportNoMatch) {
		t.Error("should show no-match message")
	}
}

func TestExportModalBlock_ScrollingWithScrollbar(t *testing.T) {
	m := New()
	m.layout.width = 100
	m.layout.height = 20 // Small terminal to force scrolling
	m.layout.bodyInnerW = 80
	m.export.open = true
	m.export.focus = exportFocusList

	// Create more items than visible.
	var items []exportProfileItem
	for i := range 20 {
		items = append(items, exportProfileItem{
			kind: exportItemProfile, modelKey: fmt.Sprintf("/models/M%d.gguf", i),
			modelDisplay: fmt.Sprintf("M%d", i),
			backend:      "llama", profileName: fmt.Sprintf("p%d", i), checked: false,
		})
	}
	m.export.items = items
	m.export.cursor = 0
	m.export.scrollOffset = 0
	m.export.outputPath = "/tmp/test.toml"
	m.export.pathInput.SetValue("/tmp/test.toml")

	block := m.exportModalBlock()
	if block == "" {
		t.Error("block should not be empty")
	}
	// Scrollable content should include a scrollbar character.
	if !contains(block, "▴") && !contains(block, "█") && !contains(block, "▾") && !contains(block, "┃") {
		t.Error("scrolling block should contain scrollbar characters")
	}
}

func TestExportModalBlock_PathFocusRendering(t *testing.T) {
	m := New()
	m.layout.width = 100
	m.layout.height = 40
	m.layout.bodyInnerW = 80
	m.export.open = true
	m.export.focus = exportFocusPath
	m.export.items = []exportProfileItem{
		{kind: exportItemProfile, modelDisplay: "A", backend: "llama", profileName: "p1", checked: true},
	}
	m.export.cursor = 0
	m.export.outputPath = "/tmp/test-output.toml"
	m.export.pathInput.SetValue("/tmp/test-output.toml")

	block := m.exportModalBlock()
	if !contains(block, "Output:") {
		t.Error("rendering should include Output label")
	}
}

func TestExportRenderFilterRow_UnfocusedWithFilter(t *testing.T) {
	m := New()
	m.export.open = true
	m.export.focus = exportFocusPath // Not filter, not list
	m.export.filterInput.SetValue("gpu")

	bodyStyle := m.ui.styles.body
	dimStyle := m.ui.styles.footer

	result := m.renderExportFilterRow(bodyStyle, dimStyle)
	if result == "" {
		t.Error("filter row should not be empty")
	}
	if !contains(result, "gpu") {
		t.Error("should contain filter value")
	}
}

func TestExportRenderFilterRow_UnfocusedEmpty(t *testing.T) {
	m := New()
	m.export.open = true
	m.export.focus = exportFocusPath
	m.export.filterInput.SetValue("")

	bodyStyle := m.ui.styles.body
	dimStyle := m.ui.styles.footer

	result := m.renderExportFilterRow(bodyStyle, dimStyle)
	if !contains(result, "filter...") {
		t.Error("should render dim placeholder when filter is empty and unfocused")
	}
}

func TestExportRealCursorIndex_FilteredOutOfBounds(t *testing.T) {
	m := New()
	m.export.open = true
	m.export.focus = exportFocusList
	m.export.items = []exportProfileItem{
		{kind: exportItemProfile, modelDisplay: "A", backend: "llama", profileName: "p1", checked: false},
	}
	// Set filteredIndices to non-nil but cursor out of bounds.
	m.export.filteredIndices = []int{0}
	m.export.cursor = -1

	idx := m.exportRealCursorIndex()
	if idx != -1 {
		t.Errorf("real cursor index = %d, want -1 for negative cursor", idx)
	}

	m.export.cursor = 5 // Beyond filteredIndices length
	idx = m.exportRealCursorIndex()
	if idx != -1 {
		t.Errorf("real cursor index = %d, want -1 for cursor beyond filteredIndices", idx)
	}
}

// --- Toggle group with invalid cursor ---
