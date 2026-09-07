package profiles

import "strings"

// Model-location parameters are the env vars and args that say *which* model to
// load or where to find it. llml supplies those itself at launch, so they are
// stripped on import and excluded from export.
//
// This file is the single source for that rule. It previously existed three
// times — stripEnvByBackend, stripArgByBackend, and the export-side
// envExcludePatterns/envExcludePrefixes — which disagreed with each other and
// with docs/profile-format.md section 8.

// locationRules is the set of env keys, env key prefixes, and argument flags
// that identify model-location parameters for one backend.
type locationRules struct {
	// envKeys are matched case-insensitively against the whole key.
	envKeys map[string]bool
	// envPrefixes are matched case-insensitively against the start of the key.
	envPrefixes []string
	// argTokens are matched against an argument's first token.
	argTokens map[string]bool
}

// isEnv reports whether key names a model-location environment variable.
func (r locationRules) isEnv(key string) bool {
	upper := strings.ToUpper(strings.TrimSpace(key))
	if upper == "" {
		return false
	}
	if r.envKeys[upper] {
		return true
	}
	for _, p := range r.envPrefixes {
		if strings.HasPrefix(upper, p) {
			return true
		}
	}
	return false
}

// isArg reports whether an argument row starts with a model-location flag.
func (r locationRules) isArg(arg string) bool {
	return r.argTokens[argFirstToken(arg)]
}

// ggufRules covers the GGUF backends. llama.cpp and KoboldCpp accept the same
// model-location flags and environment variables, so they share one value
// rather than two tables that must be kept identical by hand.
var ggufRules = locationRules{
	envKeys: map[string]bool{
		"LLAMA_CACHE": true,
		"HF_TOKEN":    true,
		// KoboldCpp's own model-location variables.
		"KOBOLDCPP_MODEL":      true,
		"KOBOLDCPP_LORA":       true,
		"KOBOLDCPP_MMPROJ":     true,
		"KOBOLDCPP_TOKENIZER":  true,
		"KOBOLDCPP_MODELS_DIR": true,
	},
	// Every LLAMA_ARG_* variable mirrors a llama.cpp flag, and the ones that
	// matter here (MODEL, HF_REPO, MMPROJ, MODELS_DIR, …) are all locations.
	envPrefixes: []string{"LLAMA_ARG_"},
	argTokens: map[string]bool{
		"-m": true, "--model": true,
		"-mu": true, "--model-url": true,
		"-md": true, "--model-draft": true,
		"-mv": true, "--model-vocoder": true,
		"-hf": true, "-hfr": true, "--hf-repo": true,
		"-hff": true, "--hf-file": true,
		"-hfd": true, "-hfrd": true, "--hf-repo-draft": true,
		"-hfv": true, "-hfrv": true, "--hf-repo-v": true,
		"-hffv": true, "--hf-file-v": true,
		"-hft": true, "--hf-token": true,
		"-dr": true, "--docker-repo": true,
		"-mm": true, "--mmproj": true,
		"-mmu": true, "--mmproj-url": true,
		"--lora": true, "--lora-scaled": true, "--lora-init-without-apply": true,
		"--control-vector": true, "--control-vector-scaled": true,
		"--models-dir": true, "--models-preset": true,
		"-lcs": true, "--lookup-cache-static": true,
		"-lcd": true, "--lookup-cache-dynamic": true,
	},
}

// vllmRules covers vLLM, whose model location is expressed through Hugging Face
// settings and cache paths rather than a file argument.
var vllmRules = locationRules{
	envKeys: map[string]bool{
		"HF_HOME":                  true,
		"HF_TOKEN":                 true,
		"HF_HUB_TOKEN":             true,
		"HUGGINGFACE_HUB_CACHE":    true,
		"HUGGING_FACE_HUB_TOKEN":   true,
		"TRANSFORMERS_CACHE":       true,
		"VLLM_CACHE_ROOT":          true,
		"VLLM_ASSETS_CACHE":        true,
		"VLLM_MODEL_REDIRECT_PATH": true,
		"VLLM_XLA_CACHE_PATH":      true,
		"VLLM_USE_MODELSCOPE":      true,
		"MODELSCOPE_CACHE":         true,
	},
	argTokens: map[string]bool{
		"--model": true, "--tokenizer": true,
		"--revision": true, "--code-revision": true, "--tokenizer-revision": true,
		"--hf-config-path": true, "--hf-token": true, "--hf-overrides": true,
		"--download-dir": true, "--load-format": true,
		"--model-loader-extra-config": true, "--config": true,
		"--qlora-adapter-name-or-path": true,
		"--lora-modules":               true, "--prompt-adapters": true,
		"--speculative-config": true, "--speculative-model": true,
		"--tokenizer-pool-extra-config": true,
	},
}

// rulesByBackend maps a backend name to its rules. llama and koboldcpp share
// ggufRules by value, so they cannot drift apart.
var rulesByBackend = map[string]locationRules{
	"llama":     ggufRules,
	"koboldcpp": ggufRules,
	"vllm":      vllmRules,
}

// isAnyBackendLocationEnv reports whether key is a model-location variable for
// any backend. Export has no backend context at the point it filters the
// environment, so it uses the union: a key that would be stripped on import for
// some backend must not be exported for any.
func isAnyBackendLocationEnv(key string) bool {
	for _, r := range rulesByBackend {
		if r.isEnv(key) {
			return true
		}
	}
	return false
}
