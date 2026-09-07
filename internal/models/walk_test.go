package models

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
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
	err := walkSearchTree(t.Context(), dir, 10, func(fullPath, _ string, _ os.DirEntry, _ int) error {
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
	err := walkSearchTree(t.Context(), dir, 10, func(_, _ string, _ os.DirEntry, _ int) error {
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
	err := walkSearchTree(t.Context(), "/nonexistent/path/12345", 10, func(_, _ string, _ os.DirEntry, _ int) error {
		return nil
	})
	if err != nil {
		t.Fatal("expected nil error for nonexistent root")
	}
}

// mkSymlink creates a symlink, skipping the test where the platform or the
// runner does not allow it (unprivileged Windows, most notably).
func mkSymlink(t *testing.T, oldname, newname string) {
	t.Helper()
	if err := os.Symlink(oldname, newname); err != nil {
		t.Skipf("cannot create symlinks here: %v", err)
	}
}

// A self-referencing link used to be bounded only by the depth cap, so the same
// subtree was rescanned once per level. The visited set must stop it outright.
func TestWalkSearchTree_terminatesOnSymlinkCycle(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	sub := filepath.Join(root, "a")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sub, "m.gguf"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	// a/loop points back at its own parent.
	mkSymlink(t, root, filepath.Join(sub, "loop"))

	seen := map[string]int{}
	err := walkSearchTree(t.Context(), root, 12, func(full, _ string, _ os.DirEntry, _ int) error {
		seen[filepath.Base(full)]++
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := seen["m.gguf"]; got != 1 {
		t.Errorf("m.gguf reported %d times, want 1", got)
	}
}

// Two links to the same directory must not double-report its contents.
func TestWalkSearchTree_visitsSharedDirectoryOnce(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	target := filepath.Join(root, "real")
	if err := os.MkdirAll(target, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(target, "m.gguf"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	mkSymlink(t, target, filepath.Join(root, "link-a"))
	mkSymlink(t, target, filepath.Join(root, "link-b"))

	count := 0
	if err := walkSearchTree(t.Context(), root, 12, func(full, _ string, _ os.DirEntry, _ int) error {
		if strings.HasSuffix(full, ".gguf") {
			count++
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Errorf("the shared directory was walked %d times, want 1", count)
	}
}

// Discovery over a tree must not change when a cycle is added to it.
func TestDiscover_cycleDoesNotChangeResults(t *testing.T) {
	t.Parallel()

	build := func(t *testing.T, withLoop bool) []string {
		t.Helper()
		root := t.TempDir()
		sub := filepath.Join(root, "a")
		if err := os.MkdirAll(sub, 0o755); err != nil {
			t.Fatal(err)
		}
		for _, name := range []string{"one.gguf", "two.gguf"} {
			if err := os.WriteFile(filepath.Join(sub, name), []byte("x"), 0o644); err != nil {
				t.Fatal(err)
			}
		}
		if withLoop {
			mkSymlink(t, root, filepath.Join(sub, "loop"))
		}
		got, err := Discover(t.Context(), Options{
			Settings:         settingsWithOllamaHost("", root),
			MaxDepth:         12,
			SkipDefaultRoots: true,
		})
		if err != nil {
			t.Fatal(err)
		}
		names := make([]string, 0, len(got))
		for _, f := range got {
			names = append(names, f.Name)
		}
		slices.Sort(names)
		return names
	}

	plain := build(t, false)
	looped := build(t, true)
	if !slices.Equal(plain, looped) {
		t.Errorf("a symlink cycle changed discovery results:\nwithout: %v\n   with: %v", plain, looped)
	}
	if len(plain) != 2 {
		t.Errorf("expected both models, got %v", plain)
	}
}

// A broken symlink is skipped rather than reported or fatal.
func TestWalkSearchTree_skipsBrokenSymlink(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	mkSymlink(t, filepath.Join(root, "nonexistent"), filepath.Join(root, "dangling"))

	var seen []string
	if err := walkSearchTree(t.Context(), root, 12, func(full, _ string, _ os.DirEntry, _ int) error {
		seen = append(seen, full)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if len(seen) != 0 {
		t.Errorf("a broken link should be skipped, got %v", seen)
	}
}
