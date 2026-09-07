package models

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestResolveVLLMActivateScript(t *testing.T) {
	t.Parallel()

	proj := t.TempDir()
	var activate string
	if runtime.GOOS == "windows" {
		scripts := filepath.Join(proj, ".venv", "Scripts")
		if err := os.MkdirAll(scripts, 0o755); err != nil {
			t.Fatal(err)
		}
		activate = filepath.Join(scripts, "activate.bat")
	} else {
		venvBin := filepath.Join(proj, ".venv", "bin")
		if err := os.MkdirAll(venvBin, 0o755); err != nil {
			t.Fatal(err)
		}
		activate = filepath.Join(venvBin, "activate")
	}
	if err := os.WriteFile(activate, []byte("# fake\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	vllmBin := filepath.Join(proj, "vllm")
	if err := os.WriteFile(vllmBin, []byte{}, 0o755); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name     string
		vllmBin  string
		venvRoot string
		vllmPath string
	}{
		{name: "vllm_path_dot_venv", vllmBin: vllmBin, vllmPath: proj},
		{name: "venv_root_explicit", vllmBin: "/other/vllm", venvRoot: filepath.Join(proj, ".venv")},
		{name: "dirname_vllm_dot_venv", vllmBin: vllmBin},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := ResolveVLLMActivateScript(tc.vllmBin, tc.venvRoot, tc.vllmPath)
			if got != activate {
				t.Fatalf("got %q want %q", got, activate)
			}
		})
	}
}

func TestResolveVLLMActivateScript_adjacentBinLayout(t *testing.T) {
	t.Parallel()

	if runtime.GOOS == "windows" {
		t.Skip("parallel test layout uses Unix venv paths")
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
	// An activate script adjacent to the binary wins even with no configured venv.
	if g := ResolveVLLMActivateScript(vllm, "", ""); g != act {
		t.Fatalf("got %q want %q", g, act)
	}
}
