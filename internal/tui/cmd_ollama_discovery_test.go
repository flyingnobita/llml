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
	"github.com/flyingnobita/llml/internal/settings"
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

// testServices returns services wired to inert fakes: no filesystem, no
// network, and an empty environment. Tests override only the fields they care
// about, and because nothing is package-level they can run in parallel.
func testServices() services {
	svc := defaultServices()
	svc.readConfig = func() (config.Config, error) { return config.Config{}, os.ErrNotExist }
	svc.writeConfig = func(config.Config) error { return nil }
	svc.discoverRuntime = func(settings.Settings) models.RuntimeInfo { return models.RuntimeInfo{} }
	svc.discoverModels = func(models.Options) ([]models.ModelFile, error) { return nil, nil }
	svc.discoverOllama = func(string) ([]models.ModelFile, error) { return nil, nil }
	svc.startOllamaDaemon = func(serverSpec) error { return nil }
	svc.waitForOllama = func(string) bool { return true }
	svc.probeOllama = func(string) bool { return false }
	svc.preloadOllama = func(string, string) error { return nil }
	svc.getenv = func(string) string { return "" }
	svc.clipboardWrite = func(string) error { return nil }
	return svc
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
	t.Parallel()

	svc := testServices()

	svc.readConfig = func() (config.Config, error) {
		return config.Config{
			SchemaVersion: config.SchemaVersion,
			Models: []config.ModelEntry{
				config.ModelEntryFromFile(testOllamaRow("cached:latest")),
			},
		}, nil
	}
	svc.writeConfig = func(config.Config) error { return nil }
	svc.discoveryConfig = func(configPaths []string, lastScan time.Time) config.DiscoveryConfig {
		return config.DiscoveryConfig{ExtraModelPaths: configPaths, LastScan: lastScan}
	}
	svc.buildConfig = config.BuildConfig

	runtimeCalls := 0
	svc.discoverRuntime = func(settings.Settings) models.RuntimeInfo {
		runtimeCalls++
		if runtimeCalls >= 2 {
			return models.RuntimeInfo{OllamaPath: "/bin/ollama", OllamaHost: "127.0.0.1:11434", OllamaRunning: true}
		}
		return models.RuntimeInfo{OllamaPath: "/bin/ollama", OllamaHost: "127.0.0.1:11434"}
	}
	svc.startOllamaDaemon = func(spec serverSpec) error {
		if spec.bin != "/bin/ollama" {
			t.Fatalf("bin %q", spec.bin)
		}
		return nil
	}
	svc.waitForOllama = func(string) bool { return true }
	svc.probeOllama = func(string) bool { return false }
	svc.discoverModels = func(models.Options) ([]models.ModelFile, error) {
		return []models.ModelFile{testOllamaRow("live:latest")}, nil
	}

	msgs := collectCmdMsgs(t, svc.applyAndFullScanCmd())
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

// TestStartupCmd_InvalidCacheStillResolvesRuntimeFromFile guards the invariant
// that used to need an eager os.Setenv: when the config file reads but its cache
// is unusable, the runtime values in that file must still reach the settings the
// subsequent write path uses, or the on-disk [runtime] table would be blanked.
func TestStartupCmd_InvalidCacheStillResolvesRuntimeFromFile(t *testing.T) {
	t.Parallel()

	svc := testServices()

	wantPath := "/test/llama.cpp"
	cfg := config.Config{
		SchemaVersion: config.SchemaVersion - 1, // invalid cache
		Runtime:       config.RuntimeConfig{DefaultLlamaCppPath: wantPath},
	}
	svc.readConfig = func() (config.Config, error) { return cfg, nil }

	msgs := collectCmdMsgs(t, svc.startupCmd())
	if len(msgs) != 1 {
		t.Fatalf("got %d msgs, want 1", len(msgs))
	}
	if _, ok := msgs[0].(startupNeedFullScanMsg); !ok {
		t.Fatalf("msg[0] %T, want startupNeedFullScanMsg", msgs[0])
	}

	got := svc.resolveSettings(cfg, true, nil)
	if got.LlamaCppPath != wantPath {
		t.Fatalf("resolved LlamaCppPath = %q, want %q", got.LlamaCppPath, wantPath)
	}
	if rc := config.RuntimeConfigFromSettings(got); rc.DefaultLlamaCppPath != wantPath {
		t.Fatalf("write path would blank the runtime table: %q", rc.DefaultLlamaCppPath)
	}
}

func TestStartupCmd_CacheHitWithStoppedOllamaFallsBackToFullScan(t *testing.T) {
	t.Parallel()

	svc := testServices()

	svc.readConfig = func() (config.Config, error) {
		return config.Config{
			SchemaVersion: config.SchemaVersion,
			Models: []config.ModelEntry{
				config.ModelEntryFromFile(testOllamaRow("cached:latest")),
			},
		}, nil
	}
	svc.discoverRuntime = func(settings.Settings) models.RuntimeInfo {
		return models.RuntimeInfo{OllamaPath: "/bin/ollama", OllamaHost: "127.0.0.1:11434"}
	}
	svc.filterExisting = func(files []models.ModelFile) []models.ModelFile {
		return files
	}
	svc.modelFilesFromCfg = config.ModelFilesFromEntries

	msgs := collectCmdMsgs(t, svc.startupCmd())
	if len(msgs) != 1 {
		t.Fatalf("got %d msgs", len(msgs))
	}
	if _, ok := msgs[0].(startupNeedFullScanMsg); !ok {
		t.Fatalf("msg[0] %T", msgs[0])
	}
}

func TestStartupCmd_CacheHitWithRunningOllamaRefreshesLiveOllamaRows(t *testing.T) {
	t.Parallel()

	svc := testServices()

	var wrote config.Config
	svc.readConfig = func() (config.Config, error) {
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
	svc.writeConfig = func(c config.Config) error {
		wrote = c
		return nil
	}
	svc.buildConfig = config.BuildConfig
	svc.discoverRuntime = func(settings.Settings) models.RuntimeInfo {
		return models.RuntimeInfo{OllamaPath: "/bin/ollama", OllamaHost: "127.0.0.1:11434", OllamaRunning: true}
	}
	svc.filterExisting = func(files []models.ModelFile) []models.ModelFile {
		return files
	}
	svc.modelFilesFromCfg = config.ModelFilesFromEntries
	svc.discoverOllama = func(string) ([]models.ModelFile, error) {
		return []models.ModelFile{testOllamaRow("live:latest")}, nil
	}

	msgs := collectCmdMsgs(t, svc.startupCmd())
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
	t.Parallel()

	svc := testServices()

	svc.readConfig = func() (config.Config, error) {
		return config.Config{
			SchemaVersion: config.SchemaVersion,
			Models: []config.ModelEntry{
				config.ModelEntryFromFile(testOllamaRow("cached:latest")),
			},
		}, nil
	}
	svc.writeConfig = func(config.Config) error { return nil }
	svc.discoveryConfig = func(configPaths []string, lastScan time.Time) config.DiscoveryConfig {
		return config.DiscoveryConfig{ExtraModelPaths: configPaths, LastScan: lastScan}
	}
	svc.buildConfig = config.BuildConfig

	svc.discoverRuntime = func(settings.Settings) models.RuntimeInfo {
		return models.RuntimeInfo{OllamaPath: "/bin/ollama", OllamaHost: "127.0.0.1:11434"}
	}
	svc.startOllamaDaemon = func(serverSpec) error { return errors.New("boom") }
	svc.waitForOllama = func(string) bool { return false }
	svc.probeOllama = func(string) bool { return false }
	svc.discoverModels = func(models.Options) ([]models.ModelFile, error) {
		return []models.ModelFile{
			{Backend: models.BackendLlama, Path: "/m.gguf", Name: "m.gguf", Size: 1, ModTime: time.Unix(1, 0)},
		}, nil
	}

	msgs := collectCmdMsgs(t, svc.applyAndFullScanCmd())
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
	t.Parallel()

	svc := testServices()

	svc.readConfig = func() (config.Config, error) {
		return config.Config{}, os.ErrNotExist
	}
	svc.writeConfig = func(config.Config) error { return nil }
	svc.discoveryConfig = func(configPaths []string, lastScan time.Time) config.DiscoveryConfig {
		return config.DiscoveryConfig{ExtraModelPaths: configPaths, LastScan: lastScan}
	}
	svc.buildConfig = config.BuildConfig

	svc.discoverRuntime = func(settings.Settings) models.RuntimeInfo {
		return models.RuntimeInfo{OllamaPath: "/bin/ollama", OllamaHost: "127.0.0.1:11434"}
	}
	svc.startOllamaDaemon = func(serverSpec) error { return errors.New("boom") }
	svc.waitForOllama = func(string) bool { return false }
	svc.probeOllama = func(string) bool { return false }
	svc.discoverModels = func(models.Options) ([]models.ModelFile, error) {
		return []models.ModelFile{
			{Backend: models.BackendLlama, Path: "/m.gguf", Name: "m.gguf", Size: 1, ModTime: time.Unix(1, 0)},
		}, nil
	}

	msgs := collectCmdMsgs(t, svc.applyAndFullScanCmd())
	done := msgs[len(msgs)-1].(fullScanDoneMsg)
	if len(done.files) != 1 || done.files[0].Backend != models.BackendLlama {
		t.Fatalf("files %+v", done.files)
	}
	if done.ollamaWarn != "boom" {
		t.Fatalf("warn %q", done.ollamaWarn)
	}
}

func TestRescanModelsCmd_StartsOllamaAndReturnsDiscoveryNote(t *testing.T) {
	t.Parallel()

	svc := testServices()

	svc.readConfig = func() (config.Config, error) {
		return config.Config{SchemaVersion: config.SchemaVersion}, nil
	}
	svc.writeConfig = func(config.Config) error { return nil }
	svc.discoveryConfig = func(configPaths []string, lastScan time.Time) config.DiscoveryConfig {
		return config.DiscoveryConfig{ExtraModelPaths: configPaths, LastScan: lastScan}
	}
	svc.buildConfig = config.BuildConfig

	runtimeCalls := 0
	svc.discoverRuntime = func(settings.Settings) models.RuntimeInfo {
		runtimeCalls++
		if runtimeCalls >= 2 {
			return models.RuntimeInfo{OllamaPath: "/bin/ollama", OllamaHost: "127.0.0.1:11434", OllamaRunning: true}
		}
		return models.RuntimeInfo{OllamaPath: "/bin/ollama", OllamaHost: "127.0.0.1:11434"}
	}
	svc.startOllamaDaemon = func(serverSpec) error { return nil }
	svc.waitForOllama = func(string) bool { return true }
	svc.probeOllama = func(string) bool { return false }
	svc.discoverModels = func(models.Options) ([]models.ModelFile, error) {
		return []models.ModelFile{testOllamaRow("live:latest")}, nil
	}

	msgs := collectCmdMsgs(t, svc.rescanModelsCmd("/models"))
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
