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

func TestFindKoboldCppBinary_envPathWins(t *testing.T) {
	dir := t.TempDir()
	bin := makeFakeExecutable(t, dir, "koboldcpp")
	t.Setenv(EnvKoboldCppPath, dir)
	t.Setenv("PATH", "/nonexistent")

	got := findKoboldCppBinary()
	if got != bin {
		t.Fatalf("got %q want %q", got, bin)
	}
}

func TestFindKoboldCppBinary_executablePathWins(t *testing.T) {
	dir := t.TempDir()
	bin := makeFakeExecutable(t, dir, "koboldcpp")
	t.Setenv(EnvKoboldCppPath, bin)
	t.Setenv("PATH", "/nonexistent")

	got := findKoboldCppBinary()
	if got != bin {
		t.Fatalf("got %q want %q", got, bin)
	}
}

func TestFindKoboldCppBinary_platformSpecificName(t *testing.T) {
	dir := t.TempDir()
	bin := makeFakeExecutable(t, dir, "koboldcpp-linux-x64")
	t.Setenv(EnvKoboldCppPath, dir)
	t.Setenv("PATH", "/nonexistent")

	got := findKoboldCppBinary()
	if got != bin {
		t.Fatalf("got %q want %q", got, bin)
	}
}

func TestFindKoboldCppBinary_platformSpecificFullPath(t *testing.T) {
	dir := t.TempDir()
	bin := makeFakeExecutable(t, dir, "koboldcpp-linux-x64")
	t.Setenv(EnvKoboldCppPath, bin)
	t.Setenv("PATH", "/nonexistent")

	got := findKoboldCppBinary()
	if got != bin {
		t.Fatalf("got %q want %q", got, bin)
	}
}

func TestFindKoboldCppBinary_prefersPrimaryVariant(t *testing.T) {
	dir := t.TempDir()
	primary := makeFakeExecutable(t, dir, "koboldcpp-linux-x64")
	makeFakeExecutable(t, dir, "koboldcpp-linux-x64-nocuda")
	makeFakeExecutable(t, dir, "koboldcpp-linux-x64-oldpc")
	t.Setenv(EnvKoboldCppPath, dir)
	t.Setenv("PATH", "/nonexistent")

	got := findKoboldCppBinary()
	if got != primary {
		t.Fatalf("got %q want primary %q", got, primary)
	}
}

func TestFindKoboldCppBinary_pathFallback(t *testing.T) {
	dir := t.TempDir()
	bin := makeFakeExecutable(t, dir, "koboldcpp")
	t.Setenv(EnvKoboldCppPath, "")
	t.Setenv("PATH", dir)

	got := findKoboldCppBinary()
	if got != bin {
		t.Fatalf("got %q want %q", got, bin)
	}
}

func TestProbeHealthEndpoint(t *testing.T) {
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
	if !probeHealthEndpoint(port) {
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

func TestFindVLLMBinary_DarwinVenvVllmMetal(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("macOS-only common path ~/.venv-vllm-metal/bin")
	}
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv(EnvVLLMPath, "")
	t.Setenv(EnvVLLMVenv, "")
	t.Setenv("PATH", "/nonexistent")

	binDir := filepath.Join(home, ".venv-vllm-metal", "bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatal(err)
	}
	vllm := filepath.Join(binDir, "vllm")
	if err := os.WriteFile(vllm, []byte{}, 0o755); err != nil {
		t.Fatal(err)
	}

	got := findVLLMBinary()
	if got != vllm {
		t.Fatalf("got %q want %q", got, vllm)
	}
	wantActivate := filepath.Join(binDir, "activate")
	if err := os.WriteFile(wantActivate, []byte{}, 0o644); err != nil {
		t.Fatal(err)
	}
	if act := ResolveVLLMActivateScript(got); act != wantActivate {
		t.Fatalf("ResolveVLLMActivateScript(%q) = %q want %q", got, act, wantActivate)
	}
}
