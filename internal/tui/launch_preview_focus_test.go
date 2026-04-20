package tui

import (
	"testing"

	"github.com/flyingnobita/llml/internal/models"
)

// setupPreviewScrollableModel returns a model with a selected model row and enough
// content in the launch preview to make it scrollable.
func setupPreviewScrollableModel() Model {
	m := newTestModel()
	// Add a file so SelectedModel returns non-empty.
	m.table.files = []models.ModelFile{
		{Path: "/tmp/test.gguf", Name: "test.gguf", Backend: models.BackendLlama},
	}
	cols := tableColumns(100, m.table.files, m.layout.homeDir, m.table.sortCol, m.table.sortDesc)
	m.table.tbl.SetRows(buildTableRows(m.table.files, cols, m.layout.homeDir))
	m = m.layoutTable()
	// Make the preview scrollable by setting launchPreviewLastCmd to something
	// that the preview won't fit in launchPreviewVisibleLines lines.
	cmd := "llama-server --model /tmp/test.gguf --port 8080 --alias test.gguf\n  --ctx-size 2048 --n-gpu-layers 0\n  --extra-arg-1 foo\n  --extra-arg-2 bar\n  --extra-arg-3 baz"
	m.preview.viewport.SetContent(cmd)
	m.preview.lastCmd = cmd
	return m
}

func setupPreviewVisibleModel() Model {
	m := newTestModel()
	m.table.files = []models.ModelFile{
		{Path: "/tmp/test.gguf", Name: "test.gguf", Backend: models.BackendLlama},
	}
	cols := tableColumns(100, m.table.files, m.layout.homeDir, m.table.sortCol, m.table.sortDesc)
	m.table.tbl.SetRows(buildTableRows(m.table.files, cols, m.layout.homeDir))
	m = m.layoutTable()
	cmd := "llama-server --model /tmp/test.gguf --port 8080"
	m.preview.viewport.SetContent(cmd)
	m.preview.lastCmd = cmd
	return m
}

func TestLaunchPreviewFocus_TabFocusesWhenScrollable(t *testing.T) {
	m := setupPreviewScrollableModel()
	if !m.launchPreviewScrollable() {
		t.Skip("preview not scrollable in this terminal size; skipping focus test")
	}
	m.preview.focused = false

	got, _ := m.Update(tabMsg())
	gm, ok := got.(Model)
	if !ok {
		t.Fatalf("Update returned unexpected type %T", got)
	}
	if !gm.preview.focused {
		t.Fatalf("expected launchPreviewFocused=true after Tab on scrollable preview, got false")
	}
}

func TestLaunchPreviewFocus_TabUnfocuses(t *testing.T) {
	m := setupPreviewScrollableModel()
	if !m.launchPreviewScrollable() {
		t.Skip("preview not scrollable in this terminal size; skipping focus test")
	}
	m.preview.focused = true

	got, _ := m.Update(tabMsg())
	gm, ok := got.(Model)
	if !ok {
		t.Fatalf("Update returned unexpected type %T", got)
	}
	if gm.preview.focused {
		t.Fatalf("expected launchPreviewFocused=false after Tab when already focused, got true")
	}
}

func TestLaunchPreviewFocus_TabFocusesWhenVisible(t *testing.T) {
	m := setupPreviewVisibleModel()
	if !m.launchPreviewVisible() {
		t.Fatal("expected preview to be visible")
	}
	if m.launchPreviewScrollable() {
		t.Fatal("expected preview to be non-scrollable for this test")
	}
	m.preview.focused = false

	got, _ := m.Update(tabMsg())
	gm, ok := got.(Model)
	if !ok {
		t.Fatalf("Update returned unexpected type %T", got)
	}
	if !gm.preview.focused {
		t.Fatalf("expected launchPreviewFocused=true after Tab on visible preview, got false")
	}
}
