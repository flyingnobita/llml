package models

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
			r:    RuntimeInfo{LlamaCLIPath: "/a/llama-cli", LlamaServerPath: "/b/llama-server", VLLMPath: "/c/vllm", KoboldCppPath: "/d/koboldcpp-linux-x64"},
			want: "llama.cpp: cli ✓ · server ✓ · vllm: ✓ · koboldcpp: ✓ stopped",
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

func TestRuntimeInfo_Available_koboldCpp(t *testing.T) {
	if !(RuntimeInfo{KoboldCppPath: "/d/koboldcpp"}).Available() {
		t.Error("Available() should be true when koboldcpp path is set")
	}
	if !(RuntimeInfo{KoboldCppRunning: true}).Available() {
		t.Error("Available() should be true when koboldcpp is running")
	}
	if !(RuntimeInfo{KoboldCppPath: "/d/koboldcpp", KoboldCppRunning: true}).Available() {
		t.Error("Available() should be true when koboldcpp path is set and running")
	}
	if (RuntimeInfo{}).Available() {
		t.Error("Available() should be false when no backends are present")
	}
}

func TestResolveKoboldCppPath(t *testing.T) {
	p := ResolveKoboldCppPath(RuntimeInfo{KoboldCppPath: "/d/koboldcpp"})
	if p != "/d/koboldcpp" {
		t.Fatalf("got %q want /d/koboldcpp", p)
	}
	empty := ResolveKoboldCppPath(RuntimeInfo{})
	if empty != "" {
		t.Fatalf("got %q want empty", empty)
	}
}

func TestKoboldCppPort_default(t *testing.T) {
	if p := KoboldCppPort(); p != defaultKoboldCppPort {
		t.Fatalf("got %d want %d", p, defaultKoboldCppPort)
	}
	t.Setenv(EnvKoboldCppPort, "6000")
	if p := KoboldCppPort(); p != 6000 {
		t.Fatalf("got %d", p)
	}
}
