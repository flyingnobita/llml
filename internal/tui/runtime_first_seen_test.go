package tui

import (
	"context"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/flyingnobita/llml/internal/config"
	"github.com/flyingnobita/llml/internal/models"
	"github.com/flyingnobita/llml/internal/settings"
)

// firstSeenFakes is an in-memory runtime state store that records every write,
// with detection returning whatever detected holds at the time of the call.
type firstSeenFakes struct {
	stored   config.RuntimeStates
	writes   []config.RuntimeStates
	detected models.RuntimeInfo
	services services
}

func newFirstSeenFakes(detected models.RuntimeInfo) *firstSeenFakes {
	f := &firstSeenFakes{detected: detected}
	svc := testServices()
	svc.readConfig = func() (config.Config, error) { return config.Config{SchemaVersion: config.SchemaVersion}, nil }
	svc.readCache = startupCache
	svc.filterExisting = func(files []models.ModelFile) []models.ModelFile { return files }
	svc.readRuntimeStates = func() (config.RuntimeStates, error) { return f.stored, nil }
	svc.writeRuntimeStates = func(s config.RuntimeStates) error {
		f.stored = s
		f.writes = append(f.writes, s)
		return nil
	}
	svc.discoverRuntime = func(_ context.Context, _ settings.Settings, skip models.BackendSet) models.RuntimeInfo {
		rt := f.detected
		rt.Skipped = skip
		return rt
	}
	f.services = svc
	return f
}

// linuxDetection finds llama-server and ollama on disk, finds no KoboldCpp
// program but gets an answer from its server, and finds neither vLLM nor
// NInfer.
func linuxDetection() models.RuntimeInfo {
	return models.RuntimeInfo{
		Platform:         linuxPlatform,
		LlamaServerPath:  "/opt/llama/bin/llama-server",
		OllamaPath:       "/usr/local/bin/ollama",
		OllamaRunning:    true,
		KoboldCppRunning: true,
	}
}

// deliver runs cmd and feeds its message to m, as the Bubble Tea loop would.
func deliver(t *testing.T, m Model, cmd tea.Cmd) Model {
	t.Helper()
	next, _ := m.Update(cmd())
	return next.(Model)
}

// runtimeAlerts returns the alert history entries raised under "Runtimes".
func runtimeAlerts(m Model) []string {
	var out []string
	for _, a := range m.alerts.history {
		if a.source == "Runtimes" {
			out = append(out, a.message)
		}
	}
	return out
}

// With an empty state file, startup records every supported Runtime once: on
// when detected, off otherwise. Unsupported Runtimes get no entry, and one
// alert names the Runtimes turned off.
func TestFirstSeen_startupDecidesUnseenRuntimes(t *testing.T) {
	t.Parallel()

	f := newFirstSeenFakes(linuxDetection())
	m := NewWithServices(f.services)
	m.layout.width, m.layout.height = 100, 30
	m = deliver(t, m, m.Init())

	if len(f.writes) != 1 {
		t.Fatalf("want the state file written once, got %d writes", len(f.writes))
	}
	want := map[models.ModelBackend]bool{
		models.BackendLlama:  true,
		models.BackendKobold: true,
		models.BackendOllama: true,
		models.BackendVLLM:   false,
		models.BackendMLXLM:  false,
		models.BackendNInfer: false,
	}
	for b, on := range want {
		got, seen := f.stored.Lookup(b)
		if !seen || got != on {
			t.Errorf("%v: stored (on=%t, seen=%t), want on=%t", b, got, seen, on)
		}
	}
	for _, b := range []models.ModelBackend{models.BackendOMLX, models.BackendSplash} {
		if _, seen := f.stored.Lookup(b); seen {
			t.Errorf("unsupported %v should get no entry", b)
		}
	}

	alerts := runtimeAlerts(m)
	wantAlert := "Runtimes not found are now off: vLLM, mlx-lm, NInfer. Turn them on in c."
	if len(alerts) != 1 || alerts[0] != wantAlert {
		t.Fatalf("want one alert %q, got %q", wantAlert, alerts)
	}
	if m.runtimeEnabled(models.BackendVLLM) {
		t.Error("the model should adopt the decided state: vLLM is off")
	}

	m = press(t, m, keyText("c"))
	if rowIsOn(t, m, "vLLM") || !rowIsOn(t, m, "KoboldCpp") {
		t.Errorf("the panel should show the decided state:\n%s", plainView(m))
	}
}

// Once recorded, a Runtime is never re-decided: a later detection that finds
// vLLM, or loses KoboldCpp, changes nothing, writes nothing, and raises no
// second alert.
func TestFirstSeen_seenRuntimesAreNeverRedecided(t *testing.T) {
	t.Parallel()

	f := newFirstSeenFakes(linuxDetection())
	m := NewWithServices(f.services)
	m = deliver(t, m, m.Init())
	if len(f.writes) != 1 {
		t.Fatalf("fixture: want one write at startup, got %d", len(f.writes))
	}

	// vLLM is installed and KoboldCpp's server goes away.
	f.detected.VLLMPath = "/opt/vllm/bin/vllm"
	f.detected.KoboldCppRunning = false
	m = deliver(t, m, f.services.reloadRuntimeCmd())
	m, cmd := m.startScan(scanModeFull)
	m = deliver(t, m, cmd)

	if len(f.writes) != 1 {
		t.Errorf("a Runtime already in the file must not be re-decided, got %d writes", len(f.writes))
	}
	if f.stored.Enabled(models.BackendVLLM) || !f.stored.Enabled(models.BackendKobold) {
		t.Errorf("stored state changed: vLLM on=%t, KoboldCpp on=%t", f.stored.Enabled(models.BackendVLLM), f.stored.Enabled(models.BackendKobold))
	}
	if m.runtimeEnabled(models.BackendVLLM) {
		t.Error("a vLLM installed after it was seen stays off until the user turns it on")
	}
	if alerts := runtimeAlerts(m); len(alerts) != 1 {
		t.Errorf("the alert must appear exactly once, got %q", alerts)
	}
}

// A state file written by an earlier session already covers every supported
// Runtime: startup neither writes nor alerts.
func TestFirstSeen_fullyRecordedFileIsLeftAlone(t *testing.T) {
	t.Parallel()

	f := newFirstSeenFakes(models.RuntimeInfo{Platform: linuxPlatform})
	for _, b := range []models.ModelBackend{models.BackendLlama, models.BackendKobold, models.BackendVLLM, models.BackendMLXLM, models.BackendNInfer, models.BackendOllama} {
		f.stored = f.stored.With(b, true)
	}
	m := NewWithServices(f.services)
	m = deliver(t, m, m.Init())

	if len(f.writes) != 0 {
		t.Errorf("nothing unseen, so nothing to write; got %d writes", len(f.writes))
	}
	if alerts := runtimeAlerts(m); len(alerts) != 0 {
		t.Errorf("want no alert, got %q", alerts)
	}
}

// When every unseen Runtime is detected, their entries are written but no
// alert is raised: nothing was turned off.
func TestFirstSeen_noAlertWhenNothingTurnedOff(t *testing.T) {
	t.Parallel()

	rt := linuxDetection()
	rt.VLLMPath = "/opt/vllm/bin/vllm"
	rt.NInferRunning = true
	rt.MLXLMPath = "/opt/venv/bin/mlx_lm.server"
	f := newFirstSeenFakes(rt)
	m := NewWithServices(f.services)
	m = deliver(t, m, f.services.reloadRuntimeCmd())

	if len(f.writes) != 1 {
		t.Fatalf("want the state file written once, got %d writes", len(f.writes))
	}
	for _, b := range []models.ModelBackend{models.BackendLlama, models.BackendKobold, models.BackendVLLM, models.BackendMLXLM, models.BackendNInfer, models.BackendOllama} {
		if on, seen := f.stored.Lookup(b); !seen || !on {
			t.Errorf("%v: stored (on=%t, seen=%t), want on", b, on, seen)
		}
	}
	if alerts := runtimeAlerts(m); len(alerts) != 0 {
		t.Errorf("want no alert when nothing was turned off, got %q", alerts)
	}
}

// Only the Runtimes missing from the file are decided; an existing entry,
// including one the user turned off, is kept as it is.
func TestFirstSeen_onlyMissingEntriesAreAdded(t *testing.T) {
	t.Parallel()

	f := newFirstSeenFakes(linuxDetection())
	f.stored = config.RuntimeStates{}.With(models.BackendLlama, false).With(models.BackendVLLM, true).With(models.BackendMLXLM, true)
	m := NewWithServices(f.services)
	m = deliver(t, m, f.services.reloadRuntimeCmd())

	if len(f.writes) != 1 {
		t.Fatalf("want one write, got %d", len(f.writes))
	}
	if f.stored.Enabled(models.BackendLlama) || !f.stored.Enabled(models.BackendVLLM) {
		t.Error("existing entries must be kept as the user set them")
	}
	if f.stored.Enabled(models.BackendNInfer) {
		t.Error("the unseen, undetected NInfer should start off")
	}
	want := "Runtimes not found are now off: NInfer. Turn them on in c."
	if alerts := runtimeAlerts(m); len(alerts) != 1 || alerts[0] != want {
		t.Errorf("want only NInfer named, got %q", alerts)
	}
}

// A state file that cannot be read is not overwritten with first-seen
// decisions: the user's choices in it may still be recoverable.
func TestFirstSeen_unreadableFileIsNotOverwritten(t *testing.T) {
	t.Parallel()

	f := newFirstSeenFakes(linuxDetection())
	f.services.readRuntimeStates = func() (config.RuntimeStates, error) {
		return config.RuntimeStates{}, errTest("bad toml")
	}
	m := NewWithServices(f.services)
	m = deliver(t, m, f.services.reloadRuntimeCmd())

	if len(f.writes) != 0 {
		t.Errorf("an unreadable state file must not be overwritten, got %d writes", len(f.writes))
	}
	if !m.runtimeEnabled(models.BackendVLLM) {
		t.Error("with an unreadable state file every Runtime stays on")
	}
}

// A failed write still applies the decisions for this session and reports
// the failure alongside the upgrade alert.
func TestFirstSeen_failedWriteIsReported(t *testing.T) {
	t.Parallel()

	f := newFirstSeenFakes(linuxDetection())
	f.services.writeRuntimeStates = func(config.RuntimeStates) error { return errTest("disk full") }
	m := NewWithServices(f.services)
	m = deliver(t, m, f.services.reloadRuntimeCmd())

	if m.runtimeEnabled(models.BackendVLLM) {
		t.Error("the decisions should apply to this session even when the write fails")
	}
	alerts := strings.Join(runtimeAlerts(m), "\n")
	if !strings.Contains(alerts, "disk full") || !strings.Contains(alerts, "now off: vLLM, mlx-lm, NInfer") {
		t.Errorf("want the write failure and the upgrade alert, got %q", alerts)
	}
}

// A Runtime turned off on first sight dims its rows as soon as detection
// arrives, because deciding its state redraws the table.
func TestFirstSeen_offRuntimeDimsRowsAtOnce(t *testing.T) {
	t.Parallel()

	f := newFirstSeenFakes(linuxDetection())
	m := NewWithServices(f.services)
	m.layout.width, m.layout.height = 140, 30
	m.loading = false
	next, _ := m.Update(modelsLoadedMsg{files: []models.ModelFile{
		testRow(models.BackendVLLM, "/m/qwen-st"),
		testRow(models.BackendLlama, "/m/gemma.gguf"),
	}})
	next, _ = next.(Model).Update(runtimeReadyMsg{
		settings: defaultSettings(),
		runtime:  linuxDetection(),
		states:   runtimeStatesRead{},
	})
	m = next.(Model)

	if !rowShowsDimmed(t, m, "qwen-st", "vllm") {
		t.Errorf("a row on vLLM, turned off on first sight, should be dimmed:\n%s", plainView(m))
	}
	if rowShowsDimmed(t, m, "gemma", "llama.cpp") {
		t.Errorf("a row on the detected llama.cpp should not be dimmed:\n%s", plainView(m))
	}
}
