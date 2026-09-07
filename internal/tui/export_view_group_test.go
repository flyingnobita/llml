package tui

import (
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/flyingnobita/llml/internal/profiles"
)

// Export panel tests: header grouping and selection state.
// Split out of export_view_test.go, which had grown past 1900 lines.

func TestExportHeaderToggleSelectsGroup(t *testing.T) {
	m := New()
	m.export.open = true
	m.export.focus = exportFocusList
	m.export.items = []exportProfileItem{
		{kind: exportItemHeader, modelKey: "/models/A.gguf", modelDisplay: "Model-A", checked: false},
		{kind: exportItemProfile, modelKey: "/models/A.gguf", modelDisplay: "Model-A", backend: "llama", profileName: "p1", checked: false},
		{kind: exportItemProfile, modelKey: "/models/A.gguf", modelDisplay: "Model-A", backend: "koboldcpp", profileName: "p2", checked: false},
		{kind: exportItemHeader, modelKey: "/models/B.gguf", modelDisplay: "Model-B", checked: false},
		{kind: exportItemProfile, modelKey: "/models/B.gguf", modelDisplay: "Model-B", backend: "vllm", profileName: "p3", checked: false},
	}
	m.export.cursor = 0 // header for Model-A

	msg := tea.KeyPressMsg{Code: ' ', Text: "space"}
	tm, _ := m.updateExportKey(msg)
	m2 := asModel(tm)

	// All Model-A profiles should now be checked.
	if !m2.export.items[1].checked {
		t.Error("profile p1 should be checked after header toggle")
	}
	if !m2.export.items[2].checked {
		t.Error("profile p2 should be checked after header toggle")
	}
	// Header should now be checked.
	if !m2.export.items[0].checked {
		t.Error("header should be checked after toggling group on")
	}
	// Model-B should be unaffected.
	if m2.export.items[3].checked {
		t.Error("Model-B header should be unaffected")
	}
	if m2.export.items[4].checked {
		t.Error("Model-B profile should be unaffected")
	}
}

func TestExportHeaderToggleUnselectsGroup(t *testing.T) {
	m := New()
	m.export.open = true
	m.export.focus = exportFocusList
	m.export.items = []exportProfileItem{
		{kind: exportItemHeader, modelKey: "/models/A.gguf", modelDisplay: "Model-A", checked: true},
		{kind: exportItemProfile, modelKey: "/models/A.gguf", modelDisplay: "Model-A", backend: "llama", profileName: "p1", checked: true},
		{kind: exportItemProfile, modelKey: "/models/A.gguf", modelDisplay: "Model-A", backend: "llama", profileName: "p2", checked: true},
	}
	m.export.cursor = 0 // header

	msg := tea.KeyPressMsg{Code: ' ', Text: "space"}
	tm, _ := m.updateExportKey(msg)
	m2 := asModel(tm)

	if m2.export.items[1].checked {
		t.Error("profile p1 should be unchecked after header toggle")
	}
	if m2.export.items[2].checked {
		t.Error("profile p2 should be unchecked after header toggle")
	}
	if m2.export.items[0].checked {
		t.Error("header should be unchecked after toggling group off")
	}
}

func TestExportHeaderStateSyncsOnProfileToggle(t *testing.T) {
	m := New()
	m.export.open = true
	m.export.focus = exportFocusList
	m.export.items = []exportProfileItem{
		{kind: exportItemHeader, modelKey: "/models/A.gguf", modelDisplay: "Model-A", checked: true},
		{kind: exportItemProfile, modelKey: "/models/A.gguf", modelDisplay: "Model-A", backend: "llama", profileName: "p1", checked: true},
		{kind: exportItemProfile, modelKey: "/models/A.gguf", modelDisplay: "Model-A", backend: "llama", profileName: "p2", checked: true},
	}
	m.export.cursor = 1 // first profile

	// Uncheck one profile.
	msg := tea.KeyPressMsg{Code: ' ', Text: "space"}
	tm, _ := m.updateExportKey(msg)
	m2 := asModel(tm)

	if m2.export.items[1].checked {
		t.Error("p1 should be unchecked")
	}
	// Header should become unchecked since not all profiles are checked.
	if m2.export.items[0].checked {
		t.Error("header should be unchecked when not all profiles are checked")
	}

	// Re-check it.
	m2.export.cursor = 1
	tm, _ = m2.updateExportKey(msg)
	m3 := asModel(tm)

	if !m3.export.items[1].checked {
		t.Error("p1 should be checked")
	}
	if !m3.export.items[0].checked {
		t.Error("header should be checked when all profiles are checked")
	}
}

func TestExportSelectAllSkipsHeaders(t *testing.T) {
	m := New()
	m.export.open = true
	m.export.focus = exportFocusList
	m.export.items = []exportProfileItem{
		{kind: exportItemHeader, modelKey: "/models/A.gguf", modelDisplay: "Model-A", checked: false},
		{kind: exportItemProfile, modelKey: "/models/A.gguf", modelDisplay: "Model-A", backend: "llama", profileName: "p1", checked: false},
		{kind: exportItemHeader, modelKey: "/models/B.gguf", modelDisplay: "Model-B", checked: false},
		{kind: exportItemProfile, modelKey: "/models/B.gguf", modelDisplay: "Model-B", backend: "vllm", profileName: "p2", checked: false},
	}
	m.export.cursor = 0

	msg := tea.KeyPressMsg{Code: 'a', Text: "a"}
	tm, _ := m.updateExportKey(msg)
	m2 := asModel(tm)

	if !m2.export.items[1].checked {
		t.Error("p1 should be checked")
	}
	if !m2.export.items[3].checked {
		t.Error("p2 should be checked")
	}
	// Headers should sync to checked since all profiles are now checked.
	if !m2.export.items[0].checked {
		t.Error("Model-A header should be checked after select-all")
	}
	if !m2.export.items[2].checked {
		t.Error("Model-B header should be checked after select-all")
	}
	if m2.exportSelectedCount() != 2 {
		t.Errorf("selected count = %d, want 2 (headers excluded)", m2.exportSelectedCount())
	}
}

func TestExportSelectNoneSkipsHeaders(t *testing.T) {
	m := New()
	m.export.open = true
	m.export.focus = exportFocusList
	m.export.items = []exportProfileItem{
		{kind: exportItemHeader, modelKey: "/models/A.gguf", modelDisplay: "Model-A", checked: true},
		{kind: exportItemProfile, modelKey: "/models/A.gguf", modelDisplay: "Model-A", backend: "llama", profileName: "p1", checked: true},
		{kind: exportItemProfile, modelKey: "/models/A.gguf", modelDisplay: "Model-A", backend: "llama", profileName: "p2", checked: true},
	}
	m.export.cursor = 0

	msg := tea.KeyPressMsg{Code: 'A', Text: "A"}
	tm, _ := m.updateExportKey(msg)
	m2 := asModel(tm)

	if m2.export.items[1].checked {
		t.Error("p1 should be unchecked")
	}
	if m2.export.items[2].checked {
		t.Error("p2 should be unchecked")
	}
	if m2.export.items[0].checked {
		t.Error("header should be unchecked after select-none")
	}
}

func TestBuildExportProfilesExcludesHeaders(t *testing.T) {
	m := New()
	m.export.items = []exportProfileItem{
		{kind: exportItemHeader, modelKey: "/models/A.gguf", modelDisplay: "Model-A", checked: true},
		{kind: exportItemProfile, modelKey: "/models/A.gguf", modelDisplay: "Model-A", backend: "llama", profileName: "p1", checked: true,
			pp: profiles.PortableProfile{Name: "p1", Backend: "llama", ModelHint: "Model-A"}},
		{kind: exportItemProfile, modelKey: "/models/A.gguf", modelDisplay: "Model-A", backend: "vllm", profileName: "p2", checked: true,
			pp: profiles.PortableProfile{Name: "p2", Backend: "vllm", ModelHint: "Model-A"}},
	}

	pps := m.buildExportProfiles()
	if len(pps) != 2 {
		t.Fatalf("len = %d, want 2 (header excluded)", len(pps))
	}
	if pps[0].Name != "p1" {
		t.Errorf("pps[0].Name = %q, want p1", pps[0].Name)
	}
	if pps[1].Name != "p2" {
		t.Errorf("pps[1].Name = %q, want p2", pps[1].Name)
	}
}

func TestExportRenderingWithHeaders(t *testing.T) {
	m := New()
	m.layout.width = 100
	m.layout.height = 40
	m.layout.bodyInnerW = 80
	m.export.open = true
	m.export.focus = exportFocusList
	m.export.items = []exportProfileItem{
		{kind: exportItemHeader, modelKey: "/models/A.gguf", modelDisplay: "Model-A", checked: true},
		{kind: exportItemProfile, modelKey: "/models/A.gguf", modelDisplay: "Model-A", backend: "llama", profileName: "default", checked: true},
		{kind: exportItemProfile, modelKey: "/models/A.gguf", modelDisplay: "Model-A", backend: "koboldcpp", profileName: "kobold", checked: false},
	}
	m.export.cursor = 0
	m.export.outputPath = "/tmp/test.toml"
	m.export.pathInput.SetValue("/tmp/test.toml")

	block := m.exportModalBlock()
	if block == "" {
		t.Error("exportModalBlock should not be empty")
	}
	if !contains(block, "Export Profiles") {
		t.Error("missing title")
	}
	if !contains(block, "Model-A") {
		t.Error("missing header model display")
	}
	if !contains(block, "default") {
		t.Error("missing profile name")
	}
	if !contains(block, "koboldcpp") {
		t.Error("missing backend name for second profile")
	}
}

func TestExportCursorOnHeaderNavigates(t *testing.T) {
	m := New()
	m.export.open = true
	m.export.focus = exportFocusList
	m.export.items = []exportProfileItem{
		{kind: exportItemHeader, modelKey: "/models/A.gguf", modelDisplay: "Model-A", checked: false},
		{kind: exportItemProfile, modelKey: "/models/A.gguf", modelDisplay: "Model-A", backend: "llama", profileName: "p1", checked: false},
		{kind: exportItemHeader, modelKey: "/models/B.gguf", modelDisplay: "Model-B", checked: false},
		{kind: exportItemProfile, modelKey: "/models/B.gguf", modelDisplay: "Model-B", backend: "vllm", profileName: "p2", checked: false},
	}
	m.export.cursor = 0

	// Down from header to first profile.
	msg := tea.KeyPressMsg{Code: 'j', Text: "down"}
	tm, _ := m.updateExportKey(msg)
	m2 := asModel(tm)
	if m2.export.cursor != 1 {
		t.Errorf("cursor = %d, want 1 (first profile)", m2.export.cursor)
	}

	// Up from profile to header.
	msg2 := tea.KeyPressMsg{Code: 'k', Text: "up"}
	tm, _ = m2.updateExportKey(msg2)
	m3 := asModel(tm)
	if m3.export.cursor != 0 {
		t.Errorf("cursor = %d, want 0 (back to header)", m3.export.cursor)
	}
}

func TestExportHeaderAtZeroCursorNavigationClamped(t *testing.T) {
	m := New()
	m.export.open = true
	m.export.focus = exportFocusList
	m.export.items = []exportProfileItem{
		{kind: exportItemHeader, modelKey: "/models/A.gguf", modelDisplay: "Model-A", checked: false},
		{kind: exportItemProfile, modelKey: "/models/A.gguf", modelDisplay: "Model-A", backend: "llama", profileName: "p1", checked: false},
	}
	m.export.cursor = 0

	// Up at start should stay at 0.
	msg := tea.KeyPressMsg{Code: 'k', Text: "up"}
	tm, _ := m.updateExportKey(msg)
	m2 := asModel(tm)
	if m2.export.cursor != 0 {
		t.Errorf("cursor = %d, want 0 (clamped at start)", m2.export.cursor)
	}

	// Down at end should stay at end.
	m2.export.cursor = 1
	_, _ = m2.updateExportKey(msg) // j — msg is 'k', overwritten below
	msg2 := tea.KeyPressMsg{Code: 'j', Text: "down"}
	tm, _ = m2.updateExportKey(msg2)
	m3 := asModel(tm)
	if m3.export.cursor != 1 {
		t.Errorf("cursor = %d, want 1 (clamped at end)", m3.export.cursor)
	}
}

// --- Filter tests ---
