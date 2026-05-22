package tui

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/flyingnobita/llml/internal/models"
)

func TestRuntimePanelLines(t *testing.T) {
	t.Setenv(models.EnvLlamaServerPort, "")
	t.Setenv(models.EnvVLLMServerPort, "")
	t.Setenv(models.EnvVLLMVenv, "")
	t.Setenv(models.EnvLlamaCppPath, "")
	t.Setenv(models.EnvVLLMPath, "")
	t.Setenv(models.EnvOllamaHost, "")
	t.Setenv(models.EnvOllamaPath, "")

	r := models.RuntimeInfo{
		LlamaServerPath: "/home/u/llama.cpp/bin/llama-server",
		LlamaServerHost: "127.0.0.1",
		VLLMPath:        "/home/u/.local/bin/vllm",
		VLLMServerHost:  "127.0.0.1",
		OllamaPath:      "/home/u/.local/bin/ollama",
		OllamaHost:      "127.0.0.1:11434",
		ServerRunning:   false,
		ProbePort:       8080,
	}
	lines := RuntimePanelLines(80, r)
	if len(lines) != 11 {
		t.Fatalf("got %d lines", len(lines))
	}
	// Alphabetical: koboldcpp path, koboldcpp port, llama-server path, llama.cpp host, llama.cpp port, ollama host, ollama path, vllm host, vllm path, vllm port, vllm venv path
	if !strings.Contains(lines[0], runtimePanelLabelKoboldCppPath) {
		t.Errorf("koboldcpp path line: %q", lines[0])
	}
	if !strings.Contains(lines[1], runtimePanelLabelKoboldCppPort) || !strings.Contains(lines[1], "5001") {
		t.Errorf("koboldcpp port line: %q", lines[1])
	}
	if !strings.Contains(lines[2], runtimePanelLabelLlamaServerPath) || !strings.Contains(lines[2], "llama-server") {
		t.Errorf("llama-server path line: %q", lines[2])
	}
	if !strings.Contains(lines[3], runtimePanelLabelLlamaServerHost) || !strings.Contains(lines[3], "127.0.0.1") {
		t.Errorf("llama.cpp host line: %q", lines[3])
	}
	if !strings.Contains(lines[4], runtimePanelLabelLlamaServerPort) || !strings.Contains(lines[4], "8080") {
		t.Errorf("llama.cpp port line: %q", lines[4])
	}
	if !strings.Contains(lines[5], runtimePanelLabelOllamaHost) || !strings.Contains(lines[5], "127.0.0.1:11434") {
		t.Errorf("ollama host line: %q", lines[5])
	}
	if !strings.Contains(lines[6], runtimePanelLabelOllamaPath) || !strings.Contains(lines[6], "ollama") {
		t.Errorf("ollama path line: %q", lines[6])
	}
	if !strings.Contains(lines[7], runtimePanelLabelVLLMHost) || !strings.Contains(lines[7], "127.0.0.1") {
		t.Errorf("vllm host line: %q", lines[7])
	}
	if !strings.Contains(lines[8], runtimePanelLabelVLLMPath) || !strings.Contains(lines[8], "vllm") {
		t.Errorf("vllm path line: %q", lines[8])
	}
	if !strings.Contains(lines[9], runtimePanelLabelVLLMPort) || !strings.Contains(lines[9], "8000") {
		t.Errorf("vllm port line: %q", lines[9])
	}
	if !strings.Contains(lines[10], runtimePanelLabelVLLMVenv) || !strings.Contains(lines[10], "—") {
		t.Errorf("vllm venv path line: %q", lines[10])
	}
}

func TestRuntimePanelLines_ServerRunningNoBinary(t *testing.T) {
	t.Setenv(models.EnvLlamaServerPort, "")
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
	t.Setenv(models.EnvLlamaCppPath, "")
	t.Setenv(models.EnvVLLMPath, "")
	t.Setenv(models.EnvVLLMVenv, "")
	t.Setenv(models.EnvLlamaServerPort, "")
	t.Setenv(models.EnvVLLMServerPort, "")
	t.Setenv("PATH", binDir)

	// Avoid host-specific DiscoverRuntime() (e.g. ~/.venv-vllm-metal before PATH).
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
