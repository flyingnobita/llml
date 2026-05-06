package models

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWalkSearchTree_filesOnly(t *testing.T) {
	dir := t.TempDir()

	// Create some files and subdirs.
	os.WriteFile(filepath.Join(dir, "a.gguf"), []byte("x"), 0o644)
	os.WriteFile(filepath.Join(dir, "b.safetensors"), []byte("y"), 0o644)
	os.MkdirAll(filepath.Join(dir, "subdir"), 0o755)
	os.WriteFile(filepath.Join(dir, "subdir", "c.gguf"), []byte("z"), 0o644)

	var found []string
	err := walkSearchTree(dir, 10, func(fullPath, _ string, _ os.DirEntry, _ int) error {
		found = append(found, filepath.Base(fullPath))
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(found) != 3 {
		t.Fatalf("expected 3 files, got %d: %v", len(found), found)
	}
}

func TestWalkSearchTree_emptyDir(t *testing.T) {
	dir := t.TempDir()

	var calls int
	err := walkSearchTree(dir, 10, func(_, _ string, _ os.DirEntry, _ int) error {
		calls++
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if calls != 0 {
		t.Errorf("expected 0 calls, got %d", calls)
	}
}

func TestWalkSearchTree_nonexistent(t *testing.T) {
	err := walkSearchTree("/nonexistent/path/12345", 10, func(_, _ string, _ os.DirEntry, _ int) error {
		return nil
	})
	if err != nil {
		t.Fatal("expected nil error for nonexistent root")
	}
}
