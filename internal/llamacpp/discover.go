// Package llamacpp provides GGUF and safetensors model discovery, metadata extraction,
// llama.cpp / vLLM binary detection, and display-formatting helpers for the LLM Launcher TUI.
package llamacpp

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
	Path    string
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
// (config.json + *.safetensors), dedupes, sorts by path, and fills Parameters.
func Discover(opts Options) ([]ModelFile, error) {
	maxD := opts.MaxDepth
	if maxD <= 0 {
		maxD = DefaultMaxDepth
	}
	roots := MergeSearchRoots(opts.ExtraRoots, opts.SkipDefaultRoots)
	seen := make(map[string]struct{})
	var out []ModelFile

	for _, root := range roots {
		st, err := os.Stat(root)
		if err != nil || !st.IsDir() {
			continue
		}
		if err := discoverGGUFModels(root, maxD, seen, &out); err != nil {
			return nil, err
		}
	}

	sort.Slice(out, func(i, j int) bool {
		return out[i].Path < out[j].Path
	})

	filtered := out[:0]
	for i := range out {
		out[i].Parameters = ggufParamsSummary(out[i].Path)
		if isAuxiliaryModel(out[i]) {
			continue
		}
		filtered = append(filtered, out[i])
	}

	vllm, err := discoverVLLMModels(opts, maxD)
	if err != nil {
		return nil, err
	}
	filtered = append(filtered, vllm...)

	sort.Slice(filtered, func(i, j int) bool {
		return filtered[i].Path < filtered[j].Path
	})

	return filtered, nil
}

// discoverGGUFModels walks root recursively. Unlike [filepath.WalkDir], it follows symbolic
// links to directories — some Hugging Face hub layouts symlink `models--org--repo` trees,
// which plain WalkDir would skip entirely.
func discoverGGUFModels(root string, maxD int, seen map[string]struct{}, out *[]ModelFile) error {
	return walkSearchTree(root, maxD, func(full, _ string, ent os.DirEntry, _ int) error {
		if !strings.EqualFold(filepath.Ext(full), ".gguf") {
			return nil
		}

		clean := filepath.Clean(full)
		if _, ok := seen[clean]; ok {
			return nil
		}
		seen[clean] = struct{}{}

		size, modTime := fileInfoSizeModTime(full, ent)
		if size < 0 {
			return nil
		}

		*out = append(*out, ModelFile{
			Backend: BackendLlama,
			Path:    clean,
			Name:    filepath.Base(clean),
			Size:    size,
			ModTime: modTime,
		})
		return nil
	})
}

// isAuxiliaryModel drops non-LLM weight files (e.g. CLIP/mmproj sidecars in multimodal repos).
func isAuxiliaryModel(f ModelFile) bool {
	switch strings.TrimSpace(strings.ToLower(f.Parameters)) {
	case "clip", "flip":
		return true
	}
	return strings.Contains(strings.ToLower(f.Name), "mmproj")
}

// fileInfoSizeModTime returns size and mod time for a walk entry. It prefers [os.Stat],
// which follows symlinks — needed for Hugging Face hub layouts where snapshots/*.gguf
// links into blobs/. Falls back to [os.DirEntry.Info] (lstat) if Stat fails.
func fileInfoSizeModTime(path string, d os.DirEntry) (size int64, modTime time.Time) {
	if fi, err := os.Stat(path); err == nil {
		return fi.Size(), fi.ModTime()
	}
	if fi, err := d.Info(); err == nil {
		return fi.Size(), fi.ModTime()
	}
	return -1, time.Time{}
}
