package models

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/flyingnobita/llml/internal/fsutil"
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
	NInferPath         string
	NInferServerHost   string
	NInferRunning      bool // ninfer-serve answered /health on NInferServerPort
	OMLXPath           string
	OMLXHost           string
	OMLXRunning        bool // an oMLX server is listing models on OMLXPort
	SplashPath         string
	SplashHost         string
	SplashRunning      bool // a Splash server is listing models on SplashPort

	// OMLXModelDirs are the directories oMLX serves models from; launch
	// passes the one holding the selected model as --model-dir.
	OMLXModelDirs []string

	// Platform is where these runtimes were detected. The TUI hides backends
	// the platform cannot run; see [Platform.Supports].
	Platform Platform

	// Resolved listen ports, carried here so launch and preview code reads them
	// from the detected runtime instead of re-reading configuration.
	LlamaServerPort int
	VLLMServerPort  int
	KoboldCppPort   int
	NInferPort      int
	OMLXPort        int
	SplashPort      int

	// VLLMVenv and VLLMConfiguredPath are the configured (not detected) vLLM
	// locations, carried so venv activation can be resolved from a RuntimeInfo alone.
	VLLMVenv           string
	VLLMConfiguredPath string
}

// Available is true if any backend binary was found, or a llama-server responded on the health probe.
func (r RuntimeInfo) Available() bool {
	return r.LlamaCLIPath != "" || r.LlamaServerPath != "" || r.VLLMPath != "" || r.OllamaPath != "" || r.KoboldCppPath != "" || r.NInferPath != "" || r.OMLXPath != "" || r.SplashPath != "" || r.OllamaRunning || r.ServerRunning || r.KoboldCppRunning || r.NInferRunning || r.OMLXRunning || r.SplashRunning
}

// binaryStatus renders "name: ✓ running" / "name: ✓ stopped" / "name: running"
// for a backend that was found or answered its probe, and reports false when
// neither happened so the caller can leave it out of the summary.
func binaryStatus(name, path string, running bool) (string, bool) {
	switch {
	case path != "" && running:
		return name + ": ✓ running", true
	case path != "":
		return name + ": ✓ stopped", true
	case running:
		return name + ": running", true
	default:
		return "", false
	}
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
	for _, b := range []struct {
		name    string
		path    string
		running bool
	}{
		{"ninfer", r.NInferPath, r.NInferRunning},
		{"omlx", r.OMLXPath, r.OMLXRunning},
		{"splash", r.SplashPath, r.SplashRunning},
	} {
		if s, ok := binaryStatus(b.name, b.path, b.running); ok {
			parts = append(parts, s)
		}
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
// The network probes run concurrently, so an unreachable backend costs one
// timeout rather than one per backend in sequence, and all of them observe ctx: a cancelled
// or expired context returns whatever the filesystem lookups found.
func DiscoverRuntime(ctx context.Context, s settings.Settings) RuntimeInfo {
	cli := findLlamaBinary("llama-cli", s.LlamaCppPath)
	srv := findLlamaBinary("llama-server", s.LlamaCppPath)
	info := RuntimeInfo{
		LlamaCLIPath:     cli,
		LlamaServerPath:  srv,
		LlamaServerHost:  s.LlamaServerHost,
		VLLMPath:         findVLLMBinary(s.VLLMPath, s.VLLMVenv),
		VLLMServerHost:   s.VLLMServerHost,
		OllamaPath:       findOllamaBinary(s.OllamaPath),
		OllamaHost:       s.OllamaHost,
		KoboldCppPath:    findKoboldCppBinary(s.KoboldCppPath),
		NInferPath:       findNInferBinary(s.NInferPath),
		NInferServerHost: s.NInferServerHost,
		OMLXPath:         findOMLXBinary(s.OMLXPath),
		OMLXHost:         s.OMLXHost,
		OMLXModelDirs:    s.OMLXModelRoots(fsutil.HomeDir()),
		SplashPath:       findSplashBinary(s.SplashPath),
		SplashHost:       s.SplashHost,
		Platform:         CurrentPlatform(),
		ProbePort:        s.LlamaServerPort,
		LlamaServerPort:  s.LlamaServerPort,
		VLLMServerPort:   s.VLLMServerPort,
		KoboldCppPort:    s.KoboldCppPort,
		NInferPort:       s.NInferServerPort,
		OMLXPort:         s.OMLXPort,
		SplashPort:       s.SplashPort,

		VLLMVenv:           s.VLLMVenv,
		VLLMConfiguredPath: s.VLLMPath,
	}

	var wg sync.WaitGroup
	probe := func(dst *bool, fn func() bool) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			*dst = fn()
		}()
	}

	// Probing llama-server is only informative when neither binary was found.
	if cli == "" && srv == "" {
		probe(&info.ServerRunning, func() bool {
			return probeHealthEndpoint(ctx, s.LlamaServerHost, s.LlamaServerPort)
		})
	}
	probe(&info.OllamaRunning, func() bool { return NewOllamaClient(s.OllamaHost).Probe(ctx) })
	var koboldRunning bool
	probe(&koboldRunning, func() bool { return probeHealthEndpoint(ctx, defaultProbeHost, s.KoboldCppPort) })
	// Platform-specific backends are probed only where they can run: oMLX and
	// Splash share port 8000 with vLLM by default, so a probe on Linux would
	// only ever find something else.
	if info.Platform.Supports(BackendNInfer) {
		probe(&info.NInferRunning, func() bool {
			return probeHealthEndpoint(ctx, probeHost(s.NInferServerHost), s.NInferServerPort)
		})
	}
	if info.Platform.Supports(BackendOMLX) {
		probe(&info.OMLXRunning, func() bool {
			return probeModelsOwner(ctx, probeHost(s.OMLXHost), s.OMLXPort, "omlx")
		})
	}
	if info.Platform.Supports(BackendSplash) {
		probe(&info.SplashRunning, func() bool {
			return probeModelsOwner(ctx, probeHost(s.SplashHost), s.SplashPort, "splash")
		})
	}
	wg.Wait()

	if koboldRunning {
		info.KoboldCppRunning = true
		info.KoboldCppProbePort = s.KoboldCppPort
	}
	return info
}
