package llamacpp

// ModelBackend selects which server command runs for a discovered model row.
type ModelBackend int

const (
	// BackendLlama is a GGUF weight file launched with llama-server.
	BackendLlama ModelBackend = iota
	// BackendVLLM is a Hugging Face-style model directory (config.json + *.safetensors)
	// launched with vllm serve.
	BackendVLLM
)
