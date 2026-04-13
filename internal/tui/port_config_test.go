package tui

import (
	"os"
	"testing"

	"github.com/flyingnobita/llm-launch/internal/llamacpp"
)

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
