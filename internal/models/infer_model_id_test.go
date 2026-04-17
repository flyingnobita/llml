package models

import (
	"path/filepath"
	"testing"
)

func TestInferModelID_empty(t *testing.T) {
	if g := InferModelID(""); g != "" {
		t.Fatalf("got %q", g)
	}
}

func TestInferModelID_huggingfaceHubFolder(t *testing.T) {
	p := filepath.FromSlash("home/u/.cache/huggingface/hub/models--google--gemma-4-E4B-it/snapshots/abc")
	if g := InferModelID(p); g != "google/gemma-4-E4B-it" {
		t.Fatalf("got %q want google/gemma-4-E4B-it", g)
	}
}

func TestInferModelID_huggingfaceMultiSegment(t *testing.T) {
	// Unusual but valid: repo id with extra path segments decode to nested ids.
	p := filepath.FromSlash("/x/hub/models--a--b--c/d")
	if g := InferModelID(p); g != "a/b/c" {
		t.Fatalf("got %q want a/b/c", g)
	}
}

func TestInferModelID_ggufInsideHub_quantizationInFilename(t *testing.T) {
	p := filepath.FromSlash("hub/models--meta-llama--Llama-3.1-8B/snapshots/h/Llama-3.1-8B-Q4_K_M.gguf")
	if g := InferModelID(p); g != "meta-llama/Llama-3.1-8B-Q4_K_M" {
		t.Fatalf("got %q want meta-llama/Llama-3.1-8B-Q4_K_M", g)
	}
}

func TestInferModelID_ggufInsideHub_repoStemUnchangedWhenNoQuantSuffix(t *testing.T) {
	p := filepath.FromSlash("hub/models--meta-llama--Llama-3.1-8B/snapshots/h/Llama-3.1-8B.gguf")
	if g := InferModelID(p); g != "meta-llama/Llama-3.1-8B" {
		t.Fatalf("got %q want meta-llama/Llama-3.1-8B", g)
	}
}

func TestInferModelID_ggufInsideHub_usesFilenameStemUnderProvider(t *testing.T) {
	p := filepath.FromSlash("hub/models--meta-llama--Llama-3.1-8B/snapshots/h/model-Q4_K_M.gguf")
	if g := InferModelID(p); g != "meta-llama/model-Q4_K_M" {
		t.Fatalf("got %q want meta-llama/model-Q4_K_M", g)
	}
}

func TestInferModelID_ggufHub_unsloth_quantization(t *testing.T) {
	p := filepath.FromSlash("hub/models--unsloth--gemma-4-31B-it/snapshots/x/gemma-4-31B-it-Q4_0.gguf")
	if g := InferModelID(p); g != "unsloth/gemma-4-31B-it-Q4_0" {
		t.Fatalf("got %q want unsloth/gemma-4-31B-it-Q4_0", g)
	}
}

func TestInferModelID_plainGguf(t *testing.T) {
	p := filepath.Join("models", "tiny.gguf")
	if g := InferModelID(p); g != "tiny" {
		t.Fatalf("got %q want tiny", g)
	}
}

func TestInferModelID_plainDirectory(t *testing.T) {
	p := filepath.Join("proj", "my-model-dir")
	if g := InferModelID(p); g != "my-model-dir" {
		t.Fatalf("got %q want my-model-dir", g)
	}
}
