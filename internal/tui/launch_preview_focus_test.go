package tui

import (
	"path/filepath"
	"testing"

	"github.com/flyingnobita/llml/internal/config"
	"github.com/flyingnobita/llml/internal/models"
	"github.com/flyingnobita/llml/internal/profiles"
)

// setupPreviewScrollableModel returns a model with a selected model row and enough
// content in the launch preview to make it scrollable.
func setupPreviewScrollableModel() Model {
	m := newTestModel()
	// Add a file so SelectedModel returns non-empty.
	m.table.files = []models.ModelFile{
		{Path: "/tmp/test.gguf", Name: "test.gguf", Backend: models.BackendLlama},
	}
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

// [ and ] scroll a launch preview too long to show at once, without moving
// focus off the table: in the idle view and beside a running split-pane
// server. The help panel lists them from the same binding.
func TestLaunchPreview_bracketsScrollWithoutFocus(t *testing.T) {
	dir := useTempConfigDir(t)
	row := testRow(models.BackendLlama, filepath.Join(dir, "gemma.gguf"))
	args := []string{"--ctx-size", "32768", "--temp", "0.7", "--top-p", "0.9", "--top-k", "40", "--min-p", "0.05", "--flash-attn", "on"}
	saveProfiles(t, row.Path, profiles.Entry{Profiles: []profiles.Profile{{Name: "long", Args: args}}})

	for _, split := range []bool{false, true} {
		m := dimModel(t, newLaunchFakes().services, linuxPlatform, config.RuntimeStates{}, row)
		m.server.running = split
		if m.preview.viewport.TotalLineCount() <= m.preview.viewport.VisibleLineCount() {
			t.Fatalf("the preview should overflow for this test:\n%s", plainView(m))
		}
		m = press(t, m, keyText("]"), keyText("]"))
		if got := m.preview.viewport.YOffset(); got != 2 {
			t.Errorf("split=%t: ] twice should scroll the preview down 2 lines, offset %d", split, got)
		}
		if m.preview.focused || m.server.splitFocused {
			t.Errorf("split=%t: ] should not move focus", split)
		}
		m = press(t, m, keyText("["))
		if got := m.preview.viewport.YOffset(); got != 1 {
			t.Errorf("split=%t: [ should scroll the preview up a line, offset %d", split, got)
		}
	}

	var listed bool
	for _, s := range helpSections() {
		for _, e := range s.entries {
			listed = listed || (e.key == DefaultKeyMap().ScrollPreviewUp.Help().Key && e.desc == DefaultKeyMap().ScrollPreviewUp.Help().Desc)
		}
	}
	if !listed {
		t.Error("the help panel should list the preview scroll binding")
	}
}
