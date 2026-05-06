package models

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGgufSourceMatch(t *testing.T) {
	dir := t.TempDir()
	var src ggufSource

	// .gguf file matches.
	ggufPath := filepath.Join(dir, "test.gguf")
	os.WriteFile(ggufPath, []byte("fake"), 0o644)
	if got := src.match(ggufPath, dir, nil); got == "" {
		t.Error("expected match for .gguf file")
	}

	// Non-.gguf file does not match.
	txtPath := filepath.Join(dir, "test.txt")
	os.WriteFile(txtPath, []byte("fake"), 0o644)
	if got := src.match(txtPath, dir, nil); got != "" {
		t.Error("expected no match for .txt file")
	}
}

func TestSafetensorsSourceMatch(t *testing.T) {
	dir := t.TempDir()
	var src safetensorsSource

	sfPath := filepath.Join(dir, "model.safetensors")
	os.WriteFile(sfPath, []byte("fake"), 0o644)
	if got := src.match(sfPath, dir, nil); got != dir {
		t.Errorf("expected match returning parent dir %q, got %q", dir, got)
	}

	// Non-.safetensors file does not match.
	txtPath := filepath.Join(dir, "test.bin")
	os.WriteFile(txtPath, []byte("fake"), 0o644)
	if got := src.match(txtPath, dir, nil); got != "" {
		t.Error("expected no match for .bin file")
	}
}
