package models

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/flyingnobita/llml/internal/settings"
)

// fakeServer answers /health and /v1/models the way one real server does. The
// shapes come from dev-docs/llml/20260928-RESEARCH-mlx-lm-mlx-vlm-servers.md
// and from live servers.
type fakeServer struct {
	// serverHeader is sent on every response when set.
	serverHeader string
	// health is the /health body; the status is always 200.
	health string
	// models is the /v1/models body, or "" when the server has no such route.
	models string
	// emptyModelsBody answers /v1/models with 200 and no body.
	emptyModelsBody bool
}

var (
	// llama-server build 8773 sends Server: llama.cpp on every response.
	fakeLlamaServer = fakeServer{
		serverHeader: "llama.cpp",
		health:       `{"status":"ok"}`,
		models:       `{"models":[{"name":"q.gguf","model":"q.gguf"}],"object":"list","data":[{"id":"q.gguf","object":"model","created":1790554728,"owned_by":"llamacpp","meta":{"n_ctx_train":40960}}]}`,
	}
	// An older llama-server is not assumed to send a Server header.
	fakeOldLlamaServer = fakeServer{
		health: fakeLlamaServer.health,
		models: fakeLlamaServer.models,
	}
	// ninfer-serve sends no Server header and the same /health body as
	// llama-server; only its model owner tells it apart.
	fakeNInferServe = fakeServer{
		health: `{"status":"ok"}`,
		models: `{"data":[{"created":1790554728,"id":"qwen3.8-27b","max_model_len":240000,"object":"model","owned_by":"ninfer"}],"object":"list"}`,
	}
	fakeVLLMServe = fakeServer{
		serverHeader: "uvicorn",
		models:       `{"object":"list","data":[{"id":"Qwen/Qwen3-8B","object":"model","created":1790554728,"owned_by":"vllm","root":"Qwen/Qwen3-8B","parent":null,"max_model_len":40960}]}`,
	}
	fakeOMLXServe = fakeServer{
		serverHeader: "uvicorn",
		health:       `{"status":"ok"}`,
		models:       `{"object":"list","data":[{"id":"Qwen3.8-27B-oQ4e-mtp","object":"model","owned_by":"omlx"}]}`,
	}
	fakeSplashServe = fakeServer{
		health: `{"status":"ok"}`,
		models: `{"object":"list","data":[{"id":"incoai/Qwen3.8-27B-Splash","object":"model","owned_by":"splash"}]}`,
	}
)

// start serves f on a loopback port and returns its host and port.
func (f fakeServer) start(t *testing.T) (string, int) {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if f.serverHeader != "" {
			w.Header().Set("Server", f.serverHeader)
		}
		var body string
		switch r.URL.Path {
		case "/health":
			body = f.health
		case "/v1/models":
			if f.emptyModelsBody {
				return
			}
			if f.models == "" {
				http.NotFound(w, r)
				return
			}
			body = f.models
		default:
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(body))
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
	return host, port
}

// skipAllBut is the skip set that leaves only keep probed, so a test cannot
// reach a real server on another Runtime's default port.
func skipAllBut(keep ...ModelBackend) BackendSet {
	skip := NewBackendSet()
	for _, b := range allBackends {
		skip[b] = struct{}{}
	}
	for _, b := range keep {
		delete(skip, b)
	}
	return skip
}

// wantRunning is whether detection should report b running when b's server is
// the one answering: only where the platform can run b, since elsewhere b gets
// no probe.
func wantRunning(b ModelBackend) bool {
	return CurrentPlatform().Supports(b)
}

// llama.cpp and NInfer share port 8080 by default, and both answer /health
// with {"status":"ok"}. Only the Runtime actually answering is running.
func TestDiscoverRuntime_llamaAndNInferOnOnePort(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name       string
		server     fakeServer
		wantLlama  bool
		wantNInfer bool
	}{
		{"ninfer-serve", fakeNInferServe, false, wantRunning(BackendNInfer)},
		{"llama-server", fakeLlamaServer, true, false},
		{"llama-server without a Server header", fakeOldLlamaServer, true, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			host, port := tc.server.start(t)
			s := settings.Resolve(settings.Defaults())
			s.LlamaServerHost, s.LlamaServerPort = host, port
			s.NInferServerHost, s.NInferServerPort = host, port

			info := DiscoverRuntime(context.Background(), s, skipAllBut(BackendLlama, BackendNInfer))
			if got := info.Status(BackendLlama).Running; got != tc.wantLlama {
				t.Errorf("llama.cpp running = %t, want %t", got, tc.wantLlama)
			}
			if got := info.Status(BackendNInfer).Running; got != tc.wantNInfer {
				t.Errorf("NInfer running = %t, want %t", got, tc.wantNInfer)
			}
		})
	}
}

// vLLM, oMLX, and Splash share port 8000 by default. vLLM counts as running
// for any server there that does not name oMLX or Splash as the model owner.
func TestDiscoverRuntime_vLLMOMLXAndSplashOnOnePort(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name       string
		server     fakeServer
		wantVLLM   bool
		wantOMLX   bool
		wantSplash bool
	}{
		{"vllm serve", fakeVLLMServe, true, false, false},
		{"omlx serve", fakeOMLXServe, false, wantRunning(BackendOMLX), false},
		{"splash serve", fakeSplashServe, false, false, wantRunning(BackendSplash)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			host, port := tc.server.start(t)
			s := settings.Resolve(settings.Defaults())
			s.VLLMServerHost, s.VLLMServerPort = host, port
			s.OMLXHost, s.OMLXPort = host, port
			s.SplashHost, s.SplashPort = host, port

			info := DiscoverRuntime(context.Background(), s, skipAllBut(BackendVLLM, BackendOMLX, BackendSplash))
			if got := info.Status(BackendVLLM).Running; got != tc.wantVLLM {
				t.Errorf("vLLM running = %t, want %t", got, tc.wantVLLM)
			}
			if got := info.Status(BackendOMLX).Running; got != tc.wantOMLX {
				t.Errorf("oMLX running = %t, want %t", got, tc.wantOMLX)
			}
			if got := info.Status(BackendSplash).Running; got != tc.wantSplash {
				t.Errorf("Splash running = %t, want %t", got, tc.wantSplash)
			}
		})
	}
}

// A port nothing listens on reports no Runtime running.
func TestDiscoverRuntime_nothingAnswering(t *testing.T) {
	t.Parallel()

	// Borrow a free port and release it: nothing answers on it afterwards.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	host, port := "127.0.0.1", ln.Addr().(*net.TCPAddr).Port
	_ = ln.Close()

	s := settings.Resolve(settings.Defaults())
	s.LlamaServerHost, s.LlamaServerPort = host, port
	s.NInferServerHost, s.NInferServerPort = host, port
	s.VLLMServerHost, s.VLLMServerPort = host, port

	info := DiscoverRuntime(context.Background(), s, skipAllBut(BackendLlama, BackendNInfer, BackendVLLM))
	for _, b := range []ModelBackend{BackendLlama, BackendNInfer, BackendVLLM} {
		if info.Status(b).Running {
			t.Errorf("%v reported running with nothing on its port", b)
		}
	}
}
