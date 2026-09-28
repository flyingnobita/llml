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

// mlxVLMRow is a Safetensors model folder, which exists on disk, outside
// oMLX's model folders: discovery gives it vLLM.
func mlxVLMRow(t *testing.T, dir string) models.ModelFile {
	t.Helper()
	p := filepath.Join(dir, "hf", "qwen-vl")
	if err := os.MkdirAll(p, 0o750); err != nil {
		t.Fatal(err)
	}
	return testRow(models.BackendVLLM, p)
}

// saveMLXVLMProfile gives row an Active Profile that chooses mlx-vlm.
func saveMLXVLMProfile(t *testing.T, row models.ModelFile) {
	t.Helper()
	saveProfiles(t, row.Path, profiles.Entry{Profiles: []profiles.Profile{{Name: "vlm", Backend: "mlx-vlm"}}})
}

// Choosing mlx-vlm in the p panel switches a Safetensors row to it, and R and
// ctrl+R start mlx_vlm.server on the model's absolute path, the configured
// host and port, and nothing else.
func TestMLXVLM_profileChoiceLaunchesMLXVLMServer(t *testing.T) {
	dir := useTempConfigDir(t)
	row := mlxVLMRow(t, dir)

	f := newLaunchFakes()
	m := dimModel(t, f.services, linuxPlatform, config.RuntimeStates{}, row)
	m = press(t, m, keyText("p"), keyTab)
	if line := paramBackendLine(t, m); !strings.Contains(line, "mlx-vlm") {
		t.Fatalf("a Safetensors row should be offered mlx-vlm: %q", line)
	}
	m = press(t, m, keyRight, keyRight, keyRight, keySpace, keyEsc) // (none) -> vllm -> mlx-lm -> mlx-vlm
	if got := runtimeCell(t, m, "qwen-vl"); got != "mlx-vlm" {
		t.Fatalf("the Runtime column should show the profile's mlx-vlm, got %q:\n%s", got, plainView(m))
	}

	press(t, press(t, m, keyText("R")), keyCtrlR)
	if len(f.specs) != 2 {
		t.Fatalf("R and ctrl+R should each launch once, got %d", len(f.specs))
	}
	want := []string{"--model", row.Path, "--host", "127.0.0.1", "--port", "8080"}
	for _, spec := range f.specs {
		if spec.backend != models.BackendMLXVLM || spec.bin != panelRuntime(linuxPlatform).MLXVLMPath {
			t.Errorf("launched %s on %v, want mlx_vlm.server", spec.bin, spec.backend)
		}
		if got := spec.directArgs(); !slices.Equal(got, want) {
			t.Errorf("argv = %q, want %q", got, want)
		}
	}
}

// --host is always passed, because mlx_vlm.server otherwise listens on all
// interfaces: with no host resolved, llml passes the loopback default. The
// profile's args follow llml's, and its env reaches the server.
func TestMLXVLM_alwaysPassesHost(t *testing.T) {
	dir := useTempConfigDir(t)
	row := mlxVLMRow(t, dir)
	saveProfiles(t, row.Path, profiles.Entry{Profiles: []profiles.Profile{{
		Name: "vlm", Backend: "mlx-vlm",
		Args: []string{"--max-tokens", "4096"},
		Env:  []profiles.EnvVar{{Key: "MLX_METAL_DEBUG", Value: "1"}},
	}}})

	for _, tc := range []struct {
		host     string
		port     int
		wantHost string
		wantPort string
	}{
		{"", 0, "127.0.0.1", "8080"},
		{"0.0.0.0", 9292, "0.0.0.0", "9292"},
	} {
		f := newLaunchFakes()
		m := dimModel(t, f.services, macPlatform, config.RuntimeStates{}, row)
		m.runtime.MLXVLMHost, m.runtime.MLXVLMPort = tc.host, tc.port
		press(t, m, keyText("R"))

		spec := lastSpec(t, f)
		want := []string{"--model", row.Path, "--host", tc.wantHost, "--port", tc.wantPort, "--max-tokens", "4096"}
		if got := spec.directArgs(); !slices.Equal(got, want) {
			t.Errorf("host %q: argv = %q, want %q", tc.host, got, want)
		}
		if !slices.Contains(spec.splitCmd().Env, "MLX_METAL_DEBUG=1") {
			t.Error("the profile's env should reach the server")
		}
	}
}

// The launch preview names the model id clients must send, the same absolute
// path, and the copied command carries no such line.
func TestMLXVLM_previewShowsModelID(t *testing.T) {
	dir := useTempConfigDir(t)
	row := mlxVLMRow(t, dir)
	saveMLXVLMProfile(t, row)

	f := newLaunchFakes()
	m := dimModel(t, f.services, linuxPlatform, config.RuntimeStates{}, row)
	m.layout.width = 240 // no wrapping
	m = m.layoutTable()
	scrolled := plainView(press(t, m, keyTab, keyDown, keyDown, keyDown, keyDown))
	if want := launchPreviewModelIDLabel + row.Path; !strings.Contains(scrolled, want) {
		t.Errorf("preview should show %q:\n%s", want, scrolled)
	}
	if !strings.Contains(scrolled, "--host 127.0.0.1") {
		t.Errorf("the preview command should pass --host:\n%s", scrolled)
	}

	press(t, m, keyTab, keyEnter) // focus the launch preview, then copy
	if len(f.clipboard) != 1 {
		t.Fatalf("want one copy, got %q", f.clipboard)
	}
	if copied := f.clipboard[0]; strings.Contains(copied, launchPreviewModelIDLabel) || !strings.Contains(copied, "mlx_vlm.server") {
		t.Errorf("the copy should be the command alone: %q", copied)
	}
}

// With mlx-vlm off, a row whose profile chooses it is dimmed with (off) and R
// starts nothing, with a warning naming mlx-vlm. Turning mlx-lm off does not
// touch it.
func TestMLXVLM_offDimsAndBlocks(t *testing.T) {
	dir := useTempConfigDir(t)
	row := mlxVLMRow(t, dir)
	other := testRow(models.BackendLlama, filepath.Join(dir, "gemma.gguf"))
	saveMLXVLMProfile(t, row)

	f := newLaunchFakes()
	mlxLMOff := config.RuntimeStates{}.With(models.BackendMLXLM, false)
	m := dimModel(t, f.services, linuxPlatform, mlxLMOff, row, other)
	m = selectRow(t, m, other)
	if rowShowsDimmed(t, m, "qwen-vl", "mlx-vlm") {
		t.Fatalf("mlx-lm being off should not dim an mlx-vlm row:\n%s", plainView(m))
	}

	off := config.RuntimeStates{}.With(models.BackendMLXVLM, false)
	m = dimModel(t, f.services, linuxPlatform, off, row, other)
	m = selectRow(t, m, other) // keep the checked row unselected
	if !rowShowsDimmed(t, m, "qwen-vl", "mlx-vlm") {
		t.Fatalf("a row on the Disabled mlx-vlm should be dimmed:\n%s", plainView(m))
	}
	m = selectRow(t, m, row)
	m = press(t, m, keyText("R"), keyCtrlR)
	if len(f.launches) != 0 {
		t.Fatalf("launch on the Disabled mlx-vlm should be blocked, launches %v", f.launches)
	}
	if a := lastAlert(t, m); a.severity != alertSeverityWarn || !strings.Contains(a.message, "mlx-vlm is off") {
		t.Errorf("want a warning naming mlx-vlm, got %+v", a)
	}
}

// A model folder removed since the scan is never passed to mlx_vlm.server,
// which would take the path for a Hugging Face repo id and download it. The
// preview says so, since the command can still be copied.
func TestMLXVLM_missingModelFolderIsRefused(t *testing.T) {
	dir := useTempConfigDir(t)
	row := mlxVLMRow(t, dir)
	saveMLXVLMProfile(t, row)

	f := newLaunchFakes()
	m := dimModel(t, f.services, linuxPlatform, config.RuntimeStates{}, row)
	if err := os.RemoveAll(row.Path); err != nil {
		t.Fatal(err)
	}
	m = m.withLaunchPreviewSynced()
	m.layout.width = 240
	scrolled := plainView(press(t, m.layoutTable(), keyTab, keyDown, keyDown, keyDown, keyDown, keyDown))
	if !strings.Contains(scrolled, "model folder not found - mlx-vlm would download") {
		t.Errorf("the preview should warn that the folder is gone:\n%s", scrolled)
	}
	m = press(t, m, keyText("R"), keyCtrlR)
	if len(f.launches) != 0 {
		t.Fatalf("a missing model folder should block the launch, launches %v", f.launches)
	}
	if a := lastAlert(t, m); a.severity != alertSeverityWarn || !strings.Contains(a.message, row.Path) || !strings.Contains(a.message, "mlx-vlm") {
		t.Errorf("want a warning naming the missing folder and mlx-vlm, got %+v", a)
	}
}

// Image and audio tags only opt llama.cpp and KoboldCpp into --mmproj;
// mlx-vlm reads its own projector from the model folder, so llml adds none.
func TestMLXVLM_imageTagAddsNoMMProj(t *testing.T) {
	dir := useTempConfigDir(t)
	row := mlxVLMRow(t, dir)
	if err := os.WriteFile(filepath.Join(row.Path, "mmproj-model.gguf"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	saveProfiles(t, row.Path, profiles.Entry{Profiles: []profiles.Profile{{
		Name: "vlm", Backend: "mlx-vlm", UseCase: profiles.UseCaseMetadata{Tags: []string{"image", "audio"}},
	}}})

	f := newLaunchFakes()
	m := dimModel(t, f.services, linuxPlatform, config.RuntimeStates{}, row)
	press(t, m, keyText("R"))
	spec := lastSpec(t, f)
	if spec.mmprojPath != "" || spec.mmprojMissing || len(spec.mmprojCandidates) > 0 || spec.mmprojNote() != "" {
		t.Errorf("mlx-vlm should get no mmproj handling: %+v", spec)
	}
	if slices.Contains(spec.directArgs(), "--mmproj") {
		t.Errorf("argv = %q, want no --mmproj", spec.directArgs())
	}
}

// With mlx-vlm on and chosen by a row's profile but its program found
// nowhere, the footer names mlx_vlm.server, and R says so instead of
// launching.
func TestMLXVLM_missingProgram(t *testing.T) {
	dir := useTempConfigDir(t)
	t.Setenv("PATH", t.TempDir())
	row := mlxVLMRow(t, dir)
	saveMLXVLMProfile(t, row)

	f := newLaunchFakes()
	m := dimModel(t, f.services, linuxPlatform, config.RuntimeStates{}, row)
	m.runtime.MLXVLMPath = ""
	m, _ = m.maybeSetMissingRuntimeFooterNote()
	if !strings.Contains(m.lastRunNote, MissingMLXVLMFooterNote) {
		t.Errorf("footer should name the missing mlx_vlm.server, note %q", m.lastRunNote)
	}
	m = press(t, m, keyText("R"))
	if len(f.launches) != 0 || !strings.Contains(m.lastRunNote, MissingMLXVLMFooterNote) {
		t.Errorf("R without mlx_vlm.server should not launch (launches %v, note %q)", f.launches, m.lastRunNote)
	}

	m.runtimeStates = config.RuntimeStates{}.With(models.BackendMLXVLM, false)
	m, _ = m.maybeSetMissingRuntimeFooterNote()
	if strings.Contains(m.lastRunNote, MissingMLXVLMFooterNote) {
		t.Errorf("a Disabled mlx-vlm should not be reported missing, note %q", m.lastRunNote)
	}
}

// mlx-vlm is offered to Safetensors rows where MLX runs, as mlx-lm is, and
// never to a GGUF row.
func TestMLXVLM_offeredWhereMLXRuns(t *testing.T) {
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
			m := dimModel(t, newLaunchFakes().services, tc.p, config.RuntimeStates{}, mlxVLMRow(t, dir), gguf)
			m = selectRow(t, m, mlxVLMRow(t, dir))
			m = press(t, m, keyText("p"))
			if got := slices.Contains(m.paramBackendOptionsForModel(), "mlx-vlm"); got != tc.want {
				t.Errorf("mlx-vlm offered = %t, want %t", got, tc.want)
			}
			m = press(t, m, keyEsc)
			m = selectRow(t, m, gguf)
			m = press(t, m, keyText("p"))
			if slices.Contains(m.paramBackendOptionsForModel(), "mlx-vlm") {
				t.Error("mlx-vlm offered to a GGUF row")
			}
		})
	}
}
