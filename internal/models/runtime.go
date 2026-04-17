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

	// EnvModelPaths is the env var for extra model search roots (comma-separated).
	EnvModelPaths = "LLML_MODEL_PATHS"
	// EnvHFHubCache is the Hugging Face hub cache directory override (HUGGINGFACE_HUB_CACHE).
	EnvHFHubCache = "HUGGINGFACE_HUB_CACHE"
	// EnvHFHome is the HF_HOME override (hub cache defaults to $HF_HOME/hub).
	EnvHFHome = "HF_HOME"
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
	return base + " · " + v
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
