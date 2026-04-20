package models

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

// hfParamsSummary returns a short summary for safetensors model dirs (architecture / model_type).
func hfParamsSummary(dir string) string {
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

// scanSafetensorsWeights returns the total size, latest mod time, and whether any
// *.safetensors files were found among entries in dir.
func scanSafetensorsWeights(dir string, entries []os.DirEntry) (size int64, latest time.Time, ok bool) {
	for _, ent := range entries {
		if ent.IsDir() || !strings.EqualFold(filepath.Ext(ent.Name()), ".safetensors") {
			continue
		}
		ok = true
		if fi, err := os.Stat(filepath.Join(dir, ent.Name())); err == nil {
			size += fi.Size()
			if fi.ModTime().After(latest) {
				latest = fi.ModTime()
			}
		}
	}
	return
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
	totalSize, latest, hasWeights := scanSafetensorsWeights(dir, entries)
	if !hasWeights {
		return ModelFile{}, false
	}
	return ModelFile{
		Backend:    BackendVLLM,
		Path:       filepath.Clean(dir),
		Name:       filepath.Base(filepath.Clean(dir)),
		Size:       totalSize,
		ModTime:    latest,
		Parameters: hfParamsSummary(dir),
	}, true
}
