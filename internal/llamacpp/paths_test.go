package llamacpp

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

func TestHuggingFaceHubCache_env(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HUGGINGFACE_HUB_CACHE", "")
	t.Setenv("HF_HOME", "")

	def := filepath.Join(home, ".cache", "huggingface", "hub")
	if got := huggingFaceHubCache(home); got != def {
		t.Fatalf("default: got %q want %q", got, def)
	}

	t.Setenv("HF_HOME", filepath.Join(home, "hfhome"))
	want := filepath.Join(home, "hfhome", "hub")
	if got := huggingFaceHubCache(home); got != want {
		t.Fatalf("HF_HOME: got %q want %q", got, want)
	}

	custom := filepath.Join(home, "custom", "hub")
	t.Setenv("HUGGINGFACE_HUB_CACHE", custom)
	t.Setenv("HF_HOME", filepath.Join(home, "ignored"))
	if got := huggingFaceHubCache(home); got != filepath.Clean(custom) {
		t.Fatalf("HUGGINGFACE_HUB_CACHE: got %q want %q", got, filepath.Clean(custom))
	}
}
