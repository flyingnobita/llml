package tui

import (
	"path/filepath"
	"testing"

	"github.com/flyingnobita/llml/internal/models"
	"github.com/flyingnobita/llml/internal/profiles"
)

// Model is copied by value everywhere in the Bubble Tea update loop. Any
// reference type on it (maps, slices) must therefore be copy-on-write, or one
// copy's edit leaks into every other copy. These tests pin that down for the
// two fields that are actually mutated after construction.

func TestModelCopy_effectiveBackendsIsCopyOnWrite(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv("HOME", dir)
	t.Setenv("AppData", dir)

	modelPath := filepath.Join(dir, "a.gguf")
	if err := profiles.SaveEntry(modelPath, profiles.Entry{
		Profiles:    []profiles.Profile{{Name: "kobold", Backend: "koboldcpp"}},
		ActiveIndex: 0,
	}); err != nil {
		t.Fatal(err)
	}

	m := New()
	m2 := m

	m2 = m2.loadEffectiveBackendForIdentity(modelPath)

	key := profiles.ModelParamsKey(modelPath)
	if got, ok := m2.table.effectiveBackends[key]; !ok || got != models.BackendKobold {
		t.Fatalf("the copy should see the new backend, got %v (present=%t)", got, ok)
	}
	if _, ok := m.table.effectiveBackends[key]; ok {
		t.Error("the original must not see the copy's write")
	}
}

func TestModelCopy_effectiveBackendsDeleteDoesNotLeak(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv("HOME", dir)
	t.Setenv("AppData", dir)

	modelPath := filepath.Join(dir, "a.gguf")
	key := profiles.ModelParamsKey(modelPath)

	m := New()
	m.table.effectiveBackends[key] = models.BackendKobold

	// No profile on disk, so this deletes the entry.
	m2 := m
	m2 = m2.loadEffectiveBackendForIdentity(modelPath)

	if _, ok := m2.table.effectiveBackends[key]; ok {
		t.Error("the copy should have dropped the entry")
	}
	if _, ok := m.table.effectiveBackends[key]; !ok {
		t.Error("the original must keep its entry")
	}
}

func TestModelCopy_exportItemsAreCopyOnWrite(t *testing.T) {
	m := New()
	m.export.items = []exportProfileItem{
		{kind: exportItemHeader, modelKey: "k", modelDisplay: "k"},
		{kind: exportItemProfile, modelKey: "k", profileName: "p", checked: false},
	}

	m2 := m
	m2 = m2.toggleGroup(true)

	if !m2.export.items[1].checked {
		t.Error("the copy should see the toggle")
	}
	if m.export.items[1].checked {
		t.Error("the original must not see the copy's toggle")
	}
}

// Guard the profiles alias used above so the import stays meaningful if these
// types are ever re-pointed.
var _ = profiles.Profile{}
