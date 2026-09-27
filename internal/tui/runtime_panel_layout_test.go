package tui

import (
	"context"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/flyingnobita/llml/internal/config"
	"github.com/flyingnobita/llml/internal/models"
	"github.com/flyingnobita/llml/internal/settings"
)

var (
	linuxPlatform = models.Platform{GOOS: "linux", GOARCH: "amd64"}
	macPlatform   = models.Platform{GOOS: "darwin", GOARCH: "arm64"}
)

// panelRuntime is detection output with a fixed program path for every
// Runtime, so the panel never falls back to a PATH lookup on the test host.
func panelRuntime(p models.Platform) models.RuntimeInfo {
	return models.RuntimeInfo{
		LlamaServerPath: "/opt/llama/bin/llama-server",
		KoboldCppPath:   "/opt/kobold/koboldcpp",
		VLLMPath:        "/opt/vllm/bin/vllm",
		OllamaPath:      "/usr/local/bin/ollama",
		NInferPath:      "/opt/ninfer/build/apps/ninfer-serve",
		OMLXPath:        "/opt/omlx/bin/omlx",
		SplashPath:      "/opt/splash/bin/splash",
		ServerRunning:   true,
		Platform:        p,
	}
}

// openPanel returns a model of size w x h on platform p with the runtime
// panel opened through the c key.
func openPanel(t *testing.T, svc services, p models.Platform, s settings.Settings, w, h int) Model {
	t.Helper()
	m := NewWithServices(svc)
	m.layout.width = w
	m.layout.height = h
	m.loading = false
	m.runtimeScanned = true
	m.settings = s
	m.runtime = panelRuntime(p)
	m = m.layoutTable()
	return press(t, m, keyText("c"))
}

func keyText(s string) tea.KeyPressMsg {
	r := []rune(s)
	return tea.KeyPressMsg{Code: r[0], Text: s}
}

var (
	keyDown     = tea.KeyPressMsg{Code: tea.KeyDown}
	keyUp       = tea.KeyPressMsg{Code: tea.KeyUp}
	keyRight    = tea.KeyPressMsg{Code: tea.KeyRight}
	keyTab      = tea.KeyPressMsg{Code: tea.KeyTab}
	keyShiftTab = tea.KeyPressMsg{Code: tea.KeyTab, Mod: tea.ModShift}
	keyEnter    = tea.KeyPressMsg{Code: tea.KeyEnter}
	keyEsc      = tea.KeyPressMsg{Code: tea.KeyEscape}
	keyCtrlU    = tea.KeyPressMsg{Code: 'u', Mod: tea.ModCtrl}
)

// press sends keys through Update, as the terminal would.
func press(t *testing.T, m Model, keys ...tea.KeyPressMsg) Model {
	t.Helper()
	for _, k := range keys {
		next, _ := m.Update(k)
		m = next.(Model)
	}
	return m
}

func typeText(t *testing.T, m Model, s string) Model {
	t.Helper()
	for _, r := range s {
		m = press(t, m, keyText(string(r)))
	}
	return m
}

func plainView(m Model) string { return ansi.Strip(m.View().Content) }

func defaultSettings() settings.Settings { return settings.Resolve(settings.Defaults()) }

// The list shows only the Runtimes the platform supports, under the Model
// Format each one runs; a format with no supported Runtime has no label.
func TestRuntimePanel_listGroupsSupportedRuntimes(t *testing.T) {
	t.Parallel()

	linux := plainView(openPanel(t, testServices(), linuxPlatform, defaultSettings(), 100, 30))
	for _, want := range []string{"GGUF", "Llama.cpp", "KoboldCpp", "Safetensors", "vLLM", "NInfer (.ninfer)", "Ollama library", "Ollama"} {
		if !strings.Contains(linux, want) {
			t.Errorf("linux panel missing %q:\n%s", want, linux)
		}
	}
	for _, absent := range []string{"oMLX", "Splash", "Active Configuration"} {
		if strings.Contains(linux, absent) {
			t.Errorf("linux panel should not show %q:\n%s", absent, linux)
		}
	}

	mac := plainView(openPanel(t, testServices(), macPlatform, defaultSettings(), 100, 30))
	for _, want := range []string{"oMLX", "Splash bundle", "Splash"} {
		if !strings.Contains(mac, want) {
			t.Errorf("macOS panel missing %q:\n%s", want, mac)
		}
	}
	if strings.Contains(mac, "NInfer") {
		t.Errorf("macOS panel should not show NInfer:\n%s", mac)
	}
}

// focusLine returns the one view line that carries the focus marker "›", and
// fails when there is not exactly one.
func focusLine(t *testing.T, m Model) string {
	t.Helper()
	var found []string
	for _, l := range strings.Split(plainView(m), "\n") {
		if strings.Contains(l, "›") {
			found = append(found, l)
		}
	}
	if len(found) != 1 {
		t.Fatalf("want exactly one focus marker, got %d:\n%s", len(found), plainView(m))
	}
	return found[0]
}

// detailHeader is the first detail pane line: the highlighted Runtime's name.
func detailHeaderShows(m Model, name string) bool {
	return strings.Contains(plainView(m), name+" · ")
}

// The cursor moves over Runtimes only, skipping group labels, and stops at
// either end of the list.
func TestRuntimePanel_cursorSkipsGroupLabels(t *testing.T) {
	t.Parallel()

	m := openPanel(t, testServices(), linuxPlatform, defaultSettings(), 100, 30)
	if !strings.Contains(focusLine(t, m), "Llama.cpp") || !detailHeaderShows(m, "Llama.cpp") {
		t.Fatalf("panel should open on Llama.cpp:\n%s", plainView(m))
	}
	steps := []struct {
		key  tea.KeyPressMsg
		want string
	}{
		{keyDown, "KoboldCpp"},
		{keyText("j"), "vLLM"}, // past the Safetensors label
		{keyDown, "NInfer"},
		{keyDown, "Ollama"},
		{keyDown, "Ollama"}, // the end of the list
		{keyText("k"), "NInfer"},
		{keyUp, "vLLM"},
	}
	for _, s := range steps {
		m = press(t, m, s.key)
		if l := focusLine(t, m); !strings.Contains(l, s.want) {
			t.Fatalf("after %q want %s highlighted, focus line %q", s.key.String(), s.want, l)
		}
		if !detailHeaderShows(m, s.want) {
			t.Fatalf("detail pane should show %s:\n%s", s.want, plainView(m))
		}
	}
}

// Tab cycles the list, then the highlighted Runtime's fields top to bottom,
// then back to the list; shift+tab reverses it. No other Runtime's field is
// ever focused.
func TestRuntimePanel_tabOrderFollowsTheDetailPane(t *testing.T) {
	t.Parallel()

	m := openPanel(t, testServices(), linuxPlatform, defaultSettings(), 100, 30)
	m = press(t, m, keyDown) // KoboldCpp: Path, Port

	wantTab := []string{"Path", "Port", "KoboldCpp", "Path"}
	for _, want := range wantTab {
		m = press(t, m, keyTab)
		if l := focusLine(t, m); !strings.Contains(l, want) {
			t.Fatalf("tab: want focus on %s, focus line %q", want, l)
		}
	}
	wantBack := []string{"KoboldCpp", "Port", "Path"}
	for _, want := range wantBack {
		m = press(t, m, keyShiftTab)
		if l := focusLine(t, m); !strings.Contains(l, want) {
			t.Fatalf("shift+tab: want focus on %s, focus line %q", want, l)
		}
	}
}

// → from the list enters the first field; typed letters then go to the field,
// not the list.
func TestRuntimePanel_rightEntersFirstField(t *testing.T) {
	t.Parallel()

	m := openPanel(t, testServices(), linuxPlatform, defaultSettings(), 100, 30)
	m = press(t, m, keyRight)
	m = typeText(t, m, "/jk")
	l := focusLine(t, m)
	if !strings.Contains(l, "Path") || !strings.Contains(l, "/jk") {
		t.Fatalf("typing after → should fill Llama.cpp's Path, focus line %q", l)
	}
	if !detailHeaderShows(m, "Llama.cpp") {
		t.Fatalf("j/k in a field must not move the list:\n%s", plainView(m))
	}
}

// Every field shows the value in use and where it came from: the environment
// variable's name, config, default, or detected for a program path found with
// nothing configured. An environment source is drawn in the warning colour.
func TestRuntimePanel_inUseLinesNameTheirSource(t *testing.T) {
	t.Parallel()

	env := map[string]string{settings.EnvLlamaServerHost: "0.0.0.0"}
	s := settings.Resolve(
		settings.FromEnv(func(k string) string { return env[k] }),
		settings.Layer{Origin: settings.OriginConfig, LlamaServerPort: ptrTo(9000)},
		settings.Defaults(),
	)
	m := openPanel(t, testServices(), linuxPlatform, s, 100, 30)

	view := plainView(m)
	for _, want := range []string{
		"in use: /opt/llama/bin/llama-server (detected)",
		"in use: 9000 (config)",
		"in use: 0.0.0.0 (LLAMA_SERVER_HOST)",
	} {
		if !strings.Contains(view, want) {
			t.Errorf("missing %q:\n%s", want, view)
		}
	}
	if tag := m.ui.styles.runtimeEnvSource.Render(" (LLAMA_SERVER_HOST)"); !strings.Contains(m.View().Content, tag) {
		t.Error("an environment source should use the warning style")
	}
	if tag := m.ui.styles.runtimeEnvSource.Render(" (config)"); strings.Contains(m.View().Content, tag) {
		t.Error("a config source should not use the warning style")
	}

	m = press(t, m, keyDown) // KoboldCpp: nothing configured
	if view := plainView(m); !strings.Contains(view, "in use: 5001 (default)") {
		t.Errorf("missing the default port line:\n%s", view)
	}
}

// A field the environment overrides stays editable, so config.toml can be
// prepared for when the variable is dropped.
func TestRuntimePanel_envSourcedFieldIsEditable(t *testing.T) {
	t.Parallel()

	env := map[string]string{settings.EnvLlamaServerHost: "0.0.0.0"}
	s := settings.Resolve(settings.FromEnv(func(k string) string { return env[k] }), settings.Defaults())
	m := openPanel(t, testServices(), linuxPlatform, s, 100, 30)
	m = press(t, m, keyTab, keyTab, keyTab, keyCtrlU) // Llama.cpp Host
	m = typeText(t, m, "10.0.0.2")
	if l := focusLine(t, m); !strings.Contains(l, "Host") || !strings.Contains(l, "10.0.0.2") {
		t.Fatalf("the env-sourced Host field should take input, focus line %q", l)
	}
}

// panelFakes records what the runtime panel writes and probes.
type panelFakes struct {
	written  []config.Config
	probed   []settings.Settings
	rescans  int
	services services
}

func newPanelFakes(getenv settings.Getenv) *panelFakes {
	f := &panelFakes{}
	svc := testServices()
	svc.getenv = getenv
	svc.writeConfig = func(c config.Config) error {
		f.written = append(f.written, c)
		return nil
	}
	svc.discoverRuntime = func(_ context.Context, s settings.Settings) models.RuntimeInfo {
		f.probed = append(f.probed, s)
		return panelRuntime(linuxPlatform)
	}
	svc.discoverModels = func(context.Context, models.Options) ([]models.ModelFile, error) {
		f.rescans++
		return nil, nil
	}
	f.services = svc
	return f
}

// enter writes the fields to config.toml and re-runs runtime detection with
// the new values, without rescanning models. Reopening shows the saved value
// as coming from config.
func TestRuntimePanel_enterSavesAndRedetects(t *testing.T) {
	t.Parallel()

	f := newPanelFakes(func(string) string { return "" })
	m := openPanel(t, f.services, linuxPlatform, defaultSettings(), 100, 30)
	m = press(t, m, keyTab, keyTab, keyCtrlU) // Llama.cpp Port
	m = typeText(t, m, "9001")
	m = press(t, m, keyEnter)

	if m.rc.open {
		t.Fatal("enter should close the panel")
	}
	if len(f.written) != 1 {
		t.Fatalf("want one config write, got %d", len(f.written))
	}
	if p := f.written[0].Runtime.DefaultLlamaServerPort; p == nil || *p != 9001 {
		t.Errorf("written llama port = %v, want 9001", p)
	}
	if len(f.probed) != 1 || f.probed[0].LlamaServerPort != 9001 {
		t.Errorf("detection should re-run once with the new port, got %d runs", len(f.probed))
	}
	if f.rescans != 0 {
		t.Errorf("saving should not rescan models, got %d scans", f.rescans)
	}

	m = press(t, m, keyText("c"))
	if view := plainView(m); !strings.Contains(view, "in use: 9001 (config)") {
		t.Errorf("reopened panel should show the saved port from config:\n%s", view)
	}
}

// A saved value for a field the environment sets goes to config.toml, but the
// environment's value stays in use.
func TestRuntimePanel_environmentStillWinsAfterSave(t *testing.T) {
	t.Parallel()

	env := map[string]string{settings.EnvLlamaServerHost: "0.0.0.0"}
	getenv := func(k string) string { return env[k] }
	f := newPanelFakes(getenv)
	s := settings.Resolve(settings.FromEnv(getenv), settings.Defaults())
	m := openPanel(t, f.services, linuxPlatform, s, 100, 30)
	m = press(t, m, keyTab, keyTab, keyTab, keyCtrlU) // Llama.cpp Host
	m = typeText(t, m, "10.0.0.2")
	m = press(t, m, keyEnter)

	if len(f.written) != 1 || f.written[0].Runtime.DefaultLlamaServerHost != "10.0.0.2" {
		t.Fatalf("the edited host should be written to config.toml, got %+v", f.written)
	}
	if len(f.probed) != 1 || f.probed[0].LlamaServerHost != "0.0.0.0" {
		t.Errorf("detection should use the environment's host")
	}
	m = press(t, m, keyText("c"))
	if view := plainView(m); !strings.Contains(view, "in use: 0.0.0.0 (LLAMA_SERVER_HOST)") {
		t.Errorf("the environment's host should stay in use:\n%s", view)
	}
}

// esc with unsaved edits asks first; confirming discards without writing.
func TestRuntimePanel_escDiscardsAfterConfirmation(t *testing.T) {
	t.Parallel()

	f := newPanelFakes(func(string) string { return "" })
	m := openPanel(t, f.services, linuxPlatform, defaultSettings(), 100, 30)
	m = press(t, m, keyTab, keyTab, keyCtrlU)
	m = typeText(t, m, "9001")
	m = press(t, m, keyEsc)
	if view := plainView(m); !strings.Contains(view, "Discard runtime changes?") {
		t.Fatalf("esc with edits should ask to discard:\n%s", view)
	}
	m = press(t, m, keyText("y"))
	if m.rc.open || len(f.written) != 0 || len(f.probed) != 0 {
		t.Fatalf("discard should close without saving (open=%t writes=%d probes=%d)", m.rc.open, len(f.written), len(f.probed))
	}
	m = press(t, m, keyText("c"))
	if view := plainView(m); !strings.Contains(view, "in use: 8080 (default)") || strings.Contains(view, "9001") {
		t.Errorf("reopened panel should show the original port:\n%s", view)
	}
}

// At 80x24 the whole panel is on screen on both platforms, whichever Runtime
// is highlighted: the title, every field with its in-use line, the legend, the
// note, the key hints, and the bottom border.
func TestRuntimePanel_fitsEightyByTwentyFour(t *testing.T) {
	t.Parallel()

	// Every path comes from the environment and is long, so the in-use lines
	// carry the longest values and source tags the panel can show.
	long := "/very/long/directory/name/that/does/not/fit/in/eighty/columns"
	env := map[string]string{}
	for _, rt := range runtimeTable {
		for _, d := range rt.fields {
			if d.isPath {
				env[d.env()] = long
			}
		}
	}
	s := settings.Resolve(settings.FromEnv(func(k string) string { return env[k] }), settings.Defaults())

	for _, p := range []models.Platform{linuxPlatform, macPlatform} {
		m := openPanel(t, testServices(), p, s, 80, 24)
		m.runtime.LlamaServerPath = long + "/llama-server"
		m.runtime.OllamaPath = long + "/ollama"
		for _, rt := range m.panelRuntimes() {
			view := plainView(m)
			lines := strings.Split(view, "\n")
			if len(lines) > 24 {
				t.Fatalf("%s/%s: view is %d lines", p.GOOS, rt.name, len(lines))
			}
			for _, want := range []string{
				"Runtime Environment",
				"● running  ○ found  ✗ not found",
				runtimeConfigModalSubtitle,
				"↑/↓: runtime · tab/→: fields · enter: save · esc: back",
			} {
				if !strings.Contains(view, want) {
					t.Errorf("%s/%s: clipped, missing %q:\n%s", p.GOOS, rt.name, want, view)
				}
			}
			if got := strings.Count(view, "in use: "); got != len(rt.fields) {
				t.Errorf("%s/%s: %d in-use lines, want %d:\n%s", p.GOOS, rt.name, got, len(rt.fields), view)
			}
			for _, d := range rt.fields {
				// Which label a field gets is tested elsewhere; here it only
				// has to survive the truncation of a long value.
				if _, src := m.runtimeFieldInUse(d); !strings.Contains(view, "("+src.label+")") {
					t.Errorf("%s/%s: source tag %q clipped:\n%s", p.GOOS, rt.name, src.label, view)
				}
			}
			// The overlay cuts a block larger than the terminal, so a block
			// that fits is drawn whole, bottom border included.
			if block := m.runtimeConfigModalBlock(); lipgloss.Height(block) > 24 || lipgloss.Width(block) > 80 {
				t.Errorf("%s/%s: panel is %dx%d, larger than 80x24", p.GOOS, rt.name, lipgloss.Width(block), lipgloss.Height(block))
			}
			for _, l := range lines {
				if w := ansi.StringWidth(l); w > 80 {
					t.Errorf("%s/%s: line is %d columns: %q", p.GOOS, rt.name, w, l)
				}
			}
			m = press(t, m, keyDown)
		}
	}
}

// The panel's styles come from the theme, so t restyles it: after a cycle from
// dark to light, the environment tag is drawn in the light palette's warning colour.
func TestRuntimePanel_restylesWithTheme(t *testing.T) {
	t.Parallel()

	env := map[string]string{settings.EnvLlamaServerHost: "0.0.0.0"}
	s := settings.Resolve(settings.FromEnv(func(k string) string { return env[k] }), settings.Defaults())
	m := NewWithServices(testServices())
	m.layout.width, m.layout.height = 100, 30
	m.loading = false
	m.settings = s
	m.runtime = panelRuntime(linuxPlatform)
	m.ui.themePick = themePickDark
	m.ui.theme = themeFromPick(m.ui.themePick, true)
	m.ui.styles = newStyles(m.ui.theme)
	m, _ = m.cycleTheme() // dark -> light
	m = press(t, m, keyText("c"))

	tag := " (LLAMA_SERVER_HOST)"
	light := newStyles(LightTheme()).runtimeEnvSource.Render(tag)
	dark := newStyles(DarkTheme()).runtimeEnvSource.Render(tag)
	if light == dark {
		t.Fatal("fixture: light and dark warning tags render the same")
	}
	if view := m.View().Content; !strings.Contains(view, light) || strings.Contains(view, dark) {
		t.Error("the environment tag should follow the theme change")
	}
}

// A field the environment sets starts from the saved value, not the
// environment's, so saving the panel never copies the variable into
// config.toml. The in-use line still shows the environment's value.
func TestRuntimePanel_envSourcedFieldStartsFromSavedValue(t *testing.T) {
	t.Parallel()

	env := map[string]string{settings.EnvLlamaServerPort: "8081"}
	getenv := func(k string) string { return env[k] }
	f := newPanelFakes(getenv)
	saved := config.Config{Runtime: config.RuntimeConfig{DefaultLlamaServerPort: ptrTo(9000)}}
	f.services.readConfig = func() (config.Config, error) { return saved, nil }
	s := settings.Resolve(settings.FromEnv(getenv), saved.Layer(), settings.Defaults())

	m := openPanel(t, f.services, linuxPlatform, s, 100, 30)
	m = press(t, m, keyTab, keyTab) // Llama.cpp Port
	if l := focusLine(t, m); !strings.Contains(l, "9000") {
		t.Errorf("the Port input should hold the saved 9000, focus line %q", l)
	}
	if view := plainView(m); !strings.Contains(view, "in use: 8081 (LLAMA_SERVER_PORT)") {
		t.Errorf("the environment's port should be in use:\n%s", view)
	}

	press(t, m, keyEnter)
	if len(f.written) != 1 {
		t.Fatalf("want one config write, got %d", len(f.written))
	}
	if p := f.written[0].Runtime.DefaultLlamaServerPort; p == nil || *p != 9000 {
		t.Errorf("saving should keep the saved port 9000, wrote %v", p)
	}
}

// A path's source is "detected" whenever the program in use was not found at
// the configured path, and a program found nowhere reads "not found".
func TestRuntimePanel_pathSourceFollowsWhereTheProgramWasFound(t *testing.T) {
	t.Parallel()

	s := settings.Resolve(
		settings.Layer{Origin: settings.OriginConfig, LlamaCppPath: ptrTo("/opt/llama/bin"), VLLMPath: ptrTo("/opt/vllm/bin")},
		settings.Defaults(),
	)
	m := openPanel(t, testServices(), linuxPlatform, s, 100, 30)
	m.runtime.LlamaServerPath = "/usr/local/bin/llama-server" // not under /opt/llama/bin
	if view := plainView(m); !strings.Contains(view, "in use: /usr/local/bin/llama-server (detected)") {
		t.Errorf("a program found outside the configured path is detected:\n%s", view)
	}

	m = press(t, m, keyDown, keyDown) // vLLM, found under its configured path
	if view := plainView(m); !strings.Contains(view, "in use: /opt/vllm/bin/vllm (config)") {
		t.Errorf("a program under the configured path comes from config:\n%s", view)
	}
}

// Not parallel: program lookups fall back to PATH, which the test empties so
// nothing is found on the host.
func TestRuntimePanel_missingProgramReadsNotFound(t *testing.T) {
	t.Setenv("PATH", t.TempDir())

	m := openPanel(t, testServices(), linuxPlatform, defaultSettings(), 100, 30)
	m.runtime.LlamaServerPath = ""
	m.runtime.ServerRunning = true // a server answering does not make the program found
	if view := plainView(m); !strings.Contains(view, "in use: not found (default)") {
		t.Errorf("a program found nowhere reads not found:\n%s", view)
	}
}
