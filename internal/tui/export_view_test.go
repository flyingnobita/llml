package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/flyingnobita/llml/internal/profiles"
)

// testLlmlDir returns the llml config directory for a temp $HOME.
func testLlmlDir(t *testing.T, homeDir string) string {
	t.Helper()
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(homeDir, ".config"))
	cfgDir, err := os.UserConfigDir()
	if err != nil {
		t.Fatal(err)
	}
	d := filepath.Join(cfgDir, "llml")
	if err := os.MkdirAll(d, 0o755); err != nil {
		t.Fatal(err)
	}
	return d
}

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
		{kind: exportItemProfile, modelDisplay: "A", backend: "llama", profileName: "p1", checked: true,
			pp: profiles.PortableProfile{Name: "p1", Backend: "llama", ModelHint: "A"}},
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
		{kind: exportItemProfile, modelDisplay: "A", backend: "llama", profileName: "p1", checked: true,
			pp: profiles.PortableProfile{Name: "p1", Backend: "llama", ModelHint: "A"}},
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
		{kind: exportItemProfile, modelDisplay: "Model-A", backend: "llama", profileName: "p1", checked: true,
			pp: profiles.PortableProfile{Name: "p1", Backend: "llama", ModelHint: "Model-A"}},
		{kind: exportItemProfile, modelDisplay: "Model-B", backend: "vllm", profileName: "p2", checked: false,
			pp: profiles.PortableProfile{Name: "p2", Backend: "vllm", ModelHint: "Model-B"}},
		{kind: exportItemProfile, modelDisplay: "Model-C", backend: "ollama", profileName: "p3", checked: true,
			pp: profiles.PortableProfile{Name: "p3", Backend: "ollama", ModelHint: "Model-C"}},
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

// --- openExportView tests ---

func TestOpenExportView_PopulatesItems(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	llmlDir := testLlmlDir(t, dir)
	paramsPath := filepath.Join(llmlDir, "model-params.json")

	data := []byte(`{
	  "version": 3,
	  "models": {
	    "/models/Alpha.gguf": {
	      "profiles": [
	        {"name": "default", "backend": "llama", "env": [], "args": ["--ctx-size", "4096"]},
	        {"name": "kobold", "backend": "koboldcpp", "env": [], "args": ["--usecublas"]}
	      ],
	      "activeIndex": 0
	    },
	    "/models/Beta.gguf": {
	      "profiles": [
	        {"name": "default", "backend": "vllm", "env": [], "args": ["--n-gpu-layers", "80"]}
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
	if m.export.focus != exportFocusList {
		t.Errorf("focus = %d, want exportFocusList (%d)", m.export.focus, exportFocusList)
	}

	// Should have: Alpha header + 2 profiles + Beta header + 1 profile = 5 items
	if len(m.export.items) != 5 {
		t.Fatalf("len(items) = %d, want 5", len(m.export.items))
	}

	// First item should be Alpha header.
	if m.export.items[0].kind != exportItemHeader {
		t.Error("items[0] should be header")
	}
	if m.export.items[0].modelDisplay != "Alpha" {
		t.Errorf("items[0].modelDisplay = %q, want Alpha", m.export.items[0].modelDisplay)
	}

	// Second item: first profile of Alpha (llama comes before koboldcpp alphabetically).
	if m.export.items[1].kind != exportItemProfile {
		t.Error("items[1] should be profile")
	}
	if m.export.items[1].backend != "koboldcpp" {
		t.Errorf("items[1].backend = %q, want koboldcpp (sorted)", m.export.items[1].backend)
	}
	if m.export.items[1].profileName != "kobold" {
		t.Errorf("items[1].profileName = %q, want kobold", m.export.items[1].profileName)
	}

	// Third: second profile of Alpha (llama).
	if m.export.items[2].kind != exportItemProfile {
		t.Error("items[2] should be profile")
	}
	if m.export.items[2].backend != "llama" {
		t.Errorf("items[2].backend = %q, want llama", m.export.items[2].backend)
	}

	// Fourth item: Beta header.
	if m.export.items[3].kind != exportItemHeader {
		t.Error("items[3] should be header")
	}
	if m.export.items[3].modelDisplay != "Beta" {
		t.Errorf("items[3].modelDisplay = %q, want Beta", m.export.items[3].modelDisplay)
	}

	// Fifth item: Beta's profile.
	if m.export.items[4].kind != exportItemProfile {
		t.Error("items[4] should be profile")
	}
	if m.export.items[4].backend != "vllm" {
		t.Errorf("items[4].backend = %q, want vllm", m.export.items[4].backend)
	}

	// Cursor should start on first profile (skip header).
	if m.export.cursor != 1 {
		t.Errorf("cursor = %d, want 1 (first profile after header)", m.export.cursor)
	}
}

func TestOpenExportView_NoFileOpensEmpty(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)

	m := New()
	m = m.openExportView()

	// With no model-params.json the modal still opens so the user gets feedback.
	if !m.export.open {
		t.Error("export should open even when no profiles exist")
	}
	if len(m.export.items) != 0 {
		t.Errorf("expected 0 items, got %d", len(m.export.items))
	}
}

// --- ScrollbarGlyph tests ---

func TestScrollbarGlyph_NoScrollWhenFits(t *testing.T) {
	got := scrollbarGlyph(0, 10, 5, 0)
	if got != "" {
		t.Errorf("when totalItems <= maxVis, should be empty, got %q", got)
	}
}

func TestScrollbarGlyph_TopArrow(t *testing.T) {
	got := scrollbarGlyph(0, 10, 20, 2)
	if got != "▴" {
		t.Errorf("top arrow expected at row 0 with offset > 0, got %q", got)
	}
}

func TestScrollbarGlyph_BottomArrow(t *testing.T) {
	// scrollOffset=11, maxVis=10, totalItems=25: items 11-20 visible, items 21-24 still below.
	got := scrollbarGlyph(9, 10, 25, 11)
	if got != "▾" {
		t.Errorf("bottom arrow expected at last visible row when more items below, got %q", got)
	}
}

func TestScrollbarGlyph_Thumb(t *testing.T) {
	// When totalItems > maxVis, and row is within thumb range.
	got := scrollbarGlyph(0, 10, 200, 0)
	// At offset 0, thumb starts at 0, first row should be thumb.
	if got != "█" {
		t.Errorf("thumb expected at row 0, offset 0 with 200 items, got %q", got)
	}
}

func TestScrollbarGlyph_Track(t *testing.T) {
	got := scrollbarGlyph(5, 10, 200, 0)
	// At offset 0 with totalItems=200, thumb is small. Row 5 could be track.
	// thumbSize = max(1, 10*10/200) = max(1, 0) = 1
	// thumbStart = 0*10/200 = 0
	// So thumb is only at row 0, row 5 is track.
	if got != "┃" {
		t.Errorf("track expected at row 5, offset 0 with 200 items, got %q", got)
	}
}

func TestScrollbarGlyph_Padding(t *testing.T) {
	got := scrollbarGlyph(10, 10, 20, 10)
	// Row 10 with maxVis=10, totalItems=20, offset=10
	// actualIdx = 10+10 = 20, which >= totalItems (20), so padding.
	if got != " " {
		t.Errorf("padding expected for out-of-range row, got %q", got)
	}
}

// --- AdjustExportScroll tests ---

func TestAdjustExportScroll_CursorAboveScrollOffset(t *testing.T) {
	m := New()
	m.export.open = true
	m.export.cursor = 0
	m.export.scrollOffset = 5

	m.adjustExportScroll()
	if m.export.scrollOffset != 0 {
		t.Errorf("scrollOffset = %d, want 0 (cursor above offset)", m.export.scrollOffset)
	}
}

func TestAdjustExportScroll_CursorBelowScrollWindow(t *testing.T) {
	m := New()
	m.layout.height = 40
	m.export.open = true
	m.export.cursor = 20
	m.export.scrollOffset = 0

	m.adjustExportScroll()
	// maxVis = max(40-13, 3) = 27
	// cursor (20) >= scrollOffset(0) + maxVis(27)? No. So offset stays 0.
	// Let me make cursor far enough.
	m.export.cursor = 30
	m.export.scrollOffset = 0
	m.adjustExportScroll()
	// maxVis = 27, cursor(30) >= 0+27 = 27, so offset = 30-27+1 = 4
	if m.export.scrollOffset != 4 {
		t.Errorf("scrollOffset = %d, want 4", m.export.scrollOffset)
	}
}

// --- doExportAttempt tests ---

func TestDoExportAttempt_SuccessNewFile(t *testing.T) {
	dir := t.TempDir()
	dest := filepath.Join(dir, "export.toml")

	m := New()
	m.export.open = true
	m.export.outputPath = dest
	m.export.items = []exportProfileItem{
		{kind: exportItemProfile, modelDisplay: "A", backend: "llama", profileName: "p1", checked: true,
			pp: profiles.PortableProfile{Name: "p1", Backend: "llama", ModelHint: "A"}},
	}
	m.export.cursor = 0

	tm, _ := m.doExportAttempt()
	m2 := asModel(tm)

	if m2.export.open {
		t.Error("export should close after successful write")
	}
	if _, err := os.Stat(dest); os.IsNotExist(err) {
		t.Error("file should exist after export")
	}
}

func TestDoExportAttempt_CollisionOpensSubModal(t *testing.T) {
	dir := t.TempDir()
	dest := filepath.Join(dir, "export.toml")

	// Create the file first so collision triggers.
	if err := os.WriteFile(dest, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	m := New()
	m.export.open = true
	m.export.outputPath = dest
	m.export.items = []exportProfileItem{
		{kind: exportItemProfile, modelDisplay: "A", backend: "llama", profileName: "p1", checked: true,
			pp: profiles.PortableProfile{Name: "p1", Backend: "llama", ModelHint: "A"}},
	}
	m.export.cursor = 0

	tm, _ := m.doExportAttempt()
	m2 := asModel(tm)

	if !m2.collision.open {
		t.Fatal("collision sub-modal should open when file exists")
	}
	if m2.collision.dest != dest {
		t.Errorf("collision.dest = %q, want %q", m2.collision.dest, dest)
	}
	if m2.collision.suffixPath == "" {
		t.Error("collision.suffixPath should not be empty")
	}
}

// --- Export key handler tests ---

func TestExportKeyFilterFocusTab(t *testing.T) {
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
		t.Errorf("focus = %d, want exportFocusPath (%d)", m2.export.focus, exportFocusPath)
	}
}

func TestExportKeyPathFocusEsc(t *testing.T) {
	m := New()
	m.export.open = true
	m.export.focus = exportFocusPath
	m.export.items = []exportProfileItem{
		{kind: exportItemProfile, modelDisplay: "A", backend: "llama", profileName: "p1", checked: false},
	}

	msg := tea.KeyPressMsg{Code: 27, Text: "esc"}
	tm, _ := m.updateExportKey(msg)
	m2 := asModel(tm)

	if m2.export.focus != exportFocusList {
		t.Errorf("focus = %d, want exportFocusList (%d)", m2.export.focus, exportFocusList)
	}
}

func TestExportKeyPathFocusTab(t *testing.T) {
	m := New()
	m.export.open = true
	m.export.focus = exportFocusPath
	m.export.items = []exportProfileItem{
		{kind: exportItemProfile, modelDisplay: "A", backend: "llama", profileName: "p1", checked: false},
	}

	msg := tea.KeyPressMsg{Code: '\t', Text: "tab"}
	tm, _ := m.updateExportKey(msg)
	m2 := asModel(tm)

	if m2.export.focus != exportFocusList {
		t.Errorf("focus = %d, want exportFocusList (%d)", m2.export.focus, exportFocusList)
	}
}

func TestExportKeyPathFocusEnter(t *testing.T) {
	dir := t.TempDir()
	dest := filepath.Join(dir, "export.toml")

	m := New()
	m.export.open = true
	m.export.focus = exportFocusPath
	m.export.outputPath = dest
	m.export.items = []exportProfileItem{
		{kind: exportItemProfile, modelDisplay: "A", backend: "llama", profileName: "p1", checked: true,
			pp: profiles.PortableProfile{Name: "p1", Backend: "llama", ModelHint: "A"}},
	}
	m.export.cursor = 0

	msg := tea.KeyPressMsg{Code: 13, Text: "enter"}
	tm, _ := m.updateExportKey(msg)
	m2 := asModel(tm)

	// Export should complete and close modal.
	if m2.export.open {
		t.Error("export should close after successful write from path focus enter")
	}
	if _, err := os.Stat(dest); os.IsNotExist(err) {
		t.Error("file should exist after enter from path")
	}
}

func TestExportKeyPageUpDown(t *testing.T) {
	m := New()
	m.export.open = true
	m.export.focus = exportFocusList
	// Create enough items to make page navigation meaningful.
	var items []exportProfileItem
	for i := 0; i < 30; i++ {
		items = append(items, exportProfileItem{
			kind: exportItemProfile, modelDisplay: "M", backend: "llama",
			profileName: fmt.Sprintf("p%d", i), checked: false,
		})
	}
	m.export.items = items
	m.export.cursor = 20
	m.layout.height = 40

	// ctrl+u: page up.
	msg := tea.KeyPressMsg{Code: 21, Text: "ctrl+u"}
	tm, _ := m.updateExportKey(msg)
	m2 := asModel(tm)
	if m2.export.cursor >= 20 {
		t.Errorf("cursor = %d, expected < 20 after page up", m2.export.cursor)
	}

	// ctrl+d: page down from top.
	m2.export.cursor = 0
	msg2 := tea.KeyPressMsg{Code: 4, Text: "ctrl+d"}
	tm, _ = m2.updateExportKey(msg2)
	m3 := asModel(tm)
	if m3.export.cursor <= 0 {
		t.Errorf("cursor = %d, expected > 0 after page down", m3.export.cursor)
	}
}

func TestExportKeySlashEntersFilter(t *testing.T) {
	m := New()
	m.export.open = true
	m.export.focus = exportFocusList
	m.export.items = []exportProfileItem{
		{kind: exportItemProfile, modelDisplay: "A", backend: "llama", profileName: "p1", checked: false},
	}

	msg := tea.KeyPressMsg{Code: '/', Text: "/"}
	tm, _ := m.updateExportKey(msg)
	m2 := asModel(tm)

	if m2.export.focus != exportFocusFilter {
		t.Errorf("focus = %d, want exportFocusFilter (%d)", m2.export.focus, exportFocusFilter)
	}
}

func TestExportKeyTabFromListToPath(t *testing.T) {
	m := New()
	m.export.open = true
	m.export.focus = exportFocusList
	m.export.items = []exportProfileItem{
		{kind: exportItemProfile, modelDisplay: "A", backend: "llama", profileName: "p1", checked: false},
	}
	m.export.cursor = 0
	m.export.outputPath = "/tmp/test.toml"

	msg := tea.KeyPressMsg{Code: '\t', Text: "tab"}
	tm, _ := m.updateExportKey(msg)
	m2 := asModel(tm)

	if m2.export.focus != exportFocusPath {
		t.Errorf("focus = %d, want exportFocusPath (%d)", m2.export.focus, exportFocusPath)
	}
}

// --- Rendering tests ---

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
	m.rebuildExportFilter()

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
	for i := 0; i < 20; i++ {
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
	m.toggleGroup(true)
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
	m.toggleGroup(true)
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
	m.rebuildExportFilter()

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
