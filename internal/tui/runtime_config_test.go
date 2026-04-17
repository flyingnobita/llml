package tui

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/flyingnobita/llml/internal/models"
)

func TestApplyVLLMPortEnv(t *testing.T) {
	t.Run("empty unsets", func(t *testing.T) {
		t.Setenv(models.EnvVLLMServerPort, "9000")
		if err := applyVLLMPortEnv(""); err != nil {
			t.Fatal(err)
		}
		if os.Getenv(models.EnvVLLMServerPort) != "" {
			t.Fatal("expected unset")
		}
	})
	t.Run("valid sets", func(t *testing.T) {
		t.Setenv(models.EnvVLLMServerPort, "")
		if err := applyVLLMPortEnv("8000"); err != nil {
			t.Fatal(err)
		}
		if os.Getenv(models.EnvVLLMServerPort) != "8000" {
			t.Fatalf("got %q", os.Getenv(models.EnvVLLMServerPort))
		}
	})
}

func TestApplyListenPortEnv(t *testing.T) {
	t.Run("empty unsets", func(t *testing.T) {
		t.Setenv(models.EnvLlamaServerPort, "9000")
		if err := applyListenPortEnv(""); err != nil {
			t.Fatal(err)
		}
		if os.Getenv(models.EnvLlamaServerPort) != "" {
			t.Fatal("expected unset")
		}
	})
	t.Run("valid sets", func(t *testing.T) {
		t.Setenv(models.EnvLlamaServerPort, "")
		if err := applyListenPortEnv("9090"); err != nil {
			t.Fatal(err)
		}
		if os.Getenv(models.EnvLlamaServerPort) != "9090" {
			t.Fatalf("got %q", os.Getenv(models.EnvLlamaServerPort))
		}
	})
	t.Run("reject out of range", func(t *testing.T) {
		if applyListenPortEnv("0") == nil {
			t.Fatal("expected error")
		}
		if applyListenPortEnv("65536") == nil {
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
