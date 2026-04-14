package llamacpp

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestResolveVLLMActivateScript(t *testing.T) {
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

	t.Run("VLLM_PATH_dot_venv", func(t *testing.T) {
		t.Setenv(EnvVLLMVenv, "")
		t.Setenv(EnvVLLMPath, proj)
		got := ResolveVLLMActivateScript(vllmBin)
		if got != activate {
			t.Fatalf("got %q want %q", got, activate)
		}
	})
	t.Run("VLLM_VENV_explicit", func(t *testing.T) {
		t.Setenv(EnvVLLMPath, "")
		t.Setenv(EnvVLLMVenv, filepath.Join(proj, ".venv"))
		got := ResolveVLLMActivateScript("/other/vllm")
		if got != activate {
			t.Fatalf("got %q want %q", got, activate)
		}
	})
	t.Run("dirname_vllm_dot_venv", func(t *testing.T) {
		t.Setenv(EnvVLLMVenv, "")
		t.Setenv(EnvVLLMPath, "")
		got := ResolveVLLMActivateScript(vllmBin)
		if got != activate {
			t.Fatalf("got %q want %q", got, activate)
		}
	})
}

func TestResolveVLLMActivateScript_adjacentBinLayout(t *testing.T) {
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
	t.Setenv(EnvVLLMPath, "")
	t.Setenv(EnvVLLMVenv, "")
	if g := ResolveVLLMActivateScript(vllm); g != act {
		t.Fatalf("got %q want %q", g, act)
	}
}
