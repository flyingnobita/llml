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
)

// String returns the canonical lowercase name for the backend ("llama" or "vllm").
func (b ModelBackend) String() string {
	switch b {
	case BackendVLLM:
		return "vllm"
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
	default:
		return 0, fmt.Errorf("unknown backend %q", s)
	}
}
