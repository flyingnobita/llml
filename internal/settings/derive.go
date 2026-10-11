package settings

import (
	"os"
	"path/filepath"
	"strings"

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
// stored: the llama.cpp, Hugging Face, and LM Studio caches, the models/
// directory of a configured NInfer checkout, and oMLX's model directories.
// It returns nil when the home directory cannot be resolved.
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
	return append(roots, s.OMLXModelRoots(home)...)
}

// OMLXModelRoots returns the directories oMLX serves models from: the
// configured OMLXModelDirs, or oMLX's default ~/.omlx/models. The default is
// returned even when oMLX is not installed; discovery skips missing roots.
func (s Settings) OMLXModelRoots(home string) []string {
	if len(s.OMLXModelDirs) > 0 {
		return s.OMLXModelDirs
	}
	if home == "" {
		return nil
	}
	return []string{filepath.Join(home, ".omlx", "models")}
}

// StrataInstallDir maps a Strata path to the install directory that holds
// serve/ and .venv/. It accepts the install directory itself, its
// serve/server.py, its engine/strata, or an engine config file
// (strata-<model>.json) inside it. An empty input returns "".
func StrataInstallDir(p string) string {
	p = strings.TrimSpace(p)
	if p == "" {
		return ""
	}
	p = filepath.Clean(p)
	base := filepath.Base(p)
	if (base == "server.py" && filepath.Base(filepath.Dir(p)) == "serve") ||
		(base == "strata" && filepath.Base(filepath.Dir(p)) == "engine") {
		return filepath.Dir(filepath.Dir(p))
	}
	if fi, err := os.Stat(p); err == nil {
		if fi.IsDir() {
			return p
		}
		return filepath.Dir(p)
	}
	// The path does not exist, so a name with an extension is a file (an
	// engine config, a launcher) inside the install; anything else is the
	// install directory itself.
	if filepath.Ext(base) != "" {
		return filepath.Dir(p)
	}
	return p
}

// StrataRoots returns the Strata install directories to look for engine config
// packs in: the configured StrataPath, the installs Strata's own settings.json
// lists, and the common ~/Applications/strata and ~/Strata locations. Roots
// that do not exist are skipped when they are read.
func (s Settings) StrataRoots(home string) []string {
	var ps fsutil.PathSet
	if d := StrataInstallDir(s.StrataPath); d != "" {
		ps.Add(d)
	}
	for _, p := range s.StrataInstalls {
		if d := StrataInstallDir(p); d != "" {
			ps.Add(d)
		}
	}
	if home != "" {
		ps.Add(filepath.Join(home, "Applications", "strata"), filepath.Join(home, "Strata"))
	}
	return ps.Slice()
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
