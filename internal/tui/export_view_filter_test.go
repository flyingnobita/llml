package tui

import (
	"testing"

	tea "charm.land/bubbletea/v2"
)

// Export panel tests: filtering.
// Split out of export_view_test.go, which had grown past 1900 lines.

func TestExportFilterEnterAndEsc(t *testing.T) {
	m := New()
	m.export.open = true
	m.export.focus = exportFocusList
	m.export.items = []exportProfileItem{
		{kind: exportItemProfile, modelDisplay: "A", backend: "llama", profileName: "p1", checked: false},
	}
	m.export.cursor = 0

	// / enters filter focus.
	msg := tea.KeyPressMsg{Code: '/', Text: "/"}
	tm, _ := m.updateExportKey(msg)
	m2 := asModel(tm)
	if m2.export.focus != exportFocusFilter {
		t.Error("focus should be filter after /")
	}

	// esc clears filter and returns to list.
	msg2 := tea.KeyPressMsg{Code: 27, Text: "esc"}
	tm, _ = m2.updateExportKey(msg2)
	m3 := asModel(tm)
	if m3.export.focus != exportFocusList {
		t.Error("focus should be list after esc from filter")
	}
	if m3.export.filteredIndices != nil {
		t.Error("filter should be cleared after esc")
	}
}

func TestExportFilterTabToPath(t *testing.T) {
	m := New()
	m.export.open = true
	m.export.focus = exportFocusFilter
	m.export.items = []exportProfileItem{
		{kind: exportItemProfile, modelDisplay: "A", backend: "llama", profileName: "p1", checked: false},
	}
	m.export.cursor = 0
	m.export.outputPath = "/tmp/test.toml"

	msg := tea.KeyPressMsg{Code: '\t', Text: "tab"}
	tm, _ := m.updateExportKey(msg)
	m2 := asModel(tm)
	if m2.export.focus != exportFocusPath {
		t.Error("focus should be path after tab from filter")
	}
}

func TestExportFilterFiltersItems(t *testing.T) {
	m := New()
	m.export.open = true
	m.export.focus = exportFocusList
	m.export.items = []exportProfileItem{
		{kind: exportItemHeader, modelKey: "/models/A.gguf", modelDisplay: "Model-A", checked: false},
		{kind: exportItemProfile, modelKey: "/models/A.gguf", modelDisplay: "Model-A", backend: "llama", profileName: "gpu", checked: false},
		{kind: exportItemProfile, modelKey: "/models/A.gguf", modelDisplay: "Model-A", backend: "koboldcpp", profileName: "cpu", checked: false},
		{kind: exportItemHeader, modelKey: "/models/B.gguf", modelDisplay: "Model-B", checked: false},
		{kind: exportItemProfile, modelKey: "/models/B.gguf", modelDisplay: "Model-B", backend: "vllm", profileName: "default", checked: false},
	}
	m.export.cursor = 0

	// Activate filter and type "gpu".
	m.export.focus = exportFocusFilter
	m.export.filterInput.SetValue("gpu")
	m = m.rebuildExportFilter()

	visible := m.exportVisibleItems()
	if len(visible) != 2 {
		t.Fatalf("visible count = %d, want 2 (header + matching profile)", len(visible))
	}
	if visible[0].kind != exportItemHeader || visible[0].modelDisplay != "Model-A" {
		t.Error("first visible should be Model-A header")
	}
	if visible[1].kind != exportItemProfile || visible[1].profileName != "gpu" {
		t.Error("second visible should be gpu profile")
	}
}

func TestExportFilterNoMatches(t *testing.T) {
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

	// Activate filter with no-matching term.
	m.export.focus = exportFocusFilter
	m.export.filterInput.SetValue("zzz")
	m = m.rebuildExportFilter()

	block := m.exportModalBlock()
	if !contains(block, FooterExportNoMatch) {
		t.Error("should show no-match message")
	}
}

func TestExportFilterClearOnEsc(t *testing.T) {
	m := New()
	m.export.open = true
	m.export.focus = exportFocusFilter
	m.export.items = []exportProfileItem{
		{kind: exportItemProfile, modelDisplay: "A", backend: "llama", profileName: "p1", checked: false},
		{kind: exportItemProfile, modelDisplay: "B", backend: "vllm", profileName: "p2", checked: false},
	}
	m.export.cursor = 0
	m.export.filterInput.SetValue("p1")
	m = m.rebuildExportFilter()

	// Verify filtered.
	if m.exportVisibleCount() != 1 {
		t.Fatalf("visible count = %d, want 1", m.exportVisibleCount())
	}

	// Esc clears.
	msg := tea.KeyPressMsg{Code: 27, Text: "esc"}
	tm, _ := m.updateExportKey(msg)
	m2 := asModel(tm)
	if m2.exportVisibleCount() != 2 {
		t.Errorf("visible count = %d, want 2 after filter cleared", m2.exportVisibleCount())
	}
	if m2.export.filteredIndices != nil {
		t.Error("filteredIndices should be nil after esc")
	}
}

func TestExportTogglePersistsAcrossFilter(t *testing.T) {
	m := New()
	m.export.open = true
	m.export.focus = exportFocusList
	m.export.items = []exportProfileItem{
		{kind: exportItemProfile, modelKey: "/models/A.gguf", modelDisplay: "Model-A", backend: "llama", profileName: "gpu", checked: false},
		{kind: exportItemProfile, modelKey: "/models/A.gguf", modelDisplay: "Model-A", backend: "koboldcpp", profileName: "cpu", checked: false},
	}
	m.export.cursor = 0

	// Toggle first profile on.
	msg := tea.KeyPressMsg{Code: ' ', Text: "space"}
	tm, _ := m.updateExportKey(msg)
	m2 := asModel(tm)
	if !m2.export.items[0].checked {
		t.Error("gpu should be checked")
	}

	// Apply filter that hides gpu.
	m2.export.filterInput.SetValue("cpu")
	m2 = m2.rebuildExportFilter()

	// Toggle the visible "cpu" profile.
	m2.export.cursor = 0 // cpu is the only visible item (index 0 in filtered view)
	tm, _ = m2.updateExportKey(msg)
	m3 := asModel(tm)

	// Clear filter.
	m3.export.filterInput.SetValue("")
	m3 = m3.rebuildExportFilter()

	// gpu should still be checked (toggled before filter).
	if !m3.export.items[0].checked {
		t.Error("gpu should still be checked after filter cleared")
	}
	// cpu should now be checked (toggled while filtered).
	if !m3.export.items[1].checked {
		t.Error("cpu should be checked after toggling while filtered")
	}
}

func TestExportFilterNavigation(t *testing.T) {
	m := New()
	m.export.open = true
	m.export.focus = exportFocusList
	m.export.items = []exportProfileItem{
		{kind: exportItemProfile, modelDisplay: "A", backend: "llama", profileName: "p1", checked: false},
		{kind: exportItemProfile, modelDisplay: "B", backend: "vllm", profileName: "p2", checked: false},
		{kind: exportItemProfile, modelDisplay: "C", backend: "ollama", profileName: "p3", checked: false},
	}
	m.export.cursor = 0

	// Filter to only "p1" and "p3".
	m.export.filterInput.SetValue("p1")
	m = m.rebuildExportFilter()
	// Only p1 matches.
	if m.exportVisibleCount() != 1 {
		t.Fatalf("visible count = %d, want 1", m.exportVisibleCount())
	}

	// Down at end should stay.
	msg := tea.KeyPressMsg{Code: 'j', Text: "down"}
	tm, _ := m.updateExportKey(msg)
	m2 := asModel(tm)
	if m2.export.cursor != 0 {
		t.Errorf("cursor = %d, want 0 (clamped at end when only 1 visible)", m2.export.cursor)
	}

	// Up at start should stay.
	msg2 := tea.KeyPressMsg{Code: 'k', Text: "up"}
	tm, _ = m2.updateExportKey(msg2)
	m3 := asModel(tm)
	if m3.export.cursor != 0 {
		t.Errorf("cursor = %d, want 0 (clamped at start)", m3.export.cursor)
	}
}

func TestExportSelectAllAffectsOnlyVisibleWhenFiltered(t *testing.T) {
	m := New()
	m.export.open = true
	m.export.focus = exportFocusList
	m.export.items = []exportProfileItem{
		{kind: exportItemProfile, modelKey: "/models/A.gguf", modelDisplay: "Model-A", backend: "llama", profileName: "gpu", checked: false},
		{kind: exportItemProfile, modelKey: "/models/A.gguf", modelDisplay: "Model-A", backend: "koboldcpp", profileName: "cpu", checked: false},
		{kind: exportItemProfile, modelKey: "/models/B.gguf", modelDisplay: "Model-B", backend: "vllm", profileName: "default", checked: false},
	}
	m.export.cursor = 0

	// Filter to only first two profiles.
	m.export.filterInput.SetValue("Model-A")
	m = m.rebuildExportFilter()

	msg := tea.KeyPressMsg{Code: 'a', Text: "a"}
	tm, _ := m.updateExportKey(msg)
	m2 := asModel(tm)

	// Only the two visible profiles should be checked.
	if !m2.export.items[0].checked {
		t.Error("gpu should be checked (was visible)")
	}
	if !m2.export.items[1].checked {
		t.Error("cpu should be checked (was visible)")
	}
	// Hidden profile should remain unchecked.
	if m2.export.items[2].checked {
		t.Error("default should be unchecked (was not visible)")
	}
}

func TestExportSelectNoneAffectsOnlyVisibleWhenFiltered(t *testing.T) {
	m := New()
	m.export.open = true
	m.export.focus = exportFocusList
	m.export.items = []exportProfileItem{
		{kind: exportItemProfile, modelKey: "/models/A.gguf", modelDisplay: "Model-A", backend: "llama", profileName: "gpu", checked: true},
		{kind: exportItemProfile, modelKey: "/models/A.gguf", modelDisplay: "Model-A", backend: "koboldcpp", profileName: "cpu", checked: true},
		{kind: exportItemProfile, modelKey: "/models/B.gguf", modelDisplay: "Model-B", backend: "vllm", profileName: "default", checked: true},
	}
	m.export.cursor = 0

	// Filter to only first two profiles.
	m.export.filterInput.SetValue("Model-A")
	m = m.rebuildExportFilter()

	msg := tea.KeyPressMsg{Code: 'A', Text: "A"}
	tm, _ := m.updateExportKey(msg)
	m2 := asModel(tm)

	// Only visible profiles should be unchecked.
	if m2.export.items[0].checked {
		t.Error("gpu should be unchecked (was visible)")
	}
	if m2.export.items[1].checked {
		t.Error("cpu should be unchecked (was visible)")
	}
	// Hidden profile should remain checked.
	if !m2.export.items[2].checked {
		t.Error("default should still be checked (was not visible)")
	}
}

func TestExportFilterHeaderVisibility(t *testing.T) {
	m := New()
	m.export.open = true
	m.export.focus = exportFocusList
	m.export.items = []exportProfileItem{
		{kind: exportItemHeader, modelKey: "/models/A.gguf", modelDisplay: "Model-A", checked: false},
		{kind: exportItemProfile, modelKey: "/models/A.gguf", modelDisplay: "Model-A", backend: "llama", profileName: "gpu", checked: false},
		{kind: exportItemHeader, modelKey: "/models/B.gguf", modelDisplay: "Model-B", checked: false},
		{kind: exportItemProfile, modelKey: "/models/B.gguf", modelDisplay: "Model-B", backend: "vllm", profileName: "default", checked: false},
	}
	m.export.cursor = 0

	// Filter matches only Model-B's profile.
	m.export.filterInput.SetValue("default")
	m = m.rebuildExportFilter()

	visible := m.exportVisibleItems()
	if len(visible) != 2 {
		t.Fatalf("visible count = %d, want 2", len(visible))
	}
	if visible[0].kind != exportItemHeader || visible[0].modelDisplay != "Model-B" {
		t.Error("first visible should be Model-B header")
	}
	if visible[1].kind != exportItemProfile || visible[1].profileName != "default" {
		t.Error("second visible should be default profile")
	}
}

func TestExportFilterMatchOnModelName(t *testing.T) {
	m := New()
	m.export.open = true
	m.export.focus = exportFocusList
	m.export.items = []exportProfileItem{
		{kind: exportItemHeader, modelKey: "/models/A.gguf", modelDisplay: "Llama-3-8B", checked: false},
		{kind: exportItemProfile, modelKey: "/models/A.gguf", modelDisplay: "Llama-3-8B", backend: "llama", profileName: "gpu", checked: false},
		{kind: exportItemHeader, modelKey: "/models/B.gguf", modelDisplay: "Mistral-7B", checked: false},
		{kind: exportItemProfile, modelKey: "/models/B.gguf", modelDisplay: "Mistral-7B", backend: "vllm", profileName: "default", checked: false},
	}
	m.export.cursor = 0

	// Filter matches a model name directly.
	m.export.filterInput.SetValue("Mistral")
	m = m.rebuildExportFilter()

	visible := m.exportVisibleItems()
	if len(visible) != 2 {
		t.Fatalf("visible count = %d, want 2", len(visible))
	}
	if visible[0].modelDisplay != "Mistral-7B" {
		t.Errorf("model = %q, want Mistral-7B", visible[0].modelDisplay)
	}
}

func TestExportFilterTabFromListSkipsFilter(t *testing.T) {
	m := New()
	m.export.open = true
	m.export.focus = exportFocusList
	m.export.items = []exportProfileItem{
		{kind: exportItemProfile, modelDisplay: "A", backend: "llama", profileName: "p1", checked: false},
	}
	m.export.cursor = 0
	m.export.outputPath = "/tmp/test.toml"

	// Tab from list should go to path, not filter.
	msg := tea.KeyPressMsg{Code: '\t', Text: "tab"}
	tm, _ := m.updateExportKey(msg)
	m2 := asModel(tm)
	if m2.export.focus != exportFocusPath {
		t.Error("tab from list should go to path (filter not in tab cycle)")
	}
}

func TestExportFilterHeaderCheckboxReflectsVisible(t *testing.T) {
	m := New()
	m.export.open = true
	m.export.focus = exportFocusList
	m.export.items = []exportProfileItem{
		{kind: exportItemHeader, modelKey: "/models/A.gguf", modelDisplay: "Model-A", checked: false},
		{kind: exportItemProfile, modelKey: "/models/A.gguf", modelDisplay: "Model-A", backend: "llama", profileName: "gpu", checked: true},
		{kind: exportItemProfile, modelKey: "/models/A.gguf", modelDisplay: "Model-A", backend: "koboldcpp", profileName: "cpu", checked: false},
	}
	m.export.cursor = 0

	// Filter to only "cpu" (which is unchecked).
	m.export.filterInput.SetValue("cpu")
	m = m.rebuildExportFilter()

	// Header should show ☐ since the only visible profile (cpu) is unchecked.
	m = m.syncHeaderStates()
	if m.export.items[0].checked {
		t.Error("header should be unchecked when visible profile is unchecked")
	}

	// Check cpu.
	m.export.cursor = 1 // visible index 1 is the cpu profile (after header at 0)
	msg := tea.KeyPressMsg{Code: ' ', Text: "space"}
	tm, _ := m.updateExportKey(msg)
	m2 := asModel(tm)

	// Header should now be checked.
	if !m2.export.items[0].checked {
		t.Error("header should be checked when all visible profiles are checked")
	}
}

// --- openExportView tests ---
