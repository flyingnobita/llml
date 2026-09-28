package tui

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/flyingnobita/llml/internal/config"
	"github.com/flyingnobita/llml/internal/models"
	"github.com/flyingnobita/llml/internal/profiles"
)

// mlxLMRow is a Safetensors model folder, which exists on disk, outside
// oMLX's model folders: discovery gives it vLLM.
func mlxLMRow(t *testing.T, dir string) models.ModelFile {
	t.Helper()
	p := filepath.Join(dir, "hf", "qwen-st")
	if err := os.MkdirAll(p, 0o750); err != nil {
		t.Fatal(err)
	}
	return testRow(models.BackendVLLM, p)
}

// Choosing mlx-lm in the p panel switches a Safetensors row to it, and R and
// ctrl+R start mlx_lm.server on the model's absolute path, the configured host
// and port, then the profile's own args, and nothing else.
func TestMLXLM_profileChoiceLaunchesMLXLMServer(t *testing.T) {
	dir := useTempConfigDir(t)
	row := mlxLMRow(t, dir)

	f := newLaunchFakes()
	m := dimModel(t, f.services, linuxPlatform, config.RuntimeStates{}, row)
	m = press(t, m, keyText("p"), keyTab)
	if line := paramBackendLine(t, m); !strings.Contains(line, "mlx-lm") {
		t.Fatalf("a Safetensors row should be offered mlx-lm: %q", line)
	}
	m = press(t, m, keyRight, keyRight, keySpace, keyEsc) // (none) -> vllm -> mlx-lm
	if got := runtimeCell(t, m, "qwen-st"); got != "mlx-lm" {
		t.Fatalf("the Runtime column should show the profile's mlx-lm, got %q:\n%s", got, plainView(m))
	}

	press(t, press(t, m, keyText("R")), keyCtrlR)
	if len(f.specs) != 2 {
		t.Fatalf("R and ctrl+R should each launch once, got %d", len(f.specs))
	}
	want := []string{"--model", row.Path, "--host", "127.0.0.1", "--port", "8080"}
	for _, spec := range f.specs {
		if spec.backend != models.BackendMLXLM || spec.bin != panelRuntime(linuxPlatform).MLXLMPath {
			t.Errorf("launched %s on %v, want mlx_lm.server", spec.bin, spec.backend)
		}
		if got := spec.directArgs(); !slices.Equal(got, want) {
			t.Errorf("argv = %q, want %q", got, want)
		}
	}
}

// The profile's args follow llml's, its env reaches the server, and the host
// and port come from the resolved runtime.
func TestMLXLM_profileArgsAndEnvPassThrough(t *testing.T) {
	dir := useTempConfigDir(t)
	row := mlxLMRow(t, dir)
	saveProfiles(t, row.Path, profiles.Entry{Profiles: []profiles.Profile{{
		Name: "long", Backend: "mlx-lm",
		Args: []string{"--max-tokens", "4096"},
		Env:  []profiles.EnvVar{{Key: "MLX_METAL_DEBUG", Value: "1"}},
	}}})

	f := newLaunchFakes()
	m := dimModel(t, f.services, linuxPlatform, config.RuntimeStates{}, row)
	m.runtime.MLXLMHost, m.runtime.MLXLMPort = "0.0.0.0", 9191
	press(t, m, keyText("R"))

	spec := lastSpec(t, f)
	want := []string{"--model", row.Path, "--host", "0.0.0.0", "--port", "9191", "--max-tokens", "4096"}
	if got := spec.directArgs(); !slices.Equal(got, want) {
		t.Errorf("argv = %q, want %q", got, want)
	}
	if !slices.Contains(spec.splitCmd().Env, "MLX_METAL_DEBUG=1") {
		t.Error("the profile's env should reach the server")
	}
}

// A model path that is not absolute is made absolute: mlx_lm.server would
// take anything else for a Hugging Face repo id.
func TestMLXLM_modelPathIsAbsolute(t *testing.T) {
	dir := useTempConfigDir(t)
	t.Chdir(dir)
	mlxLMRow(t, dir)
	row := testRow(models.BackendVLLM, filepath.Join("hf", "qwen-st"))
	saveProfiles(t, row.Path, profiles.Entry{Profiles: []profiles.Profile{{Name: "mlx", Backend: "mlx-lm"}}})
	abs, err := filepath.Abs(row.Path)
	if err != nil {
		t.Fatal(err)
	}

	f := newLaunchFakes()
	m := dimModel(t, f.services, linuxPlatform, config.RuntimeStates{}, row)
	press(t, m, keyText("R"))
	if got := lastSpec(t, f).directArgs(); len(got) < 2 || got[1] != abs {
		t.Errorf("--model = %q, want %s", got, abs)
	}
}

// The launch preview names the model id clients must send, the same absolute
// path, on a line of its own under the command. The copied command matches
// the command in the preview and carries no such line.
func TestMLXLM_previewShowsModelIDAndCopiesCommand(t *testing.T) {
	dir := useTempConfigDir(t)
	row := mlxLMRow(t, dir)
	saveProfiles(t, row.Path, profiles.Entry{Profiles: []profiles.Profile{{Name: "mlx", Backend: "mlx-lm"}}})

	f := newLaunchFakes()
	m := dimModel(t, f.services, linuxPlatform, config.RuntimeStates{}, row)
	m.layout.width = 240 // no wrapping
	m = m.layoutTable()
	top := plainView(m)
	// Focus the launch preview and scroll to its end.
	scrolled := plainView(press(t, m, keyTab, keyDown, keyDown, keyDown, keyDown))
	want := launchPreviewModelIDLabel + row.Path
	if !strings.Contains(scrolled, want) {
		t.Errorf("preview should show %q:\n%s", want, scrolled)
	}
	view := top + scrolled

	press(t, m, keyTab, keyEnter) // focus the launch preview, then copy
	if len(f.clipboard) != 1 {
		t.Fatalf("want one copy, got %q", f.clipboard)
	}
	copied := f.clipboard[0]
	if strings.Contains(copied, launchPreviewModelIDLabel) || !strings.Contains(copied, "mlx_lm.server") {
		t.Errorf("the copy should be the command alone: %q", copied)
	}
	for _, line := range strings.Split(copied, "\n") {
		if !strings.Contains(view, strings.TrimSpace(line)) {
			t.Errorf("copied line %q is not in the preview:\n%s", line, view)
		}
	}
}

// Other Runtimes' previews have no model-id line.
func TestMLXLM_previewModelIDOnlyForMLXLM(t *testing.T) {
	dir := useTempConfigDir(t)
	m := dimModel(t, newLaunchFakes().services, linuxPlatform, config.RuntimeStates{}, mlxLMRow(t, dir))
	m.layout.width = 240
	m = press(t, m.layoutTable(), keyTab, keyDown, keyDown, keyDown, keyDown)
	if view := plainView(m); strings.Contains(view, launchPreviewModelIDLabel) || !strings.Contains(view, "--port 8000") {
		t.Errorf("a vLLM row should show its whole command and no model-id line:\n%s", view)
	}
}

// With mlx-lm off, a row whose profile chooses it is dimmed with (off) and R
// starts nothing, with a warning naming mlx-lm.
func TestMLXLM_offDimsAndBlocks(t *testing.T) {
	dir := useTempConfigDir(t)
	row := mlxLMRow(t, dir)
	other := testRow(models.BackendLlama, filepath.Join(dir, "gemma.gguf"))
	saveProfiles(t, row.Path, profiles.Entry{Profiles: []profiles.Profile{{Name: "mlx", Backend: "mlx-lm"}}})

	f := newLaunchFakes()
	off := config.RuntimeStates{}.With(models.BackendMLXLM, false)
	m := dimModel(t, f.services, linuxPlatform, off, row, other)
	m = selectRow(t, m, other) // keep the checked row unselected
	if !rowShowsDimmed(t, m, "qwen-st", "mlx-lm") {
		t.Fatalf("a row on the Disabled mlx-lm should be dimmed:\n%s", plainView(m))
	}
	m = selectRow(t, m, row)
	m = press(t, m, keyText("R"), keyCtrlR)
	if len(f.launches) != 0 {
		t.Fatalf("launch on the Disabled mlx-lm should be blocked, launches %v", f.launches)
	}
	if a := lastAlert(t, m); a.severity != alertSeverityWarn || !strings.Contains(a.message, "mlx-lm is off") {
		t.Errorf("want a warning naming mlx-lm, got %+v", a)
	}
}

// A model folder removed since the scan is never passed to mlx_lm.server,
// which would take the path for a Hugging Face repo id and download it.
func TestMLXLM_missingModelFolderIsRefused(t *testing.T) {
	dir := useTempConfigDir(t)
	row := mlxLMRow(t, dir)
	saveProfiles(t, row.Path, profiles.Entry{Profiles: []profiles.Profile{{Name: "mlx", Backend: "mlx-lm"}}})

	f := newLaunchFakes()
	m := dimModel(t, f.services, linuxPlatform, config.RuntimeStates{}, row)
	if err := os.RemoveAll(row.Path); err != nil {
		t.Fatal(err)
	}
	m = m.withLaunchPreviewSynced()
	m.layout.width = 240
	scrolled := plainView(press(t, m.layoutTable(), keyTab, keyDown, keyDown, keyDown, keyDown, keyDown))
	if !strings.Contains(scrolled, "model folder not found - mlx-lm would download") {
		t.Errorf("the preview should warn that the folder is gone, since the command can be copied:\n%s", scrolled)
	}
	m = press(t, m, keyText("R"), keyCtrlR)
	if len(f.launches) != 0 {
		t.Fatalf("a missing model folder should block the launch, launches %v", f.launches)
	}
	a := lastAlert(t, m)
	if a.severity != alertSeverityWarn || !strings.Contains(a.message, row.Path) {
		t.Errorf("want a warning naming the missing folder, got %+v", a)
	}

	// A file where the folder was is not a model folder either.
	if err := os.WriteFile(row.Path, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	press(t, m, keyText("R"))
	if len(f.launches) != 0 {
		t.Errorf("a file is not a model folder, launches %v", f.launches)
	}
}

// Image and audio tags only opt llama.cpp and KoboldCpp into --mmproj;
// mlx-lm gets no projector and no multimodal warning.
func TestMLXLM_imageTagAddsNoMMProj(t *testing.T) {
	dir := useTempConfigDir(t)
	row := mlxLMRow(t, dir)
	if err := os.WriteFile(filepath.Join(row.Path, "mmproj-model.gguf"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	saveProfiles(t, row.Path, profiles.Entry{Profiles: []profiles.Profile{{
		Name: "mlx", Backend: "mlx-lm", UseCase: profiles.UseCaseMetadata{Tags: []string{"image", "audio"}},
	}}})

	f := newLaunchFakes()
	m := dimModel(t, f.services, linuxPlatform, config.RuntimeStates{}, row)
	press(t, m, keyText("R"))
	spec := lastSpec(t, f)
	if spec.mmprojPath != "" || spec.mmprojMissing || len(spec.mmprojCandidates) > 0 || spec.mmprojNote() != "" {
		t.Errorf("mlx-lm should get no mmproj handling: %+v", spec)
	}
	if slices.Contains(spec.directArgs(), "--mmproj") {
		t.Errorf("argv = %q, want no --mmproj", spec.directArgs())
	}
}

// With mlx-lm on and chosen by a row's profile but its program found nowhere,
// the footer names mlx_lm.server, and R says so instead of launching.
func TestMLXLM_missingProgram(t *testing.T) {
	dir := useTempConfigDir(t)
	t.Setenv("PATH", t.TempDir())
	row := mlxLMRow(t, dir)
	saveProfiles(t, row.Path, profiles.Entry{Profiles: []profiles.Profile{{Name: "mlx", Backend: "mlx-lm"}}})

	f := newLaunchFakes()
	m := dimModel(t, f.services, linuxPlatform, config.RuntimeStates{}, row)
	m.runtime.MLXLMPath = ""
	m, _ = m.maybeSetMissingRuntimeFooterNote()
	if !strings.Contains(m.lastRunNote, MissingMLXLMFooterNote) {
		t.Errorf("footer should name the missing mlx_lm.server, note %q", m.lastRunNote)
	}
	m = press(t, m, keyText("R"))
	if len(f.launches) != 0 || !strings.Contains(m.lastRunNote, MissingMLXLMFooterNote) {
		t.Errorf("R without mlx_lm.server should not launch (launches %v, note %q)", f.launches, m.lastRunNote)
	}

	m.runtimeStates = config.RuntimeStates{}.With(models.BackendMLXLM, false)
	m, _ = m.maybeSetMissingRuntimeFooterNote()
	if strings.Contains(m.lastRunNote, MissingMLXLMFooterNote) {
		t.Errorf("a Disabled mlx-lm should not be reported missing, note %q", m.lastRunNote)
	}
}

// mlx-lm is offered to Safetensors rows on Apple Silicon and Linux, and not
// where MLX does not run. It is never offered to a GGUF row.
func TestMLXLM_offeredWhereMLXRuns(t *testing.T) {
	for _, tc := range []struct {
		p    models.Platform
		want bool
	}{
		{macPlatform, true},
		{linuxPlatform, true},
		{models.Platform{GOOS: "linux", GOARCH: "arm64"}, true},
		{models.Platform{GOOS: "darwin", GOARCH: "amd64"}, false},
		{models.Platform{GOOS: "windows", GOARCH: "amd64"}, false},
	} {
		t.Run(tc.p.GOOS+"/"+tc.p.GOARCH, func(t *testing.T) {
			dir := useTempConfigDir(t)
			gguf := testRow(models.BackendLlama, filepath.Join(dir, "gemma.gguf"))
			m := dimModel(t, newLaunchFakes().services, tc.p, config.RuntimeStates{}, mlxLMRow(t, dir), gguf)
			m = selectRow(t, m, mlxLMRow(t, dir))
			m = press(t, m, keyText("p"))
			if got := slices.Contains(m.paramBackendOptionsForModel(), "mlx-lm"); got != tc.want {
				t.Errorf("mlx-lm offered = %t, want %t", got, tc.want)
			}
			m = press(t, m, keyEsc)
			m = selectRow(t, m, gguf)
			m = press(t, m, keyText("p"))
			if slices.Contains(m.paramBackendOptionsForModel(), "mlx-lm") {
				t.Error("mlx-lm offered to a GGUF row")
			}
		})
	}
}
