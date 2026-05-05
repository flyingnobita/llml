package models

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

// Environment variables for locating llama.cpp binaries and probing a running server.
const (
	EnvLlamaCppPath    = "LLAMA_CPP_PATH"
	EnvLlamaServerPort = "LLAMA_SERVER_PORT"
	// EnvVLLMServerPort is the TCP port for vllm serve (default 8000 when unset or invalid; matches vLLM's typical default).
	EnvVLLMServerPort = "VLLM_SERVER_PORT"
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

const defaultLlamaServerPort = 8080

const defaultVLLMServerPort = 8000

const defaultKoboldCppPort = 5001

// RuntimeInfo describes detected llama-cli / llama-server binaries, optional vLLM CLI, and optional running server.
type RuntimeInfo struct {
	LlamaCLIPath    string
	LlamaServerPath string
	VLLMPath        string
	OllamaPath      string
	OllamaHost      string
	KoboldCppPath   string
	OllamaRunning   bool
	ServerRunning   bool
	ProbePort       int // port used when ServerRunning is true (0 if not probed)
}

// Available is true if any backend binary was found, or a llama-server responded on the health probe.
func (r RuntimeInfo) Available() bool {
	return r.LlamaCLIPath != "" || r.LlamaServerPath != "" || r.VLLMPath != "" || r.OllamaPath != "" || r.KoboldCppPath != "" || r.OllamaRunning || r.ServerRunning
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
	if showOllama {
		return base + " · " + v + " · " + k + " · " + o
	}
	return base + " · " + v + " · " + k
}

// DiscoverRuntime locates llama-cli and llama-server using LLAMA_CPP_PATH, common install
// directories (including Homebrew on Apple Silicon), then PATH. If neither binary exists,
// it probes http://127.0.0.1:{LLAMA_SERVER_PORT}/health (default port 8080) with a short timeout.
func DiscoverRuntime() RuntimeInfo {
	cli := findLlamaBinary("llama-cli")
	srv := findLlamaBinary("llama-server")
	port := ListenPort()
	info := RuntimeInfo{
		LlamaCLIPath:    cli,
		LlamaServerPath: srv,
		VLLMPath:        findVLLMBinary(),
		OllamaPath:      findOllamaBinary(),
		OllamaHost:      OllamaHost(),
		KoboldCppPath:   findKoboldCppBinary(),
		ProbePort:       port,
	}
	if cli == "" && srv == "" {
		if probeLlamaServerHealth(port) {
			info.ServerRunning = true
		}
	}
	if ProbeOllama() {
		info.OllamaRunning = true
	}
	return info
}

// portFromEnv reads a port number from the named env var, returning def if unset or invalid.
func portFromEnv(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		if p, err := strconv.Atoi(strings.TrimSpace(v)); err == nil && p > 0 && p <= 65535 {
			return p
		}
	}
	return def
}

// ListenPort returns the TCP port from LLAMA_SERVER_PORT, or 8080 if unset or invalid.
func ListenPort() int { return portFromEnv(EnvLlamaServerPort, defaultLlamaServerPort) }

// VLLMPort returns the TCP port from VLLM_SERVER_PORT, or 8000 if unset or invalid.
func VLLMPort() int { return portFromEnv(EnvVLLMServerPort, defaultVLLMServerPort) }

// KoboldCppPort returns the TCP port from KOBOLDCPP_PORT, or 5001 if unset or invalid.
func KoboldCppPort() int { return portFromEnv(EnvKoboldCppPort, defaultKoboldCppPort) }

// resolvePath returns existing if non-empty, otherwise the first match for cmdName on PATH.
func resolvePath(existing, cmdName string) string {
	if existing != "" {
		return existing
	}
	if p, err := exec.LookPath(cmdName); err == nil {
		return p
	}
	return ""
}

// ResolveLlamaServerPath returns the detected llama-server binary path, or the first match on PATH.
func ResolveLlamaServerPath(r RuntimeInfo) string {
	return resolvePath(r.LlamaServerPath, "llama-server")
}

// ResolveVLLMPath returns the detected vllm binary path, or the first match on PATH.
func ResolveVLLMPath(r RuntimeInfo) string {
	return resolvePath(r.VLLMPath, "vllm")
}

// ResolveOllamaPath returns the detected ollama binary path, or the first match on PATH.
func ResolveOllamaPath(r RuntimeInfo) string {
	return resolvePath(r.OllamaPath, "ollama")
}

// ResolveKoboldCppPath returns the detected koboldcpp binary path, or the first match on PATH.
func ResolveKoboldCppPath(r RuntimeInfo) string {
	return resolvePath(r.KoboldCppPath, "koboldcpp")
}
