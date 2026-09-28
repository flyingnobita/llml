package tui

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"

	"github.com/flyingnobita/llml/internal/config"
	"github.com/flyingnobita/llml/internal/models"
	"github.com/flyingnobita/llml/internal/profiles"
)

// omlxRow is a Safetensors row inside oMLX's model folder under dir, which
// discovery gives oMLX.
func omlxRow(dir string) models.ModelFile {
	return testRow(models.BackendOMLX, filepath.Join(dir, ".omlx", "models", "qwen-mlx"))
}

// hfRow is a Safetensors row outside oMLX's model folders, which discovery
// gives vLLM.
func hfRow(dir string) models.ModelFile {
	return testRow(models.BackendVLLM, filepath.Join(dir, "hf", "qwen-st"))
}

// runtimeCell returns the Runtime column of the table row whose Model ID is id.
func runtimeCell(t *testing.T, m Model, id string) string {
	t.Helper()
	line := ansi.Strip(tableLine(t, m, id))
	fields := strings.Fields(line[strings.Index(line, id)+len(id):])
	if len(fields) == 0 {
		t.Fatalf("row %s has no Runtime cell: %q", id, line)
	}
	return fields[0]
}

// paramBackendLine returns the p panel's Backend line.
func paramBackendLine(t *testing.T, m Model) string {
	t.Helper()
	for _, l := range strings.Split(plainView(m), "\n") {
		if strings.Contains(l, "Backend") && strings.Contains(l, "(") {
			return l
		}
	}
	t.Fatalf("no Backend row:\n%s", plainView(m))
	return ""
}

// lastSpec returns the spec of the most recent launch.
func lastSpec(t *testing.T, f *launchFakes) serverSpec {
	t.Helper()
	if len(f.specs) == 0 {
		t.Fatal("nothing was launched")
	}
	return f.specs[len(f.specs)-1]
}

// A model in oMLX's folder whose profile picks vLLM in the p panel shows vLLM
// in the table and launches `vllm serve` for its own path.
func TestSafetensorsRuntime_omlxRowSwitchedToVLLMLaunchesVLLM(t *testing.T) {
	dir := useTempConfigDir(t)
	row := omlxRow(dir)

	f := newLaunchFakes()
	m := dimModel(t, f.services, macPlatform, config.RuntimeStates{}, row)
	if got := runtimeCell(t, m, "qwen-mlx"); got != "omlx" {
		t.Fatalf("with no profile the row should be oMLX, got %q", got)
	}

	m = press(t, m, keyText("p"), keyTab)
	line := paramBackendLine(t, m)
	if !strings.Contains(line, "vllm") || !strings.Contains(line, "omlx") {
		t.Fatalf("a row in oMLX's folder should be offered vLLM and oMLX: %q", line)
	}
	m = press(t, m, keyRight, keySpace, keyEsc) // (none) -> vllm
	if got := runtimeCell(t, m, "qwen-mlx"); got != "vllm" {
		t.Fatalf("the Runtime column should show the profile's vLLM, got %q:\n%s", got, plainView(m))
	}

	press(t, press(t, m, keyText("R")), keyCtrlR)
	if len(f.specs) != 2 {
		t.Fatalf("R and ctrl+R should each launch once, got %d", len(f.specs))
	}
	for _, spec := range f.specs {
		if spec.backend != models.BackendVLLM {
			t.Fatalf("launched on %v, want vLLM", spec.backend)
		}
		args := spec.directArgs()
		if len(args) < 2 || args[0] != "serve" || args[1] != row.Path {
			t.Errorf("want vllm serve %s, got %q", row.Path, args)
		}
	}

	ent, err := profiles.LoadEntry(profiles.ModelParamsKey(row.Path))
	if err != nil || len(ent.Profiles) == 0 || ent.Profiles[ent.ActiveIndex].Backend != "vllm" {
		t.Errorf("the profile should keep backend vllm after saving, got %+v (err %v)", ent, err)
	}
}

// A row outside oMLX's folders is never offered oMLX, on any platform.
func TestSafetensorsRuntime_omlxOfferedOnlyInItsFolders(t *testing.T) {
	for _, p := range []models.Platform{macPlatform, linuxPlatform} {
		t.Run(p.GOOS, func(t *testing.T) {
			dir := useTempConfigDir(t)
			m := dimModel(t, newLaunchFakes().services, p, config.RuntimeStates{}, hfRow(dir))
			m = press(t, m, keyText("p"), keyTab)
			if opts := m.paramBackendOptionsForModel(); slices.Contains(opts, "omlx") {
				t.Errorf("oMLX offered outside its folders: %q", opts)
			}
			if strings.Contains(plainView(m), "omlx") {
				t.Errorf("the p panel should not mention oMLX:\n%s", plainView(m))
			}
		})
	}
}

// oMLX is never offered where the platform cannot run it, even for a row
// that says oMLX (a cache copied from a Mac).
func TestSafetensorsRuntime_omlxNotOfferedOffAppleSilicon(t *testing.T) {
	dir := useTempConfigDir(t)
	m := dimModel(t, newLaunchFakes().services, linuxPlatform, config.RuntimeStates{}, omlxRow(dir))
	m = press(t, m, keyText("p"), keyTab)
	if opts := m.paramBackendOptionsForModel(); slices.Contains(opts, "omlx") {
		t.Errorf("oMLX offered on Linux: %q", opts)
	}
}

// With no profile override, and with the (none) option, each row keeps the
// Runtime discovery gave it.
func TestSafetensorsRuntime_defaultUnchangedWithoutOverride(t *testing.T) {
	dir := useTempConfigDir(t)
	omlx, hf := omlxRow(dir), hfRow(dir)
	saveProfiles(t, hf.Path, profiles.Entry{Profiles: []profiles.Profile{{Name: "default", Backend: ""}}})

	f := newLaunchFakes()
	m := dimModel(t, f.services, macPlatform, config.RuntimeStates{}, omlx, hf)
	if got := runtimeCell(t, m, "qwen-mlx"); got != "omlx" {
		t.Errorf("oMLX-folder row: got %q, want omlx", got)
	}
	if got := runtimeCell(t, m, "qwen-st"); got != "vllm" {
		t.Errorf("row outside oMLX's folders: got %q, want vllm", got)
	}

	for _, row := range []models.ModelFile{omlx, hf} {
		m = selectRow(t, m, row)
		m = press(t, m, keyText("R"))
		if got := lastSpec(t, f); got.backend != row.Backend {
			t.Errorf("%s launched on %v, want %v", row.Path, got.backend, row.Backend)
		}
	}

	// Choosing vLLM and then (none) again returns the row to oMLX.
	m = selectRow(t, m, omlx)
	m = press(t, m, keyText("p"), keyTab, keyRight, keySpace, keyLeft, keySpace, keyEsc)
	if got := runtimeCell(t, m, "qwen-mlx"); got != "omlx" {
		t.Errorf("(none) should return the row to oMLX, got %q", got)
	}
}

// A profile naming a Runtime the row may not use (from an import, say) is
// ignored: the row keeps the Runtime discovery gave it, in the table and at
// launch. GGUF rows follow the same rule.
func TestSafetensorsRuntime_unusableProfileBackendFallsBack(t *testing.T) {
	dir := useTempConfigDir(t)
	omlx, hf := omlxRow(dir), hfRow(dir)
	gguf := testRow(models.BackendLlama, filepath.Join(dir, "gemma.gguf"))
	saveProfiles(t, hf.Path, profiles.Entry{Profiles: []profiles.Profile{{Name: "imported", Backend: "omlx"}}})
	saveProfiles(t, omlx.Path, profiles.Entry{Profiles: []profiles.Profile{{Name: "imported", Backend: "koboldcpp"}}})
	saveProfiles(t, gguf.Path, profiles.Entry{Profiles: []profiles.Profile{{Name: "imported", Backend: "vllm"}}})

	f := newLaunchFakes()
	m := dimModel(t, f.services, macPlatform, config.RuntimeStates{}, omlx, hf, gguf)
	for _, tc := range []struct {
		row  models.ModelFile
		id   string
		cell string
	}{
		{omlx, "qwen-mlx", "omlx"},
		{hf, "qwen-st", "vllm"},
		{gguf, "gemma", "llama.cpp"},
	} {
		if got := runtimeCell(t, m, tc.id); got != tc.cell {
			t.Errorf("%s: Runtime column %q, want %q", tc.id, got, tc.cell)
		}
		m = selectRow(t, m, tc.row)
		m = press(t, m, keyText("R"))
		if got := lastSpec(t, f); got.backend != tc.row.Backend {
			t.Errorf("%s launched on %v, want %v", tc.id, got.backend, tc.row.Backend)
		}
	}
}

// A Safetensors row whose chosen Runtime is off is dimmed with (off) and
// refused at launch; the Runtime discovery gave it no longer matters.
func TestSafetensorsRuntime_chosenRuntimeOffDimsAndBlocks(t *testing.T) {
	dir := useTempConfigDir(t)
	row := omlxRow(dir)
	saveProfiles(t, row.Path, profiles.Entry{Profiles: []profiles.Profile{{Name: "vllm", Backend: "vllm"}}})

	f := newLaunchFakes()
	vllmOff := config.RuntimeStates{}.With(models.BackendVLLM, false)
	m := dimModel(t, f.services, macPlatform, vllmOff, row, hfRow(dir))
	m = selectRow(t, m, hfRow(dir)) // keep the checked row unselected
	if !rowShowsDimmed(t, m, "qwen-mlx", "vllm") {
		t.Fatalf("a row on the Disabled vLLM should be dimmed:\n%s", plainView(m))
	}
	m = selectRow(t, m, row)
	m = press(t, m, keyText("R"))
	if len(f.launches) != 0 || !strings.Contains(lastAlert(t, m).message, "vLLM") {
		t.Fatalf("launch on the Disabled vLLM should be blocked (launches %v)", f.launches)
	}

	omlxOff := config.RuntimeStates{}.With(models.BackendOMLX, false)
	m = dimModel(t, f.services, macPlatform, omlxOff, row, hfRow(dir))
	m = selectRow(t, m, hfRow(dir))
	if rowShowsDimmed(t, m, "qwen-mlx", "vllm") {
		t.Fatalf("with oMLX off, a row switched to vLLM should not dim:\n%s", plainView(m))
	}
	m = selectRow(t, m, row)
	press(t, m, keyText("R"))
	if len(f.launches) != 1 || f.launches[0] != models.BackendVLLM {
		t.Errorf("the row should launch on vLLM, launches %v", f.launches)
	}
}

// With no override an oMLX-folder row runs on oMLX, so the p panel labels
// (none) (off) along with omlx while oMLX is off.
func TestSafetensorsRuntime_noneOptionFollowsDiscoveryRuntime(t *testing.T) {
	dir := useTempConfigDir(t)
	off := config.RuntimeStates{}.With(models.BackendOMLX, false)
	m := dimModel(t, newLaunchFakes().services, macPlatform, off, omlxRow(dir))
	line := paramBackendLine(t, press(t, m, keyText("p"), keyTab))
	if !strings.Contains(line, "(none) (off)") || !strings.Contains(line, "omlx (off)") {
		t.Errorf("(none) and omlx should both be labelled (off): %q", line)
	}
	if strings.Contains(line, "vllm (off)") {
		t.Errorf("vLLM is on: %q", line)
	}
}

// selectRow moves the table cursor to row.
func selectRow(t *testing.T, m Model, row models.ModelFile) Model {
	t.Helper()
	for i, f := range m.table.files {
		if f.Identity() == row.Identity() {
			m.table.tbl.SetCursor(i)
			return m.withLaunchPreviewSynced()
		}
	}
	t.Fatalf("no row %s", row.Path)
	return m
}

// Importing a profile that picks vLLM for a model in oMLX's folder switches
// the row at once: the Runtime column and R follow it without a rescan.
func TestSafetensorsRuntime_importedProfileChoosesRuntime(t *testing.T) {
	dir := useTempConfigDir(t)
	row := omlxRow(dir)
	file := filepath.Join(dir, "qwen.toml")
	toml := "schema_version = 3\n\n[[profiles]]\nname = \"on-vllm\"\nbackend = \"vllm\"\nmodel_hint = \"qwen-mlx\"\n"
	if err := os.WriteFile(file, []byte(toml), 0o600); err != nil {
		t.Fatal(err)
	}

	f := newLaunchFakes()
	m := dimModel(t, f.services, macPlatform, config.RuntimeStates{}, row)
	m = press(t, m, keyText("I"), keyTab) // from the file picker to the path input
	for _, r := range file {
		m = press(t, m, keyText(string(r)))
	}
	m = press(t, m, keyEnter, keyEnter) // parse, then import
	if m.importView.open {
		t.Fatalf("the import should have finished:\n%s", plainView(m))
	}
	if got := runtimeCell(t, m, "qwen-mlx"); got != "vllm" {
		t.Fatalf("the imported vLLM profile should switch the row, got %q:\n%s", got, plainView(m))
	}
	press(t, m, keyText("R"))
	if got := lastSpec(t, f); got.backend != models.BackendVLLM {
		t.Errorf("launched on %v, want vLLM", got.backend)
	}
}

// The missing-Runtime footer follows the Runtime each row launches with: a
// row switched off its discovery Runtime no longer counts toward it, and a
// row with no override still does.
func TestMissingRuntimeFooter_followsChosenRuntime(t *testing.T) {
	dir := useTempConfigDir(t)
	omlx := omlxRow(dir)
	gguf := testRow(models.BackendLlama, filepath.Join(dir, "gemma.gguf"))
	saveProfiles(t, omlx.Path, profiles.Entry{Profiles: []profiles.Profile{{Name: "vllm", Backend: "vllm"}}})
	saveProfiles(t, gguf.Path, profiles.Entry{Profiles: []profiles.Profile{{Name: "kobold", Backend: "koboldcpp"}}})

	m := dimModel(t, newLaunchFakes().services, macPlatform, config.RuntimeStates{}, omlx, gguf)
	m.runtime.OMLXPath, m.runtime.LlamaServerPath, m.runtime.ServerRunning = "", "", false
	m, _ = m.maybeSetMissingRuntimeFooterNote()
	if m.lastRunNote != "" {
		t.Errorf("no row launches with oMLX or llama.cpp, yet the footer says %q", m.lastRunNote)
	}

	m = dimModel(t, newLaunchFakes().services, macPlatform, config.RuntimeStates{}, omlxRow(dir), hfRow(dir))
	m.runtime.VLLMPath = ""
	m, _ = m.maybeSetMissingRuntimeFooterNote()
	if !strings.Contains(m.lastRunNote, MissingVLLMFooterNote) {
		t.Errorf("a row with no override on the missing vLLM should be reported, note %q", m.lastRunNote)
	}
}
