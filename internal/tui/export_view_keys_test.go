package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/flyingnobita/llml/internal/profiles"
)

// Export panel tests: key handling.
// Split out of export_view_test.go, which had grown past 1900 lines.

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
	for i := range 30 {
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
