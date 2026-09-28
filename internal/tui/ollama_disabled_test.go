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

// ollamaFakes records every contact with the Ollama daemon: starting it,
// probing or waiting on it, and each API call. Detection finds an installed
// ollama that answers only once the daemon has been started, and never probes
// it while Ollama is skipped, as the real detection does.
type ollamaFakes struct {
	stored config.RuntimeStates
	cache  config.CacheFile

	daemonUp bool
	starts   int
	probes   int
	waits    int
	apiCalls int
	skips    []models.BackendSet

	services services
}

func newOllamaFakes(stored config.RuntimeStates, cached ...models.ModelFile) *ollamaFakes {
	f := &ollamaFakes{stored: stored, cache: config.CacheFromFiles(cached, time.Unix(2, 0))}
	svc := testServices()
	svc.readConfig = func() (config.Config, error) { return config.Config{SchemaVersion: config.SchemaVersion}, nil }
	svc.readCache = func() (config.CacheFile, error) { return f.cache, nil }
	svc.writeCache = func(c config.CacheFile) error { f.cache = c; return nil }
	svc.filterExisting = func(files []models.ModelFile) []models.ModelFile { return files }
	svc.readRuntimeStates = func() (config.RuntimeStates, error) { return f.stored, nil }
	svc.writeRuntimeStates = func(s config.RuntimeStates) error { f.stored = s; return nil }
	svc.discoverRuntime = func(_ context.Context, _ settings.Settings, skip models.BackendSet) models.RuntimeInfo {
		f.skips = append(f.skips, skip)
		rt := panelRuntime(linuxPlatform)
		rt.OllamaHost = "127.0.0.1:11434"
		rt.Skipped = skip
		rt.OllamaRunning = !skip.Has(models.BackendOllama) && f.daemonUp
		return rt
	}
	svc.discoverModels = func(context.Context, models.Options) ([]models.ModelFile, error) {
		return []models.ModelFile{testRow(models.BackendLlama, "/m/gemma.gguf")}, nil
	}
	svc.startOllamaDaemon = func(serverSpec) error { f.starts++; f.daemonUp = true; return nil }
	svc.probeOllama = func(context.Context, string) bool { f.probes++; return f.daemonUp }
	svc.waitForOllama = func(context.Context, string) bool { f.waits++; return f.daemonUp }
	svc.discoverOllama = func(context.Context, string) ([]models.ModelFile, error) {
		f.apiCalls++
		return []models.ModelFile{testOllamaRow("live:latest")}, nil
	}
	svc.preloadOllama = func(context.Context, string, string) error { f.apiCalls++; return nil }
	f.services = svc
	return f
}

// contacts is the number of times anything reached the Ollama daemon.
func (f *ollamaFakes) contacts() int { return f.starts + f.probes + f.waits + f.apiCalls }

// runDiscovery runs a discovery command and feeds its messages to m, following
// startup's hand-off to a full scan, as the Bubble Tea loop would.
func runDiscovery(t *testing.T, m Model, cmd tea.Cmd) Model {
	t.Helper()
	for _, msg := range collectCmdMsgs(t, cmd) {
		next, follow := m.Update(msg)
		m = next.(Model)
		if _, ok := msg.(startupNeedFullScanMsg); ok {
			m = runDiscovery(t, m, follow)
		}
	}
	return m
}

// startUp runs startup to completion on a new model.
func startUp(t *testing.T, svc services) Model {
	t.Helper()
	m := NewWithServices(svc)
	m.layout.width, m.layout.height = 140, 30
	return runDiscovery(t, m, m.Init())
}

// rescan presses S and runs the scan it starts.
func rescan(t *testing.T, m Model) Model {
	t.Helper()
	next, cmd := m.Update(keyText("S"))
	return runDiscovery(t, next.(Model), cmd)
}

var ollamaOffStates = config.RuntimeStates{}.With(models.BackendOllama, false)

// With Ollama off, neither startup nor S starts the daemon, probes it, or
// calls its API, and every detection is told to skip it. The Ollama rows from
// the last discovery cache stay in the table, dimmed, and stay in the cache.
func TestOllamaOff_startupAndRescanLeaveDaemonAlone(t *testing.T) {
	t.Parallel()

	f := newOllamaFakes(ollamaOffStates, testRow(models.BackendLlama, "/m/gemma.gguf"), testOllamaRow("cached:latest"))
	m := startUp(t, f.services)
	if !rowShowsDimmed(t, m, "cached:latest", "ollama") {
		t.Errorf("after startup the cached Ollama row should be dimmed:\n%s", plainView(m))
	}

	m = rescan(t, m)
	if f.contacts() != 0 {
		t.Errorf("Ollama is off: starts %d, probes %d, waits %d, API calls %d",
			f.starts, f.probes, f.waits, f.apiCalls)
	}
	for i, skip := range f.skips {
		if !skip.Has(models.BackendOllama) {
			t.Errorf("detection %d should skip Ollama, skip set %v", i, skip)
		}
	}
	if !rowShowsDimmed(t, m, "cached:latest", "ollama") {
		t.Errorf("after S the cached Ollama row should still be dimmed:\n%s", plainView(m))
	}
	if view := plainView(m); strings.Contains(view, "live:latest") || strings.Contains(view, "Starting Ollama") {
		t.Errorf("nothing should come from the daemon:\n%s", view)
	}
	var kept bool
	for _, e := range config.ModelFilesFromEntries(f.cache.Models) {
		kept = kept || e.Identity() == "cached:latest"
	}
	if !kept {
		t.Errorf("the rescan should keep the Ollama rows in the cache, cache %+v", f.cache.Models)
	}
}

// Without a usable cache, startup's full scan leaves the daemon alone too.
func TestOllamaOff_startupFullScanLeavesDaemonAlone(t *testing.T) {
	t.Parallel()

	f := newOllamaFakes(ollamaOffStates)
	m := startUp(t, f.services)
	if f.contacts() != 0 {
		t.Errorf("Ollama is off: starts %d, probes %d, waits %d, API calls %d",
			f.starts, f.probes, f.waits, f.apiCalls)
	}
	if !strings.Contains(plainView(m), "gemma") {
		t.Errorf("the filesystem scan should still run:\n%s", plainView(m))
	}
}

// With Ollama on, an installed but stopped daemon is started for discovery at
// startup, and its models come from the API, undimmed; S asks the API again.
func TestOllamaOn_startupStartsDaemonForDiscovery(t *testing.T) {
	t.Parallel()

	on := config.RuntimeStates{}.With(models.BackendOllama, true)
	f := newOllamaFakes(on, testRow(models.BackendLlama, "/m/gemma.gguf"), testOllamaRow("cached:latest"))
	m := startUp(t, f.services)
	if f.starts != 1 || f.apiCalls != 1 {
		t.Fatalf("startup should start the daemon once and list its models: starts %d, API calls %d", f.starts, f.apiCalls)
	}
	if rowShowsDimmed(t, m, "live:latest", "ollama") {
		t.Errorf("a live Ollama row should not be dimmed:\n%s", plainView(m))
	}
	if strings.Contains(plainView(m), "cached:latest") {
		t.Errorf("live rows should replace the cached ones:\n%s", plainView(m))
	}

	rescan(t, m)
	if f.starts != 1 || f.apiCalls != 2 {
		t.Errorf("S should list the running daemon's models again: starts %d, API calls %d", f.starts, f.apiCalls)
	}
}

// Turning Ollama back on in c resumes discovery on the next scan.
func TestOllamaOff_turningOnResumesOnNextScan(t *testing.T) {
	t.Parallel()

	f := newOllamaFakes(ollamaOffStates, testRow(models.BackendLlama, "/m/gemma.gguf"), testOllamaRow("cached:latest"))
	m := startUp(t, f.services)
	m = press(t, m, keyText("c"), keyDown, keyDown, keyDown, keyDown, keyDown)
	if !strings.Contains(listRow(t, m, "Ollama"), "[ ] Ollama") {
		t.Fatalf("fixture: Ollama should be highlighted and off:\n%s", plainView(m))
	}
	m = press(t, m, keySpace, keyEnter)
	if f.contacts() != 0 {
		t.Fatalf("saving should not contact the daemon before a scan, contacts %d", f.contacts())
	}

	m = rescan(t, m)
	if f.starts != 1 || f.apiCalls != 1 {
		t.Errorf("the next scan should start the daemon and list its models: starts %d, API calls %d", f.starts, f.apiCalls)
	}
	if rowShowsDimmed(t, m, "live:latest", "ollama") {
		t.Errorf("rows from the running daemon should not be dimmed:\n%s", plainView(m))
	}
}
