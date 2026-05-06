package models

import (
	"path/filepath"
	"testing"
)

func TestPathSetAdd(t *testing.T) {
	ps := NewPathSet()

	ps.Add("/a/b")
	ps.Add("/a/b")       // duplicate
	ps.Add("")           // empty, skipped
	ps.Add(".")          // dot, skipped
	ps.Add("  /a/b/c  ") // with spaces

	got := ps.Slice()
	if len(got) != 2 {
		t.Fatalf("expected 2 entries, got %d: %v", len(got), got)
	}
	if got[0] != filepath.Clean("/a/b") {
		t.Errorf("got[0] = %q", got[0])
	}
	if got[1] != filepath.Clean("/a/b/c") {
		t.Errorf("got[1] = %q", got[1])
	}
}

func TestPathSetSlice(t *testing.T) {
	ps := NewPathSet()
	if len(ps.Slice()) != 0 {
		t.Error("expected empty slice from empty set")
	}

	ps.Add("/x")
	if len(ps.Slice()) != 1 {
		t.Error("expected 1 entry")
	}
}

func TestPathSetTildeExpansion(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	ps := NewPathSet()
	ps.Add("~/models")
	got := ps.Slice()
	if len(got) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(got))
	}
	want := filepath.Join(home, "models")
	if got[0] != want {
		t.Errorf("got %q, want %q", got[0], want)
	}
}

func TestNewPathSet(t *testing.T) {
	ps := NewPathSet()
	if ps == nil {
		t.Fatal("NewPathSet returned nil")
	}
	if ps.seen == nil {
		t.Error("seen map is nil")
	}
}

func TestPathSetAdd_skipsDotAndEmpty(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	ps := NewPathSet()
	ps.Add(".")
	ps.Add("")
	ps.Add("  ")
	if s := ps.Slice(); len(s) != 0 {
		t.Errorf("expected empty slice, got %v", s)
	}

	// ExpandTildePath("~") returns home; that's fine to add.
	ps.Add("~")
	if s := ps.Slice(); len(s) != 1 {
		t.Errorf("expected 1 entry for ~, got %v", s)
	}
}
