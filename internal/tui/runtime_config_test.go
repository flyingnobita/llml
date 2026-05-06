package tui

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/flyingnobita/llml/internal/models"
)

func TestApplyPortEnv(t *testing.T) {
	t.Run("VLLM empty unsets", func(t *testing.T) {
		t.Setenv(models.EnvVLLMServerPort, "9000")
		if err := applyPortEnv(models.EnvVLLMServerPort, "", models.VLLMPort()); err != nil {
			t.Fatal(err)
		}
		if os.Getenv(models.EnvVLLMServerPort) != "" {
			t.Fatal("expected unset")
		}
	})
	t.Run("VLLM valid sets", func(t *testing.T) {
		t.Setenv(models.EnvVLLMServerPort, "")
		if err := applyPortEnv(models.EnvVLLMServerPort, "8000", models.VLLMPort()); err != nil {
			t.Fatal(err)
		}
		if os.Getenv(models.EnvVLLMServerPort) != "8000" {
			t.Fatalf("got %q", os.Getenv(models.EnvVLLMServerPort))
		}
	})
	t.Run("llama empty unsets", func(t *testing.T) {
		t.Setenv(models.EnvLlamaServerPort, "9000")
		if err := applyPortEnv(models.EnvLlamaServerPort, "", models.ListenPort()); err != nil {
			t.Fatal(err)
		}
		if os.Getenv(models.EnvLlamaServerPort) != "" {
			t.Fatal("expected unset")
		}
	})
	t.Run("llama valid sets", func(t *testing.T) {
		t.Setenv(models.EnvLlamaServerPort, "")
		if err := applyPortEnv(models.EnvLlamaServerPort, "9090", models.ListenPort()); err != nil {
			t.Fatal(err)
		}
		if os.Getenv(models.EnvLlamaServerPort) != "9090" {
			t.Fatalf("got %q", os.Getenv(models.EnvLlamaServerPort))
		}
	})
	t.Run("llama reject out of range", func(t *testing.T) {
		if applyPortEnv(models.EnvLlamaServerPort, "0", models.ListenPort()) == nil {
			t.Fatal("expected error")
		}
		if applyPortEnv(models.EnvLlamaServerPort, "65536", models.ListenPort()) == nil {
			t.Fatal("expected error")
		}
	})
	t.Run("kobold empty unsets", func(t *testing.T) {
		t.Setenv(models.EnvKoboldCppPort, "6000")
		if err := applyPortEnv(models.EnvKoboldCppPort, "", models.KoboldCppPort()); err != nil {
			t.Fatal(err)
		}
		if os.Getenv(models.EnvKoboldCppPort) != "" {
			t.Fatal("expected unset")
		}
	})
	t.Run("kobold valid sets", func(t *testing.T) {
		t.Setenv(models.EnvKoboldCppPort, "")
		if err := applyPortEnv(models.EnvKoboldCppPort, "5001", models.KoboldCppPort()); err != nil {
			t.Fatal(err)
		}
		if os.Getenv(models.EnvKoboldCppPort) != "5001" {
			t.Fatalf("got %q", os.Getenv(models.EnvKoboldCppPort))
		}
	})
	t.Run("kobold reject out of range", func(t *testing.T) {
		if applyPortEnv(models.EnvKoboldCppPort, "0", models.KoboldCppPort()) == nil {
			t.Fatal("expected error")
		}
		if applyPortEnv(models.EnvKoboldCppPort, "65536", models.KoboldCppPort()) == nil {
			t.Fatal("expected error")
		}
	})
}

func TestValidatePortInput(t *testing.T) {
	if err := validatePortInput("8080"); err != nil {
		t.Fatal(err)
	}
	if validatePortInput("12a") == nil {
		t.Fatal("expected error")
	}
	if validatePortInput("123456") == nil {
		t.Fatal("expected error")
	}
}

func TestValidatePortCommit(t *testing.T) {
	if err := validatePortCommit(""); err != nil {
		t.Fatal(err)
	}
	if err := validatePortCommit("8080"); err != nil {
		t.Fatal(err)
	}
	if validatePortCommit("0") == nil {
		t.Fatal("expected error")
	}
}

func TestPrefillPort(t *testing.T) {
	t.Run("set env returns env", func(t *testing.T) {
		t.Setenv(models.EnvVLLMServerPort, "7777")
		if g := prefillPort(models.EnvVLLMServerPort, 8000); g != "7777" {
			t.Fatalf("got %q", g)
		}
	})
	t.Run("unset env returns effective", func(t *testing.T) {
		t.Setenv(models.EnvVLLMServerPort, "")
		if g := prefillPort(models.EnvVLLMServerPort, 8000); g != "8000" {
			t.Fatalf("got %q want 8000", g)
		}
	})
}

func TestApplyPathEnv(t *testing.T) {
	t.Setenv(models.EnvLlamaCppPath, "/old")
	applyPathEnv(models.EnvLlamaCppPath, "/new/path")
	if os.Getenv(models.EnvLlamaCppPath) != "/new/path" {
		t.Fatalf("got %q", os.Getenv(models.EnvLlamaCppPath))
	}
	applyPathEnv(models.EnvLlamaCppPath, "  ")
	if os.Getenv(models.EnvLlamaCppPath) != "" {
		t.Fatal("expected unset for whitespace-only")
	}
}

func TestApplyPathEnv_tilde(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv(models.EnvVLLMPath, "")
	applyPathEnv(models.EnvVLLMPath, "~/my-vllm")
	want := filepath.Join(home, "my-vllm")
	if got := os.Getenv(models.EnvVLLMPath); got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}
