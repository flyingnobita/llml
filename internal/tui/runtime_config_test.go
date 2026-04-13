package tui

import (
	"os"
	"testing"

	"github.com/flyingnobita/llm-launch/internal/llamacpp"
)

func TestApplyVLLMPortEnv(t *testing.T) {
	t.Run("empty unsets", func(t *testing.T) {
		t.Setenv(llamacpp.EnvVLLMServerPort, "9000")
		if err := applyVLLMPortEnv(""); err != nil {
			t.Fatal(err)
		}
		if os.Getenv(llamacpp.EnvVLLMServerPort) != "" {
			t.Fatal("expected unset")
		}
	})
	t.Run("valid sets", func(t *testing.T) {
		t.Setenv(llamacpp.EnvVLLMServerPort, "")
		if err := applyVLLMPortEnv("8000"); err != nil {
			t.Fatal(err)
		}
		if os.Getenv(llamacpp.EnvVLLMServerPort) != "8000" {
			t.Fatalf("got %q", os.Getenv(llamacpp.EnvVLLMServerPort))
		}
	})
}

func TestApplyListenPortEnv(t *testing.T) {
	t.Run("empty unsets", func(t *testing.T) {
		t.Setenv(llamacpp.EnvLlamaServerPort, "9000")
		if err := applyListenPortEnv(""); err != nil {
			t.Fatal(err)
		}
		if os.Getenv(llamacpp.EnvLlamaServerPort) != "" {
			t.Fatal("expected unset")
		}
	})
	t.Run("valid sets", func(t *testing.T) {
		t.Setenv(llamacpp.EnvLlamaServerPort, "")
		if err := applyListenPortEnv("9090"); err != nil {
			t.Fatal(err)
		}
		if os.Getenv(llamacpp.EnvLlamaServerPort) != "9090" {
			t.Fatalf("got %q", os.Getenv(llamacpp.EnvLlamaServerPort))
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
		t.Setenv(llamacpp.EnvVLLMServerPort, "7777")
		if g := prefillPort(llamacpp.EnvVLLMServerPort, 8000); g != "7777" {
			t.Fatalf("got %q", g)
		}
	})
	t.Run("unset env returns effective", func(t *testing.T) {
		t.Setenv(llamacpp.EnvVLLMServerPort, "")
		if g := prefillPort(llamacpp.EnvVLLMServerPort, 8000); g != "8000" {
			t.Fatalf("got %q want 8000", g)
		}
	})
}

func TestApplyPathEnv(t *testing.T) {
	t.Setenv(llamacpp.EnvLlamaCppPath, "/old")
	applyPathEnv(llamacpp.EnvLlamaCppPath, "/new/path")
	if os.Getenv(llamacpp.EnvLlamaCppPath) != "/new/path" {
		t.Fatalf("got %q", os.Getenv(llamacpp.EnvLlamaCppPath))
	}
	applyPathEnv(llamacpp.EnvLlamaCppPath, "  ")
	if os.Getenv(llamacpp.EnvLlamaCppPath) != "" {
		t.Fatal("expected unset for whitespace-only")
	}
}
