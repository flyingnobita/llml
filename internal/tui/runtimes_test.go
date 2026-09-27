package tui

import (
	"slices"
	"testing"

	"github.com/flyingnobita/llml/internal/models"
)

// The table lists Runtimes grouped by the Model Format they run, in the order
// the runtime panel shows them.
func TestRuntimeTableGroupOrder(t *testing.T) {
	t.Parallel()

	type entry struct{ format, name string }
	want := []entry{
		{"GGUF", "Llama.cpp"},
		{"GGUF", "KoboldCpp"},
		{"Safetensors", "vLLM"},
		{"Safetensors", "oMLX"},
		{"NInfer", "NInfer"},
		{"Splash bundle", "Splash"},
		{"Ollama library", "Ollama"},
	}
	var got []entry
	for _, rt := range runtimeTable {
		got = append(got, entry{rt.format.String(), rt.name})
	}
	if !slices.Equal(got, want) {
		t.Errorf("runtime table order\n got %v\nwant %v", got, want)
	}
}

// Every field of the runtime panel belongs to exactly one Runtime, in the
// order the panel shows it, and names the environment variable that overrides it.
func TestRuntimeTableFields(t *testing.T) {
	t.Parallel()

	want := map[models.ModelBackend][]string{
		models.BackendLlama:  {"LLAMA_CPP_PATH", "LLAMA_SERVER_PORT", "LLAMA_SERVER_HOST"},
		models.BackendKobold: {"KOBOLDCPP_PATH", "KOBOLDCPP_PORT"},
		models.BackendVLLM:   {"VLLM_PATH", "VLLM_VENV", "VLLM_SERVER_PORT", "VLLM_SERVER_HOST"},
		models.BackendOMLX:   {"OMLX_PATH", "OMLX_PORT", "OMLX_HOST"},
		models.BackendNInfer: {"NINFER_PATH", "NINFER_SERVER_PORT", "NINFER_SERVER_HOST"},
		models.BackendSplash: {"SPLASH_PATH", "SPLASH_PORT", "SPLASH_HOST"},
		models.BackendOllama: {"OLLAMA_PATH", "OLLAMA_HOST"},
	}
	seen := map[runtimeField]bool{}
	for _, rt := range runtimeTable {
		var envs []string
		for _, f := range rt.fields {
			if seen[f.field] {
				t.Errorf("field %d listed twice", f.field)
			}
			seen[f.field] = true
			envs = append(envs, f.env)
		}
		if !slices.Equal(envs, want[rt.backend]) {
			t.Errorf("%s fields: got %v, want %v", rt.name, envs, want[rt.backend])
		}
	}
	if len(seen) != int(runtimeFieldCount) {
		t.Errorf("table covers %d fields, want %d", len(seen), runtimeFieldCount)
	}
}

// Detection status reads "found" from the program lookup and "running" from
// the server probe, per Runtime.
func TestRuntimeTableStatus(t *testing.T) {
	t.Setenv("PATH", t.TempDir()) // no program is found through PATH lookup

	r := models.RuntimeInfo{
		LlamaServerPath:  "/opt/llama/bin/llama-server",
		KoboldCppRunning: true,
		VLLMPath:         "/opt/vllm/bin/vllm",
		OMLXPath:         "/opt/omlx/bin/omlx",
		OMLXRunning:      true,
		NInferRunning:    true,
		OllamaRunning:    true,
	}
	want := map[models.ModelBackend]runtimeStatus{
		models.BackendLlama:  {found: true},
		models.BackendKobold: {running: true},
		models.BackendVLLM:   {found: true},
		models.BackendOMLX:   {found: true, running: true},
		models.BackendNInfer: {running: true},
		models.BackendSplash: {},
		models.BackendOllama: {running: true},
	}
	for _, rt := range runtimeTable {
		if got := rt.status(r); got != want[rt.backend] {
			t.Errorf("%s status = %+v, want %+v", rt.name, got, want[rt.backend])
		}
	}
}
