package llamacpp

import (
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
)

const defaultLlamaServerPort = 8080

const defaultVLLMServerPort = 8000

// RuntimeInfo describes detected llama-cli / llama-server binaries, optional vLLM CLI, and optional running server.
type RuntimeInfo struct {
	LlamaCLIPath    string
	LlamaServerPath string
	VLLMPath        string
	ServerRunning   bool
	ProbePort       int // port used when ServerRunning is true (0 if not probed)
}

// Available is true if either binary was found, vLLM was found, or a llama-server responded on the health probe.
func (r RuntimeInfo) Available() bool {
	return r.LlamaCLIPath != "" || r.LlamaServerPath != "" || r.VLLMPath != "" || r.ServerRunning
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
		ProbePort:       port,
	}
	if cli == "" && srv == "" {
		if probeLlamaServerHealth(port) {
			info.ServerRunning = true
		}
	}
	return info
}

// ListenPort returns the TCP port from LLAMA_SERVER_PORT, or 8080 if unset or invalid.
func ListenPort() int {
	if v := os.Getenv(EnvLlamaServerPort); v != "" {
		if p, err := strconv.Atoi(strings.TrimSpace(v)); err == nil && p > 0 && p <= 65535 {
			return p
		}
	}
	return defaultLlamaServerPort
}

// VLLMPort returns the TCP port from VLLM_SERVER_PORT, or 8000 if unset or invalid.
func VLLMPort() int {
	if v := os.Getenv(EnvVLLMServerPort); v != "" {
		if p, err := strconv.Atoi(strings.TrimSpace(v)); err == nil && p > 0 && p <= 65535 {
			return p
		}
	}
	return defaultVLLMServerPort
}

// ResolveLlamaServerPath returns the detected llama-server binary path, or the first match on PATH.
func ResolveLlamaServerPath(r RuntimeInfo) string {
	if r.LlamaServerPath != "" {
		return r.LlamaServerPath
	}
	if p, err := exec.LookPath("llama-server"); err == nil {
		return p
	}
	return ""
}

// ResolveVLLMPath returns the detected vllm binary path, or the first match on PATH.
func ResolveVLLMPath(r RuntimeInfo) string {
	if r.VLLMPath != "" {
		return r.VLLMPath
	}
	if p, err := exec.LookPath("vllm"); err == nil {
		return p
	}
	return ""
}
