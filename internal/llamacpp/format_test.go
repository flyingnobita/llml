package llamacpp

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFormatModelFolderDisplay_hfSnapshots(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	file := filepath.Join(home, ".cache", "huggingface", "hub", "models--unsloth--gemma-GGUF", "snapshots", "8bacec5c8e829a25502cdfe3c3f5b6aabee3218c", "model.gguf")
	got := FormatModelFolderDisplay(file)
	want := filepath.Join("~", ".cache", "huggingface", "hub", "models--unsloth--gemma-GGUF")
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestFormatModelFolderDisplay_directInRepo(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	file := filepath.Join(home, ".cache", "huggingface", "hub", "models--unsloth--gemma-GGUF", "model.gguf")
	got := FormatModelFolderDisplay(file)
	want := filepath.Join("~", ".cache", "huggingface", "hub", "models--unsloth--gemma-GGUF")
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestFormatVLLMModelName_hfHubRepo(t *testing.T) {
	home := t.TempDir()
	hub := filepath.Join(home, ".cache", "huggingface", "hub", "models--google--gemma-4-E4B-it", "snapshots", "83df0a889143b1dbfc61b591bbc639540fd9cea")
	if err := os.MkdirAll(hub, 0o755); err != nil {
		t.Fatal(err)
	}
	got := FormatVLLMModelName(hub)
	want := "google/gemma-4-E4B-it"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestFormatVLLMModelName_nestedOrg(t *testing.T) {
	home := t.TempDir()
	hub := filepath.Join(home, "hub", "models--meta-llama--Llama-2-7b-hf", "snapshots", "abc")
	if err := os.MkdirAll(hub, 0o755); err != nil {
		t.Fatal(err)
	}
	got := FormatVLLMModelName(hub)
	want := "meta-llama/Llama-2-7b-hf"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestFormatVLLMModelName_nonHubFallback(t *testing.T) {
	home := t.TempDir()
	dir := filepath.Join(home, "weights", "my-checkpoint")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	got := FormatVLLMModelName(dir)
	if got != "my-checkpoint" {
		t.Fatalf("got %q want my-checkpoint", got)
	}
}

func TestFormatModelFolderDisplay_safetensorsDirPath(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	dir := filepath.Join(home, ".cache", "huggingface", "hub", "models--org--repo", "snapshots", "abc123")
	got := FormatModelFolderDisplay(dir)
	want := filepath.Join("~", ".cache", "huggingface", "hub", "models--org--repo")
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestFormatModelFolderDisplay_noHFRepoDir(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	file := filepath.Join(home, "models", "weights", "a.gguf")
	got := FormatModelFolderDisplay(file)
	want := filepath.Join("~", "models", "weights")
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestFormatPathDisplay_underHome(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	p := filepath.Join(home, "models", "x.gguf")
	got := FormatPathDisplay(p)
	want := filepath.Join("~", "models", "x.gguf")
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestFormatPathDisplay_homeDir(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	got := FormatPathDisplay(home)
	if got != "~" {
		t.Fatalf("got %q want ~", got)
	}
}

func TestFormatPathDisplay_outsideHome(t *testing.T) {
	t.Setenv("HOME", "/tmp/llm-launch-test-home")

	abs := "/other/mount/model.gguf"
	if got := FormatPathDisplay(abs); got != abs {
		t.Fatalf("got %q want %q", got, abs)
	}
}
