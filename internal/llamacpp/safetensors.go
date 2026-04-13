package llamacpp

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const hfConfigFileName = "config.json"

// hfConfig holds fields we read from Hugging Face config.json for the Parameters column.
type hfConfig struct {
	ModelType     string   `json:"model_type"`
	Architectures []string `json:"architectures"`
}

// hfConfigSummary returns a short summary for safetensors model dirs (architecture / model_type).
func hfConfigSummary(dir string) string {
	p := filepath.Join(dir, hfConfigFileName)
	data, err := os.ReadFile(p)
	if err != nil {
		return "vllm · —"
	}
	var cfg hfConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return "vllm · —"
	}
	var parts []string
	parts = append(parts, "vllm")
	if cfg.ModelType != "" {
		parts = append(parts, cfg.ModelType)
	}
	if len(cfg.Architectures) > 0 && strings.TrimSpace(cfg.Architectures[0]) != "" {
		parts = append(parts, cfg.Architectures[0])
	}
	if len(parts) <= 1 {
		return "vllm · —"
	}
	return strings.Join(parts, " · ")
}

// collectSafetensorModelDirs walks root like discoverWalkRoot and records each directory
// that directly contains at least one *.safetensors file.
func collectSafetensorModelDirs(root string, maxD int, out map[string]struct{}) error {
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
			if !strings.EqualFold(filepath.Ext(full), ".safetensors") {
				continue
			}

			out[filepath.Clean(dir)] = struct{}{}
		}
		return nil
	}
	return walk(root)
}

// tryVLLMModelDir builds a [ModelFile] if dir contains config.json and at least one
// *.safetensors file. It returns false if the directory is not a usable HF weights folder.
func tryVLLMModelDir(dir string) (ModelFile, bool) {
	cfgPath := filepath.Join(dir, hfConfigFileName)
	if st, err := os.Stat(cfgPath); err != nil || st.IsDir() {
		return ModelFile{}, false
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return ModelFile{}, false
	}

	var totalSize int64
	var latest time.Time
	var hasWeights bool
	for _, ent := range entries {
		if ent.IsDir() {
			continue
		}
		name := ent.Name()
		if !strings.EqualFold(filepath.Ext(name), ".safetensors") {
			continue
		}
		hasWeights = true
		full := filepath.Join(dir, name)
		if fi, err := os.Stat(full); err == nil {
			totalSize += fi.Size()
			if fi.ModTime().After(latest) {
				latest = fi.ModTime()
			}
		}
	}
	if !hasWeights {
		return ModelFile{}, false
	}

	return ModelFile{
		Backend:    BackendVLLM,
		Path:       filepath.Clean(dir),
		Name:       FormatVLLMModelName(dir),
		Size:       totalSize,
		ModTime:    latest,
		Parameters: hfConfigSummary(dir),
	}, true
}

// discoverVLLMModels scans the same roots as GGUF discovery for Hugging Face-style
// safetensors checkpoints (config.json + *.safetensors in the same directory).
func discoverVLLMModels(opts Options, maxD int) ([]ModelFile, error) {
	roots := MergeSearchRoots(opts.ExtraRoots, opts.SkipDefaultRoots)
	candidates := make(map[string]struct{})
	for _, root := range roots {
		st, err := os.Stat(root)
		if err != nil || !st.IsDir() {
			continue
		}
		if err := collectSafetensorModelDirs(root, maxD, candidates); err != nil {
			return nil, err
		}
	}

	var out []ModelFile
	for dir := range candidates {
		mf, ok := tryVLLMModelDir(dir)
		if !ok {
			continue
		}
		out = append(out, mf)
	}
	return out, nil
}
