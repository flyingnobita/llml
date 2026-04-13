package llamacpp

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
)

func TestFindLlamaBinary_LLamaCppPathWins(t *testing.T) {
	dir := t.TempDir()
	name := "llama-cli"
	bin := filepath.Join(dir, name)
	if err := os.WriteFile(bin, []byte{}, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv(EnvLlamaCppPath, dir)
	t.Setenv("PATH", "/nonexistent")

	got := findLlamaBinary(name)
	if got != bin {
		t.Fatalf("got %q want %q", got, bin)
	}
}

func TestProbeLlamaServerHealth(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/health" {
			http.NotFound(w, r)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(ts.Close)
	u, err := url.Parse(ts.URL)
	if err != nil {
		t.Fatal(err)
	}
	port, err := strconv.Atoi(u.Port())
	if err != nil {
		t.Fatal(err)
	}
	if !probeLlamaServerHealth(port) {
		t.Fatal("expected health probe success")
	}
}

func TestRuntimePanelLines(t *testing.T) {
	t.Setenv(EnvLlamaServerPort, "")
	t.Setenv(EnvVLLMServerPort, "")
	t.Setenv(EnvLlamaCppPath, "/home/u/llama.cpp/bin")
	t.Setenv(EnvVLLMPath, "/home/u/.local/bin")
	t.Setenv(EnvVLLMVenv, "")
	lines := RuntimePanelLines(80)
	if len(lines) != 5 {
		t.Fatalf("got %d lines", len(lines))
	}
	if !strings.Contains(lines[0], EnvLlamaCppPath) || !strings.Contains(lines[0], "llama.cpp") {
		t.Errorf("LLAMA_CPP_PATH line: %q", lines[0])
	}
	if !strings.Contains(lines[1], EnvVLLMPath) || !strings.Contains(lines[1], ".local") {
		t.Errorf("VLLM_PATH line: %q", lines[1])
	}
	if !strings.Contains(lines[2], EnvVLLMVenv) || !strings.Contains(lines[2], "—") {
		t.Errorf("VLLM_VENV line: %q", lines[2])
	}
	if !strings.Contains(lines[3], EnvLlamaServerPort) || !strings.Contains(lines[3], "8080") {
		t.Errorf("LLAMA_SERVER_PORT line: %q", lines[3])
	}
	if !strings.Contains(lines[4], EnvVLLMServerPort) || !strings.Contains(lines[4], "8000") {
		t.Errorf("VLLM_SERVER_PORT line: %q", lines[4])
	}
}

func TestResolveVLLMActivateScript(t *testing.T) {
	proj := t.TempDir()
	var activate string
	if runtime.GOOS == "windows" {
		scripts := filepath.Join(proj, ".venv", "Scripts")
		if err := os.MkdirAll(scripts, 0o755); err != nil {
			t.Fatal(err)
		}
		activate = filepath.Join(scripts, "activate.bat")
	} else {
		venvBin := filepath.Join(proj, ".venv", "bin")
		if err := os.MkdirAll(venvBin, 0o755); err != nil {
			t.Fatal(err)
		}
		activate = filepath.Join(venvBin, "activate")
	}
	if err := os.WriteFile(activate, []byte("# fake\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	vllmBin := filepath.Join(proj, "vllm")
	if err := os.WriteFile(vllmBin, []byte{}, 0o755); err != nil {
		t.Fatal(err)
	}

	t.Run("VLLM_PATH_dot_venv", func(t *testing.T) {
		t.Setenv(EnvVLLMVenv, "")
		t.Setenv(EnvVLLMPath, proj)
		got := ResolveVLLMActivateScript(vllmBin)
		if got != activate {
			t.Fatalf("got %q want %q", got, activate)
		}
	})
	t.Run("VLLM_VENV_explicit", func(t *testing.T) {
		t.Setenv(EnvVLLMPath, "")
		t.Setenv(EnvVLLMVenv, filepath.Join(proj, ".venv"))
		got := ResolveVLLMActivateScript("/other/vllm")
		if got != activate {
			t.Fatalf("got %q want %q", got, activate)
		}
	})
	t.Run("dirname_vllm_dot_venv", func(t *testing.T) {
		t.Setenv(EnvVLLMVenv, "")
		t.Setenv(EnvVLLMPath, "")
		got := ResolveVLLMActivateScript(vllmBin)
		if got != activate {
			t.Fatalf("got %q want %q", got, activate)
		}
	})
}

func TestResolveVLLMActivateScript_adjacentBinLayout(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("parallel test layout uses Unix venv paths")
	}
	proj := t.TempDir()
	binDir := filepath.Join(proj, ".venv", "bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatal(err)
	}
	act := filepath.Join(binDir, "activate")
	vllm := filepath.Join(binDir, "vllm")
	if err := os.WriteFile(act, []byte("#\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(vllm, []byte{}, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv(EnvVLLMPath, "")
	t.Setenv(EnvVLLMVenv, "")
	if g := ResolveVLLMActivateScript(vllm); g != act {
		t.Fatalf("got %q want %q", g, act)
	}
}

func TestFindVLLMBinary_VLLMPath_dotVenv(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix .venv/bin layout")
	}
	proj := t.TempDir()
	binDir := filepath.Join(proj, ".venv", "bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatal(err)
	}
	vllm := filepath.Join(binDir, "vllm")
	if err := os.WriteFile(vllm, []byte{}, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv(EnvVLLMPath, proj)
	t.Setenv(EnvVLLMVenv, "")
	t.Setenv("PATH", "/nonexistent")
	if got := findVLLMBinary(); got != vllm {
		t.Fatalf("got %q want %q", got, vllm)
	}
}

func TestRuntimeInfo_Summary(t *testing.T) {
	cases := []struct {
		r    RuntimeInfo
		want string
	}{
		{
			r:    RuntimeInfo{LlamaCLIPath: "/a/llama-cli", LlamaServerPath: "/b/llama-server"},
			want: "llama.cpp: cli ✓ · server ✓ · vllm: —",
		},
		{
			r:    RuntimeInfo{LlamaCLIPath: "/a/llama-cli", LlamaServerPath: "/b/llama-server", VLLMPath: "/c/vllm"},
			want: "llama.cpp: cli ✓ · server ✓ · vllm: ✓",
		},
		{
			r:    RuntimeInfo{ServerRunning: true, ProbePort: 8000},
			want: "llama.cpp: binaries not on PATH — server running :8000 · vllm: —",
		},
	}
	for _, tc := range cases {
		if g := tc.r.Summary(); g != tc.want {
			t.Errorf("Summary() = %q want %q", g, tc.want)
		}
	}
}

func TestListenPort_default(t *testing.T) {
	os.Unsetenv(EnvLlamaServerPort)
	if p := ListenPort(); p != defaultLlamaServerPort {
		t.Fatalf("got %d", p)
	}
	t.Setenv(EnvLlamaServerPort, "9000")
	if p := ListenPort(); p != 9000 {
		t.Fatalf("got %d", p)
	}
}

func TestVLLMPort_default(t *testing.T) {
	os.Unsetenv(EnvVLLMServerPort)
	if p := VLLMPort(); p != defaultVLLMServerPort {
		t.Fatalf("got %d", p)
	}
	t.Setenv(EnvVLLMServerPort, "8000")
	if p := VLLMPort(); p != 8000 {
		t.Fatalf("got %d", p)
	}
}
