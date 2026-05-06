package tui

import (
	"os"
	"path/filepath"
	"testing"

	tea "charm.land/bubbletea/v2"
)

func TestExportOpenClose(t *testing.T) {
	m := New()

	if m.export.open {
		t.Fatal("export.open should be false initially")
	}

	m.export.open = true
	m.export.focus = exportFocusList
	m.export.items = []exportProfileItem{
		{kind: exportItemProfile, modelDisplay: "Model-A", backend: "llama", profileName: "default", checked: true},
		{kind: exportItemProfile, modelDisplay: "Model-B", backend: "vllm", profileName: "gpu", checked: false},
	}
	m.export.cursor = 0
	m.export.outputPath = "/tmp/export.toml"

	if m.exportSelectedCount() != 1 {
		t.Fatalf("selected count = %d, want 1", m.exportSelectedCount())
	}

	m = m.closeExportView()
	if m.export.open {
		t.Fatal("export.open should be false after close")
	}
	if len(m.export.items) != 0 {
		t.Fatalf("items should be nil after close, got %d", len(m.export.items))
	}
}

func asModel(tm tea.Model) Model { return tm.(Model) }

func TestExportToggleCheckbox(t *testing.T) {
	m := New()
	m.export.open = true
	m.export.focus = exportFocusList
	m.export.items = []exportProfileItem{
		{kind: exportItemProfile, modelDisplay: "Model-A", backend: "llama", profileName: "default", checked: true},
		{kind: exportItemProfile, modelDisplay: "Model-B", backend: "vllm", profileName: "gpu", checked: false},
	}
	m.export.cursor = 0

	msg := tea.KeyPressMsg{Code: ' ', Text: "space"}
	tm, _ := m.updateExportKey(msg)
	m2 := asModel(tm)
	if m2.export.items[0].checked {
		t.Error("item 0 should be unchecked after space")
	}

	m2.export.cursor = 1
	tm, _ = m2.updateExportKey(msg)
	m3 := asModel(tm)
	if !m3.export.items[1].checked {
		t.Error("item 1 should be checked after space")
	}
}

func TestExportSelectAll(t *testing.T) {
	m := New()
	m.export.open = true
	m.export.focus = exportFocusList
	m.export.items = []exportProfileItem{
		{kind: exportItemProfile, modelDisplay: "Model-A", backend: "llama", profileName: "p1", checked: false},
		{kind: exportItemProfile, modelDisplay: "Model-A", backend: "llama", profileName: "p2", checked: false},
		{kind: exportItemProfile, modelDisplay: "Model-B", backend: "vllm", profileName: "p1", checked: true},
	}
	m.export.cursor = 0

	msg := tea.KeyPressMsg{Code: 'a', Text: "a"}
	tm, _ := m.updateExportKey(msg)
	m2 := asModel(tm)
	for i, it := range m2.export.items {
		if !it.checked {
			t.Errorf("item %d should be checked after select-all", i)
		}
	}
	if m2.exportSelectedCount() != 3 {
		t.Errorf("selected count = %d, want 3", m2.exportSelectedCount())
	}
}

func TestExportSelectNone(t *testing.T) {
	m := New()
	m.export.open = true
	m.export.focus = exportFocusList
	m.export.items = []exportProfileItem{
		{kind: exportItemProfile, modelDisplay: "Model-A", backend: "llama", profileName: "p1", checked: true},
		{kind: exportItemProfile, modelDisplay: "Model-A", backend: "llama", profileName: "p2", checked: true},
		{kind: exportItemProfile, modelDisplay: "Model-B", backend: "vllm", profileName: "p1", checked: true},
	}
	m.export.cursor = 0

	msg := tea.KeyPressMsg{Code: 'A', Text: "A"}
	tm, _ := m.updateExportKey(msg)
	m2 := asModel(tm)
	for i, it := range m2.export.items {
		if it.checked {
			t.Errorf("item %d should be unchecked after select-none", i)
		}
	}
	if m2.exportSelectedCount() != 0 {
		t.Errorf("selected count = %d, want 0", m2.exportSelectedCount())
	}
}

func TestExportEscapeCloses(t *testing.T) {
	m := New()
	m.export.open = true
	m.export.focus = exportFocusList
	m.export.items = []exportProfileItem{
		{kind: exportItemProfile, modelDisplay: "Model-A", backend: "llama", profileName: "default", checked: true},
	}

	msg := tea.KeyPressMsg{Code: 27, Text: "esc"}
	tm, _ := m.updateExportKey(msg)
	m2 := asModel(tm)
	if m2.export.open {
		t.Error("export should be closed after esc")
	}
}

func TestExportTabFocusChange(t *testing.T) {
	m := New()
	m.export.open = true
	m.export.focus = exportFocusList
	m.export.items = []exportProfileItem{
		{kind: exportItemProfile, modelDisplay: "Model-A", backend: "llama", profileName: "default", checked: true},
	}
	m.export.cursor = 0
	m.export.outputPath = "/tmp/export.toml"

	msg := tea.KeyPressMsg{Code: '\t', Text: "tab"}
	tm, _ := m.updateExportKey(msg)
	m2 := asModel(tm)
	if m2.export.focus != exportFocusPath {
		t.Error("focus should switch to path after tab")
	}

	tm, _ = m2.updateExportKey(msg)
	m3 := asModel(tm)
	if m3.export.focus != exportFocusList {
		t.Error("focus should switch back to list after tab")
	}
}

func TestExportCursorNavigation(t *testing.T) {
	m := New()
	m.export.open = true
	m.export.focus = exportFocusList
	m.export.items = []exportProfileItem{
		{kind: exportItemProfile, modelDisplay: "A", backend: "llama", profileName: "p1", checked: true},
		{kind: exportItemProfile, modelDisplay: "B", backend: "llama", profileName: "p2", checked: true},
		{kind: exportItemProfile, modelDisplay: "C", backend: "vllm", profileName: "p3", checked: true},
	}
	m.export.cursor = 0

	msg := tea.KeyPressMsg{Code: 'j', Text: "down"}
	tm, _ := m.updateExportKey(msg)
	m2 := asModel(tm)
	if m2.export.cursor != 1 {
		t.Errorf("cursor = %d, want 1", m2.export.cursor)
	}

	msg2 := tea.KeyPressMsg{Code: 'k', Text: "up"}
	tm, _ = m2.updateExportKey(msg2)
	m3 := asModel(tm)
	if m3.export.cursor != 0 {
		t.Errorf("cursor = %d, want 0", m3.export.cursor)
	}

	m3.export.cursor = 2
	msg3 := tea.KeyPressMsg{Code: 'j', Text: "down"}
	tm, _ = m3.updateExportKey(msg3)
	m4 := asModel(tm)
	if m4.export.cursor != 2 {
		t.Errorf("cursor = %d, want 2 (at end)", m4.export.cursor)
	}

	m4.export.cursor = 0
	msg4 := tea.KeyPressMsg{Code: 'k', Text: "up"}
	tm, _ = m4.updateExportKey(msg4)
	m5 := asModel(tm)
	if m5.export.cursor != 0 {
		t.Errorf("cursor = %d, want 0 (at start)", m5.export.cursor)
	}
}

func TestExportZeroSelectedEnterNoop(t *testing.T) {
	m := New()
	m.export.open = true
	m.export.focus = exportFocusList
	m.export.items = []exportProfileItem{
		{kind: exportItemProfile, modelDisplay: "A", backend: "llama", profileName: "p1", checked: false},
	}
	m.export.cursor = 0
	m.export.outputPath = filepath.Join(t.TempDir(), "test.toml")

	msg := tea.KeyPressMsg{Code: 13, Text: "enter"}
	tm, cmd := m.updateExportKey(msg)
	m2 := asModel(tm)
	if cmd != nil {
		t.Error("enter with no selection should be no-op")
	}
	if !m2.export.open {
		t.Error("export should stay open when no selection")
	}
}

func TestCollisionEscapeCloses(t *testing.T) {
	m := New()
	m.collision.open = true
	m.collision.dest = "/tmp/export.toml"
	m.collision.suffixPath = "/tmp/export-2.toml"

	msg := tea.KeyPressMsg{Code: 27, Text: "esc"}
	tm, _ := m.updateCollisionKey(msg)
	m2 := asModel(tm)
	if m2.collision.open {
		t.Error("collision should be closed after esc")
	}
}

func TestCollisionOverwrite(t *testing.T) {
	dir := t.TempDir()
	dest := filepath.Join(dir, "export.toml")

	m := New()
	m.collision.open = true
	m.collision.dest = dest
	m.collision.suffixPath = filepath.Join(dir, "export-2.toml")
	m.export.items = []exportProfileItem{
		{kind: exportItemProfile, modelDisplay: "A", backend: "llama", profileName: "p1", checked: true},
	}
	m.export.outputPath = dest

	msg := tea.KeyPressMsg{Code: 'o', Text: "o"}
	tm, _ := m.updateCollisionKey(msg)
	m2 := asModel(tm)
	if m2.collision.open {
		t.Error("collision should be closed after overwrite")
	}
	if m2.export.open {
		t.Error("export should be closed after successful write")
	}
}

func TestCollisionSaveAs(t *testing.T) {
	dir := t.TempDir()
	dest := filepath.Join(dir, "export.toml")
	suffix := filepath.Join(dir, "export-2.toml")

	m := New()
	m.collision.open = true
	m.collision.dest = dest
	m.collision.suffixPath = suffix
	m.export.items = []exportProfileItem{
		{kind: exportItemProfile, modelDisplay: "A", backend: "llama", profileName: "p1", checked: true},
	}
	m.export.outputPath = suffix

	msg := tea.KeyPressMsg{Code: 'n', Text: "n"}
	tm, _ := m.updateCollisionKey(msg)
	m2 := asModel(tm)
	if m2.collision.open {
		t.Error("collision should be closed after save-as")
	}
	if m2.export.open {
		t.Error("export should be closed after successful write")
	}

	if _, err := os.Stat(suffix); os.IsNotExist(err) {
		t.Error("file should exist at suffix path")
	}
}

func TestBuildExportProfiles(t *testing.T) {
	m := New()
	m.export.items = []exportProfileItem{
		{kind: exportItemProfile, modelDisplay: "Model-A", backend: "llama", profileName: "p1", checked: true},
		{kind: exportItemProfile, modelDisplay: "Model-B", backend: "vllm", profileName: "p2", checked: false},
		{kind: exportItemProfile, modelDisplay: "Model-C", backend: "ollama", profileName: "p3", checked: true},
	}

	pps := m.buildExportProfiles()
	if len(pps) != 2 {
		t.Fatalf("len = %d, want 2", len(pps))
	}
	if pps[0].Name != "p1" || pps[0].Backend != "llama" {
		t.Errorf("profile 0: name=%q backend=%q", pps[0].Name, pps[0].Backend)
	}
	if pps[1].Name != "p3" || pps[1].Backend != "ollama" {
		t.Errorf("profile 1: name=%q backend=%q", pps[1].Name, pps[1].Backend)
	}
}

func TestExportModalBlockRendering(t *testing.T) {
	m := New()
	m.layout.width = 100
	m.layout.height = 40
	m.layout.bodyInnerW = 80
	m.export.open = true
	m.export.focus = exportFocusList
	m.export.items = []exportProfileItem{
		{kind: exportItemHeader, modelKey: "/models/A.gguf", modelDisplay: "Model-A", checked: true},
		{kind: exportItemProfile, modelKey: "/models/A.gguf", modelDisplay: "Model-A", backend: "llama", profileName: "gpu-full", checked: true},
		{kind: exportItemHeader, modelKey: "/models/B.gguf", modelDisplay: "Model-B", checked: false},
		{kind: exportItemProfile, modelKey: "/models/B.gguf", modelDisplay: "Model-B", backend: "vllm", profileName: "default", checked: false},
	}
	m.export.cursor = 0
	m.export.outputPath = "/tmp/llml-profiles-20260506.toml"
	m.export.pathInput.SetValue("/tmp/llml-profiles-20260506.toml")

	block := m.exportModalBlock()
	if block == "" {
		t.Error("exportModalBlock should not be empty")
	}
	if !contains(block, "Export Profiles") {
		t.Error("missing title")
	}
	if !contains(block, "gpu-full") {
		t.Error("missing profile name")
	}
	if !contains(block, "Model-A") {
		t.Error("missing model display")
	}
	if !contains(block, "Model-B") {
		t.Error("missing second model display")
	}
}

func TestCollisionModalBlockRendering(t *testing.T) {
	m := New()
	m.layout.width = 100
	m.layout.height = 40
	m.collision.open = true
	m.collision.dest = "/tmp/llml-profiles-20260506.toml"
	m.collision.suffixPath = "/tmp/llml-profiles-20260506-2.toml"

	block := m.collisionModalBlock()
	if block == "" {
		t.Error("collisionModalBlock should not be empty")
	}
	if !contains(block, "File exists") {
		t.Error("missing title")
	}
	if !contains(block, "already exists") {
		t.Error("missing collision message")
	}
	if !contains(block, "Overwrite") {
		t.Error("missing overwrite option")
	}
	if !contains(block, "llml-profiles-20260506-2.toml") {
		t.Error("missing suffix path")
	}
}

func TestExportEscPriorityOverParams(t *testing.T) {
	m := New()
	m.export.open = true
	m.export.focus = exportFocusList
	m.export.items = []exportProfileItem{
		{kind: exportItemProfile, modelDisplay: "A", backend: "llama", profileName: "p1", checked: true},
	}
	m.params.open = true

	msg := tea.KeyPressMsg{Code: 27, Text: "esc"}
	m2, _ := m.handleKey(msg)
	m3 := asModel(m2)
	if m3.export.open {
		t.Error("export should be closed after esc (export has priority over params)")
	}
	if !m3.params.open {
		t.Error("params should still be open after esc (only export closed)")
	}
}

// --- Header grouping tests ---

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
		{kind: exportItemProfile, modelKey: "/models/A.gguf", modelDisplay: "Model-A", backend: "llama", profileName: "p1", checked: true},
		{kind: exportItemProfile, modelKey: "/models/A.gguf", modelDisplay: "Model-A", backend: "vllm", profileName: "p2", checked: true},
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
	tm, _ = m2.updateExportKey(msg) // j
	m3 := asModel(tm)
	// Actually let me fix this — msg is 'k' not 'j'.
	msg2 := tea.KeyPressMsg{Code: 'j', Text: "down"}
	tm, _ = m2.updateExportKey(msg2)
	m3 = asModel(tm)
	if m3.export.cursor != 1 {
		t.Errorf("cursor = %d, want 1 (clamped at end)", m3.export.cursor)
	}
}

// --- Filter tests ---

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
	m.rebuildExportFilter()

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
	m.rebuildExportFilter()

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
	m.rebuildExportFilter()

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
	m2.rebuildExportFilter()

	// Toggle the visible "cpu" profile.
	m2.export.cursor = 0 // cpu is the only visible item (index 0 in filtered view)
	tm, _ = m2.updateExportKey(msg)
	m3 := asModel(tm)

	// Clear filter.
	m3.export.filterInput.SetValue("")
	m3.rebuildExportFilter()

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
	m.rebuildExportFilter()
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
	m.rebuildExportFilter()

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
	m.rebuildExportFilter()

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
	m.rebuildExportFilter()

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
	m.rebuildExportFilter()

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
	m.rebuildExportFilter()

	// Header should show ☐ since the only visible profile (cpu) is unchecked.
	m.syncHeaderStates()
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

func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
