package tui

import (
	"strings"
	"testing"
)

func TestShellSingleQuoted(t *testing.T) {
	if g := shellSingleQuoted(`a'b`); g != `'a'"'"'b'` {
		t.Fatalf("got %q", g)
	}
	if g := shellSingleQuoted("/opt/bin/llama-server"); g != "'/opt/bin/llama-server'" {
		t.Fatalf("got %q", g)
	}
}

func TestFormatLlamaServerInvocation(t *testing.T) {
	got := formatLlamaServerInvocation("/bin/llama-server", "/m/a.gguf", 9090, ModelParams{})
	want := "+ '/bin/llama-server' -m '/m/a.gguf' --port 9090"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
	p := ModelParams{
		Env:  []EnvVar{{Key: "FOO", Value: "bar"}},
		Args: []string{"--n-gpu-layers", "99"},
	}
	got2 := formatLlamaServerInvocation("/bin/llama-server", "/m/a.gguf", 9090, p)
	if !strings.Contains(got2, "FOO='bar'") || !strings.Contains(got2, "--n-gpu-layers") {
		t.Fatalf("expected env and args: %q", got2)
	}
}

func TestFormatVLLMServerInvocation(t *testing.T) {
	got := formatVLLMServerInvocation("/bin/vllm", "/m/hf-model", 9090, "", ModelParams{})
	want := "+ '/bin/vllm' serve '/m/hf-model' --port 9090"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
	got2 := formatVLLMServerInvocation("/bin/vllm", "/m/hf-model", 9090, "/proj/.venv/bin/activate", ModelParams{})
	want2 := "+ . '/proj/.venv/bin/activate' && '/bin/vllm' serve '/m/hf-model' --port 9090"
	if got2 != want2 {
		t.Fatalf("got %q want %q", got2, want2)
	}
}

func TestUnixVLLMServerScript_containsRead(t *testing.T) {
	s := unixVLLMServerScript("/bin/vllm", "/m/model-dir", 8080, "", ModelParams{})
	if !strings.Contains(s, "read -r _") {
		t.Fatalf("expected read pause: %q", s)
	}
	if !strings.Contains(s, "'/bin/vllm' serve") {
		t.Fatalf("expected vllm serve: %q", s)
	}
	s2 := unixVLLMServerScript("/bin/vllm", "/m/model-dir", 8080, "/x/.venv/bin/activate", ModelParams{})
	if !strings.Contains(s2, ". '/x/.venv/bin/activate'") {
		t.Fatalf("expected venv source: %q", s2)
	}
}

func TestUnixLlamaServerScript_containsRead(t *testing.T) {
	s := unixLlamaServerScript("/bin/llama-server", "/m/model.gguf", 8080, ModelParams{})
	if !strings.Contains(s, "read -r _") {
		t.Fatalf("expected read pause: %q", s)
	}
	if !strings.Contains(s, "'/bin/llama-server'") {
		t.Fatalf("expected quoted bin: %q", s)
	}
	if !strings.Contains(s, "printf") {
		t.Fatalf("expected echo of invocation: %q", s)
	}
}

func TestUnixVLLMSplitScript_mergesStderr(t *testing.T) {
	s := unixVLLMSplitScript("/bin/vllm", "/m/model-dir", 8080, "", ModelParams{})
	if !strings.HasSuffix(strings.TrimSpace(s), "2>&1") {
		t.Fatalf("expected 2>&1 suffix: %q", s)
	}
	if !strings.Contains(s, "'/bin/vllm' serve") {
		t.Fatalf("expected vllm serve: %q", s)
	}
}

func TestMergeEnv(t *testing.T) {
	base := []string{"PATH=/usr/bin", "FOO=old"}
	ex := []EnvVar{{Key: "FOO", Value: "new"}}
	got := mergeEnv(base, ex)
	var path, foo string
	for _, line := range got {
		if strings.HasPrefix(line, "PATH=") {
			path = line
		}
		if strings.HasPrefix(line, "FOO=") {
			foo = line
		}
	}
	if path != "PATH=/usr/bin" {
		t.Fatalf("PATH: %q", path)
	}
	if foo != "FOO=new" {
		t.Fatalf("FOO: %q", foo)
	}
}
