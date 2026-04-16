package llamacpp

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestHfParamsSummary_missingConfig(t *testing.T) {
	d := t.TempDir()
	if g := hfParamsSummary(d); g != "vllm · —" {
		t.Fatalf("got %q want %q", g, "vllm · —")
	}
}

func TestHfParamsSummary_withConfig(t *testing.T) {
	d := t.TempDir()
	cfg := map[string]any{
		"model_type":    "llama",
		"architectures": []string{"LlamaForCausalLM"},
	}
	b, err := json.Marshal(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(d, hfConfigFileName), b, 0o644); err != nil {
		t.Fatal(err)
	}
	got := hfParamsSummary(d)
	for _, sub := range []string{"vllm", "llama", "LlamaForCausalLM"} {
		if !strings.Contains(got, sub) {
			t.Fatalf("summary %q should contain %q", got, sub)
		}
	}
}

func TestCollectSafetensorModelDirs_findsDir(t *testing.T) {
	root := t.TempDir()
	weights := filepath.Join(root, "repo", "snap", "w")
	if err := os.MkdirAll(weights, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(weights, "model.safetensors"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	out := make(map[string]struct{})
	if err := collectSafetensorModelDirs(root, DefaultMaxDepth, out); err != nil {
		t.Fatal(err)
	}
	if _, ok := out[filepath.Clean(weights)]; !ok {
		t.Fatalf("expected weights dir in map, got %v", out)
	}
}

func TestTryVLLMModelDir_validLayout(t *testing.T) {
	d := t.TempDir()
	cfg := map[string]any{"model_type": "llama"}
	b, err := json.Marshal(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(d, hfConfigFileName), b, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(d, "w.safetensors"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	mf, ok := tryVLLMModelDir(d)
	if !ok {
		t.Fatal("expected valid model dir")
	}
	if mf.Backend != BackendVLLM {
		t.Fatalf("backend %v", mf.Backend)
	}
	if mf.Path != filepath.Clean(d) {
		t.Fatalf("path %q", mf.Path)
	}
}
