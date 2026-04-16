package llamacpp

import (
	"os"
	"testing"
)

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
