package llamacpp

import (
	"path/filepath"
	"testing"
)

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
