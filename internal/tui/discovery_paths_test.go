package tui

import (
	"strings"
	"testing"
	"time"

	"github.com/flyingnobita/llml/internal/config"
	"github.com/flyingnobita/llml/internal/models"
)

func TestDiscoveryPathsModal_opensAndLoadsPaths(t *testing.T) {
	m := New()
	if m.discovery.open {
		t.Fatal("should not open on startup")
	}

	m.discovery.paths = []string{"/prev/path"}

	m2, _ := m.openDiscoveryPathsModal()
	if !m2.discovery.open {
		t.Fatal("should open modal")
	}
	if len(m2.discovery.paths) != 1 || m2.discovery.paths[0] != "/prev/path" {
		t.Fatal("should preserve loaded paths")
	}

	view := m2.discoveryPathsModalBlock()
	if !strings.Contains(view, "/prev/path") {
		t.Fatalf("missing config path in view:\n%s", view)
	}
	if !strings.Contains(view, "Defaults (read-only):") {
		t.Fatalf("missing defaults header in view:\n%s", view)
	}
	roots := models.DefaultSearchRoots()
	if len(roots) > 0 && !strings.Contains(view, roots[0]) {
		t.Fatalf("missing default path %q in view:\n%s", roots[0], view)
	}
}

func TestDiscoveryPathsModal_CancelDiscardsEdits(t *testing.T) {
	m := New()
	m.discovery.paths = []string{"/start"}
	m, _ = m.openDiscoveryPathsModal()
	m.discovery.cursor = 0

	m, _ = m.startDiscoveryPathEdit(false)
	m.discovery.editInput.SetValue("/edited")

	m = m.cancelDiscoveryPathEdit()

	if m.discovery.editOpen {
		t.Fatal("should exit edit mode")
	}
	if m.discovery.paths[0] != "/start" {
		t.Fatalf("expected /start, got %s", m.discovery.paths[0])
	}
}

func TestDiscoveryPathsModal_AddCommitDelete(t *testing.T) {
	m := New()
	m, _ = m.openDiscoveryPathsModal()

	m, _ = m.startDiscoveryPathEdit(true)
	m.discovery.editInput.SetValue("/new/path")
	m = m.commitDiscoveryPathEdit()

	if len(m.discovery.paths) != 1 || m.discovery.paths[0] != "/new/path" {
		t.Fatalf("paths: %v", m.discovery.paths)
	}

	m.discovery.cursor = 0
	m = m.deleteDiscoveryPathRow()

	if len(m.discovery.paths) != 0 {
		t.Fatalf("expected empty paths, got %v", m.discovery.paths)
	}
}

func TestDiscoveryPathsModal_SaveTriggersRescanIfChanged(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv("HOME", dir)
	t.Setenv("AppData", dir)

	m := New()
	m.discovery.paths = []string{"/new"}
	m, _ = m.openDiscoveryPathsModal()

	m2, cmd := m.saveDiscoveryPaths()

	if m2.discovery.open {
		t.Fatal("should close modal")
	}
	if !strings.Contains(m2.lastRunNote, "Rescanning models") {
		t.Fatalf("got note: %s", m2.lastRunNote)
	}
	if cmd == nil {
		t.Fatal("expected rescanModelsCmd, got nil")
	}

	msg := cmd()
	if _, ok := msg.(modelRescanDoneMsg); !ok {
		t.Fatalf("expected modelRescanDoneMsg, got %T", msg)
	}
}

func TestDiscoveryPathsModal_SaveSkipsRescanIfUnchanged(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv("HOME", dir)
	t.Setenv("AppData", dir)

	cfg := config.Config{
		SchemaVersion: config.SchemaVersion,
		Discovery: config.DiscoveryConfig{
			ExtraModelPaths: []string{"/unchanged"},
			LastScan:        time.Now(),
		},
	}
	if err := config.WriteFile(cfg); err != nil {
		t.Fatal(err)
	}

	m := New()
	m.discovery.paths = []string{"/unchanged"}
	m, _ = m.openDiscoveryPathsModal()

	m2, cmd := m.saveDiscoveryPaths()

	if m2.discovery.open {
		t.Fatal("should close modal")
	}
	if !strings.Contains(m2.lastRunNote, "unchanged") {
		t.Fatalf("got note: %s", m2.lastRunNote)
	}

	if cmd != nil {
		t.Fatal("expected nil cmd, got one")
	}
}

func TestEmptyStateIncludesModelPathsKey(t *testing.T) {
	m := New()
	m.layout.width = 100
	m.layout.height = 30
	m.loading = false
	m.table.files = nil
	view := m.View()
	if !strings.Contains(view.Content, "Press 'm' to add search paths") {
		t.Fatalf("empty state copy missing keybinding:\n%s", view.Content)
	}
}

func TestDiscoveryPathsModal_E2EFlow(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv("HOME", dir)
	t.Setenv("AppData", dir)

	// 1. no models found
	m := New()
	m.loading = false

	// 2. open model-paths modal
	m, _ = m.openDiscoveryPathsModal()
	if !m.discovery.open {
		t.Fatal("modal did not open")
	}

	// 3. add path
	m, _ = m.startDiscoveryPathEdit(true)
	m.discovery.editInput.SetValue("/e2e/test/path")
	m = m.commitDiscoveryPathEdit()

	// 4. save
	m, cmd := m.saveDiscoveryPaths()
	if m.discovery.open {
		t.Fatal("modal did not close after save")
	}
	if cmd == nil {
		t.Fatal("expected rescan command")
	}

	// 5 & 6. verify model discovery reruns automatically and config written
	msg := cmd()
	rescanMsg, ok := msg.(modelRescanDoneMsg)
	if !ok {
		t.Fatalf("expected modelRescanDoneMsg, got %T", msg)
	}

	// Check state from msg
	if len(rescanMsg.configPaths) != 1 || rescanMsg.configPaths[0] != "/e2e/test/path" {
		t.Fatalf("expected path in rescan message, got %v", rescanMsg.configPaths)
	}

	// Verify config written to disk
	cfg, err := config.ReadFile()
	if err != nil {
		t.Fatalf("failed to read config: %v", err)
	}
	if len(cfg.Discovery.ExtraModelPaths) != 1 || cfg.Discovery.ExtraModelPaths[0] != "/e2e/test/path" {
		t.Fatalf("expected path in config.toml, got %v", cfg.Discovery.ExtraModelPaths)
	}
}
