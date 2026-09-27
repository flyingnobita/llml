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
