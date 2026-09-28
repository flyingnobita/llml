package models

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/flyingnobita/llml/internal/settings"
)

var (
	// mlx_lm.server is Python's stdlib http.server: it sends a BaseHTTP
	// Server header and the same /health body as llama-server. Its model
	// list names no owner and is often empty.
	fakeMLXLMServer = fakeServer{
		serverHeader: "BaseHTTP/0.6 Python/3.13.1",
		health:       `{"status": "ok"}`,
		models:       `{"object": "list", "data": []}`,
	}
	// With no Hugging Face cache directory, mlx-lm 0.31.3 sends the 200
	// headers for /v1/models and then no body.
	fakeMLXLMServerNoHFCache = fakeServer{
		serverHeader:    fakeMLXLMServer.serverHeader,
		health:          fakeMLXLMServer.health,
		emptyModelsBody: true,
	}
)

// llama.cpp, NInfer, and mlx-lm all default to port 8080, and all three answer
// /health with status ok. Only the Runtime actually answering is running.
func TestDiscoverRuntime_llamaNInferAndMLXLMOnOnePort(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name       string
		server     fakeServer
		wantLlama  bool
		wantNInfer bool
		wantMLXLM  bool
	}{
		{"mlx_lm.server", fakeMLXLMServer, false, false, wantRunning(BackendMLXLM)},
		{"mlx_lm.server with an empty model list body", fakeMLXLMServerNoHFCache, false, false, wantRunning(BackendMLXLM)},
		{"llama-server", fakeLlamaServer, true, false, false},
		{"llama-server without a Server header", fakeOldLlamaServer, true, false, false},
		{"ninfer-serve", fakeNInferServe, false, wantRunning(BackendNInfer), false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			host, port := tc.server.start(t)
			s := settings.Resolve(settings.Defaults())
			s.LlamaServerHost, s.LlamaServerPort = host, port
			s.NInferServerHost, s.NInferServerPort = host, port
			s.MLXLMHost, s.MLXLMPort = host, port

			info := DiscoverRuntime(context.Background(), s, skipAllBut(BackendLlama, BackendNInfer, BackendMLXLM))
			if got := info.Status(BackendLlama).Running; got != tc.wantLlama {
				t.Errorf("llama.cpp running = %t, want %t", got, tc.wantLlama)
			}
			if got := info.Status(BackendNInfer).Running; got != tc.wantNInfer {
				t.Errorf("NInfer running = %t, want %t", got, tc.wantNInfer)
			}
			if got := info.Status(BackendMLXLM).Running; got != tc.wantMLXLM {
				t.Errorf("mlx-lm running = %t, want %t", got, tc.wantMLXLM)
			}
		})
	}
}

// Another stdlib Python server, or one that answers /health with a different
// status, is not mlx-lm: both the status and the Server header must match.
func TestDiscoverRuntime_mlxLMNeedsStatusAndHeader(t *testing.T) {
	t.Parallel()

	for name, srv := range map[string]fakeServer{
		"status not ok":    {serverHeader: fakeMLXLMServer.serverHeader, health: `{"status": "unavailable"}`},
		"no Server header": {health: `{"status": "ok"}`},
		"not JSON":         {serverHeader: fakeMLXLMServer.serverHeader, health: `ok`},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			host, port := srv.start(t)
			s := settings.Resolve(settings.Defaults())
			s.MLXLMHost, s.MLXLMPort = host, port
			info := DiscoverRuntime(context.Background(), s, skipAllBut(BackendMLXLM))
			if info.Status(BackendMLXLM).Running {
				t.Error("mlx-lm reported running")
			}
		})
	}
}

// mlx_lm.server is found from MLX_LM_PATH as the script itself or the folder
// holding it (a venv's bin/), and otherwise on PATH. llml does not look inside
// a venv root for it.
func TestDiscoverRuntime_findsMLXLMScript(t *testing.T) {
	pathDir := t.TempDir()
	t.Setenv("PATH", pathDir)

	venv := t.TempDir()
	bin := filepath.Join(venv, "bin")
	if err := os.MkdirAll(bin, 0o750); err != nil {
		t.Fatal(err)
	}
	script := makeFakeExecutable(t, bin, mlxLMServerName)

	found := func(configured string) string {
		s := settings.Resolve(settings.Defaults())
		s.MLXLMPath = configured
		// Every probe skipped: program lookup needs no network.
		return DiscoverRuntime(context.Background(), s, skipAllBut()).Status(BackendMLXLM).Path
	}

	for _, configured := range []string{script, bin} {
		if got := found(configured); got != script {
			t.Errorf("MLX_LM_PATH=%q: found %q, want %q", configured, got, script)
		}
	}
	if got := found(venv); got != "" {
		t.Errorf("a venv root is not searched, found %q", got)
	}
	if got := found(""); got != "" {
		t.Errorf("nothing on PATH, found %q", got)
	}

	onPath := makeFakeExecutable(t, pathDir, mlxLMServerName)
	if got := found(""); got != onPath {
		t.Errorf("unset: found %q, want %q from PATH", got, onPath)
	}
	if got := found(venv); got != onPath {
		t.Errorf("a path holding no script falls back to PATH: found %q, want %q", got, onPath)
	}
	if got := found(bin); got != script {
		t.Errorf("the configured path wins over PATH: found %q, want %q", got, script)
	}
}
