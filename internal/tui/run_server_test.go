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

func llamaSpec(bin, modelPath string, port int, params ModelParams) serverSpec {
	return serverSpec{backend: models.BackendLlama, bin: bin, port: port, modelPath: modelPath, params: params}
}

func vllmSpec(bin, modelPath string, port int, activateScript string, params ModelParams) serverSpec {
	return serverSpec{backend: models.BackendVLLM, bin: bin, port: port, modelPath: modelPath, params: params, activateScript: activateScript}
}

func TestFormatLlamaServerInvocation(t *testing.T) {
	got := llamaSpec("/bin/llama-server", "/m/a.gguf", 9090, ModelParams{}).invocationEcho()
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
	got2 := llamaSpec("/bin/llama-server", "/m/a.gguf", 9090, p).invocationEcho()
	if !strings.Contains(got2, "FOO='bar'") || !strings.Contains(got2, "--n-gpu-layers") {
		t.Fatalf("expected env and args: %q", got2)
	}
}

func TestFormatVLLMServerInvocation(t *testing.T) {
	got := vllmSpec("/bin/vllm", "/m/hf-model", 9090, "", ModelParams{}).invocationEcho()
	want := "" +
		"+ '/bin/vllm' \\\n" +
		"  serve \\\n" +
		"  '/m/hf-model' \\\n" +
		"  --served-model-name 'hf-model' \\\n" +
		"  --port 9090"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
	got2 := vllmSpec("/bin/vllm", "/m/hf-model", 9090, "/proj/.venv/bin/activate", ModelParams{}).invocationEcho()
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
	t.Setenv("HOME", dir)
	t.Setenv("AppData", dir)
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
	want := llamaSpec("/bin/llama-server", modelPath, 9090, p).invocationEcho()
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

	wantPreview := llamaSpec("/bin/llama-server", modelPath, 9090, p).previewLine()
	if g := launchPreviewCommandLine(m); g != wantPreview {
		t.Fatalf("launchPreviewCommandLine got %q want %q", g, wantPreview)
	}
}

func TestLaunchPreviewCommandLine_vllmOmitsActivateWrapper(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv("HOME", dir)
	t.Setenv("AppData", dir)
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

	// previewLine omits the activate wrapper — use a spec with no activateScript
	want := vllmSpec("/proj/.venv/bin/vllm", modelPath, 8000, "", p).previewLine()
	g := launchPreviewCommandLine(m)
	if g != want {
		t.Fatalf("got %q want %q", g, want)
	}
	if strings.HasPrefix(strings.TrimSpace(g), ".") {
		t.Fatalf("preview should not start with venv dot-source: %q", g)
	}
}

func TestUnixVLLMServerScript_containsRead(t *testing.T) {
	s := vllmSpec("/bin/vllm", "/m/model-dir", 8080, "", ModelParams{}).unixForegroundScript()
	if !strings.Contains(s, "read -r _") {
		t.Fatalf("expected read pause: %q", s)
	}
	if !strings.Contains(s, "'/bin/vllm' serve") {
		t.Fatalf("expected vllm serve: %q", s)
	}
	s2 := vllmSpec("/bin/vllm", "/m/model-dir", 8080, "/x/.venv/bin/activate", ModelParams{}).unixForegroundScript()
	if !strings.Contains(s2, ". '/x/.venv/bin/activate'") {
		t.Fatalf("expected venv source: %q", s2)
	}
}

func TestUnixLlamaServerScript_containsRead(t *testing.T) {
	s := llamaSpec("/bin/llama-server", "/m/model.gguf", 8080, ModelParams{}).unixForegroundScript()
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
	s := vllmSpec("/bin/vllm", "/m/model-dir", 8080, "", ModelParams{}).unixSplitScript()
	if !strings.HasSuffix(strings.TrimSpace(s), "2>&1") {
		t.Fatalf("expected 2>&1 suffix: %q", s)
	}
	if !strings.Contains(s, "'/bin/vllm' serve") {
		t.Fatalf("expected vllm serve: %q", s)
	}
}

func koboldSpec(bin, modelPath string, port int, params ModelParams) serverSpec {
	return serverSpec{backend: models.BackendKobold, bin: bin, port: port, modelPath: modelPath, params: params}
}

func TestFormatKoboldCppInvocation(t *testing.T) {
	got := koboldSpec("/bin/koboldcpp", "/m/a.gguf", 5001, ModelParams{}).invocationEcho()
	want := "" +
		"+ '/bin/koboldcpp' \\\n" +
		"  '/m/a.gguf' \\\n" +
		"  --port 5001"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
	p := ModelParams{
		Env:  []EnvVar{{Key: "FOO", Value: "bar"}},
		Args: []string{"--usecublas", "--gpulayers", "99"},
	}
	got2 := koboldSpec("/bin/koboldcpp", "/m/a.gguf", 5001, p).invocationEcho()
	if !strings.Contains(got2, "FOO='bar'") || !strings.Contains(got2, "--usecublas") {
		t.Fatalf("expected env and args: %q", got2)
	}
}

func TestKoboldCppCommandWords(t *testing.T) {
	got := koboldSpec("/bin/koboldcpp", "/m/a.gguf", 5001, ModelParams{}).commandWords()
	if len(got) != 4 || got[1] != "'/m/a.gguf'" || got[2] != "--port" || got[3] != "5001" {
		t.Fatalf("got %v", got)
	}
}

func TestKoboldCppDirectArgs(t *testing.T) {
	got := koboldSpec("/bin/koboldcpp", "/m/a.gguf", 5001, ModelParams{}).directArgs()
	if len(got) != 3 || got[0] != "/m/a.gguf" || got[1] != "--port" || got[2] != "5001" {
		t.Fatalf("got %v", got)
	}

	gotWithArgs := koboldSpec("/bin/koboldcpp", "/m/a.gguf", 5001, ModelParams{Args: []string{"--usecublas", "--gpulayers", "99"}}).directArgs()
	if len(gotWithArgs) != 6 {
		t.Fatalf("got len %d want 6: %v", len(gotWithArgs), gotWithArgs)
	}
	if gotWithArgs[0] != "/m/a.gguf" || gotWithArgs[1] != "--port" || gotWithArgs[2] != "5001" || gotWithArgs[3] != "--usecublas" || gotWithArgs[4] != "--gpulayers" || gotWithArgs[5] != "99" {
		t.Fatalf("got %v", gotWithArgs)
	}
}

func TestKoboldCppPreviewLine(t *testing.T) {
	got := koboldSpec("/bin/koboldcpp", "/m/a.gguf", 5001, ModelParams{}).previewLine()
	if !strings.Contains(got, "/bin/koboldcpp") || !strings.Contains(got, "/m/a.gguf") || !strings.Contains(got, "--port 5001") {
		t.Fatalf("got %q", got)
	}
	if strings.HasPrefix(strings.TrimSpace(got), "+") {
		t.Fatalf("preview should not have + prefix: %q", got)
	}
}

func TestBuildServerSpec_koboldCppStrictMissing(t *testing.T) {
	rt := models.RuntimeInfo{}
	_, err := buildServerSpec(models.BackendKobold, "/m/a.gguf", ModelParams{}, rt, true)
	if err == nil || !strings.Contains(err.Error(), "koboldcpp not found") {
		t.Fatalf("expected missing-koboldcpp error, got %v", err)
	}
}

func TestBuildServerSpec_koboldCppNonStrict(t *testing.T) {
	rt := models.RuntimeInfo{}
	spec, err := buildServerSpec(models.BackendKobold, "/m/a.gguf", ModelParams{}, rt, false)
	if err != nil {
		t.Fatal(err)
	}
	if spec.bin != "koboldcpp" {
		t.Fatalf("expected placeholder bin, got %q", spec.bin)
	}
	if spec.port != models.KoboldCppPort() {
		t.Fatalf("expected default koboldcpp port, got %d", spec.port)
	}
}

func TestSplitServerInvocationEcho_koboldCppProfile(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv("HOME", dir)
	t.Setenv("AppData", dir)
	t.Setenv(models.EnvKoboldCppPort, "5001")
	modelPath := filepath.Join(dir, "a.gguf")
	m := New()
	m.loading = false
	m.loadErr = nil
	m.table.files = []models.ModelFile{
		{Backend: models.BackendLlama, Path: modelPath, Name: "a", Size: 1},
	}
	m.runtime = models.RuntimeInfo{KoboldCppPath: "/bin/koboldcpp"}
	m.table.tbl.SetRows([]btable.Row{{"a", "a", "llama.cpp", "1 B", "", modelPath}})
	m.table.tbl.SetCursor(0)

	p := ModelParams{Args: []string{"--usecublas"}}
	ent := modelEntry{
		Profiles: []ParameterProfile{
			{Name: "kobold", Backend: "koboldcpp", Env: p.Env, Args: p.Args},
		},
		ActiveIndex: 0,
	}
	if err := saveModelEntry(modelPath, ent); err != nil {
		t.Fatal(err)
	}

	got := splitServerInvocationEcho(m)
	if !strings.Contains(got, "/bin/koboldcpp") {
		t.Fatalf("expected koboldcpp invocation, got %q", got)
	}
	if !strings.Contains(got, "--port 5001") {
		t.Fatalf("expected koboldcpp port, got %q", got)
	}
	if strings.Contains(got, "llama-server") {
		t.Fatalf("should not contain llama-server: %q", got)
	}
}

func TestLaunchPreviewCommandLine_koboldCppProfile(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv("HOME", dir)
	t.Setenv("AppData", dir)
	t.Setenv(models.EnvKoboldCppPort, "5001")
	modelPath := filepath.Join(dir, "a.gguf")
	m := New()
	m.loading = false
	m.loadErr = nil
	m.table.files = []models.ModelFile{
		{Backend: models.BackendLlama, Path: modelPath, Name: "a", Size: 1},
	}
	m.runtime = models.RuntimeInfo{KoboldCppPath: "/bin/koboldcpp"}
	m.table.tbl.SetRows([]btable.Row{{"a", "a", "llama.cpp", "1 B", "", modelPath}})
	m.table.tbl.SetCursor(0)

	ent := modelEntry{
		Profiles: []ParameterProfile{
			{Name: "kobold", Backend: "koboldcpp"},
		},
		ActiveIndex: 0,
	}
	if err := saveModelEntry(modelPath, ent); err != nil {
		t.Fatal(err)
	}

	got := launchPreviewCommandLine(m)
	if !strings.Contains(got, "koboldcpp") {
		t.Fatalf("expected koboldcpp in preview, got %q", got)
	}
	if strings.Contains(got, "llama-server") {
		t.Fatalf("should not contain llama-server: %q", got)
	}
}

func TestKoboldCppSplitCmd(t *testing.T) {
	s := koboldSpec("/bin/koboldcpp", "/m/a.gguf", 5001, ModelParams{})
	cmd := s.splitCmd()
	if cmd.Path != "/bin/koboldcpp" {
		t.Fatalf("got %q", cmd.Path)
	}
	if len(cmd.Args) < 2 || cmd.Args[1] != "/m/a.gguf" {
		t.Fatalf("args %v", cmd.Args)
	}
}

func TestKoboldCppForegroundCmd(t *testing.T) {
	s := koboldSpec("/bin/koboldcpp", "/m/a.gguf", 5001, ModelParams{})
	cmd := s.foregroundCmd()
	if !strings.Contains(strings.Join(cmd.Args, " "), "/bin/koboldcpp") {
		t.Fatalf("expected koboldcpp in foreground cmd: %q", strings.Join(cmd.Args, " "))
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
