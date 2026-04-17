package models

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFormatVLLMModelName_hfHubRepo(t *testing.T) {
	home := t.TempDir()
	hub := filepath.Join(home, ".cache", "huggingface", "hub", "models--google--gemma-4-E4B-it", "snapshots", "83df0a889143b1dbfc61b591bbc639540fd9cea")
	if err := os.MkdirAll(hub, 0o755); err != nil {
		t.Fatal(err)
	}
	got := FormatVLLMModelName(hub)
	want := "google/gemma-4-E4B-it"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestFormatVLLMModelName_nestedOrg(t *testing.T) {
	home := t.TempDir()
	hub := filepath.Join(home, "hub", "models--meta-llama--Llama-2-7b-hf", "snapshots", "abc")
	if err := os.MkdirAll(hub, 0o755); err != nil {
		t.Fatal(err)
	}
	got := FormatVLLMModelName(hub)
	want := "meta-llama/Llama-2-7b-hf"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestFormatVLLMModelName_nonHubFallback(t *testing.T) {
	home := t.TempDir()
	dir := filepath.Join(home, "weights", "my-checkpoint")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	got := FormatVLLMModelName(dir)
	if got != "my-checkpoint" {
		t.Fatalf("got %q want my-checkpoint", got)
	}
}

func TestFormatRuntimeLabel(t *testing.T) {
	tests := []struct {
		b    ModelBackend
		want string
	}{
		{BackendLlama, "llama.cpp"},
		{BackendVLLM, "vllm"},
	}
	for _, tt := range tests {
		if got := FormatRuntimeLabel(tt.b); got != tt.want {
			t.Fatalf("FormatRuntimeLabel(%v): got %q want %q", tt.b, got, tt.want)
		}
	}
}
