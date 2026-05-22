package tui

import (
	"testing"

	tea "charm.land/bubbletea/v2"
)

func TestRuntimeConfigEscapeClosesByKeyCode(t *testing.T) {
	m := newTestModel()
	m2, _ := m.openRuntimeConfig()

	got, _ := m2.updateRuntimeConfigKey(tea.KeyPressMsg{Code: tea.KeyEscape})
	if got.rc.open {
		t.Fatal("expected escape key code to close runtime config")
	}
}

func TestDiscoveryPathsEnterStartsEditByKeyCode(t *testing.T) {
	m := newTestModel()
	m.discovery.open = true
	m.discovery.paths = []string{"/models"}
	m.discovery.cursor = 0

	got, _ := m.updateDiscoveryPathsKey(tea.KeyPressMsg{Code: tea.KeyEnter})
	if !got.discovery.editOpen {
		t.Fatal("expected enter key code to open discovery path edit")
	}
}

func TestImportPathTabSwitchesPickerByKeyCode(t *testing.T) {
	m := newTestModel()
	m.import_.open = true
	m.import_.focus = importFocusPath

	got, _ := m.updateImportKey(tea.KeyPressMsg{Code: tea.KeyTab})
	m2 := got.(Model)
	if m2.import_.focus != importFocusPicker {
		t.Fatalf("expected tab key code to switch import focus to picker, got %v", m2.import_.focus)
	}
}

func TestImportPathEnterParsesByKeyCode(t *testing.T) {
	m := newTestModel()
	m.import_.open = true
	m.import_.focus = importFocusPath
	m.import_.filePath = ""

	got, _ := m.updateImportKey(tea.KeyPressMsg{Code: tea.KeyEnter})
	m2 := got.(Model)
	if m2.import_.parseError == "" {
		t.Fatal("expected enter key code to trigger import parse validation")
	}
}

func TestExportPathEnterAttemptsExportByKeyCode(t *testing.T) {
	m := New()
	m.export.open = true
	m.export.focus = exportFocusPath
	m.export.items = []exportProfileItem{
		{kind: exportItemProfile, modelDisplay: "A", backend: "llama", profileName: "p1", checked: true},
	}
	m.export.outputPath = t.TempDir() + "/export.toml"

	got, _ := m.updateExportKey(tea.KeyPressMsg{Code: tea.KeyEnter})
	m2 := got.(Model)
	if m2.lastRunNote == "" {
		t.Fatal("expected enter key code to trigger export attempt")
	}
}

func TestExportFilterTabSwitchesFocusByKeyCode(t *testing.T) {
	m := newTestModel()
	m.export.open = true
	m.export.focus = exportFocusFilter

	got, _ := m.updateExportKey(tea.KeyPressMsg{Code: tea.KeyTab})
	m2 := got.(Model)
	if m2.export.focus != exportFocusPath {
		t.Fatalf("expected tab key code to switch export focus to path, got %v", m2.export.focus)
	}
}

func TestCollisionEscapeClosesByKeyCode(t *testing.T) {
	m := newTestModel()
	m.collision.open = true
	m.collision.dest = "/tmp/export.toml"
	m.collision.suffixPath = "/tmp/export-2.toml"

	got, _ := m.updateCollisionKey(tea.KeyPressMsg{Code: tea.KeyEscape})
	m2 := got.(Model)
	if m2.collision.open {
		t.Fatal("expected escape key code to close collision dialog")
	}
}
