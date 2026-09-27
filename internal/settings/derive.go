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

// DefaultSearchRoots returns the common directories where model weights are
// stored: the llama.cpp, Hugging Face, and LM Studio caches, plus the models/
// directory of a configured NInfer checkout. It returns nil when the home
// directory cannot be resolved.
func (s Settings) DefaultSearchRoots() []string {
	home := fsutil.HomeDir()
	if home == "" {
		return nil
	}
	roots := []string{
		filepath.Join(home, "models"),
		filepath.Join(home, ".cache", "llama.cpp"),
		s.HuggingFaceHubCache(home),
		filepath.Join(home, ".cache", "lm-studio", "models"),
	}
	if d := s.NInferModelsDir(); d != "" {
		roots = append(roots, d)
	}
	return roots
}

// NInferModelsDir returns the models/ directory of the configured NInfer
// checkout, where NInfer's own download instructions place *.ninfer
// artifacts. It is "" when NInferPath is unset. NInferPath may also name the
// build output directory (build/apps) or the ninfer-serve binary inside it,
// since NInfer has no install target; both are walked back to the checkout
// root. The directory is not checked for existence here; discovery skips
// roots that do not exist.
func (s Settings) NInferModelsDir() string {
	p := s.NInferPath
	if p == "" {
		return ""
	}
	if filepath.Base(p) == "ninfer-serve" {
		p = filepath.Dir(p)
	}
	if filepath.Base(p) == "apps" && filepath.Base(filepath.Dir(p)) == "build" {
		p = filepath.Dir(filepath.Dir(p))
	}
	return filepath.Join(p, "models")
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
