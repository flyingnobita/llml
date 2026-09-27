package tui

import (
	"flag"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/flyingnobita/llml/internal/config"
	"github.com/flyingnobita/llml/internal/models"
	"github.com/flyingnobita/llml/internal/settings"
)

// clockRE matches the [HH:MM:SS] stamp on alert history lines.
var clockRE = regexp.MustCompile(`\[\d{2}:\d{2}:\d{2}\]`)

// updateGolden rewrites the expected output instead of comparing against it.
// Run with: go test ./internal/tui -run Golden -update
var updateGolden = flag.Bool("update", false, "rewrite golden files")

// assertGolden compares got against testdata/<name>.golden, writing the file
// when -update is passed.
//
// Golden files catch layout regressions that assertion-by-assertion tests miss:
// a pane that shifts by a column, a truncated title, a footer that wraps. ANSI
// styling is stripped first, so a theme change does not churn every file; the
// theme itself is covered by theme_test.go.
func assertGolden(t *testing.T, name, got string) {
	t.Helper()

	got = ansi.Strip(got)
	// Alert lines carry a wall clock. Blank the digits but keep the width, so
	// the layout is still checked and the file does not depend on the time.
	got = clockRE.ReplaceAllString(got, "[--:--:--]")
	// Trailing spaces are invisible in a diff and irrelevant to layout.
	lines := strings.Split(got, "\n")
	for i, l := range lines {
		lines[i] = strings.TrimRight(l, " ")
	}
	got = strings.Join(lines, "\n")

	path := filepath.Join("testdata", name+".golden")
	if *updateGolden {
		if err := os.MkdirAll("testdata", 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}

	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading golden file: %v (run: go test ./internal/tui -run Golden -update)", err)
	}
	if got != string(want) {
		t.Errorf("rendered view does not match %s.\n--- want ---\n%s\n--- got ---\n%s", path, want, got)
	}
}

// goldenModel builds a model with fixed content and size, so the rendering is
// reproducible: no real discovery, no clock, no terminal detection.
func goldenModel(t *testing.T) Model {
	t.Helper()

	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv("HOME", dir)
	t.Setenv("AppData", dir)
	// Program lookups fall back to PATH; an empty one keeps the runtime
	// panel's status marks independent of what the host has installed.
	t.Setenv("PATH", t.TempDir())

	m := NewWithServices(testServices())
	m.layout.width = 100
	m.layout.height = 30
	m.layout.homeDir = "/home/u"
	m.loading = false
	m.runtime = models.RuntimeInfo{
		LlamaServerPath:  "/opt/llama/bin/llama-server",
		LlamaServerHost:  "127.0.0.1",
		LlamaServerPort:  8080,
		VLLMPath:         "/opt/vllm/bin/vllm",
		VLLMServerHost:   "127.0.0.1",
		VLLMServerPort:   8000,
		OllamaHost:       "127.0.0.1:11434",
		OllamaPath:       "/usr/local/bin/ollama",
		KoboldCppPort:    5001,
		NInferPath:       "/opt/ninfer/build/apps/ninfer-serve",
		NInferServerHost: "127.0.0.1",
		NInferPort:       18080,
		Platform:         models.Platform{GOOS: "linux", GOARCH: "amd64"},
	}
	m.table.files = []models.ModelFile{
		{
			Backend: models.BackendLlama,
			Path:    "/home/u/models/qwen3-8b-q4.gguf",
			Name:    "qwen3-8b-q4.gguf",
			Size:    4_800_000_000,
			ModTime: time.Unix(1_700_000_000, 0).UTC(),
			// Parameters is GGUF metadata; fixed here so the row is stable.
			Parameters: "qwen3 · 8B · Q4_K_M",
		},
		{
			Backend:    models.BackendOllama,
			ID:         "llama3.2:latest",
			Location:   "ollama://llama3.2:latest",
			Name:       "llama3.2:latest",
			Size:       2_000_000_000,
			ModTime:    time.Unix(1_700_000_000, 0).UTC(),
			Parameters: "ollama · llama · 3B · Q4_0",
		},
	}
	m = m.layoutTable()
	m.table.tbl.SetCursor(0)
	return m
}

func TestGolden_mainView(t *testing.T) {
	m := goldenModel(t)
	assertGolden(t, "main_view", m.View().Content)
}

// Rows on a Disabled Runtime stay in the table, dimmed, with (off) after the
// Runtime, which widens the Runtime column. The highlighted row is unchanged.
func TestGolden_mainViewDimmedRows(t *testing.T) {
	m := goldenModel(t)
	m.table.files = append(m.table.files, models.ModelFile{
		Backend: models.BackendVLLM,
		Path:    "/home/u/models/Qwen3-4B",
		Name:    "Qwen3-4B",
		Size:    8_000_000_000,
		ModTime: time.Unix(1_700_000_000, 0).UTC(),
	})
	m.runtimeStates = config.RuntimeStates{}.With(models.BackendVLLM, false)
	m = m.layoutTable()
	assertGolden(t, "main_view_dimmed", m.View().Content)
}

func TestGolden_mainViewEmpty(t *testing.T) {
	m := goldenModel(t)
	m.table.files = nil
	m = m.layoutTable()
	assertGolden(t, "main_view_empty", m.View().Content)
}

// goldenPanelSettings sets one value from each source, so the golden files
// show every kind of in-use line: config, an environment variable, detected,
// and default.
func goldenPanelSettings() settings.Settings {
	env := map[string]string{settings.EnvLlamaServerPort: "8081"}
	return settings.Resolve(
		settings.FromEnv(func(k string) string { return env[k] }),
		settings.Layer{Origin: settings.OriginConfig, LlamaCppPath: ptrTo("/opt/llama/bin")},
		settings.Defaults(),
	)
}

func TestGolden_runtimeConfigPanel(t *testing.T) {
	m := goldenModel(t)
	m.settings = goldenPanelSettings()
	m, _ = m.openRuntimeConfig()
	assertGolden(t, "runtime_config_panel", m.View().Content)
}

// On Apple Silicon the list gains oMLX and the Splash bundle group and loses
// NInfer; oMLX is highlighted so its fields are pinned too.
func TestGolden_runtimeConfigPanelMacOS(t *testing.T) {
	m := goldenModel(t)
	m.settings = goldenPanelSettings()
	m.runtime.Platform = models.Platform{GOOS: "darwin", GOARCH: "arm64"}
	m.runtime.OMLXPath = "/Users/u/.omlx/bin/omlx"
	m.runtime.OMLXRunning = true
	m.runtime.SplashPath = "/opt/homebrew/bin/splash"
	m, _ = m.openRuntimeConfig()
	for range 3 {
		m, _ = m.updateRuntimeConfigKey(tea.KeyPressMsg{Code: tea.KeyDown})
	}
	assertGolden(t, "runtime_config_panel_macos", m.View().Content)
}

// A Disabled Runtime shows an empty checkbox and "off" in the list, and its
// detail pane says "Off" while its fields stay in place for editing.
func TestGolden_runtimeConfigPanelDisabled(t *testing.T) {
	m := goldenModel(t)
	m.settings = goldenPanelSettings()
	m.runtimeStates = config.RuntimeStates{}.With(models.BackendKobold, false)
	m, _ = m.openRuntimeConfig()
	m, _ = m.updateRuntimeConfigKey(tea.KeyPressMsg{Code: tea.KeyDown})
	assertGolden(t, "runtime_config_panel_disabled", m.View().Content)
}

func TestGolden_helpPanel(t *testing.T) {
	m := goldenModel(t)
	m.helpOpen = true
	assertGolden(t, "help_panel", m.View().Content)
}

func TestGolden_alertHistoryPane(t *testing.T) {
	m := goldenModel(t)
	m = m.addAlert(alertSeverityWarn, "Ollama", "Ollama is installed but not running")
	m = m.addAlert(alertSeverityError, "Discovery", "permission denied reading /opt/models")
	m = m.toggleAlerts()
	m = m.layoutTable()
	assertGolden(t, "alert_history", m.View().Content)
}
