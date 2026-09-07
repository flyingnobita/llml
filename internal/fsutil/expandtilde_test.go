package fsutil

import (
	"path/filepath"
	"testing"
)

func TestExpandTildePath(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	if got := ExpandTildePath("~"); got != home {
		t.Fatalf("~: got %q want %q", got, home)
	}
	want := filepath.Join(home, "models", "weights")
	if got := ExpandTildePath("~/models/weights"); got != want {
		t.Fatalf("~/...: got %q want %q", got, want)
	}
	if got := ExpandTildePath("  ~/x  "); got != filepath.Join(home, "x") {
		t.Fatalf("trim: got %q", got)
	}
	if got := ExpandTildePath("/usr/bin"); got != "/usr/bin" {
		t.Fatalf("abs: got %q", got)
	}
	if got := ExpandTildePath(""); got != "" {
		t.Fatalf("empty: got %q", got)
	}
}
