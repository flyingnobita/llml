package tui

import (
	"os"
	"path/filepath"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

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

func TestExportModalBlock_FooterTracksFocus(t *testing.T) {
	m := New()
	m.layout.width = 100
	m.layout.height = 40
	m.layout.bodyInnerW = 80
	m.export.open = true
	m.export.items = []exportProfileItem{
		{kind: exportItemProfile, modelDisplay: "Model-A", backend: "llama", profileName: "gpu-full", checked: true},
	}
	m.export.outputPath = "/tmp/llml-profiles.toml"

	m.export.focus = exportFocusList
	listBlock := ansi.Strip(m.exportModalBlock())
	if !contains(listBlock, "/: filter") {
		t.Fatal("expected list footer to advertise filter entry")
	}
	if contains(listBlock, "type: filter") {
		t.Fatal("did not expect filter-focus footer while list is focused")
	}

	m.export.focus = exportFocusFilter
	filterBlock := ansi.Strip(m.exportModalBlock())
	if !contains(filterBlock, "type: filter") {
		t.Fatal("expected filter footer after entering filter focus")
	}
	if contains(filterBlock, "/: filter") {
		t.Fatal("did not expect list footer after entering filter focus")
	}

	m.export.focus = exportFocusPath
	pathBlock := ansi.Strip(m.exportModalBlock())
	if !contains(pathBlock, "tab: list") {
		t.Fatal("expected path footer to advertise returning to list")
	}
}

func TestExportFilterTextPersistsWhenRefocusedFromList(t *testing.T) {
	m := New()
	m.export.open = true
	m.export.focus = exportFocusList
	m.export.items = []exportProfileItem{
		{kind: exportItemProfile, modelDisplay: "Model-A", backend: "llama", profileName: "gpu-full", checked: true},
		{kind: exportItemProfile, modelDisplay: "Model-B", backend: "vllm", profileName: "cpu", checked: false},
	}
	m.export.filterInput.SetValue("gpu")
	m = m.rebuildExportFilter()

	tm, _ := m.updateExportKey(tea.KeyPressMsg{Code: '/', Text: "/"})
	m2 := asModel(tm)
	if m2.export.focus != exportFocusFilter {
		t.Fatal("expected / to focus filter from list")
	}
	if got := m2.export.filterInput.Value(); got != "gpu" {
		t.Fatalf("expected existing filter text to persist, got %q", got)
	}
}

func TestExportFilterTextPersistsWhenRefocusedFromPath(t *testing.T) {
	m := New()
	m.export.open = true
	m.export.focus = exportFocusPath
	m.export.outputPath = "/tmp/export.toml"
	m.export.pathInput.SetValue(m.export.outputPath)
	m.export.items = []exportProfileItem{
		{kind: exportItemProfile, modelDisplay: "Model-A", backend: "llama", profileName: "gpu-full", checked: true},
		{kind: exportItemProfile, modelDisplay: "Model-B", backend: "vllm", profileName: "cpu", checked: false},
	}
	m.export.filterInput.SetValue("gpu")
	m = m.rebuildExportFilter()

	tm, _ := m.updateExportKey(tea.KeyPressMsg{Code: '/', Text: "/"})
	m2 := asModel(tm)
	if m2.export.focus != exportFocusFilter {
		t.Fatal("expected / to focus filter from path")
	}
	if got := m2.export.filterInput.Value(); got != "gpu" {
		t.Fatalf("expected existing filter text to persist, got %q", got)
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
