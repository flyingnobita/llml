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
	VLLMRunning        bool // a server other than oMLX or Splash answered on VLLMServerPort
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
	NInferRunning      bool // a server listing models owned by ninfer answered on NInferPort
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

	// Skipped are the Runtimes detection was told to skip: the Disabled
	// Runtimes. They got no network probe, so their Running flags are false
	// whether or not a server is up. Their programs are still looked up.
	Skipped BackendSet

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
	return r.LlamaCLIPath != "" || r.LlamaServerPath != "" || r.VLLMPath != "" || r.OllamaPath != "" || r.KoboldCppPath != "" || r.NInferPath != "" || r.OMLXPath != "" || r.SplashPath != "" || r.OllamaRunning || r.ServerRunning || r.VLLMRunning || r.KoboldCppRunning || r.NInferRunning || r.OMLXRunning || r.SplashRunning
}

// binaryStatus renders "name: ✓ running" / "name: ✓ stopped" / "name: running"
// for a backend that was found or answered its probe, and reports false when
// neither happened so the caller can leave it out of the summary.
func binaryStatus(name string, st RuntimeStatus) (string, bool) {
	switch {
	case st.Found() && st.Running:
		return name + ": ✓ running", true
	case st.Found():
		return name + ": ✓ stopped", true
	case st.Running:
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

// llamaSummary is the llama.cpp part of [RuntimeInfo.Summary].
func (r RuntimeInfo) llamaSummary() string {
	switch {
	case r.LlamaCLIPath != "" && r.LlamaServerPath != "":
		return fmt.Sprintf("llama.cpp: cli %s · server %s", formatBinLabel(r.LlamaCLIPath), formatBinLabel(r.LlamaServerPath))
	case r.LlamaCLIPath != "":
		return fmt.Sprintf("llama.cpp: cli %s · server —", formatBinLabel(r.LlamaCLIPath))
	case r.LlamaServerPath != "":
		return fmt.Sprintf("llama.cpp: cli — · server %s", formatBinLabel(r.LlamaServerPath))
	case r.ServerRunning:
		return fmt.Sprintf("llama.cpp: binaries not on PATH — server running :%d", r.ProbePort)
	default:
		return "llama.cpp: not found — set " + settings.EnvLlamaCppPath + " or install to PATH (Homebrew: ensure /opt/homebrew/bin is on PATH)"
	}
}

// Summary is a single-line status for the TUI (no trailing newline). The
// Runtimes detection skipped (the Disabled Runtimes) are left out.
func (r RuntimeInfo) Summary() string {
	var parts []string
	if !r.Skipped.Has(BackendLlama) {
		parts = append(parts, r.llamaSummary())
	}
	if !r.Skipped.Has(BackendVLLM) {
		v := "vllm: —"
		if r.Status(BackendVLLM).Found() {
			v = "vllm: ✓"
		}
		parts = append(parts, v)
	}
	for _, b := range []struct {
		backend ModelBackend
		name    string
	}{
		{BackendKobold, "koboldcpp"},
		{BackendNInfer, "ninfer"},
		{BackendOMLX, "omlx"},
		{BackendSplash, "splash"},
		{BackendOllama, "ollama"},
	} {
		if r.Skipped.Has(b.backend) {
			continue
		}
		if s, ok := binaryStatus(b.name, r.Status(b.backend)); ok {
			parts = append(parts, s)
		}
	}
	return strings.Join(parts, " · ")
}

// DiscoverRuntime locates each Runtime's program and probes its server.
//
// llama-cli and llama-server are looked up in s.LlamaCppPath, common install
// directories (including Homebrew on Apple Silicon), then PATH; the other
// programs follow the same pattern. Program lookups are cheap and run for every
// Runtime. The network probes skip every Runtime in skip, the Disabled
// Runtimes, so a host the user does not use costs nothing; see [probeRuntimes].
func DiscoverRuntime(ctx context.Context, s settings.Settings, skip BackendSet) RuntimeInfo {
	info := locateRuntimes(s)
	info.Skipped = skip
	probeRuntimes(ctx, s, &info, skip)
	return info
}

// locateRuntimes fills in every Runtime's program path and resolved address,
// without touching the network.
func locateRuntimes(s settings.Settings) RuntimeInfo {
	return RuntimeInfo{
		LlamaCLIPath:     findLlamaBinary("llama-cli", s.LlamaCppPath),
		LlamaServerPath:  findLlamaBinary("llama-server", s.LlamaCppPath),
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
}

// probeRuntimes asks each Runtime's server whether it is up and records the
// answers on info. A Runtime in skip, or one info.Platform cannot run, gets no
// request.
//
// Several Runtimes share a default port (llama.cpp and NInfer on 8080; vLLM,
// oMLX, and Splash on 8000), so each probe checks who answers, not only that
// something does: see [probeLlamaServer], [probeVLLM], and [probeModelsOwner].
//
// The probes run concurrently, so an unreachable host costs one timeout rather
// than one per Runtime in sequence, and all of them observe ctx: a cancelled or
// expired context leaves the Running flags false.
func probeRuntimes(ctx context.Context, s settings.Settings, info *RuntimeInfo, skip BackendSet) {
	var wg sync.WaitGroup
	probe := func(b ModelBackend, dst *bool, fn func() bool) {
		// Platform-specific Runtimes are probed only where they can run: oMLX
		// and Splash share port 8000 with vLLM by default, so a probe on Linux
		// would only ever find something else.
		if skip.Has(b) || !info.Platform.Supports(b) {
			return
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			*dst = fn()
		}()
	}

	probe(BackendLlama, &info.ServerRunning, func() bool {
		return probeLlamaServer(ctx, s.LlamaServerHost, s.LlamaServerPort)
	})
	probe(BackendVLLM, &info.VLLMRunning, func() bool {
		return probeVLLM(ctx, probeHost(s.VLLMServerHost), s.VLLMServerPort)
	})
	probe(BackendOllama, &info.OllamaRunning, func() bool { return NewOllamaClient(s.OllamaHost).Probe(ctx) })
	var koboldRunning bool
	probe(BackendKobold, &koboldRunning, func() bool { return probeHealthEndpoint(ctx, defaultProbeHost, s.KoboldCppPort) })
	probe(BackendNInfer, &info.NInferRunning, func() bool {
		return probeModelsOwner(ctx, probeHost(s.NInferServerHost), s.NInferServerPort, ownerNInfer)
	})
	probe(BackendOMLX, &info.OMLXRunning, func() bool {
		return probeModelsOwner(ctx, probeHost(s.OMLXHost), s.OMLXPort, ownerOMLX)
	})
	probe(BackendSplash, &info.SplashRunning, func() bool {
		return probeModelsOwner(ctx, probeHost(s.SplashHost), s.SplashPort, ownerSplash)
	})
	wg.Wait()

	if koboldRunning {
		info.KoboldCppRunning = true
		info.KoboldCppProbePort = s.KoboldCppPort
	}
}
