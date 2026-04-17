package models

import (
	"os"
	"path/filepath"
	"strings"
)

// ExpandTildePath trims s and, if it is "~" or begins with "~/", replaces that prefix with the
// current user's home directory from [os.UserHomeDir]. If the home directory cannot be resolved,
// the trimmed input is returned unchanged. Other paths are returned trimmed only.
func ExpandTildePath(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	if s == "~" || strings.HasPrefix(s, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return s
		}
		if s == "~" {
			return home
		}
		return filepath.Join(home, strings.TrimPrefix(s, "~/"))
	}
	return s
}

// huggingFaceHubCache returns the Hugging Face Hub "models--*" directory root.
// It respects the same env vars as huggingface_hub: HUGGINGFACE_HUB_CACHE, then HF_HOME/hub.
// See https://huggingface.co/docs/huggingface_hub/package_reference/environment_variables
func huggingFaceHubCache(home string) string {
	if v := os.Getenv(EnvHFHubCache); v != "" {
		return filepath.Clean(v)
	}
	if v := os.Getenv(EnvHFHome); v != "" {
		return filepath.Join(filepath.Clean(v), "hub")
	}
	return filepath.Join(home, ".cache", "huggingface", "hub")
}

// DefaultSearchRoots returns common directories where GGUF weights are stored for llama.cpp workflows.
func DefaultSearchRoots() []string {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil
	}
	return []string{
		filepath.Join(home, "models"),
		filepath.Join(home, ".cache", "llama.cpp"),
		huggingFaceHubCache(home),
		filepath.Join(home, ".cache", "lm-studio", "models"),
	}
}

// MergeSearchRoots combines defaults, optional extras from the environment, and caller-provided dirs.
// If skipDefaults is true, default home directories are omitted (tests, isolated scans).
// Entries that do not exist are skipped later during discovery.
func MergeSearchRoots(extra []string, skipDefaults bool) []string {
	seen := make(map[string]struct{})
	var out []string

	add := func(p string) {
		p = filepath.Clean(ExpandTildePath(p))
		if p == "" || p == "." {
			return
		}
		if _, ok := seen[p]; ok {
			return
		}
		seen[p] = struct{}{}
		out = append(out, p)
	}
	if !skipDefaults {
		for _, p := range DefaultSearchRoots() {
			add(p)
		}
	}
	if v := os.Getenv(EnvModelPaths); v != "" {
		for _, part := range strings.Split(v, ",") {
			part = strings.TrimSpace(part)
			if part != "" {
				add(part)
			}
		}
	}
	for _, p := range extra {
		if p != "" {
			add(p)
		}
	}
	return out
}
