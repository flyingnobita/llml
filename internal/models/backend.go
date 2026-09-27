package models

import (
	"fmt"
	"strings"
)

// ModelBackend selects which server command runs for a discovered model row.
type ModelBackend int

const (
	// BackendLlama is a GGUF weight file launched with llama-server.
	BackendLlama ModelBackend = iota
	// BackendVLLM is a Hugging Face-style model directory (config.json + *.safetensors)
	// launched with vllm serve.
	BackendVLLM
	// BackendOllama is a daemon-managed Ollama model identified by model[:tag].
	BackendOllama
	// BackendKobold is a GGUF weight file launched with KoboldCpp instead of llama-server.
	BackendKobold
	// BackendNInfer is a native NInfer artifact (*.ninfer) launched with ninfer-serve.
	BackendNInfer
	// BackendOMLX is an MLX model directory inside one of oMLX's model dirs,
	// served by `omlx serve --model-dir`.
	BackendOMLX
	// BackendSplash is a Splash packed-weight bundle launched with `splash serve`.
	BackendSplash
)

// String returns the canonical lowercase name for the backend.
func (b ModelBackend) String() string {
	switch b {
	case BackendOllama:
		return "ollama"
	case BackendVLLM:
		return "vllm"
	case BackendKobold:
		return "koboldcpp"
	case BackendNInfer:
		return "ninfer"
	case BackendOMLX:
		return "omlx"
	case BackendSplash:
		return "splash"
	default:
		return "llama"
	}
}

// ParseBackend converts a string to a [ModelBackend]. An empty string maps to [BackendLlama].
func ParseBackend(s string) (ModelBackend, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "llama", "":
		return BackendLlama, nil
	case "vllm":
		return BackendVLLM, nil
	case "ollama":
		return BackendOllama, nil
	case "koboldcpp":
		return BackendKobold, nil
	case "ninfer":
		return BackendNInfer, nil
	case "omlx":
		return BackendOMLX, nil
	case "splash":
		return BackendSplash, nil
	default:
		return 0, fmt.Errorf("unknown backend %q", s)
	}
}

// BackendSet is a set of Runtimes, named by their backend. The zero value is
// an empty set, and a nil set answers every [BackendSet.Has] with false.
type BackendSet map[ModelBackend]struct{}

// NewBackendSet returns a set holding bs.
func NewBackendSet(bs ...ModelBackend) BackendSet {
	s := make(BackendSet, len(bs))
	for _, b := range bs {
		s[b] = struct{}{}
	}
	return s
}

// Has reports whether b is in the set.
func (s BackendSet) Has(b ModelBackend) bool {
	_, ok := s[b]
	return ok
}
