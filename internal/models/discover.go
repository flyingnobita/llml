// Package models provides GGUF and safetensors model discovery, metadata extraction,
// llama.cpp / vLLM binary detection, and display-formatting helpers for the LLM Launcher TUI.
package models

import (
	"os"
	"path/filepath"
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
	// ID is the stable row identity and parameter-profile key. For path-backed
	// models it matches Path; for Ollama it is model[:tag].
	ID string
	// Path is the filesystem launch target for path-backed models. It is empty for
	// backends that do not launch from a local path.
	Path string
	// Location is the displayable source string. When empty, Path is used.
	Location string
	// Name is the File Name column: leaf of Path (.gguf file name or safetensors dir name).
	Name    string
	Size    int64
	ModTime time.Time
	// Parameters is GGUF metadata for BackendLlama; for BackendVLLM it summarizes config.json.
	Parameters string
}

// Identity returns the stable per-row key used for selection, caching, and parameter profiles.
func (f ModelFile) Identity() string {
	if id := strings.TrimSpace(f.ID); id != "" {
		return id
	}
	if p := strings.TrimSpace(f.Path); p != "" {
		return filepath.Clean(p)
	}
	return ""
}

// DisplayLocation returns the source string shown in the table path/location column.
func (f ModelFile) DisplayLocation() string {
	if loc := strings.TrimSpace(f.Location); loc != "" {
		return loc
	}
	if p := strings.TrimSpace(f.Path); p != "" {
		return filepath.Clean(p)
	}
	return f.Identity()
}

// LaunchTarget returns the backend-specific target used by preview and launch code.
func (f ModelFile) LaunchTarget() string {
	if f.Backend == BackendOllama {
		return f.Identity()
	}
	return filepath.Clean(f.Path)
}

// Options configures discovery.
type Options struct {
	ExtraRoots        []string
	MaxDepth          int
	SkipDefaultRoots  bool
	DisableAPISources bool
}

// candidate is an internal (source-index, path) pair used during filesystem scan.
type candidate struct {
	srcIdx int
	path   string
}

// collectCandidates walks all roots and returns deduplicated (source, path) pairs in
// walk order. Non-existent roots are silently skipped.
func collectCandidates(roots []string, sources []modelSource, maxD int) ([]candidate, error) {
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
	return ordered, nil
}

// buildModelFiles converts candidates to ModelFile values, filtering auxiliary models.
func buildModelFiles(candidates []candidate, sources []modelSource) []ModelFile {
	var out []ModelFile
	for _, c := range candidates {
		mf, ok := sources[c.srcIdx].build(c.path)
		if !ok {
			continue
		}
		if isAuxiliaryModel(mf) {
			continue
		}
		out = append(out, mf)
	}
	return out
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

	candidates, err := collectCandidates(roots, sources, maxD)
	if err != nil {
		return nil, err
	}

	out := buildModelFiles(candidates, sources)
	if !opts.DisableAPISources {
		if ollamaRows, err := DiscoverOllamaModels(); err == nil {
			out = append(out, ollamaRows...)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		return compareForDefaultOrder(out[i], out[j])
	})
	return out, nil
}

func compareForDefaultOrder(a, b ModelFile) bool {
	al := a.DisplayLocation()
	bl := b.DisplayLocation()
	if al != bl {
		return al < bl
	}
	return a.Identity() < b.Identity()
}

// isAuxiliaryModel drops non-LLM weight files (e.g. CLIP/mmproj sidecars in multimodal repos).
// Applied to both GGUF and safetensors models after Parameters is populated.
func isAuxiliaryModel(f ModelFile) bool {
	switch strings.TrimSpace(strings.ToLower(f.Parameters)) {
	case "clip", "flip":
		return true
	}
	return isMMProjName(f.Name)
}
