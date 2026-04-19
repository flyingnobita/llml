package tui

import (
	"testing"
	"time"

	"github.com/flyingnobita/llml/internal/models"
)

func modelForFocusRestoreTest(t *testing.T) Model {
	t.Helper()
	m := New()
	m.layout.width = 120
	m.layout.height = 40
	m.loading = false
	m.table.files = []models.ModelFile{
		{Backend: models.BackendLlama, Path: "/x.gguf", Name: "x", Size: 1, ModTime: time.Unix(0, 0)},
	}
	m = m.layoutTable()
	m.table.tbl.SetCursor(0)
	return m
}

func TestCloseParamPanel_RestoresLaunchPreviewFocus(t *testing.T) {
	m := modelForFocusRestoreTest(t)
	m.preview.focused = true
	m.table.tbl.Blur()

	m, _ = m.openParamPanel()
	if m.preview.focused {
		t.Fatal("expected launch preview unfocused while parameter modal is open")
	}
	m = m.closeParamPanel()
	if !m.preview.focused {
		t.Fatal("expected launch preview focus restored after closing parameters")
	}
}

func TestCloseParamPanel_RestoresSplitLogFocus(t *testing.T) {
	m := modelForFocusRestoreTest(t)
	m.server.running = true
	m.server.splitFocused = true
	m.preview.focused = false
	m.table.tbl.Blur()

	m, _ = m.openParamPanel()
	// saveMainPaneFocusForModal only clears preview.focused; split log focus stays true while the modal is open.
	if !m.server.splitFocused {
		t.Fatal("expected split log focus unchanged while modal open")
	}
	m = m.closeParamPanel()
	if !m.server.splitFocused {
		t.Fatal("expected split log focus restored after closing parameters")
	}
	if m.preview.focused {
		t.Fatal("did not expect launch preview focused when user had log focused")
	}
}

func TestCloseRuntimeConfig_RestoresLaunchPreviewFocus(t *testing.T) {
	m := modelForFocusRestoreTest(t)
	m.preview.focused = true
	m.table.tbl.Blur()

	m, _ = m.openRuntimeConfigFocused(runtimeFieldLlamaCppPath)
	if m.preview.focused {
		t.Fatal("expected launch preview unfocused while runtime modal is open")
	}
	m = m.closeRuntimeConfig()
	if !m.preview.focused {
		t.Fatal("expected launch preview focus restored after closing runtime config")
	}
}

func TestCloseRuntimeConfig_RestoresSplitLogFocus(t *testing.T) {
	m := modelForFocusRestoreTest(t)
	m.server.running = true
	m.server.splitFocused = true
	m.preview.focused = false
	m.table.tbl.Blur()

	m, _ = m.openRuntimeConfigFocused(runtimeFieldLlamaCppPath)
	if !m.server.splitFocused {
		t.Fatal("expected split log focus unchanged while modal open")
	}
	m = m.closeRuntimeConfig()
	if !m.server.splitFocused {
		t.Fatal("expected split log focus restored after closing runtime config")
	}
}

func TestCloseDiscoveryPathsModal_RestoresLaunchPreviewFocus(t *testing.T) {
	m := modelForFocusRestoreTest(t)
	m.preview.focused = true
	m.table.tbl.Blur()

	m, _ = m.openDiscoveryPathsModal()
	if m.preview.focused {
		t.Fatal("expected launch preview unfocused while discovery modal is open")
	}
	m = m.closeDiscoveryPathsModal()
	if !m.preview.focused {
		t.Fatal("expected launch preview focus restored after closing discovery modal")
	}
}

func TestCloseDiscoveryPathsModal_RestoresSplitLogFocus(t *testing.T) {
	m := modelForFocusRestoreTest(t)
	m.server.running = true
	m.server.splitFocused = true
	m.preview.focused = false
	m.table.tbl.Blur()

	m, _ = m.openDiscoveryPathsModal()
	if !m.server.splitFocused {
		t.Fatal("expected split log focus unchanged while modal open")
	}
	m = m.closeDiscoveryPathsModal()
	if !m.server.splitFocused {
		t.Fatal("expected split log focus restored after closing discovery modal")
	}
}

func TestSaveMainPaneFocusForModal_SecondSaveOverwritesSnapshot(t *testing.T) {
	m := modelForFocusRestoreTest(t)
	m.preview.focused = true
	m = m.saveMainPaneFocusForModal()
	m.server.splitFocused = true
	m = m.saveMainPaneFocusForModal()
	m = m.restoreMainPaneFocusAfterModal()
	if m.preview.focused || !m.server.splitFocused {
		t.Fatalf("got preview=%v split=%v want preview=false split=true (latest snapshot wins)", m.preview.focused, m.server.splitFocused)
	}
}

func TestCloseParamPanel_NoRestoreWithoutSnapshot(t *testing.T) {
	m := modelForFocusRestoreTest(t)
	m.preview.focused = true
	m.table.tbl.Blur()
	m.params.open = true
	m.params.profiles = []ParameterProfile{{Name: "p"}}
	m.params.profileIndex = 0
	m.params.modelPath = "/x.gguf"
	m.params.loadCurrentProfileIn()

	m = m.closeParamPanel()
	if !m.preview.focused {
		t.Fatal("without saveMainPaneFocusForModal, close must not change launch preview (restore is a no-op)")
	}
}
