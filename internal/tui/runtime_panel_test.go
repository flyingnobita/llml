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

	r := models.RuntimeInfo{
		LlamaServerPath: "/home/u/llama.cpp/bin/llama-server",
		VLLMPath:        "/home/u/.local/bin/vllm",
		ServerRunning:   false,
		ProbePort:       8080,
	}
	lines := RuntimePanelLines(80, r)
	if len(lines) != 5 {
		t.Fatalf("got %d lines", len(lines))
	}
	// Alphabetical: llama-server path, llama-server port, vllm path, vllm port, vllm venv path
	if !strings.Contains(lines[0], runtimePanelLabelLlamaServerPath) || !strings.Contains(lines[0], "llama-server") {
		t.Errorf("llama-server path line: %q", lines[0])
	}
	if !strings.Contains(lines[1], runtimePanelLabelLlamaServerPort) || !strings.Contains(lines[1], "8080") {
		t.Errorf("llama-server port line: %q", lines[1])
	}
	if !strings.Contains(lines[2], runtimePanelLabelVLLMPath) || !strings.Contains(lines[2], "vllm") {
		t.Errorf("vllm path line: %q", lines[2])
	}
	if !strings.Contains(lines[3], runtimePanelLabelVLLMPort) || !strings.Contains(lines[3], "8000") {
		t.Errorf("vllm port line: %q", lines[3])
	}
	if !strings.Contains(lines[4], runtimePanelLabelVLLMVenv) || !strings.Contains(lines[4], "—") {
		t.Errorf("vllm venv path line: %q", lines[4])
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

	info := models.DiscoverRuntime()
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatal(err)
	}
	want := FormatPathDisplay(filepath.Join(proj, ".venv"), home)
	if got := vllmVenvPanelDisplay(info); got != want {
		t.Fatalf("vllmVenvPanelDisplay: got %q want %q", got, want)
	}
}
