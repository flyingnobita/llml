package profiles

import (
	"slices"
	"testing"
)

// Import and export used to encode the model-location rule separately and
// disagreed. This asserts they now answer the same way for every key the rules
// name, in both cases.
func TestLocationRules_importAndExportAgree(t *testing.T) {
	t.Parallel()

	for backend, rules := range rulesByBackend {
		for key := range rules.envKeys {
			t.Run(backend+"/"+key, func(t *testing.T) {
				t.Parallel()

				kept, _, dropped, _ := StripModelLocationParams(backend,
					[]PortableEnvVar{{Key: key, Value: "v"}}, nil)
				if len(dropped) != 1 || len(kept) != 0 {
					t.Errorf("import should strip %q for %s (kept %v, dropped %v)", key, backend, kept, dropped)
				}
				if !ShouldExcludeEnv(key) {
					t.Errorf("export should exclude %q", key)
				}
			})
		}
	}
}

// Env key matching is case-insensitive, so a lowercase key in a hand-written
// profile is stripped just like the canonical uppercase spelling.
func TestLocationRules_envMatchingIsCaseInsensitive(t *testing.T) {
	t.Parallel()

	for _, key := range []string{"llama_arg_model", "Llama_Cache", "hf_token"} {
		if !ggufRules.isEnv(key) {
			t.Errorf("ggufRules should match %q", key)
		}
		if !ShouldExcludeEnv(key) {
			t.Errorf("ShouldExcludeEnv should match %q", key)
		}
	}
}

func TestLocationRules_argMatchingUsesFirstToken(t *testing.T) {
	t.Parallel()

	if !ggufRules.isArg("-m /models/a.gguf") {
		t.Error("should match a flag with its value")
	}
	if !ggufRules.isArg("--mmproj") {
		t.Error("should match a bare flag")
	}
	if ggufRules.isArg("--ctx-size 4096") {
		t.Error("must not match an unrelated flag")
	}
	if ggufRules.isArg("") {
		t.Error("must not match an empty row")
	}
}

// llama and koboldcpp accept the same flags, so their rules must stay identical.
func TestLocationRules_ggufBackendsShareOneTable(t *testing.T) {
	t.Parallel()

	llama, kobold := rulesByBackend["llama"], rulesByBackend["koboldcpp"]
	for key := range llama.envKeys {
		if !kobold.isEnv(key) {
			t.Errorf("koboldcpp should strip env %q", key)
		}
	}
	for tok := range llama.argTokens {
		if !kobold.isArg(tok) {
			t.Errorf("koboldcpp should strip arg %q", tok)
		}
	}
	if !slices.Equal(llama.envPrefixes, kobold.envPrefixes) {
		t.Errorf("env prefixes differ: %v vs %v", llama.envPrefixes, kobold.envPrefixes)
	}
}

// An unknown backend has no rules, so nothing is stripped. Dropping everything
// (or panicking) on an unrecognized backend would be worse than passing it through.
func TestStripModelLocationParams_unknownBackendKeepsEverything(t *testing.T) {
	t.Parallel()

	env := []PortableEnvVar{{Key: "LLAMA_ARG_MODEL", Value: "/a.gguf"}}
	args := []string{"-m /a.gguf"}
	keptEnv, keptArgs, droppedEnv, droppedArgs := StripModelLocationParams("nonesuch", env, args)

	if len(keptEnv) != 1 || len(keptArgs) != 1 {
		t.Errorf("unknown backend should keep everything, got env %v args %v", keptEnv, keptArgs)
	}
	if len(droppedEnv) != 0 || len(droppedArgs) != 0 {
		t.Errorf("unknown backend should drop nothing, got %v %v", droppedEnv, droppedArgs)
	}
}

// The export filter must cover keys that only some backend names, since it has
// no backend context of its own.
func TestShouldExcludeEnv_usesUnionAcrossBackends(t *testing.T) {
	t.Parallel()

	// vLLM-only key.
	if !ShouldExcludeEnv("VLLM_CACHE_ROOT") {
		t.Error("should exclude a vLLM model-location key")
	}
	// GGUF-only key.
	if !ShouldExcludeEnv("KOBOLDCPP_MMPROJ") {
		t.Error("should exclude a KoboldCpp model-location key")
	}
	if ShouldExcludeEnv("CUDA_VISIBLE_DEVICES") {
		t.Error("must not exclude an unrelated key")
	}
	if ShouldExcludeEnv("") || ShouldExcludeEnv("   ") {
		t.Error("must not exclude an empty key")
	}
}

// docs/profile-format.md section 8 documents this rule; keep the doc honest by
// asserting the shapes it promises are the ones the code implements.
func TestLocationRules_documentedShapesArePresent(t *testing.T) {
	t.Parallel()

	for _, key := range []string{"LLAMA_CACHE", "HF_HOME", "HUGGINGFACE_HUB_CACHE"} {
		if !ShouldExcludeEnv(key) {
			t.Errorf("documented key %q is not excluded", key)
		}
	}
	// The documented LLAMA_ARG_ prefix rule covers keys the tables never list.
	if !ggufRules.isEnv("LLAMA_ARG_SOMETHING_NEW") {
		t.Error("documented LLAMA_ARG_ prefix rule is missing")
	}
}

// ninfer-serve takes the artifact positionally, so only the chat template
// override is machine-specific; tuning flags must survive import.
func TestStripModelLocationParams_ninfer(t *testing.T) {
	t.Parallel()

	args := []string{"--chat-template /home/u/t.jinja", "--kv-dtype fp8", "--spec mtp", "--draft-tokens 3"}
	_, kept, _, dropped := StripModelLocationParams("ninfer", nil, args)
	if want := []string{"--kv-dtype fp8", "--spec mtp", "--draft-tokens 3"}; !slices.Equal(kept, want) {
		t.Errorf("kept = %v, want %v", kept, want)
	}
	if len(dropped) != 1 {
		t.Errorf("dropped = %v, want the chat template only", dropped)
	}
	if got := NormalizeBackendInput(" NInfer "); got != "ninfer" {
		t.Errorf("NormalizeBackendInput = %q", got)
	}
}

// mlx_lm.server takes the model, a LoRA adapter, and a draft model by local
// path or Hugging Face repo id, and HF_TOKEN reaches the Hub. llml supplies
// the model, so all of them are stripped; server tuning flags are kept.
func TestStripModelLocationParams_mlxLM(t *testing.T) {
	t.Parallel()

	env := []PortableEnvVar{{Key: "HF_TOKEN", Value: "hf_x"}, {Key: "MLX_METAL_DEBUG", Value: "1"}}
	args := []string{
		"--model mlx-community/Qwen3-8B-4bit",
		"--adapter-path /home/u/adapters/a",
		"--draft-model mlx-community/Qwen3-0.6B-4bit",
		"--max-tokens 4096",
		"--chat-template-args {\"enable_thinking\":false}",
		"--num-draft-tokens 3",
	}
	keptEnv, keptArgs, droppedEnv, droppedArgs := StripModelLocationParams("mlx-lm", env, args)

	if len(keptEnv) != 1 || keptEnv[0].Key != "MLX_METAL_DEBUG" || len(droppedEnv) != 1 {
		t.Errorf("env: kept %v, dropped %v, want HF_TOKEN dropped", keptEnv, droppedEnv)
	}
	if want := args[3:]; !slices.Equal(keptArgs, want) {
		t.Errorf("kept args = %v, want %v", keptArgs, want)
	}
	if len(droppedArgs) != 3 {
		t.Errorf("dropped args = %v, want the model, adapter, and draft", droppedArgs)
	}
	if !ShouldExcludeEnv("HF_TOKEN") {
		t.Error("export should exclude HF_TOKEN")
	}
}

// A portable profile for mlx-lm keeps its backend through parsing and
// conversion to a local profile.
func TestPortableProfile_mlxLMBackend(t *testing.T) {
	t.Parallel()

	body := []byte("schema_version = 3\n\n[[profiles]]\nname = \"long\"\nbackend = \"mlx-lm\"\nargs = [\"--max-tokens 8192\"]\n")
	f, err := parsePortable(body)
	if err != nil {
		t.Fatal(err)
	}
	p := PortableToProfile(f.Profiles[0])
	if p.Backend != "mlx-lm" {
		t.Errorf("backend = %q, want mlx-lm", p.Backend)
	}
	if got := NormalizeBackendInput(" MLX-LM "); got != "mlx-lm" {
		t.Errorf("NormalizeBackendInput = %q, want mlx-lm", got)
	}
	if got := ProfileToPortable(p, "/m/qwen").Backend; got != "mlx-lm" {
		t.Errorf("exported backend = %q, want mlx-lm", got)
	}
}

// mlx_vlm.server takes the model, a LoRA adapter, a draft model, extra model
// folders (--model-dir, 0.7.x), and the image, speech, embedding, and reranker
// models it can also serve (0.7.x) by local path or Hugging Face repo id,
// and HF_TOKEN reaches the Hub. llml supplies the model, so all of them are
// stripped; server tuning flags are kept.
func TestStripModelLocationParams_mlxVLM(t *testing.T) {
	t.Parallel()

	env := []PortableEnvVar{{Key: "HF_TOKEN", Value: "hf_x"}, {Key: "MLX_METAL_DEBUG", Value: "1"}}
	args := []string{
		"--model mlx-community/Qwen2.5-VL-7B-Instruct-4bit",
		"--adapter-path /home/u/adapters/a",
		"--draft-model mlx-community/Qwen3-0.6B-4bit",
		"--model-dir /home/u/models",
		"--image-model mlx-community/FLUX.1-schnell-4bit",
		"--tts-model mlx-community/Kokoro-82M-bf16",
		"--stt-model mlx-community/whisper-large-v3-turbo",
		"--embedding-model mlx-community/bge-small-en-v1.5",
		"--reranker-model mlx-community/bge-reranker-base",
		"--max-tokens 4096",
		"--kv-bits 4",
		"--trust-remote-code",
	}
	keptEnv, keptArgs, droppedEnv, droppedArgs := StripModelLocationParams("mlx-vlm", env, args)

	if len(keptEnv) != 1 || keptEnv[0].Key != "MLX_METAL_DEBUG" || len(droppedEnv) != 1 {
		t.Errorf("env: kept %v, dropped %v, want HF_TOKEN dropped", keptEnv, droppedEnv)
	}
	if want := args[9:]; !slices.Equal(keptArgs, want) {
		t.Errorf("kept args = %v, want %v", keptArgs, want)
	}
	if len(droppedArgs) != 9 {
		t.Errorf("dropped args = %v, want every model-location flag", droppedArgs)
	}
}

// A portable profile for mlx-vlm keeps its backend through parsing and
// conversion to a local profile, and is not mistaken for mlx-lm.
func TestPortableProfile_mlxVLMBackend(t *testing.T) {
	t.Parallel()

	body := []byte("schema_version = 3\n\n[[profiles]]\nname = \"vision\"\nbackend = \"mlx-vlm\"\nargs = [\"--max-tokens 8192\"]\n")
	f, err := parsePortable(body)
	if err != nil {
		t.Fatal(err)
	}
	p := PortableToProfile(f.Profiles[0])
	if p.Backend != "mlx-vlm" {
		t.Errorf("backend = %q, want mlx-vlm", p.Backend)
	}
	if got := NormalizeBackendInput(" MLX-VLM "); got != "mlx-vlm" {
		t.Errorf("NormalizeBackendInput = %q, want mlx-vlm", got)
	}
	if got := NormalizeBackendInput("mlx_vlm"); got != "" {
		t.Errorf("NormalizeBackendInput(mlx_vlm) = %q, want no backend", got)
	}
	if got := ProfileToPortable(p, "/m/qwen-vl").Backend; got != "mlx-vlm" {
		t.Errorf("exported backend = %q, want mlx-vlm", got)
	}
}
