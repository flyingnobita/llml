package tui

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/flyingnobita/llml/internal/models"
)

// The panel renders purely from RuntimeInfo, which carries the resolved hosts
// and ports, so this test needs no environment at all.
func TestRuntimePanelLines(t *testing.T) {
	t.Parallel()

	r := models.RuntimeInfo{
		LlamaServerPath:  "/home/u/llama.cpp/bin/llama-server",
		LlamaServerHost:  "127.0.0.1",
		LlamaServerPort:  8080,
		VLLMPath:         "/home/u/.local/bin/vllm",
		VLLMServerHost:   "127.0.0.1",
		VLLMServerPort:   8000,
		OllamaPath:       "/home/u/.local/bin/ollama",
		OllamaHost:       "127.0.0.1:11434",
		KoboldCppPort:    5001,
		NInferPath:       "/home/u/ninfer/build/apps/ninfer-serve",
		NInferServerHost: "127.0.0.1",
		NInferPort:       18080,
		Platform:         models.Platform{GOOS: "linux", GOARCH: "amd64"},
		ServerRunning:    false,
		ProbePort:        8080,
	}
	lines := RuntimePanelLines(80, r)
	// Rows are sorted alphabetically by label.
	want := []struct{ label, value string }{
		{runtimePanelLabelKoboldCppPath, ""},
		{runtimePanelLabelKoboldCppPort, "5001"},
		{runtimePanelLabelLlamaServerPath, "llama-server"},
		{runtimePanelLabelLlamaServerHost, "127.0.0.1"},
		{runtimePanelLabelLlamaServerPort, "8080"},
		{runtimePanelLabelNInferHost, "127.0.0.1"},
		{runtimePanelLabelNInferPath, "ninfer-serve"},
		{runtimePanelLabelNInferPort, "18080"},
		{runtimePanelLabelOllamaHost, "127.0.0.1:11434"},
		{runtimePanelLabelOllamaPath, "ollama"},
		{runtimePanelLabelVLLMHost, "127.0.0.1"},
		{runtimePanelLabelVLLMPath, "vllm"},
		{runtimePanelLabelVLLMPort, "8000"},
		{runtimePanelLabelVLLMVenv, "—"},
	}
	if len(lines) != len(want) {
		t.Fatalf("got %d lines, want %d", len(lines), len(want))
	}
	for i, w := range want {
		if !strings.HasPrefix(lines[i], w.label+" ") || !strings.Contains(lines[i], w.value) {
			t.Errorf("line %d = %q, want label %q with value %q", i, lines[i], w.label, w.value)
		}
	}
}

// On Apple Silicon the panel shows oMLX and Splash and hides NInfer, which
// needs CUDA; the reverse holds on Linux.
func TestRuntimePanelLines_platformGating(t *testing.T) {
	t.Parallel()

	r := models.RuntimeInfo{
		OMLXPath:   "/Users/u/.omlx/bin/omlx",
		OMLXHost:   "127.0.0.1",
		OMLXPort:   8000,
		SplashPath: "/opt/homebrew/bin/splash",
		SplashHost: "127.0.0.1",
		SplashPort: 8000,
		NInferPort: 18080,
	}
	has := func(lines []string, label string) bool {
		for _, l := range lines {
			if strings.HasPrefix(l, label+" ") {
				return true
			}
		}
		return false
	}

	r.Platform = models.Platform{GOOS: "darwin", GOARCH: "arm64"}
	mac := RuntimePanelLines(120, r)
	for _, label := range []string{runtimePanelLabelOMLXPath, runtimePanelLabelOMLXPort, runtimePanelLabelSplashPath, runtimePanelLabelSplashHost} {
		if !has(mac, label) {
			t.Errorf("darwin/arm64: missing %q in %q", label, mac)
		}
	}
	if has(mac, runtimePanelLabelNInferPath) {
		t.Errorf("darwin/arm64 should hide NInfer: %q", mac)
	}

	r.Platform = models.Platform{GOOS: "linux", GOARCH: "amd64"}
	linux := RuntimePanelLines(120, r)
	if has(linux, runtimePanelLabelOMLXPath) || has(linux, runtimePanelLabelSplashPath) {
		t.Errorf("linux should hide oMLX and Splash: %q", linux)
	}
	if !has(linux, runtimePanelLabelNInferPath) {
		t.Errorf("linux should show NInfer: %q", linux)
	}
}

func TestRuntimePanelLines_ServerRunningNoBinary(t *testing.T) {
	t.Setenv("PATH", t.TempDir()) // ResolveLlamaServerPath must not find llama-server via LookPath
	r := models.RuntimeInfo{
		LlamaServerPath: "",
		ServerRunning:   true,
		ProbePort:       8080,
	}
	lines := RuntimePanelLines(120, r)
	want := "(server at :8080)"
	found := false
	for _, ln := range lines {
		if strings.Contains(ln, want) {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected %q in lines: %v", want, lines)
	}
}

func TestRuntimePanelLines_VLLMVenvInferred(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix .venv/bin layout")
	}
	proj := t.TempDir()
	binDir := filepath.Join(proj, ".venv", "bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatal(err)
	}
	act := filepath.Join(binDir, "activate")
	vllm := filepath.Join(binDir, "vllm")
	if err := os.WriteFile(act, []byte("#\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(vllm, []byte{}, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", binDir)

	// Avoid host-specific DiscoverRuntime (e.g. ~/.venv-vllm-metal before PATH).
	info := models.RuntimeInfo{VLLMPath: vllm}
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatal(err)
	}
	want := FormatPathDisplay(filepath.Join(proj, ".venv"), home)
	if got := vllmVenvPanelDisplay(info); got != want {
		t.Fatalf("vllmVenvPanelDisplay: got %q want %q", got, want)
	}
}

// Tab moves through the fields the platform shows, skipping hidden backends.
func TestStepRuntimeFieldSkipsHiddenBackends(t *testing.T) {
	t.Parallel()

	m := Model{}
	m.runtime.Platform = models.Platform{GOOS: "linux", GOARCH: "amd64"}
	if got := m.stepRuntimeField(runtimeFieldNInferHost, 1); got != runtimeFieldVLLMPath {
		t.Errorf("linux: after NInfer host got %d, want vLLM path (Splash hidden)", got)
	}
	if got := m.stepRuntimeField(runtimeFieldKoboldCppPort, 1); got != runtimeFieldLlamaCppPath {
		t.Errorf("linux: after KoboldCpp port got %d, want wrap to llama path (oMLX hidden)", got)
	}

	m.runtime.Platform = models.Platform{GOOS: "darwin", GOARCH: "arm64"}
	if got := m.stepRuntimeField(runtimeFieldOllamaHost, 1); got != runtimeFieldSplashPath {
		t.Errorf("darwin: after Ollama host got %d, want Splash path (NInfer hidden)", got)
	}
	if got := m.stepRuntimeField(runtimeFieldLlamaCppPath, -1); got != runtimeFieldOMLXHost {
		t.Errorf("darwin: before llama path got %d, want oMLX host", got)
	}
}
