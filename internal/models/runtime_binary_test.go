package models

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"testing"
)

func TestFindLlamaBinary_LLamaCppPathWins(t *testing.T) {
	dir := t.TempDir()
	name := "llama-cli"
	bin := makeFakeExecutable(t, dir, name)
	t.Setenv(EnvLlamaCppPath, dir)
	t.Setenv("PATH", "/nonexistent")

	got := findLlamaBinary(name)
	if got != bin {
		t.Fatalf("got %q want %q", got, bin)
	}
}

func TestFindLlamaBinary_ExecutablePathWins(t *testing.T) {
	dir := t.TempDir()
	name := "llama-server"
	bin := makeFakeExecutable(t, dir, name)
	// Set the environment variable to the exact binary path instead of the directory
	t.Setenv(EnvLlamaCppPath, bin)
	t.Setenv("PATH", "/nonexistent")

	got := findLlamaBinary(name)
	if got != bin {
		t.Fatalf("got %q want %q", got, bin)
	}
}

func TestFindVLLMBinary_ExecutablePathWins(t *testing.T) {
	dir := t.TempDir()
	name := "vllm"
	bin := makeFakeExecutable(t, dir, name)
	// Set the environment variable to the exact binary path instead of the directory
	t.Setenv(EnvVLLMPath, bin)
	t.Setenv("PATH", "/nonexistent")

	got := findVLLMBinary()
	if got != bin {
		t.Fatalf("got %q want %q", got, bin)
	}
}

func TestProbeLlamaServerHealth(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/health" {
			http.NotFound(w, r)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(ts.Close)
	u, err := url.Parse(ts.URL)
	if err != nil {
		t.Fatal(err)
	}
	port, err := strconv.Atoi(u.Port())
	if err != nil {
		t.Fatal(err)
	}
	if !probeLlamaServerHealth(port) {
		t.Fatal("expected health probe success")
	}
}

func TestFindVLLMBinary_VLLMPath_dotVenv(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix .venv/bin layout")
	}
	proj := t.TempDir()
	binDir := filepath.Join(proj, ".venv", "bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatal(err)
	}
	vllm := filepath.Join(binDir, "vllm")
	if err := os.WriteFile(vllm, []byte{}, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv(EnvVLLMPath, proj)
	t.Setenv(EnvVLLMVenv, "")
	t.Setenv("PATH", "/nonexistent")
	if got := findVLLMBinary(); got != vllm {
		t.Fatalf("got %q want %q", got, vllm)
	}
}
