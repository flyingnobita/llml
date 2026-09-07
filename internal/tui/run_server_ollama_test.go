package tui

import (
	"strings"
	"testing"

	"github.com/flyingnobita/llml/internal/profiles"

	"github.com/flyingnobita/llml/internal/models"
)

func TestBuildServerSpec_OllamaPreview(t *testing.T) {
	spec, err := buildServerSpec(models.BackendOllama, "qwen3.5:latest", profiles.ModelParams{}, models.RuntimeInfo{
		OllamaHost: "127.0.0.1:11434",
	}, false)
	if err != nil {
		t.Fatal(err)
	}
	got := spec.previewLine()
	if !strings.Contains(got, "serve") || !strings.Contains(got, "ollama") {
		t.Fatalf("preview %q", got)
	}
	if !strings.Contains(got, "\"keep_alive\":-1") {
		t.Fatalf("preview %q", got)
	}
	if !strings.Contains(got, "qwen3.5:latest") {
		t.Fatalf("preview %q", got)
	}
}
