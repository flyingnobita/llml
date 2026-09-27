package tui

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/flyingnobita/llml/internal/models"
)

func TestVLLMVenvInUse_inferred(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix .venv/bin layout")
	}
	proj := t.TempDir()
	binDir := filepath.Join(proj, ".venv", "bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatal(err)
	}
	act := filepath.Join(binDir, "activate")
	vllm := filepath.Join(binDir, "vllm")
	if err := os.WriteFile(act, []byte("#\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(vllm, []byte{}, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", binDir)

	// Avoid host-specific DiscoverRuntime (e.g. ~/.venv-vllm-metal before PATH).
	info := models.RuntimeInfo{VLLMPath: vllm}
	want := filepath.Join(proj, ".venv")
	if got := vllmVenvInUse(info); got != want {
		t.Fatalf("vllmVenvInUse: got %q want %q", got, want)
	}
}
