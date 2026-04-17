package tui

import (
	"path/filepath"
	"strings"
	"testing"

	btable "charm.land/bubbles/v2/table"
	"github.com/flyingnobita/llml/internal/models"
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
	want := "" +
		"+ '/bin/llama-server' \\\n" +
		"  --model '/m/a.gguf' \\\n" +
		"  --alias 'a.gguf' \\\n" +
		"  --port 9090"
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
	want := "" +
		"+ '/bin/vllm' \\\n" +
		"  serve \\\n" +
		"  '/m/hf-model' \\\n" +
		"  --served-model-name 'hf-model' \\\n" +
		"  --port 9090"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
	got2 := formatVLLMServerInvocation("/bin/vllm", "/m/hf-model", 9090, "/proj/.venv/bin/activate", ModelParams{})
	want2 := "" +
		"+ . '/proj/.venv/bin/activate' && \\\n" +
		"  '/bin/vllm' \\\n" +
		"  serve \\\n" +
		"  '/m/hf-model' \\\n" +
		"  --served-model-name 'hf-model' \\\n" +
		"  --port 9090"
	if got2 != want2 {
		t.Fatalf("got %q want %q", got2, want2)
	}
}

func TestSplitServerInvocationEcho_matchesLlamaSplitLogLine(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv(models.EnvLlamaServerPort, "9090")
	modelPath := filepath.Join(dir, "a.gguf")
	m := New()
	m.loading = false
	m.loadErr = nil
	m.table.files = []models.ModelFile{
		{Backend: models.BackendLlama, Path: modelPath, Name: "a", Size: 1},
	}
	m.runtime = models.RuntimeInfo{LlamaServerPath: "/bin/llama-server"}
	m.table.tbl.SetRows([]btable.Row{{"a", "a", "llama.cpp", "1 B", "", modelPath}})
	m.table.tbl.SetCursor(0)

	p := ModelParams{
		Env:  []EnvVar{{Key: "FOO", Value: "bar"}},
		Args: []string{"--n-gpu-layers", "99"},
	}
	want := formatLlamaServerInvocation("/bin/llama-server", modelPath, 9090, p)
	ent := modelEntry{
		Profiles: []ParameterProfile{
			{Name: "default", Env: p.Env, Args: p.Args},
		},
		ActiveIndex: 0,
	}
	if err := saveModelEntry(modelPath, ent); err != nil {
		t.Fatal(err)
	}

	got := splitServerInvocationEcho(m)
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}

	wantPreview := shellCommandDisplayMultiline(false, "", p.Env, llamaCommandWords("/bin/llama-server", modelPath, 9090, p))
	if g := launchPreviewCommandLine(m); g != wantPreview {
		t.Fatalf("launchPreviewCommandLine got %q want %q", g, wantPreview)
	}
}

func TestLaunchPreviewCommandLine_vllmOmitsActivateWrapper(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv(models.EnvVLLMServerPort, "8000")
	modelPath := filepath.Join(dir, "hf-model")
	m := New()
	m.loading = false
	m.loadErr = nil
	m.table.files = []models.ModelFile{
		{Backend: models.BackendVLLM, Path: modelPath, Name: "m", Size: 1},
	}
	m.runtime = models.RuntimeInfo{VLLMPath: "/proj/.venv/bin/vllm"}
	m.table.tbl.SetRows([]btable.Row{{"m", "hf-model", "vllm", "1 B", "", modelPath}})
	m.table.tbl.SetCursor(0)

	p := ModelParams{Env: []EnvVar{{Key: "CUDA_VISIBLE_DEVICES", Value: "0"}}}
	if err := saveModelEntry(modelPath, modelEntry{
		Profiles:    []ParameterProfile{{Name: "default", Env: p.Env, Args: nil}},
		ActiveIndex: 0,
	}); err != nil {
		t.Fatal(err)
	}

	want := shellCommandDisplayMultiline(false, "", p.Env, vllmCommandWords("/proj/.venv/bin/vllm", modelPath, 8000, p))
	g := launchPreviewCommandLine(m)
	if g != want {
		t.Fatalf("got %q want %q", g, want)
	}
	if strings.HasPrefix(strings.TrimSpace(g), ".") {
		t.Fatalf("preview should not start with venv dot-source: %q", g)
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
