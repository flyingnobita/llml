package tui

import (
	"context"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/flyingnobita/llml/internal/config"
	"github.com/flyingnobita/llml/internal/models"
	"github.com/flyingnobita/llml/internal/settings"
)

var keySpace = tea.KeyPressMsg{Code: ' ', Text: " "}

// stateFakes is an in-memory runtime state store and a detection fake that
// records the skip set of every run. One value can back several models, which
// is how a restart is simulated.
type stateFakes struct {
	stored   config.RuntimeStates
	writes   int
	skips    []models.BackendSet
	rescans  int
	services services
}

func newStateFakes() *stateFakes {
	f := &stateFakes{}
	svc := testServices()
	svc.readRuntimeStates = func() (config.RuntimeStates, error) { return f.stored, nil }
	svc.writeRuntimeStates = func(s config.RuntimeStates) error {
		f.stored = s
		f.writes++
		return nil
	}
	svc.discoverRuntime = func(_ context.Context, _ settings.Settings, skip models.BackendSet) models.RuntimeInfo {
		f.skips = append(f.skips, skip)
		rt := panelRuntime(linuxPlatform)
		rt.Skipped = skip
		// An installed Ollama that is not answering makes startup and a full
		// scan start the daemon first; a running one keeps both on their
		// plain paths.
		rt.OllamaRunning = !skip.Has(models.BackendOllama)
		return rt
	}
	svc.discoverModels = func(context.Context, models.Options) ([]models.ModelFile, error) {
		f.rescans++
		return nil, nil
	}
	f.services = svc
	return f
}

// listRow returns the runtime list line naming rt, failing when there is none.
func listRow(t *testing.T, m Model, rt string) string {
	t.Helper()
	for _, l := range strings.Split(plainView(m), "\n") {
		// The list pane is the left column; the detail pane header also names
		// the Runtime, but never with a checkbox.
		if strings.Contains(l, "] "+rt) {
			return l
		}
	}
	t.Fatalf("no list row for %s:\n%s", rt, plainView(m))
	return ""
}

func rowIsOn(t *testing.T, m Model, rt string) bool {
	t.Helper()
	l := listRow(t, m, rt)
	switch {
	case strings.Contains(l, "[✓] "+rt) && !strings.Contains(l, rt+" off"):
		return true
	case strings.Contains(l, "[ ] "+rt) && strings.Contains(l, " off"):
		return false
	}
	t.Fatalf("row for %s is neither on nor off: %q", rt, l)
	return false
}

// space in the list flips the highlighted Runtime's checkbox; a Disabled
// Runtime shows off in place of its status mark and "Off" in the detail header.
func TestRuntimePanel_spaceTogglesHighlightedRuntime(t *testing.T) {
	t.Parallel()

	m := openPanel(t, newStateFakes().services, linuxPlatform, defaultSettings(), 100, 30)
	if !rowIsOn(t, m, "Llama.cpp") || !rowIsOn(t, m, "KoboldCpp") {
		t.Fatalf("a Runtime with no stored state starts on:\n%s", plainView(m))
	}
	m = press(t, m, keyDown, keySpace) // KoboldCpp off
	if rowIsOn(t, m, "KoboldCpp") {
		t.Fatalf("space should turn KoboldCpp off:\n%s", plainView(m))
	}
	if !rowIsOn(t, m, "Llama.cpp") {
		t.Fatal("space must only toggle the highlighted Runtime")
	}
	if view := plainView(m); !strings.Contains(view, "KoboldCpp · Off") {
		t.Errorf("the detail header should say Off:\n%s", view)
	}
	m = press(t, m, keySpace)
	if !rowIsOn(t, m, "KoboldCpp") {
		t.Fatal("a second space should turn KoboldCpp back on")
	}
}

// A Disabled Runtime's fields stay editable.
func TestRuntimePanel_disabledRuntimeFieldsStayEditable(t *testing.T) {
	t.Parallel()

	m := openPanel(t, newStateFakes().services, linuxPlatform, defaultSettings(), 100, 30)
	m = press(t, m, keySpace, keyRight, keyCtrlU)
	m = typeText(t, m, "/opt/x")
	if l := focusLine(t, m); !strings.Contains(l, "Path") || !strings.Contains(l, "/opt/x") {
		t.Fatalf("an Off Runtime's Path should take input, focus line %q", l)
	}
}

// Typing a space in a field edits the field and never toggles the Runtime.
func TestRuntimePanel_spaceInAFieldDoesNotToggle(t *testing.T) {
	t.Parallel()

	f := newStateFakes()
	m := openPanel(t, f.services, linuxPlatform, defaultSettings(), 100, 30)
	m = press(t, m, keyRight, keyCtrlU) // Llama.cpp Path
	m = typeText(t, m, "/my")
	m = press(t, m, keySpace)
	m = typeText(t, m, "dir")
	if l := focusLine(t, m); !strings.Contains(l, "/my dir") {
		t.Fatalf("the space should reach the field, focus line %q", l)
	}
	if !rowIsOn(t, m, "Llama.cpp") {
		t.Fatal("a space typed in a field must not toggle the Runtime")
	}
	press(t, m, keyEnter)
	if f.writes != 0 {
		t.Errorf("no toggle changed, so the state file should not be written (got %d writes)", f.writes)
	}
}

// enter writes the toggles to the state file, re-runs detection with the new
// skip set, and does not rescan models.
func TestRuntimePanel_enterPersistsToggles(t *testing.T) {
	t.Parallel()

	f := newStateFakes()
	m := openPanel(t, f.services, linuxPlatform, defaultSettings(), 100, 30)
	m = press(t, m, keyDown, keySpace, keyDown, keyDown, keyDown, keySpace) // KoboldCpp and Ollama off
	m = press(t, m, keyEnter)

	if m.rc.open {
		t.Fatal("enter should close the panel")
	}
	if f.writes != 1 {
		t.Fatalf("want one state file write, got %d", f.writes)
	}
	for b, want := range map[models.ModelBackend]bool{models.BackendKobold: false, models.BackendOllama: false, models.BackendLlama: true} {
		if got := f.stored.Enabled(b); got != want {
			t.Errorf("stored %v on = %t, want %t", b, got, want)
		}
	}
	if len(f.skips) != 1 {
		t.Fatalf("detection should re-run once, got %d runs", len(f.skips))
	}
	if skip := f.skips[0]; !skip.Has(models.BackendKobold) || !skip.Has(models.BackendOllama) || len(skip) != 2 {
		t.Errorf("detection skip set = %v, want {koboldcpp, ollama}", skip)
	}
	if f.rescans != 0 {
		t.Errorf("toggling should not rescan models, got %d scans", f.rescans)
	}

	m = press(t, m, keyText("c"))
	if rowIsOn(t, m, "KoboldCpp") || rowIsOn(t, m, "Ollama") || !rowIsOn(t, m, "Llama.cpp") {
		t.Errorf("the reopened panel should show the saved toggles:\n%s", plainView(m))
	}
}

// esc discards toggles like field edits: after confirming, nothing is written
// and the panel reopens with the stored state.
func TestRuntimePanel_escDiscardsToggles(t *testing.T) {
	t.Parallel()

	f := newStateFakes()
	m := openPanel(t, f.services, linuxPlatform, defaultSettings(), 100, 30)
	m = press(t, m, keySpace, keyEsc)
	if view := plainView(m); !strings.Contains(view, "Discard runtime changes?") {
		t.Fatalf("esc with a toggle should ask to discard:\n%s", view)
	}
	m = press(t, m, keyText("y"))
	if m.rc.open || f.writes != 0 || len(f.skips) != 0 {
		t.Fatalf("discard should close without saving (open=%t writes=%d detections=%d)", m.rc.open, f.writes, len(f.skips))
	}
	m = press(t, m, keyText("c"))
	if !rowIsOn(t, m, "Llama.cpp") {
		t.Errorf("the discarded toggle should not survive:\n%s", plainView(m))
	}
}

// startupCache is a valid discovery cache, so startup takes the cache-hit path
// and runs detection once without a filesystem walk.
func startupCache() (config.CacheFile, error) {
	return config.CacheFromFiles([]models.ModelFile{{
		Backend: models.BackendLlama, Path: "/m/a.gguf", Name: "a.gguf", Size: 1, ModTime: time.Unix(1, 0),
	}}, time.Unix(2, 0)), nil
}

// Toggles saved in one session are read back by the next: startup detection
// skips the Disabled Runtimes and the panel shows them off.
func TestRuntimeToggles_surviveRestart(t *testing.T) {
	t.Parallel()

	f := newStateFakes()
	f.services.readCache = startupCache
	f.services.filterExisting = func(files []models.ModelFile) []models.ModelFile { return files }

	m := openPanel(t, f.services, linuxPlatform, defaultSettings(), 100, 30)
	press(t, m, keyDown, keySpace, keyEnter) // KoboldCpp off
	if f.writes != 1 {
		t.Fatalf("want one state file write, got %d", f.writes)
	}

	next := NewWithServices(f.services)
	next.layout.width, next.layout.height = 100, 30
	msg := next.Init()()
	updated, _ := next.Update(msg)
	next = updated.(Model)

	if last := f.skips[len(f.skips)-1]; !last.Has(models.BackendKobold) || len(last) != 1 {
		t.Errorf("startup detection skip set = %v, want {koboldcpp}", last)
	}
	next = press(t, next, keyText("c"))
	if rowIsOn(t, next, "KoboldCpp") || !rowIsOn(t, next, "Llama.cpp") {
		t.Errorf("after a restart KoboldCpp should still be off:\n%s", plainView(next))
	}
}

// r and a full scan read the state store too, so every detection honors it.
func TestRuntimeToggles_reloadAndFullScanSkipDisabled(t *testing.T) {
	t.Parallel()

	f := newStateFakes()
	f.stored = config.RuntimeStates{}.With(models.BackendKobold, false)
	f.services.readConfig = func() (config.Config, error) { return config.Config{SchemaVersion: config.SchemaVersion}, nil }

	msg := f.services.reloadRuntimeCmd()()
	ready, ok := msg.(runtimeReadyMsg)
	if !ok {
		t.Fatalf("reload returned %T", msg)
	}
	if !ready.runtime.Skipped.Has(models.BackendKobold) {
		t.Errorf("r should skip the Disabled KoboldCpp, skip set %v", f.skips)
	}

	m := NewWithServices(f.services)
	m, cmd := m.startScan(scanModeFull)
	done, ok := cmd().(scanDoneMsg)
	if !ok {
		t.Fatal("full scan did not finish")
	}
	if !done.result.runtime.Skipped.Has(models.BackendKobold) {
		t.Errorf("a full scan should skip the Disabled KoboldCpp, skip set %v", f.skips)
	}
	updated, _ := m.Update(done)
	m = updated.(Model)
	if m.runtimeStates.Enabled(models.BackendKobold) {
		t.Error("the model should adopt the stored state from the scan")
	}
}

// A Disabled Runtime is left out of the main screen's runtime status: rows
// that need it do not produce a "not found" note for it.
//
// Not parallel: program lookups fall back to PATH, which the test empties so
// nothing is found on the host.
func TestRuntimeToggles_disabledRuntimeLeftOutOfMissingNote(t *testing.T) {
	t.Setenv("PATH", t.TempDir())

	m := NewWithServices(testServices())
	m.runtime = models.RuntimeInfo{Platform: linuxPlatform}
	m.table.files = []models.ModelFile{
		{Backend: models.BackendLlama, Path: "/m/a.gguf", Name: "a.gguf"},
		{Backend: models.BackendVLLM, Path: "/m/b", Name: "b"},
	}
	m, _ = m.maybeSetMissingRuntimeFooterNote()
	if !strings.Contains(m.lastRunNote, MissingLlamaServerFooterNote) {
		t.Fatalf("fixture: an enabled missing llama.cpp should be reported, note %q", m.lastRunNote)
	}

	m.runtimeStates = config.RuntimeStates{}.With(models.BackendLlama, false)
	m, _ = m.maybeSetMissingRuntimeFooterNote()
	if strings.Contains(m.lastRunNote, MissingLlamaServerFooterNote) {
		t.Errorf("a Disabled llama.cpp should not be reported missing, note %q", m.lastRunNote)
	}
	if !strings.Contains(m.lastRunNote, MissingVLLMFooterNote) {
		t.Errorf("an enabled missing vLLM should still be reported, note %q", m.lastRunNote)
	}
}

// A state file that cannot be read is reported once per detection and every
// Runtime is treated as on.
func TestRuntimeToggles_unreadableStateFileAlerts(t *testing.T) {
	t.Parallel()

	svc := testServices()
	svc.readRuntimeStates = func() (config.RuntimeStates, error) {
		return config.RuntimeStates{}, errTest("bad toml")
	}
	svc.readConfig = func() (config.Config, error) { return config.Config{SchemaVersion: config.SchemaVersion}, nil }
	m := NewWithServices(svc)
	updated, _ := m.Update(svc.reloadRuntimeCmd()())
	m = updated.(Model)
	if len(m.alerts.history) != 1 || !strings.Contains(m.alerts.history[0].message, "bad toml") {
		t.Fatalf("want one alert about the state file, got %+v", m.alerts.history)
	}
	if !m.runtimeStates.Enabled(models.BackendLlama) {
		t.Error("an unreadable state file should leave every Runtime on")
	}
}

type errTest string

func (e errTest) Error() string { return string(e) }

// countConfigWrites counts writes to config.toml through f's services.
func (f *stateFakes) countConfigWrites() *int {
	n := new(int)
	f.services.writeConfig = func(config.Config) error {
		*n++
		return nil
	}
	return n
}

// A save that only toggles Runtimes writes the state file and leaves
// config.toml alone, so a hand-edited file keeps its comments.
func TestRuntimePanel_toggleOnlySaveLeavesConfigAlone(t *testing.T) {
	t.Parallel()

	f := newStateFakes()
	configWrites := f.countConfigWrites()
	m := openPanel(t, f.services, linuxPlatform, defaultSettings(), 100, 30)
	press(t, m, keyDown, keySpace, keyEnter) // KoboldCpp off

	if f.writes != 1 {
		t.Errorf("the toggle should be written to the state file, got %d writes", f.writes)
	}
	if *configWrites != 0 {
		t.Errorf("no field changed, so config.toml should not be written, got %d writes", *configWrites)
	}
}

// A save that edits a field writes config.toml, alongside any toggles.
func TestRuntimePanel_fieldEditSaveWritesConfig(t *testing.T) {
	t.Parallel()

	f := newStateFakes()
	configWrites := f.countConfigWrites()
	m := openPanel(t, f.services, linuxPlatform, defaultSettings(), 100, 30)
	m = press(t, m, keyRight, keyCtrlU) // Llama.cpp Path
	m = typeText(t, m, "/opt/llama")
	press(t, m, keyEnter)

	if *configWrites != 1 {
		t.Errorf("an edited field should be written to config.toml once, got %d writes", *configWrites)
	}
	if f.writes != 0 {
		t.Errorf("no toggle changed, so the state file should not be written, got %d writes", f.writes)
	}
}
