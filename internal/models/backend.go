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
	default:
		return 0, fmt.Errorf("unknown backend %q", s)
	}
}
