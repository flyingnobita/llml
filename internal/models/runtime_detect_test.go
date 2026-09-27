package models

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync/atomic"
	"testing"

	"github.com/flyingnobita/llml/internal/settings"
)

// probeTarget is a local server standing in for one Runtime; it counts the
// requests it receives.
type probeTarget struct {
	host string
	port int
	hits *atomic.Int32
}

func newProbeTarget(t *testing.T) probeTarget {
	t.Helper()
	hits := &atomic.Int32{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hits.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)
	host, portStr, err := net.SplitHostPort(srv.Listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		t.Fatal(err)
	}
	return probeTarget{host: host, port: port, hits: hits}
}

// probeTargets starts one local server for every Runtime detection probes and
// returns settings that point each Runtime at its server.
func probeTargets(t *testing.T) (settings.Settings, map[ModelBackend]probeTarget) {
	t.Helper()
	targets := map[ModelBackend]probeTarget{}
	for _, b := range []ModelBackend{BackendLlama, BackendKobold, BackendOllama, BackendNInfer, BackendOMLX, BackendSplash} {
		targets[b] = newProbeTarget(t)
	}
	s := settings.Resolve(settings.Defaults())
	s.LlamaServerHost, s.LlamaServerPort = targets[BackendLlama].host, targets[BackendLlama].port
	// KoboldCpp has no host setting; it is probed on loopback, as the test server is.
	s.KoboldCppPort = targets[BackendKobold].port
	s.OllamaHost = net.JoinHostPort(targets[BackendOllama].host, strconv.Itoa(targets[BackendOllama].port))
	s.NInferServerHost, s.NInferServerPort = targets[BackendNInfer].host, targets[BackendNInfer].port
	s.OMLXHost, s.OMLXPort = targets[BackendOMLX].host, targets[BackendOMLX].port
	s.SplashHost, s.SplashPort = targets[BackendSplash].host, targets[BackendSplash].port
	return s, targets
}

// TestProbeRuntimes_skipsDisabledRuntimes proves that a Runtime in the skip
// set receives no request, while the same Runtime is probed when enabled. The
// platform fixtures cover every probed Runtime between them.
func TestProbeRuntimes_skipsDisabledRuntimes(t *testing.T) {
	t.Parallel()

	for _, p := range []Platform{{GOOS: "linux", GOARCH: "amd64"}, {GOOS: "darwin", GOARCH: "arm64"}} {
		for _, b := range []ModelBackend{BackendLlama, BackendKobold, BackendOllama, BackendNInfer, BackendOMLX, BackendSplash} {
			if !p.Supports(b) {
				continue
			}
			t.Run(p.GOOS+"/"+b.String(), func(t *testing.T) {
				t.Parallel()

				s, targets := probeTargets(t)
				info := RuntimeInfo{Platform: p}
				probeRuntimes(context.Background(), s, &info, NewBackendSet(b))
				if n := targets[b].hits.Load(); n != 0 {
					t.Errorf("disabled %v received %d probe requests, want none", b, n)
				}
				for other, tg := range targets {
					if other != b && p.Supports(other) && tg.hits.Load() == 0 {
						t.Errorf("enabled %v was not probed while %v was skipped", other, b)
					}
				}

				s, targets = probeTargets(t)
				info = RuntimeInfo{Platform: p}
				probeRuntimes(context.Background(), s, &info, nil)
				if targets[b].hits.Load() == 0 {
					t.Errorf("enabled %v received no probe request", b)
				}
			})
		}
	}
}

// TestDiscoverRuntime_skipSet drives the public entry point. Ollama and
// KoboldCpp are probed whether or not their programs are installed, so their
// requests do not depend on the machine running the test.
func TestDiscoverRuntime_skipSet(t *testing.T) {
	t.Parallel()

	s, targets := probeTargets(t)
	skip := NewBackendSet(BackendOllama, BackendKobold)
	info := DiscoverRuntime(context.Background(), s, skip)
	for _, b := range []ModelBackend{BackendOllama, BackendKobold} {
		if n := targets[b].hits.Load(); n != 0 {
			t.Errorf("disabled %v received %d probe requests, want none", b, n)
		}
	}
	if info.OllamaRunning || info.KoboldCppRunning {
		t.Error("a skipped Runtime must not be reported running")
	}
	if !info.Skipped.Has(BackendOllama) || !info.Skipped.Has(BackendKobold) || len(info.Skipped) != 2 {
		t.Errorf("Skipped = %v, want the skip set", info.Skipped)
	}

	s, targets = probeTargets(t)
	info = DiscoverRuntime(context.Background(), s, nil)
	for _, b := range []ModelBackend{BackendOllama, BackendKobold} {
		if targets[b].hits.Load() == 0 {
			t.Errorf("enabled %v received no probe request", b)
		}
	}
	if !info.KoboldCppRunning {
		t.Error("KoboldCpp answered its probe but is not reported running")
	}
}

func TestRuntimeInfo_Summary_omitsSkipped(t *testing.T) {
	t.Parallel()

	r := RuntimeInfo{
		LlamaCLIPath: "/a/llama-cli", LlamaServerPath: "/b/llama-server",
		VLLMPath: "/c/vllm", KoboldCppPath: "/d/koboldcpp", OllamaPath: "/e/ollama",
		Skipped: NewBackendSet(BackendLlama, BackendKobold, BackendOllama),
	}
	if got, want := r.Summary(), "vllm: ✓"; got != want {
		t.Errorf("Summary() = %q, want %q", got, want)
	}
	r.Skipped = NewBackendSet(BackendVLLM)
	if got, want := r.Summary(), "llama.cpp: cli ✓ · server ✓ · koboldcpp: ✓ stopped · ollama: ✓ stopped"; got != want {
		t.Errorf("Summary() = %q, want %q", got, want)
	}
}
