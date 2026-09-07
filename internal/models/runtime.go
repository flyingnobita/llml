package models

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/flyingnobita/llml/internal/settings"
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

	// Resolved listen ports, carried here so launch and preview code reads them
	// from the detected runtime instead of re-reading configuration.
	LlamaServerPort int
	VLLMServerPort  int
	KoboldCppPort   int

	// VLLMVenv and VLLMConfiguredPath are the configured (not detected) vLLM
	// locations, carried so venv activation can be resolved from a RuntimeInfo alone.
	VLLMVenv           string
	VLLMConfiguredPath string
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
		base = "llama.cpp: not found — set " + settings.EnvLlamaCppPath + " or install to PATH (Homebrew: ensure /opt/homebrew/bin is on PATH)"
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

// DiscoverRuntime locates llama-cli and llama-server using s.LlamaCppPath, common install
// directories (including Homebrew on Apple Silicon), then PATH. If neither binary exists,
// it probes http://{s.LlamaServerHost}:{s.LlamaServerPort}/health.
//
// The three network probes run concurrently, so an unreachable backend costs one
// timeout rather than three in sequence, and all of them observe ctx: a cancelled
// or expired context returns whatever the filesystem lookups found.
func DiscoverRuntime(ctx context.Context, s settings.Settings) RuntimeInfo {
	cli := findLlamaBinary("llama-cli", s.LlamaCppPath)
	srv := findLlamaBinary("llama-server", s.LlamaCppPath)
	info := RuntimeInfo{
		LlamaCLIPath:    cli,
		LlamaServerPath: srv,
		LlamaServerHost: s.LlamaServerHost,
		VLLMPath:        findVLLMBinary(s.VLLMPath, s.VLLMVenv),
		VLLMServerHost:  s.VLLMServerHost,
		OllamaPath:      findOllamaBinary(s.OllamaPath),
		OllamaHost:      s.OllamaHost,
		KoboldCppPath:   findKoboldCppBinary(s.KoboldCppPath),
		ProbePort:       s.LlamaServerPort,
		LlamaServerPort: s.LlamaServerPort,
		VLLMServerPort:  s.VLLMServerPort,
		KoboldCppPort:   s.KoboldCppPort,

		VLLMVenv:           s.VLLMVenv,
		VLLMConfiguredPath: s.VLLMPath,
	}

	var wg sync.WaitGroup
	var llamaRunning, ollamaRunning, koboldRunning bool

	// Probing llama-server is only informative when neither binary was found.
	if cli == "" && srv == "" {
		wg.Add(1)
		go func() {
			defer wg.Done()
			llamaRunning = probeHealthEndpoint(ctx, s.LlamaServerHost, s.LlamaServerPort)
		}()
	}
	wg.Add(2)
	go func() {
		defer wg.Done()
		ollamaRunning = NewOllamaClient(s.OllamaHost).Probe(ctx)
	}()
	go func() {
		defer wg.Done()
		koboldRunning = probeHealthEndpoint(ctx, defaultProbeHost, s.KoboldCppPort)
	}()
	wg.Wait()

	info.ServerRunning = llamaRunning
	info.OllamaRunning = ollamaRunning
	if koboldRunning {
		info.KoboldCppRunning = true
		info.KoboldCppProbePort = s.KoboldCppPort
	}
	return info
}
