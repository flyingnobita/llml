package tui

import (
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/flyingnobita/llml/internal/config"
	"github.com/flyingnobita/llml/internal/models"
)

func collectCmdMsgs(t *testing.T, cmd tea.Cmd) []tea.Msg {
	t.Helper()
	if cmd == nil {
		t.Fatal("expected non-nil cmd")
	}
	msg := cmd()
	if msg == nil {
		return nil
	}
	if batch, ok := msg.(tea.BatchMsg); ok {
		var out []tea.Msg
		for _, c := range batch {
			if c == nil {
				continue
			}
			out = append(out, c())
		}
		return out
	}
	return []tea.Msg{msg}
}

func restoreDiscoveryTestSeams() func() {
	oldReadConfig := readConfigFileFn
	oldWriteConfig := writeConfigFileFn
	oldApplyRuntime := applyRuntimeFromConfigFn
	oldExtraPaths := extraModelPathsFromEnvFn
	oldMergeRoots := mergeExtraRootsFn
	oldDiscInputs := discoveryConfigInputsFn
	oldBuildConfig := buildConfigFn
	oldRuntimeFromEnv := runtimeFromEnvFn
	oldModelFiles := modelFilesFromEntriesFn
	oldFilterExisting := filterExistingPathsFn
	oldDiscoverRuntime := discoverRuntimeFn
	oldDiscoverModels := discoverModelsFn
	oldDiscoverOllamaModels := discoverOllamaModelsFn
	oldStartOllama := startOllamaDaemonFn
	oldWaitForOllama := waitForOllamaFn
	oldProbeOllama := probeOllamaFn
	oldPreloadOllama := preloadOllamaFn
	return func() {
		readConfigFileFn = oldReadConfig
		writeConfigFileFn = oldWriteConfig
		applyRuntimeFromConfigFn = oldApplyRuntime
		extraModelPathsFromEnvFn = oldExtraPaths
		mergeExtraRootsFn = oldMergeRoots
		discoveryConfigInputsFn = oldDiscInputs
		buildConfigFn = oldBuildConfig
		runtimeFromEnvFn = oldRuntimeFromEnv
		modelFilesFromEntriesFn = oldModelFiles
		filterExistingPathsFn = oldFilterExisting
		discoverRuntimeFn = oldDiscoverRuntime
		discoverModelsFn = oldDiscoverModels
		discoverOllamaModelsFn = oldDiscoverOllamaModels
		startOllamaDaemonFn = oldStartOllama
		waitForOllamaFn = oldWaitForOllama
		probeOllamaFn = oldProbeOllama
		preloadOllamaFn = oldPreloadOllama
	}
}

func testOllamaRow(id string) models.ModelFile {
	return models.ModelFile{
		Backend:    models.BackendOllama,
		ID:         id,
		Location:   "ollama://" + id,
		Name:       id,
		Size:       1,
		ModTime:    time.Unix(1, 0),
		Parameters: "ollama",
	}
}

func TestApplyAndFullScanCmd_StartsOllamaForDiscovery(t *testing.T) {
	t.Cleanup(restoreDiscoveryTestSeams())

	readConfigFileFn = func() (config.Config, error) {
		return config.Config{
			SchemaVersion: config.SchemaVersion,
			Models: []config.ModelEntry{
				config.ModelEntryFromFile(testOllamaRow("cached:latest")),
			},
		}, nil
	}
	writeConfigFileFn = func(config.Config) error { return nil }
	applyRuntimeFromConfigFn = func(*config.RuntimeConfig) {}
	extraModelPathsFromEnvFn = func() []string { return nil }
	mergeExtraRootsFn = func(a, b []string) []string { return nil }
	discoveryConfigInputsFn = func(configPaths []string, lastScan time.Time) config.DiscoveryConfig {
		return config.DiscoveryConfig{ExtraModelPaths: configPaths, LastScan: lastScan}
	}
	buildConfigFn = config.BuildConfig
	runtimeFromEnvFn = config.RuntimeFromEnv

	runtimeCalls := 0
	discoverRuntimeFn = func() models.RuntimeInfo {
		runtimeCalls++
		if runtimeCalls >= 2 {
			return models.RuntimeInfo{OllamaPath: "/bin/ollama", OllamaHost: "127.0.0.1:11434", OllamaRunning: true}
		}
		return models.RuntimeInfo{OllamaPath: "/bin/ollama", OllamaHost: "127.0.0.1:11434"}
	}
	startOllamaDaemonFn = func(spec serverSpec) error {
		if spec.bin != "/bin/ollama" {
			t.Fatalf("bin %q", spec.bin)
		}
		return nil
	}
	waitForOllamaFn = func() bool { return true }
	probeOllamaFn = func() bool { return false }
	discoverModelsFn = func(models.Options) ([]models.ModelFile, error) {
		return []models.ModelFile{testOllamaRow("live:latest")}, nil
	}

	msgs := collectCmdMsgs(t, applyAndFullScanCmd())
	if len(msgs) != 2 {
		t.Fatalf("got %d msgs", len(msgs))
	}
	started, ok := msgs[0].(ollamaDiscoveryStartedMsg)
	if !ok {
		t.Fatalf("msg[0] %T", msgs[0])
	}
	if !strings.Contains(started.note, "Starting Ollama") {
		t.Fatalf("started note %q", started.note)
	}
	done, ok := msgs[1].(fullScanDoneMsg)
	if !ok {
		t.Fatalf("msg[1] %T", msgs[1])
	}
	if !done.runtime.OllamaRunning {
		t.Fatal("expected runtime refresh after successful startup")
	}
	if done.ollamaWarn != "" {
		t.Fatalf("warn %q", done.ollamaWarn)
	}
	if !strings.Contains(done.ollamaNote, "Started Ollama for model discovery") {
		t.Fatalf("note %q", done.ollamaNote)
	}
	if len(done.files) != 1 || done.files[0].Identity() != "live:latest" {
		t.Fatalf("files %+v", done.files)
	}
}

func TestStartupCmd_CacheHitWithStoppedOllamaFallsBackToFullScan(t *testing.T) {
	t.Cleanup(restoreDiscoveryTestSeams())

	readConfigFileFn = func() (config.Config, error) {
		return config.Config{
			SchemaVersion: config.SchemaVersion,
			Models: []config.ModelEntry{
				config.ModelEntryFromFile(testOllamaRow("cached:latest")),
			},
		}, nil
	}
	applyRuntimeFromConfigFn = func(*config.RuntimeConfig) {}
	discoverRuntimeFn = func() models.RuntimeInfo {
		return models.RuntimeInfo{OllamaPath: "/bin/ollama", OllamaHost: "127.0.0.1:11434"}
	}
	filterExistingPathsFn = func(files []models.ModelFile) []models.ModelFile {
		return files
	}
	modelFilesFromEntriesFn = config.ModelFilesFromEntries

	msgs := collectCmdMsgs(t, startupCmd())
	if len(msgs) != 1 {
		t.Fatalf("got %d msgs", len(msgs))
	}
	if _, ok := msgs[0].(startupNeedFullScanMsg); !ok {
		t.Fatalf("msg[0] %T", msgs[0])
	}
}

func TestStartupCmd_CacheHitWithRunningOllamaRefreshesLiveOllamaRows(t *testing.T) {
	t.Cleanup(restoreDiscoveryTestSeams())

	var wrote config.Config
	readConfigFileFn = func() (config.Config, error) {
		return config.Config{
			SchemaVersion: config.SchemaVersion,
			Models: []config.ModelEntry{
				config.ModelEntryFromFile(testOllamaRow("cached:latest")),
				config.ModelEntryFromFile(models.ModelFile{
					Backend: models.BackendLlama,
					Path:    "/m.gguf",
					Name:    "m.gguf",
					Size:    1,
					ModTime: time.Unix(1, 0),
				}),
			},
			Discovery: config.DiscoveryConfig{LastScan: time.Unix(2, 0)},
		}, nil
	}
	applyRuntimeFromConfigFn = func(*config.RuntimeConfig) {}
	writeConfigFileFn = func(c config.Config) error {
		wrote = c
		return nil
	}
	buildConfigFn = config.BuildConfig
	runtimeFromEnvFn = config.RuntimeFromEnv
	discoverRuntimeFn = func() models.RuntimeInfo {
		return models.RuntimeInfo{OllamaPath: "/bin/ollama", OllamaHost: "127.0.0.1:11434", OllamaRunning: true}
	}
	filterExistingPathsFn = func(files []models.ModelFile) []models.ModelFile {
		return files
	}
	modelFilesFromEntriesFn = config.ModelFilesFromEntries
	discoverOllamaModelsFn = func() ([]models.ModelFile, error) {
		return []models.ModelFile{testOllamaRow("live:latest")}, nil
	}

	msgs := collectCmdMsgs(t, startupCmd())
	if len(msgs) != 1 {
		t.Fatalf("got %d msgs", len(msgs))
	}
	hit, ok := msgs[0].(startupCacheHitMsg)
	if !ok {
		t.Fatalf("msg[0] %T", msgs[0])
	}
	if len(hit.files) != 2 {
		t.Fatalf("files %+v", hit.files)
	}
	if hit.files[0].Backend != models.BackendLlama {
		t.Fatalf("expected non-ollama row preserved, got %+v", hit.files)
	}
	if hit.files[1].Identity() != "live:latest" {
		t.Fatalf("expected live ollama row, got %+v", hit.files)
	}
	if hit.writeErr != nil {
		t.Fatalf("unexpected writeErr: %v", hit.writeErr)
	}
	got := config.ModelFilesFromEntries(wrote.Models)
	if len(got) != 2 || got[1].Identity() != "live:latest" {
		t.Fatalf("persisted files %+v", got)
	}
}

func TestApplyAndFullScanCmd_FailedStartupMergesCachedOllamaRows(t *testing.T) {
	t.Cleanup(restoreDiscoveryTestSeams())

	readConfigFileFn = func() (config.Config, error) {
		return config.Config{
			SchemaVersion: config.SchemaVersion,
			Models: []config.ModelEntry{
				config.ModelEntryFromFile(testOllamaRow("cached:latest")),
			},
		}, nil
	}
	writeConfigFileFn = func(config.Config) error { return nil }
	applyRuntimeFromConfigFn = func(*config.RuntimeConfig) {}
	extraModelPathsFromEnvFn = func() []string { return nil }
	mergeExtraRootsFn = func(a, b []string) []string { return nil }
	discoveryConfigInputsFn = func(configPaths []string, lastScan time.Time) config.DiscoveryConfig {
		return config.DiscoveryConfig{ExtraModelPaths: configPaths, LastScan: lastScan}
	}
	buildConfigFn = config.BuildConfig
	runtimeFromEnvFn = config.RuntimeFromEnv

	discoverRuntimeFn = func() models.RuntimeInfo {
		return models.RuntimeInfo{OllamaPath: "/bin/ollama", OllamaHost: "127.0.0.1:11434"}
	}
	startOllamaDaemonFn = func(serverSpec) error { return errors.New("boom") }
	waitForOllamaFn = func() bool { return false }
	probeOllamaFn = func() bool { return false }
	discoverModelsFn = func(models.Options) ([]models.ModelFile, error) {
		return []models.ModelFile{
			{Backend: models.BackendLlama, Path: "/m.gguf", Name: "m.gguf", Size: 1, ModTime: time.Unix(1, 0)},
		}, nil
	}

	msgs := collectCmdMsgs(t, applyAndFullScanCmd())
	done := msgs[len(msgs)-1].(fullScanDoneMsg)
	if done.ollamaNote != "" {
		t.Fatalf("note %q", done.ollamaNote)
	}
	if done.ollamaWarn != "boom" {
		t.Fatalf("warn %q", done.ollamaWarn)
	}
	if len(done.files) != 2 {
		t.Fatalf("files %+v", done.files)
	}
	if done.files[1].Backend != models.BackendOllama {
		t.Fatalf("expected cached ollama row in %+v", done.files)
	}
}

func TestApplyAndFullScanCmd_FailedStartupWithoutCacheKeepsNonOllamaRows(t *testing.T) {
	t.Cleanup(restoreDiscoveryTestSeams())

	readConfigFileFn = func() (config.Config, error) {
		return config.Config{}, os.ErrNotExist
	}
	writeConfigFileFn = func(config.Config) error { return nil }
	applyRuntimeFromConfigFn = func(*config.RuntimeConfig) {}
	extraModelPathsFromEnvFn = func() []string { return nil }
	mergeExtraRootsFn = func(a, b []string) []string { return nil }
	discoveryConfigInputsFn = func(configPaths []string, lastScan time.Time) config.DiscoveryConfig {
		return config.DiscoveryConfig{ExtraModelPaths: configPaths, LastScan: lastScan}
	}
	buildConfigFn = config.BuildConfig
	runtimeFromEnvFn = config.RuntimeFromEnv

	discoverRuntimeFn = func() models.RuntimeInfo {
		return models.RuntimeInfo{OllamaPath: "/bin/ollama", OllamaHost: "127.0.0.1:11434"}
	}
	startOllamaDaemonFn = func(serverSpec) error { return errors.New("boom") }
	waitForOllamaFn = func() bool { return false }
	probeOllamaFn = func() bool { return false }
	discoverModelsFn = func(models.Options) ([]models.ModelFile, error) {
		return []models.ModelFile{
			{Backend: models.BackendLlama, Path: "/m.gguf", Name: "m.gguf", Size: 1, ModTime: time.Unix(1, 0)},
		}, nil
	}

	msgs := collectCmdMsgs(t, applyAndFullScanCmd())
	done := msgs[len(msgs)-1].(fullScanDoneMsg)
	if len(done.files) != 1 || done.files[0].Backend != models.BackendLlama {
		t.Fatalf("files %+v", done.files)
	}
	if done.ollamaWarn != "boom" {
		t.Fatalf("warn %q", done.ollamaWarn)
	}
}

func TestRescanModelsCmd_StartsOllamaAndReturnsDiscoveryNote(t *testing.T) {
	t.Cleanup(restoreDiscoveryTestSeams())

	readConfigFileFn = func() (config.Config, error) {
		return config.Config{SchemaVersion: config.SchemaVersion}, nil
	}
	writeConfigFileFn = func(config.Config) error { return nil }
	applyRuntimeFromConfigFn = func(*config.RuntimeConfig) {}
	extraModelPathsFromEnvFn = func() []string { return nil }
	mergeExtraRootsFn = func(a, b []string) []string { return nil }
	discoveryConfigInputsFn = func(configPaths []string, lastScan time.Time) config.DiscoveryConfig {
		return config.DiscoveryConfig{ExtraModelPaths: configPaths, LastScan: lastScan}
	}
	buildConfigFn = config.BuildConfig
	runtimeFromEnvFn = config.RuntimeFromEnv

	runtimeCalls := 0
	discoverRuntimeFn = func() models.RuntimeInfo {
		runtimeCalls++
		if runtimeCalls >= 2 {
			return models.RuntimeInfo{OllamaPath: "/bin/ollama", OllamaHost: "127.0.0.1:11434", OllamaRunning: true}
		}
		return models.RuntimeInfo{OllamaPath: "/bin/ollama", OllamaHost: "127.0.0.1:11434"}
	}
	startOllamaDaemonFn = func(serverSpec) error { return nil }
	waitForOllamaFn = func() bool { return true }
	probeOllamaFn = func() bool { return false }
	discoverModelsFn = func(models.Options) ([]models.ModelFile, error) {
		return []models.ModelFile{testOllamaRow("live:latest")}, nil
	}

	msgs := collectCmdMsgs(t, rescanModelsCmd("/models"))
	done := msgs[len(msgs)-1].(modelRescanDoneMsg)
	if !strings.Contains(done.ollamaNote, "Started Ollama for model discovery") {
		t.Fatalf("note %q", done.ollamaNote)
	}
	if done.configPaths[0] != "/models" {
		t.Fatalf("paths %v", done.configPaths)
	}
}

func TestMergeCachedOllamaRows_AppendsOnlyWhenLiveRowsMissing(t *testing.T) {
	cfg := config.Config{
		SchemaVersion: config.SchemaVersion,
		Models: []config.ModelEntry{
			config.ModelEntryFromFile(testOllamaRow("cached:latest")),
		},
	}
	live := []models.ModelFile{testOllamaRow("live:latest")}
	got := mergeCachedOllamaRows(cfg, append([]models.ModelFile(nil), live...), models.RuntimeInfo{})
	if len(got) != 1 || got[0].Identity() != "live:latest" {
		t.Fatalf("live rows should win, got %+v", got)
	}

	got = mergeCachedOllamaRows(cfg, []models.ModelFile{
		{Backend: models.BackendLlama, Path: "/m.gguf", Name: "m.gguf", Size: 1, ModTime: time.Unix(1, 0)},
	}, models.RuntimeInfo{})
	if len(got) != 2 || got[1].Identity() != "cached:latest" {
		t.Fatalf("expected cached ollama append, got %+v", got)
	}
}

func TestUpdate_OllamaDiscoveryMessages(t *testing.T) {
	m := newTestModel()
	next, cmd := m.Update(ollamaDiscoveryStartedMsg{note: "Starting Ollama..."})
	if cmd != nil {
		t.Fatal("unexpected cmd for started msg")
	}
	m = next.(Model)
	if m.alerts.current != "Starting Ollama..." {
		t.Fatalf("current status %q", m.alerts.current)
	}

	next, cmd = m.Update(fullScanDoneMsg{
		runtime:    models.RuntimeInfo{},
		files:      []models.ModelFile{},
		lastScan:   time.Now(),
		ollamaWarn: "startup failed",
	})
	m = next.(Model)
	if m.alerts.current != "" {
		t.Fatalf("expected current status cleared, got %q", m.alerts.current)
	}
	if !strings.Contains(m.lastRunNote, "startup failed") {
		t.Fatalf("lastRunNote %q", m.lastRunNote)
	}
	if cmd == nil {
		t.Fatal("expected clear cmd from warning flash")
	}
}
