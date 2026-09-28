package models

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/flyingnobita/llml/internal/settings"
)

var (
	// mlx-vlm 0.4.4 is a FastAPI app under uvicorn, which names itself in the
	// Server header. /health says "healthy", not "ok", and /v1/models names
	// no owner.
	fakeMLXVLMServer044 = fakeServer{
		serverHeader: "uvicorn",
		health:       `{"status":"healthy","loaded_model":null,"loaded_adapter":null}`,
		models:       `{"object":"list","data":[]}`,
	}
	// From 0.5.0 mlx-vlm sends its own Server header, and 0.7.x adds more
	// fields to /health and lists the loaded model with "loaded".
	fakeMLXVLMServer073 = fakeServer{
		serverHeader: "mlx_vlm/0.7.3",
		health:       `{"status":"healthy","loaded_model":"mlx-community/SmolLM-135M-Instruct-4bit","loaded_adapter":null,"loaded_models":{"text_generation":{"model":"mlx-community/SmolLM-135M-Instruct-4bit"}},"loaded_context_size":null,"configured_context_limit":null,"effective_context_limit":null,"loaded_tool_parser":null,"continuous_batching_enabled":false,"apc_enabled":false}`,
		models:       `{"object":"list","data":[{"id":"mlx-community/SmolLM-135M-Instruct-4bit","object":"model","created":1790533157,"loaded":true}]}`,
	}
)

// llama.cpp, NInfer, mlx-lm, and mlx-vlm all default to port 8080. Whichever
// of them answers is the only one reported running; mlx-vlm is recognised by
// its "healthy" status in both its old and current shapes, header or not.
func TestDiscoverRuntime_mlxVLMOnSharedPort(t *testing.T) {
	t.Parallel()

	type want struct{ llama, ninfer, mlxLM, mlxVLM bool }
	cases := []struct {
		name   string
		server fakeServer
		want   want
	}{
		{"mlx-vlm 0.4.4", fakeMLXVLMServer044, want{mlxVLM: wantRunning(BackendMLXVLM)}},
		{"mlx-vlm 0.7.3", fakeMLXVLMServer073, want{mlxVLM: wantRunning(BackendMLXVLM)}},
		{"mlx-vlm without a Server header", fakeServer{health: fakeMLXVLMServer044.health}, want{mlxVLM: wantRunning(BackendMLXVLM)}},
		{"mlx_lm.server", fakeMLXLMServer, want{mlxLM: wantRunning(BackendMLXLM)}},
		{"llama-server", fakeLlamaServer, want{llama: true}},
		{"llama-server without a Server header", fakeOldLlamaServer, want{llama: true}},
		{"ninfer-serve", fakeNInferServe, want{ninfer: wantRunning(BackendNInfer)}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			host, port := tc.server.start(t)
			s := settings.Resolve(settings.Defaults())
			s.LlamaServerHost, s.LlamaServerPort = host, port
			s.NInferServerHost, s.NInferServerPort = host, port
			s.MLXLMHost, s.MLXLMPort = host, port
			s.MLXVLMHost, s.MLXVLMPort = host, port

			info := DiscoverRuntime(context.Background(), s, skipAllBut(BackendLlama, BackendNInfer, BackendMLXLM, BackendMLXVLM))
			got := want{
				llama:  info.Status(BackendLlama).Running,
				ninfer: info.Status(BackendNInfer).Running,
				mlxLM:  info.Status(BackendMLXLM).Running,
				mlxVLM: info.Status(BackendMLXVLM).Running,
			}
			if got != tc.want {
				t.Errorf("running = %+v, want %+v", got, tc.want)
			}
		})
	}
}

// An answer whose /health status is not "healthy" is not mlx-vlm, whatever
// its Server header says.
func TestDiscoverRuntime_mlxVLMNeedsHealthyStatus(t *testing.T) {
	t.Parallel()

	for name, srv := range map[string]fakeServer{
		"status ok":            {serverHeader: "mlx_vlm/0.7.3", health: `{"status":"ok"}`},
		"no status":            {serverHeader: "uvicorn", health: `{"loaded_model":null}`},
		"not JSON":             {serverHeader: "mlx_vlm/0.7.3", health: `healthy`},
		"vLLM on its own port": fakeVLLMServe,
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			host, port := srv.start(t)
			s := settings.Resolve(settings.Defaults())
			s.MLXVLMHost, s.MLXVLMPort = host, port
			info := DiscoverRuntime(context.Background(), s, skipAllBut(BackendMLXVLM))
			if info.Status(BackendMLXVLM).Running {
				t.Error("mlx-vlm reported running")
			}
		})
	}
}

// mlx_vlm.server is found from MLX_VLM_PATH as the script itself or the
// folder holding it (a venv's bin/), and otherwise on PATH. llml does not look
// inside a venv root for it, and mlx-lm's script is not mistaken for it.
func TestDiscoverRuntime_findsMLXVLMScript(t *testing.T) {
	pathDir := t.TempDir()
	t.Setenv("PATH", pathDir)

	venv := t.TempDir()
	bin := filepath.Join(venv, "bin")
	if err := os.MkdirAll(bin, 0o750); err != nil {
		t.Fatal(err)
	}
	makeFakeExecutable(t, bin, mlxLMServerName)
	script := makeFakeExecutable(t, bin, mlxVLMServerName)

	found := func(configured string) string {
		s := settings.Resolve(settings.Defaults())
		s.MLXVLMPath = configured
		// Every probe skipped: program lookup needs no network.
		return DiscoverRuntime(context.Background(), s, skipAllBut()).Status(BackendMLXVLM).Path
	}

	for _, configured := range []string{script, bin} {
		if got := found(configured); got != script {
			t.Errorf("MLX_VLM_PATH=%q: found %q, want %q", configured, got, script)
		}
	}
	if got := found(filepath.Join(bin, mlxLMServerName)); got != "" {
		t.Errorf("mlx_lm.server is not mlx_vlm.server, found %q", got)
	}
	if got := found(venv); got != "" {
		t.Errorf("a venv root is not searched, found %q", got)
	}
	if got := found(""); got != "" {
		t.Errorf("nothing on PATH, found %q", got)
	}

	onPath := makeFakeExecutable(t, pathDir, mlxVLMServerName)
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
