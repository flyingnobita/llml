package tui

import (
	"path/filepath"
	"testing"
)

func TestFormatModelFolderDisplay_hfSnapshots(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	file := filepath.Join(home, ".cache", "huggingface", "hub", "models--unsloth--gemma-GGUF", "snapshots", "8bacec5c8e829a25502cdfe3c3f5b6aabee3218c", "model.gguf")
	got := FormatModelFolderDisplay(file, home)
	want := filepath.Join("~", ".cache", "huggingface", "hub", "models--unsloth--gemma-GGUF")
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestFormatModelFolderDisplay_directInRepo(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	file := filepath.Join(home, ".cache", "huggingface", "hub", "models--unsloth--gemma-GGUF", "model.gguf")
	got := FormatModelFolderDisplay(file, home)
	want := filepath.Join("~", ".cache", "huggingface", "hub", "models--unsloth--gemma-GGUF")
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestFormatModelFolderDisplay_safetensorsDirPath(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	dir := filepath.Join(home, ".cache", "huggingface", "hub", "models--org--repo", "snapshots", "abc123")
	got := FormatModelFolderDisplay(dir, home)
	want := filepath.Join("~", ".cache", "huggingface", "hub", "models--org--repo")
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestFormatModelFolderDisplay_noHFRepoDir(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	file := filepath.Join(home, "models", "weights", "a.gguf")
	got := FormatModelFolderDisplay(file, home)
	want := filepath.Join("~", "models", "weights")
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestFormatPathDisplay_underHome(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	p := filepath.Join(home, "models", "x.gguf")
	got := FormatPathDisplay(p, home)
	want := filepath.Join("~", "models", "x.gguf")
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestFormatPathDisplay_homeDir(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	got := FormatPathDisplay(home, home)
	if got != "~" {
		t.Fatalf("got %q want ~", got)
	}
}

func TestFormatPathDisplay_outsideHome(t *testing.T) {
	home := "/tmp/llm-launch-test-home"
	t.Setenv("HOME", home)

	abs := "/other/mount/model.gguf"
	if got := FormatPathDisplay(abs, home); got != abs {
		t.Fatalf("got %q want %q", got, abs)
	}
}

func TestFormatPathDisplay_emptyHomeNoTilde(t *testing.T) {
	p := "/foo/bar"
	if got := FormatPathDisplay(p, ""); got != p {
		t.Fatalf("got %q want %q", got, p)
	}
}

func TestTruncateRunes(t *testing.T) {
	s := "hello世界"
	if TruncateRunes(s, 100) != s {
		t.Fatalf("short string changed")
	}
	got := TruncateRunes("abcdefghijklmnopqrstuvwxyz", 8)
	if len(got) < 2 {
		t.Fatalf("expected ellipsis suffix")
	}
}
