// Package llamacpp provides GGUF model discovery, metadata extraction, llama.cpp
// binary detection, and display-formatting helpers for the llm-launch TUI.
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

// ModelFile is one GGUF on disk plus parsed metadata for the Parameters column.
type ModelFile struct {
	Path       string
	Name       string
	Size       int64
	ModTime    time.Time
	Parameters string
}

// Options configures discovery.
type Options struct {
	ExtraRoots       []string
	MaxDepth         int
	SkipDefaultRoots bool
}

// Discover scans configured paths for .gguf files, dedupes by path, sorts by path, and fills Parameters.
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
		if err := discoverWalkRoot(root, maxD, seen, &out); err != nil {
			return nil, err
		}
	}

	sort.Slice(out, func(i, j int) bool {
		return out[i].Path < out[j].Path
	})

	filtered := out[:0]
	for i := range out {
		out[i].Parameters = parametersSummary(out[i].Path)
		if skipListedModel(out[i]) {
			continue
		}
		filtered = append(filtered, out[i])
	}

	return filtered, nil
}

// discoverWalkRoot walks root recursively. Unlike [filepath.WalkDir], it follows symbolic
// links to directories — some Hugging Face hub layouts symlink `models--org--repo` trees,
// which plain WalkDir would skip entirely.
func discoverWalkRoot(root string, maxD int, seen map[string]struct{}, out *[]ModelFile) error {
	var walk func(string) error
	walk = func(dir string) error {
		entries, err := os.ReadDir(dir)
		if err != nil {
			return nil
		}
		for _, ent := range entries {
			name := ent.Name()
			full := filepath.Join(dir, name)
			rel, err := filepath.Rel(root, full)
			if err != nil {
				continue
			}
			depth := strings.Count(rel, string(filepath.Separator))

			st, err := os.Stat(full)
			if err != nil {
				continue
			}
			if st.IsDir() {
				if _, skip := skipDirNames[name]; skip {
					continue
				}
				if depth >= maxD {
					continue
				}
				if err := walk(full); err != nil {
					return err
				}
				continue
			}

			if depth > maxD {
				continue
			}
			if !strings.EqualFold(filepath.Ext(full), ".gguf") {
				continue
			}

			clean := filepath.Clean(full)
			if _, ok := seen[clean]; ok {
				continue
			}
			seen[clean] = struct{}{}

			size, modTime := fileInfoSizeModTime(full, ent)
			if size < 0 {
				continue
			}

			*out = append(*out, ModelFile{
				Path:    clean,
				Name:    filepath.Base(clean),
				Size:    size,
				ModTime: modTime,
			})
		}
		return nil
	}
	return walk(root)
}

// skipListedModel drops non-LLM weight files (e.g. CLIP/mmproj sidecars in multimodal repos).
func skipListedModel(f ModelFile) bool {
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
