package settings

import (
	"path/filepath"

	"github.com/flyingnobita/llml/internal/fsutil"
)

// HuggingFaceHubCache returns the Hugging Face Hub "models--*" directory root,
// following the same precedence as huggingface_hub itself: HUGGINGFACE_HUB_CACHE,
// then $HF_HOME/hub, then ~/.cache/huggingface/hub.
//
// See https://huggingface.co/docs/huggingface_hub/package_reference/environment_variables
func (s Settings) HuggingFaceHubCache(home string) string {
	if s.HFHubCache != "" {
		return s.HFHubCache
	}
	if s.HFHome != "" {
		return filepath.Join(s.HFHome, "hub")
	}
	return filepath.Join(home, ".cache", "huggingface", "hub")
}

// DefaultSearchRoots returns the common directories where GGUF weights are
// stored for llama.cpp workflows. It returns nil when the home directory cannot
// be resolved.
func (s Settings) DefaultSearchRoots() []string {
	home := fsutil.HomeDir()
	if home == "" {
		return nil
	}
	return []string{
		filepath.Join(home, "models"),
		filepath.Join(home, ".cache", "llama.cpp"),
		s.HuggingFaceHubCache(home),
		filepath.Join(home, ".cache", "lm-studio", "models"),
	}
}

// SearchRoots combines the default roots, the resolved ExtraModelPaths, and any
// caller-provided extras into one deduplicated, normalized list. When
// skipDefaults is true the home directories are omitted, which isolated scans
// and tests rely on. Roots that do not exist are skipped later, during discovery.
func (s Settings) SearchRoots(extra []string, skipDefaults bool) []string {
	var ps fsutil.PathSet
	if !skipDefaults {
		ps.Add(s.DefaultSearchRoots()...)
	}
	ps.Add(s.ExtraModelPaths...)
	ps.Add(extra...)
	return ps.Slice()
}

// OllamaAPIBaseURL returns the Ollama API base URL for the resolved host.
func (s Settings) OllamaAPIBaseURL() string {
	return "http://" + s.OllamaHost + "/api"
}
