package tui

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/flyingnobita/llml/internal/config"
	"github.com/flyingnobita/llml/internal/models"
	"github.com/flyingnobita/llml/internal/profiles"
)

var keyCtrlR = tea.KeyPressMsg{Code: 'r', Mod: tea.ModCtrl}

// launchFakes records every server launch and every Ollama daemon call, so a
// test can prove a blocked launch started nothing.
type launchFakes struct {
	launches  []models.ModelBackend
	specs     []serverSpec
	ollamaHit int
	clipboard []string
	services  services
}

func newLaunchFakes() *launchFakes {
	f := &launchFakes{}
	svc := testServices()
	svc.launchServer = func(_ services, spec serverSpec, _ runServerMode) tea.Cmd {
		f.launches = append(f.launches, spec.backend)
		f.specs = append(f.specs, spec)
		return nil
	}
	svc.startOllamaDaemon = func(serverSpec) error { f.ollamaHit++; return nil }
	svc.preloadOllama = func(context.Context, string, string) error { f.ollamaHit++; return nil }
	svc.clipboardWrite = func(s string) error { f.clipboard = append(f.clipboard, s); return nil }
	f.services = svc
	return f
}

func testRow(b models.ModelBackend, path string) models.ModelFile {
	return models.ModelFile{Backend: b, Path: path, Name: filepath.Base(path), Size: 1, ModTime: time.Unix(1, 0)}
}

// dimModel loads files, then delivers detection with the stored on/off
// states, in the order startup can: the scan may finish before detection.
func dimModel(t *testing.T, svc services, p models.Platform, states config.RuntimeStates, files ...models.ModelFile) Model {
	t.Helper()
	m := NewWithServices(svc)
	m.layout.width, m.layout.height = 140, 30
	m.loading = false
	next, _ := m.Update(modelsLoadedMsg{files: files})
	next, _ = next.(Model).Update(runtimeReadyMsg{
		settings: defaultSettings(),
		runtime:  panelRuntime(p),
		states:   runtimeStatesRead{states: states},
	})
	return next.(Model)
}

// tableLine returns the raw (styled) view line of the table row whose Model
// ID is id: the first line naming it, since the table is drawn first.
func tableLine(t *testing.T, m Model, id string) string {
	t.Helper()
	for _, l := range strings.Split(m.View().Content, "\n") {
		if strings.Contains(ansi.Strip(l), id) {
			return l
		}
	}
	t.Fatalf("no table row for %s:\n%s", id, plainView(m))
	return ""
}

// rowShowsDimmed reports whether the row with Model ID id says (off) after its
// Runtime. Unless the row is highlighted, where the selection style wins, a
// row saying (off) must also be drawn in the active theme's muted style.
func rowShowsDimmed(t *testing.T, m Model, id, runtime string) bool {
	t.Helper()
	raw := tableLine(t, m, id)
	off := strings.Contains(ansi.Strip(raw), runtime+" (off)")
	if sel, ok := m.SelectedModelFile(); ok && modelIDForRow(sel) == id {
		return off
	}
	muted := strings.Contains(raw, m.ui.styles.bodyDim.Render(runtime+" (off)")) &&
		strings.Contains(raw, m.ui.styles.bodyDim.Render(id))
	if off != muted {
		t.Fatalf("row %s: (off) shown = %t but muted = %t: %q", id, off, muted, raw)
	}
	return off
}

func lastAlert(t *testing.T, m Model) alertEntry {
	t.Helper()
	if len(m.alerts.history) == 0 {
		t.Fatal("no alert was appended")
	}
	return m.alerts.history[len(m.alerts.history)-1]
}

// Rows whose Runtime is off are muted and say (off); rows on an enabled
// Runtime are drawn as before. Sorting is unchanged, so the rows keep their
// Runtime order.
func TestDimmedRows_offRuntimeRowsAreMutedWithOff(t *testing.T) {
	t.Parallel()

	off := config.RuntimeStates{}.With(models.BackendVLLM, false)
	m := dimModel(t, testServices(), linuxPlatform, off,
		testRow(models.BackendVLLM, "/m/qwen-st"),
		testRow(models.BackendLlama, "/m/gemma.gguf"),
	)
	if !rowShowsDimmed(t, m, "qwen-st", "vllm") {
		t.Errorf("a row on the Disabled vLLM should be dimmed:\n%s", plainView(m))
	}
	if rowShowsDimmed(t, m, "gemma", "llama.cpp") {
		t.Errorf("a row on an enabled Runtime should not be dimmed:\n%s", plainView(m))
	}
	view := plainView(m)
	if strings.Index(view, "gemma.gguf") > strings.Index(view, "qwen-st") {
		t.Errorf("dimming must not change the Runtime sort order:\n%s", view)
	}
}

// t restyles dimmed rows with the new theme's muted colour.
func TestDimmedRows_restyleWithTheme(t *testing.T) {
	t.Parallel()

	off := config.RuntimeStates{}.With(models.BackendVLLM, false)
	m := dimModel(t, testServices(), linuxPlatform, off,
		testRow(models.BackendLlama, "/m/gemma.gguf"),
		testRow(models.BackendVLLM, "/m/qwen-st"),
	)
	before := m.ui.styles.bodyDim.Render("vllm (off)")
	for range themePickCount {
		m = press(t, m, keyText("t"))
		if !rowShowsDimmed(t, m, "qwen-st", "vllm") {
			t.Fatalf("theme %d: the row should stay dimmed in the new theme's style", m.ui.themePick)
		}
	}
	if after := m.ui.styles.bodyDim.Render("vllm (off)"); after != before {
		t.Fatalf("a full theme cycle should return to the starting style")
	}
}

// The highlighted row keeps the selection style over its whole width, and
// still says (off).
func TestDimmedRows_selectedDimmedRowKeepsSelectionStyle(t *testing.T) {
	t.Parallel()

	off := config.RuntimeStates{}.With(models.BackendVLLM, false)
	m := dimModel(t, testServices(), linuxPlatform, off,
		testRow(models.BackendLlama, "/m/gemma.gguf"),
		testRow(models.BackendVLLM, "/m/qwen-st"),
	)
	m = press(t, m, keyDown)
	raw := tableLine(t, m, "qwen-st")
	if !strings.Contains(ansi.Strip(raw), "vllm (off)") {
		t.Fatalf("the selected dimmed row should say (off): %q", raw)
	}
	if strings.Contains(raw, m.ui.styles.bodyDim.Render("qwen-st")) {
		t.Errorf("muted cells would cut the selection background short: %q", raw)
	}
}

// A dimmed row stays selectable and its launch command can still be copied.
func TestDimmedRows_stayCopyable(t *testing.T) {
	t.Parallel()

	f := newLaunchFakes()
	off := config.RuntimeStates{}.With(models.BackendVLLM, false)
	m := dimModel(t, f.services, linuxPlatform, off, testRow(models.BackendVLLM, "/m/qwen-st"))
	press(t, m, keyTab, keyEnter) // focus the launch preview, then copy
	if len(f.clipboard) != 1 || !strings.Contains(f.clipboard[0], "/m/qwen-st") {
		t.Errorf("enter on a dimmed row should copy its command, clipboard %q", f.clipboard)
	}
}

// R and ctrl+R on a dimmed row start nothing and append a warning that names
// the Runtime and points at the c panel.
func TestDimmedRows_launchIsBlocked(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		p       models.Platform
		row     models.ModelFile
		runtime string
	}{
		{"vllm", linuxPlatform, testRow(models.BackendVLLM, "/m/qwen-st"), "vLLM"},
		{"ninfer", linuxPlatform, testRow(models.BackendNInfer, "/m/q.ninfer"), "NInfer"},
		{"ollama", linuxPlatform, testOllamaRow("llama3.2:latest"), "Ollama"},
		{"omlx", macPlatform, testRow(models.BackendOMLX, "/Users/u/.omlx/models/qwen-mlx"), "oMLX"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			f := newLaunchFakes()
			off := config.RuntimeStates{}.With(tc.row.Backend, false)
			m := dimModel(t, f.services, tc.p, off, tc.row)
			for _, k := range []tea.KeyPressMsg{keyText("R"), keyCtrlR} {
				before := len(m.alerts.history)
				m = press(t, m, k)
				if len(m.alerts.history) != before+1 {
					t.Fatalf("%s: want one new alert, got %d", k, len(m.alerts.history)-before)
				}
				a := lastAlert(t, m)
				if a.severity != alertSeverityWarn || !strings.Contains(a.message, tc.runtime) || !strings.Contains(a.message, "c") {
					t.Errorf("%s: alert should warn about %s and point at c: %+v", k, tc.runtime, a)
				}
			}
			if len(f.launches) != 0 || f.ollamaHit != 0 || m.server.running {
				t.Errorf("a dimmed row must not launch: launches %v, ollama calls %d", f.launches, f.ollamaHit)
			}
		})
	}
}

// With oMLX off, rows in oMLX's model folders stay oMLX rows, dimmed; they
// never fall back to vLLM, which cannot load MLX weights.
func TestDimmedRows_omlxRowsStayOMLX(t *testing.T) {
	t.Parallel()

	off := config.RuntimeStates{}.With(models.BackendOMLX, false)
	m := dimModel(t, testServices(), macPlatform, off, testRow(models.BackendOMLX, "/Users/u/.omlx/models/qwen-mlx"))
	if !rowShowsDimmed(t, m, "qwen-mlx", "omlx") {
		t.Errorf("an oMLX row should stay oMLX and dim when oMLX is off:\n%s", plainView(m))
	}
	if strings.Contains(plainView(m), "vllm") {
		t.Errorf("an oMLX row must not be offered as vLLM:\n%s", plainView(m))
	}
}

// saveProfiles stores entry for modelPath in a model-params.json under a
// temporary config dir.
func saveProfiles(t *testing.T, modelPath string, entry profiles.Entry) {
	t.Helper()
	if err := profiles.SaveEntry(modelPath, entry); err != nil {
		t.Fatal(err)
	}
}

// useTempConfigDir points the user config dir at a temp dir. Parameter
// profiles are read from and written to disk directly, not through services,
// so tests that switch profiles cannot run in parallel.
func useTempConfigDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv("HOME", dir)
	t.Setenv("AppData", dir)
	return dir
}

// A GGUF row dims only when its Active Profile uses the Disabled Runtime:
// switching the profile between llama.cpp and KoboldCpp dims and undims it,
// and launching follows. The p panel opens on a dimmed row.
func TestDimmedRows_ggufRowFollowsActiveProfile(t *testing.T) {
	dir := useTempConfigDir(t)
	path := filepath.Join(dir, "gemma.gguf")
	saveProfiles(t, path, profiles.Entry{Profiles: []profiles.Profile{
		{Name: "cpp"},
		{Name: "kobold", Backend: "koboldcpp"},
	}})

	f := newLaunchFakes()
	off := config.RuntimeStates{}.With(models.BackendKobold, false)
	m := dimModel(t, f.services, linuxPlatform, off, testRow(models.BackendLlama, path))
	if rowShowsDimmed(t, m, "gemma", "llama.cpp") {
		t.Fatalf("the llama.cpp profile is on, so the row should not dim:\n%s", plainView(m))
	}

	m = press(t, m, keyText("p"), keyDown, keyEsc) // make "kobold" the Active Profile
	if !rowShowsDimmed(t, m, "gemma", "koboldcpp") {
		t.Fatalf("a KoboldCpp profile should dim the row while KoboldCpp is off:\n%s", plainView(m))
	}
	m = press(t, m, keyText("R"))
	if len(f.launches) != 0 || !strings.Contains(lastAlert(t, m).message, "KoboldCpp") {
		t.Fatalf("launch on the KoboldCpp profile should be blocked (launches %v)", f.launches)
	}

	m = press(t, m, keyText("p"))
	if !strings.Contains(plainView(m), "Parameter Profiles — gemma.gguf") {
		t.Fatalf("p should open on a dimmed row:\n%s", plainView(m))
	}
	m = press(t, m, keyUp, keyEsc) // back to "cpp"
	if rowShowsDimmed(t, m, "gemma", "llama.cpp") {
		t.Fatalf("switching back to llama.cpp should undim the row:\n%s", plainView(m))
	}
	press(t, m, keyText("R"))
	if len(f.launches) != 1 || f.launches[0] != models.BackendLlama {
		t.Errorf("an undimmed row should launch on llama.cpp, launches %v", f.launches)
	}
}

// backendRow returns the p panel's Backend line.
func backendRow(t *testing.T, m Model) string {
	t.Helper()
	for _, l := range strings.Split(plainView(m), "\n") {
		if strings.Contains(l, "Backend") && strings.Contains(l, "koboldcpp") {
			return l
		}
	}
	t.Fatalf("no Backend row:\n%s", plainView(m))
	return ""
}

// The p panel keeps a Disabled Runtime among the Backend options, labelled
// (off), and selecting it dims the row.
func TestParamPanel_backendOptionsMarkDisabledRuntimes(t *testing.T) {
	dir := useTempConfigDir(t)
	path := filepath.Join(dir, "gemma.gguf")

	off := config.RuntimeStates{}.With(models.BackendKobold, false)
	m := dimModel(t, newLaunchFakes().services, linuxPlatform, off, testRow(models.BackendLlama, path))
	m = press(t, m, keyText("p"), keyTab)
	row := backendRow(t, m)
	if !strings.Contains(row, "koboldcpp (off)") {
		t.Errorf("the Disabled KoboldCpp should be labelled (off): %q", row)
	}
	if strings.Contains(row, "llama (off)") || strings.Contains(row, "(none) (off)") {
		t.Errorf("enabled options should not be labelled (off): %q", row)
	}

	m = press(t, m, keyRight, keyRight, keySpace, keyEsc)
	if !rowShowsDimmed(t, m, "gemma", "koboldcpp") {
		t.Errorf("selecting the (off) KoboldCpp should dim the row:\n%s", plainView(m))
	}
}

// With llama.cpp off, the no-override option launches on llama.cpp too, so
// it is labelled (off) along with llama.
func TestParamPanel_defaultBackendOptionFollowsLlamaCpp(t *testing.T) {
	dir := useTempConfigDir(t)
	path := filepath.Join(dir, "gemma.gguf")

	off := config.RuntimeStates{}.With(models.BackendLlama, false)
	m := dimModel(t, newLaunchFakes().services, linuxPlatform, off, testRow(models.BackendLlama, path))
	if !rowShowsDimmed(t, m, "gemma", "llama.cpp") {
		t.Fatalf("a GGUF row with no profile runs on llama.cpp, so it should dim:\n%s", plainView(m))
	}
	row := backendRow(t, press(t, m, keyText("p"), keyTab))
	if !strings.Contains(row, "(none) (off)") || !strings.Contains(row, "llama (off)") {
		t.Errorf("both llama.cpp options should be labelled (off): %q", row)
	}
	if strings.Contains(row, "koboldcpp (off)") {
		t.Errorf("KoboldCpp is on: %q", row)
	}
}

// Saving a toggle in the c panel dims the rows at once, without a rescan.
func TestDimmedRows_followPanelSave(t *testing.T) {
	t.Parallel()

	f := newStateFakes()
	m := dimModel(t, f.services, linuxPlatform, config.RuntimeStates{}, testRow(models.BackendVLLM, "/m/qwen-st"))
	if rowShowsDimmed(t, m, "qwen-st", "vllm") {
		t.Fatal("vLLM starts on")
	}
	m = press(t, m, keyText("c"), keyDown, keyDown, keySpace, keyEnter) // vLLM off
	if !rowShowsDimmed(t, m, "qwen-st", "vllm") {
		t.Errorf("saving vLLM off should dim its rows:\n%s", plainView(m))
	}
	if f.rescans != 0 {
		t.Errorf("toggling should not rescan, got %d", f.rescans)
	}
}

// The launch preview of a dimmed row still shows its command, with a note,
// like the mmproj warning, that the Runtime is off. A row on a Runtime that
// is on gets none.
func TestDimmedRows_launchPreviewNotesRuntimeOff(t *testing.T) {
	t.Parallel()

	previewText := func(m Model) string { return ansi.Strip(m.preview.viewport.GetContent()) }

	off := config.RuntimeStates{}.With(models.BackendVLLM, false)
	m := dimModel(t, newLaunchFakes().services, linuxPlatform, off, testRow(models.BackendVLLM, "/m/qwen-st"))
	got := previewText(m)
	if !strings.Contains(got, "/m/qwen-st") {
		t.Fatalf("the preview should still show the command:\n%s", got)
	}
	if !strings.Contains(got, "vLLM is off") || !strings.Contains(got, "("+FooterKeyConfigPort+")") {
		t.Errorf("the preview should note that vLLM is off and point at c:\n%s", got)
	}
	if note := m.ui.styles.warnLine.Render(fmt.Sprintf(runtimeOffPreviewNote, "vLLM")); !strings.Contains(m.preview.viewport.GetContent(), note) {
		t.Error("the note should use the warning style")
	}

	on := dimModel(t, newLaunchFakes().services, linuxPlatform, config.RuntimeStates{}, testRow(models.BackendVLLM, "/m/qwen-st"))
	if got := previewText(on); strings.Contains(got, "is off") {
		t.Errorf("a row on an enabled Runtime should have no off note:\n%s", got)
	}
}

// Launch warnings sit above the command, after the model id line, so a long
// command cannot push them out of the preview's visible lines. The copied
// command carries none of them.
func TestLaunchPreview_warningsAboveCommand(t *testing.T) {
	useTempConfigDir(t)
	// A fixed, short, missing folder: a temp dir's path varies in length by OS
	// (macOS's /var/folders/… wraps where Linux's /tmp/… does not), and the
	// wrapped model id line would decide whether the warnings fit.
	gone := testRow(models.BackendVLLM, filepath.FromSlash("/nonexistent-llml/hf/gone"))
	args := []string{"--max-tokens", "4096", "--temp", "0.7", "--top-p", "0.9", "--chat-template-args", "{}", "--prompt-cache-size", "8"}
	saveProfiles(t, gone.Path, profiles.Entry{Profiles: []profiles.Profile{{Name: "mlx", Backend: "mlx-lm", Args: args}}})

	f := newLaunchFakes()
	off := config.RuntimeStates{}.With(models.BackendMLXLM, false)
	m := dimModel(t, f.services, linuxPlatform, off, gone)
	m.layout.width = 100
	m = m.layoutTable()

	content := ansi.Strip(m.preview.viewport.GetContent())
	cmdAt := strings.Index(content, "mlx_lm.server")
	offAt := strings.Index(content, "mlx-lm is off")
	missingAt := strings.Index(content, "model folder not found")
	idAt := strings.Index(content, strings.TrimSpace(launchPreviewModelIDLabel))
	if cmdAt < 0 || offAt < 0 || missingAt < 0 || idAt < 0 {
		t.Fatalf("preview should show the id, both warnings, and the command:\n%s", content)
	}
	if offAt > cmdAt || missingAt > cmdAt || idAt > offAt {
		t.Errorf("want the id line, then the warnings, then the command:\n%s", content)
	}
	if view := plainView(m); !strings.Contains(view, "mlx-lm is off") || !strings.Contains(view, "model folder not found") {
		t.Errorf("the warnings should show without scrolling:\n%s", view)
	}

	press(t, m, keyTab, keyEnter) // focus the launch preview, then copy
	if len(f.clipboard) != 1 {
		t.Fatalf("want one copy, got %q", f.clipboard)
	}
	if c := f.clipboard[0]; strings.Contains(c, "is off") || strings.Contains(c, "not found") {
		t.Errorf("the copy should be the command alone: %q", c)
	}
}
