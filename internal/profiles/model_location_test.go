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
