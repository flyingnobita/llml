package models

import (
	"fmt"
	"strings"
)

// Environment variables for locating llama.cpp binaries and probing a running server.
const (
	EnvLlamaCppPath    = "LLAMA_CPP_PATH"
	EnvLlamaServerPort = "LLAMA_SERVER_PORT"
	// EnvLlamaServerHost is the listen host for llama-server (default 127.0.0.1).
	EnvLlamaServerHost = "LLAMA_SERVER_HOST"
	// EnvVLLMServerPort is the TCP port for vllm serve (default 8000 when unset or invalid; matches vLLM's typical default).
	EnvVLLMServerPort = "VLLM_SERVER_PORT"
	// EnvVLLMServerHost is the listen host for vllm serve (default 127.0.0.1).
	EnvVLLMServerHost = "VLLM_SERVER_HOST"
	// EnvVLLMPath is an optional directory containing a `vllm` executable (checked before PATH).
	EnvVLLMPath = "VLLM_PATH"
	// EnvVLLMVenv is an optional Python venv root (directory containing bin/activate on Unix).
	// When set (or when $VLLM_PATH/.venv or dirname(vllm)/.venv exists), R sources activate before vllm serve.
	EnvVLLMVenv = "VLLM_VENV"
	// EnvOllamaPath is an optional directory or absolute binary path for the ollama executable.
	EnvOllamaPath = "OLLAMA_PATH"
	// EnvOllamaHost is the Ollama API bind/listen host (host:port by default).
	EnvOllamaHost = "OLLAMA_HOST"
	// EnvKoboldCppPath is an optional directory or absolute binary path for the koboldcpp executable.
	EnvKoboldCppPath = "KOBOLDCPP_PATH"
	// EnvKoboldCppPort is the TCP port for KoboldCpp (default 5001 when unset or invalid).
	EnvKoboldCppPort = "KOBOLDCPP_PORT"

	// EnvModelPaths is the env var for extra model search roots (comma-separated).
	EnvModelPaths = "LLML_MODEL_PATHS"
	// EnvHFHubCache is the Hugging Face hub cache directory override (HUGGINGFACE_HUB_CACHE).
	EnvHFHubCache = "HUGGINGFACE_HUB_CACHE"
	// EnvHFHome is the HF_HOME override (hub cache defaults to $HF_HOME/hub).
	EnvHFHome = "HF_HOME"
)

// RuntimeInfo describes detected llama-cli / llama-server binaries, optional vLLM CLI, and optional running server.
type RuntimeInfo struct {
	LlamaCLIPath       string
	LlamaServerPath    string
	LlamaServerHost    string
	VLLMPath           string
	VLLMServerHost     string
	OllamaPath         string
	OllamaHost         string
	KoboldCppPath      string
	OllamaRunning      bool
	ServerRunning      bool
	ProbePort          int // port used when ServerRunning is true (0 if not probed)
	KoboldCppRunning   bool
	KoboldCppProbePort int // port used when KoboldCppRunning is true
}

// Available is true if any backend binary was found, or a llama-server responded on the health probe.
func (r RuntimeInfo) Available() bool {
	return r.LlamaCLIPath != "" || r.LlamaServerPath != "" || r.VLLMPath != "" || r.OllamaPath != "" || r.KoboldCppPath != "" || r.OllamaRunning || r.ServerRunning || r.KoboldCppRunning
}

func formatBinLabel(abs string) string {
	if abs == "" {
		return "—"
	}
	return "✓"
}

// Summary is a single-line status for the TUI (no trailing newline).
func (r RuntimeInfo) Summary() string {
	var base string
	switch {
	case r.LlamaCLIPath != "" && r.LlamaServerPath != "":
		base = fmt.Sprintf("llama.cpp: cli %s · server %s", formatBinLabel(r.LlamaCLIPath), formatBinLabel(r.LlamaServerPath))
	case r.LlamaCLIPath != "":
		base = fmt.Sprintf("llama.cpp: cli %s · server —", formatBinLabel(r.LlamaCLIPath))
	case r.LlamaServerPath != "":
		base = fmt.Sprintf("llama.cpp: cli — · server %s", formatBinLabel(r.LlamaServerPath))
	case r.ServerRunning:
		base = fmt.Sprintf("llama.cpp: binaries not on PATH — server running :%d", r.ProbePort)
	default:
		base = "llama.cpp: not found — set " + EnvLlamaCppPath + " or install to PATH (Homebrew: ensure /opt/homebrew/bin is on PATH)"
	}
	v := "vllm: —"
	if r.VLLMPath != "" {
		v = "vllm: ✓"
	}
	k := "koboldcpp: —"
	if r.KoboldCppPath != "" {
		k = "koboldcpp: ✓"
	}
	showKobold := r.KoboldCppPath != "" || r.KoboldCppRunning
	if showKobold {
		switch {
		case r.KoboldCppPath != "" && r.KoboldCppRunning:
			k = "koboldcpp: ✓ running"
		case r.KoboldCppPath != "":
			k = "koboldcpp: ✓ stopped"
		case r.KoboldCppRunning:
			k = "koboldcpp: running"
		}
	}
	o := "ollama: —"
	showOllama := r.OllamaPath != "" || r.OllamaRunning
	switch {
	case r.OllamaPath != "" && r.OllamaRunning:
		o = "ollama: ✓ running"
	case r.OllamaPath != "":
		o = "ollama: ✓ stopped"
	case r.OllamaRunning:
		o = "ollama: running"
	}
	var parts []string
	parts = append(parts, base, v)
	if showKobold {
		parts = append(parts, k)
	}
	if showOllama {
		parts = append(parts, o)
	}
	return strings.Join(parts, " · ")
}

// DiscoverRuntime locates llama-cli and llama-server using LLAMA_CPP_PATH, common install
// directories (including Homebrew on Apple Silicon), then PATH. If neither binary exists,
// it probes http://127.0.0.1:{LLAMA_SERVER_PORT}/health (default port 8080) with a short timeout.
func DiscoverRuntime() RuntimeInfo {
	cli := findLlamaBinary("llama-cli")
	srv := findLlamaBinary("llama-server")
	port := ListenPort()
	host := LlamaServerHost()
	info := RuntimeInfo{
		LlamaCLIPath:    cli,
		LlamaServerPath: srv,
		LlamaServerHost: host,
		VLLMPath:        findVLLMBinary(),
		VLLMServerHost:  VllmServerHost(),
		OllamaPath:      findOllamaBinary(),
		OllamaHost:      OllamaHost(),
		KoboldCppPath:   findKoboldCppBinary(),
		ProbePort:       port,
	}
	if cli == "" && srv == "" {
		if probeHealthEndpoint(host, port) {
			info.ServerRunning = true
		}
	}
	if ProbeOllama() {
		info.OllamaRunning = true
	}
	kPort := KoboldCppPort()
	if probeHealthEndpoint("127.0.0.1", kPort) {
		info.KoboldCppRunning = true
		info.KoboldCppProbePort = kPort
	}
	return info
}
