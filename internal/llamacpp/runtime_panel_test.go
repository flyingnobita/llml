package llamacpp

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestRuntimePanelLines(t *testing.T) {
	t.Setenv(EnvLlamaServerPort, "")
	t.Setenv(EnvVLLMServerPort, "")
	t.Setenv(EnvLlamaCppPath, "/home/u/llama.cpp/bin")
	t.Setenv(EnvVLLMPath, "/home/u/.local/bin")
	t.Setenv(EnvVLLMVenv, "")
	lines := RuntimePanelLines(80, DiscoverRuntime())
	if len(lines) != 5 {
		t.Fatalf("got %d lines", len(lines))
	}
	if !strings.Contains(lines[0], EnvLlamaCppPath) || !strings.Contains(lines[0], "llama.cpp") {
		t.Errorf("LLAMA_CPP_PATH line: %q", lines[0])
	}
	if !strings.Contains(lines[1], EnvLlamaServerPort) || !strings.Contains(lines[1], "8080") {
		t.Errorf("LLAMA_SERVER_PORT line: %q", lines[1])
	}
	if !strings.Contains(lines[2], EnvVLLMPath) || !strings.Contains(lines[2], ".local") {
		t.Errorf("VLLM_PATH line: %q", lines[2])
	}
	if !strings.Contains(lines[3], EnvVLLMServerPort) || !strings.Contains(lines[3], "8000") {
		t.Errorf("VLLM_SERVER_PORT line: %q", lines[3])
	}
	if !strings.Contains(lines[4], EnvVLLMVenv) || !strings.Contains(lines[4], "—") {
		t.Errorf("VLLM_VENV line: %q", lines[4])
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
	t.Setenv(EnvLlamaCppPath, "")
	t.Setenv(EnvVLLMPath, "")
	t.Setenv(EnvVLLMVenv, "")
	t.Setenv(EnvLlamaServerPort, "")
	t.Setenv(EnvVLLMServerPort, "")
	t.Setenv("PATH", binDir)

	info := DiscoverRuntime()
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatal(err)
	}
	want := FormatPathDisplay(filepath.Join(proj, ".venv"), home)
	if got := vllmVenvPanelDisplay(info); got != want {
		t.Fatalf("vllmVenvPanelDisplay: got %q want %q", got, want)
	}
}
