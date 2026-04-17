// Package models provides GGUF and safetensors model discovery, metadata extraction,
// llama.cpp / vLLM binary detection, and display-formatting helpers for the LLM Launcher TUI.
package models

import (
	"os"
	"sort"
	"strings"
	"time"
)

// DefaultMaxDepth limits recursion depth below each search root.
const DefaultMaxDepth = 12

var skipDirNames = map[string]struct{}{
	".git":          {},
	"node_modules":  {},
	"__pycache__":   {},
	".venv":         {},
	"venv":          {},
	"dist":          {},
	"build":         {},
	".mypy_cache":   {},
	".pytest_cache": {},
}

// ModelFile is one local model (GGUF file or Hugging Face-style safetensors directory)
// plus parsed metadata for the Parameters column.
type ModelFile struct {
	Backend ModelBackend
	Path    string
	// Name is the File Name column: leaf of Path (.gguf file name or safetensors dir name).
	Name    string
	Size    int64
	ModTime time.Time
	// Parameters is GGUF metadata for BackendLlama; for BackendVLLM it summarizes config.json.
	Parameters string
}

// Options configures discovery.
type Options struct {
	ExtraRoots       []string
	MaxDepth         int
	SkipDefaultRoots bool
}

// Discover scans configured paths for .gguf files and Hugging Face-style safetensors directories
// (config.json + *.safetensors) in a single filesystem walk, dedupes, sorts by path, and fills
// Parameters. isAuxiliaryModel is applied to both backends after Parameters is populated.
func Discover(opts Options) ([]ModelFile, error) {
	maxD := opts.MaxDepth
	if maxD <= 0 {
		maxD = DefaultMaxDepth
	}
	roots := MergeSearchRoots(opts.ExtraRoots, opts.SkipDefaultRoots)

	sources := []modelSource{ggufSource{}, safetensorsSource{}}

	type candidate struct {
		srcIdx int
		path   string
	}
	seen := make(map[candidate]struct{})
	var ordered []candidate

	for _, root := range roots {
		if st, err := os.Stat(root); err != nil || !st.IsDir() {
			continue
		}
		if err := walkSearchTree(root, maxD, func(full, parentDir string, ent os.DirEntry, _ int) error {
			for i, src := range sources {
				if p := src.match(full, parentDir, ent); p != "" {
					c := candidate{i, p}
					if _, ok := seen[c]; !ok {
						seen[c] = struct{}{}
						ordered = append(ordered, c)
					}
				}
			}
			return nil
		}); err != nil {
			return nil, err
		}
	}

	var out []ModelFile
	for _, c := range ordered {
		mf, ok := sources[c.srcIdx].build(c.path)
		if !ok {
			continue
		}
		if isAuxiliaryModel(mf) {
			continue
		}
		out = append(out, mf)
	}

	sort.Slice(out, func(i, j int) bool {
		return out[i].Path < out[j].Path
	})
	return out, nil
}

// isAuxiliaryModel drops non-LLM weight files (e.g. CLIP/mmproj sidecars in multimodal repos).
// Applied to both GGUF and safetensors models after Parameters is populated.
func isAuxiliaryModel(f ModelFile) bool {
	switch strings.TrimSpace(strings.ToLower(f.Parameters)) {
	case "clip", "flip":
		return true
	}
	return strings.Contains(strings.ToLower(f.Name), "mmproj")
}
