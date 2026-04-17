package models

import (
	"fmt"
	"path/filepath"
	"strings"
)

// FormatSize renders a byte size with binary (IEC) units.
func FormatSize(b int64) string {
	const unit = 1024
	if b < 0 {
		return "—"
	}
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div := float64(b)
	units := []string{"KiB", "MiB", "GiB", "TiB", "PiB"}
	idx := -1
	for div >= unit && idx < len(units)-1 {
		div /= unit
		idx++
	}
	return fmt.Sprintf("%.1f %s", div, units[idx])
}

// FormatRuntimeLabel returns a short table label for the model backend ("llama.cpp", "vllm").
func FormatRuntimeLabel(b ModelBackend) string {
	switch b {
	case BackendVLLM:
		return "vllm"
	default:
		return "llama.cpp"
	}
}

// hfHubRepoDirPrefix marks Hugging Face hub cache repo directories under .../hub/.
const hfHubRepoDirPrefix = "models--"

// FormatVLLMModelName returns a short label for a safetensors checkpoint directory.
// For Hugging Face hub layouts it decodes the nearest `models--*` folder: the cache encodes
// repo ids by replacing "/" with "--", so we invert that for a readable `org/model` name.
// Otherwise it falls back to the directory basename (e.g. a non-Hub layout).
func FormatVLLMModelName(absDir string) string {
	clean := filepath.Clean(absDir)
	for d := clean; ; d = filepath.Dir(d) {
		base := filepath.Base(d)
		if strings.HasPrefix(base, hfHubRepoDirPrefix) {
			rest := strings.TrimPrefix(base, hfHubRepoDirPrefix)
			if rest == "" {
				break
			}
			return strings.ReplaceAll(rest, "--", "/")
		}
		up := filepath.Dir(d)
		if up == d {
			break
		}
	}
	return filepath.Base(clean)
}
