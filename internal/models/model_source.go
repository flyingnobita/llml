package models

import (
	"os"
	"path/filepath"
	"strings"
)

// modelSource matches walk entries and builds ModelFile values.
// GGUF is file-based (build path = the .gguf file itself);
// safetensors is directory-based (build path = the parent dir of any .safetensors file).
type modelSource interface {
	// match returns the path to pass to build for this walk entry, or "" to skip.
	match(full, parentDir string, ent os.DirEntry) string
	// build creates a ModelFile for the given path, or (ModelFile{}, false) to skip.
	build(path string) (ModelFile, bool)
}

type ggufSource struct{}

func (ggufSource) match(full, _ string, _ os.DirEntry) string {
	if strings.EqualFold(filepath.Ext(full), ".gguf") {
		return filepath.Clean(full)
	}
	return ""
}

func (ggufSource) build(path string) (ModelFile, bool) {
	fi, err := os.Stat(path) // follows symlinks — needed for HF hub blob symlinks
	if err != nil {
		return ModelFile{}, false
	}
	return ModelFile{
		Backend:    BackendLlama,
		Path:       path,
		Name:       filepath.Base(path),
		Size:       fi.Size(),
		ModTime:    fi.ModTime(),
		Parameters: ggufParamsSummary(path),
	}, true
}

type safetensorsSource struct{}

func (safetensorsSource) match(full, parentDir string, _ os.DirEntry) string {
	if strings.EqualFold(filepath.Ext(full), ".safetensors") {
		return filepath.Clean(parentDir)
	}
	return ""
}

func (safetensorsSource) build(path string) (ModelFile, bool) {
	return tryVLLMModelDir(path)
}
