package models

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

// splashManifestName is the file at the root of every Splash bundle. A bundle
// holds prepacked weights in target/, draft/, tokenizer/, and vision/ rather
// than safetensors, so nothing else in discovery recognizes it.
const splashManifestName = "manifest.json"

// splashManifest holds the manifest fields llml reads. The format name marks
// the file as Splash's, since manifest.json is a common name.
type splashManifest struct {
	Model  string `json:"model"`
	Format struct {
		Name string `json:"name"`
	} `json:"format"`
	Artifacts []struct {
		Size int64 `json:"size"`
	} `json:"artifacts"`
}

type splashSource struct{}

func (splashSource) match(_, parentDir string, ent os.DirEntry) string {
	if ent.IsDir() || ent.Name() != splashManifestName {
		return ""
	}
	if st, err := os.Stat(filepath.Join(parentDir, "target")); err != nil || !st.IsDir() {
		return ""
	}
	return filepath.Clean(parentDir)
}

func (splashSource) build(dir string) (ModelFile, bool) {
	p := filepath.Join(dir, splashManifestName)
	fi, err := os.Stat(p)
	if err != nil {
		return ModelFile{}, false
	}
	data, err := os.ReadFile(p) //nolint:gosec // G304: path from model discovery — trusted source.
	if err != nil {
		return ModelFile{}, false
	}
	var m splashManifest
	if err := json.Unmarshal(data, &m); err != nil || !strings.HasPrefix(m.Format.Name, "splash") {
		return ModelFile{}, false
	}
	// The manifest lists every artifact with its size, which saves stat-ing
	// the ~80 weight files of a bundle.
	var size int64
	for _, a := range m.Artifacts {
		size += a.Size
	}
	parts := []string{"splash"}
	for _, v := range []string{m.Model, m.Format.Name} {
		if v = strings.TrimSpace(v); v != "" {
			parts = append(parts, v)
		}
	}
	return ModelFile{
		Backend:    BackendSplash,
		ID:         dir,
		Path:       dir,
		Location:   dir,
		Name:       filepath.Base(dir),
		Size:       size,
		ModTime:    fi.ModTime(),
		Parameters: strings.Join(parts, " · "),
	}, true
}

// SplashModelRef returns what `splash serve --model` should be given for a
// bundle at dir. Splash resolves models by Hugging Face repo id, so a bundle in
// the hub cache is named by the repo it was downloaded from; anything else is
// passed as its path.
func SplashModelRef(dir string) string {
	if id, ok := decodeHFModelsRepoID(filepath.ToSlash(filepath.Clean(dir))); ok {
		return id
	}
	return filepath.Clean(dir)
}

// claimOMLXRows turns safetensors rows under one of oMLX's model directories
// into oMLX rows. oMLX serves every model in those directories and keeps its
// per-model settings keyed by them, so a model placed there is oMLX's, not a
// vLLM candidate.
func claimOMLXRows(files []ModelFile, omlxDirs []string) {
	for i, f := range files {
		if f.Backend != BackendVLLM || OMLXModelDirFor(f.Path, omlxDirs) == "" {
			continue
		}
		files[i].Backend = BackendOMLX
		files[i].Parameters = "omlx" + strings.TrimPrefix(f.Parameters, "vllm")
	}
}

// OMLXModelDirFor returns the entry of omlxDirs that contains path, or "" when
// none does. It is the directory `omlx serve --model-dir` must be given for
// oMLX to find the model at path.
func OMLXModelDirFor(path string, omlxDirs []string) string {
	clean := filepath.Clean(path)
	for _, d := range omlxDirs {
		root := filepath.Clean(d)
		if rel, err := filepath.Rel(root, clean); err == nil && rel != "." && !strings.HasPrefix(rel, "..") {
			return root
		}
	}
	return ""
}
