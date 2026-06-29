package tui

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	btable "charm.land/bubbles/v2/table"
	tea "charm.land/bubbletea/v2"

	"github.com/flyingnobita/llml/internal/models"
	"github.com/flyingnobita/llml/internal/profiles"
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
	return serverSpec{backend: models.BackendLlama, bin: bin, host: "127.0.0.1", port: port, modelPath: modelPath, params: params}
}

func vllmSpec(bin, modelPath string, port int, activateScript string, params ModelParams) serverSpec {
	return serverSpec{backend: models.BackendVLLM, bin: bin, host: "127.0.0.1", port: port, modelPath: modelPath, params: params, activateScript: activateScript}
}

func TestServerSpecDirectArgsUseLaunchArgvSource(t *testing.T) {
	specs := map[string]serverSpec{
		"llama":  llamaSpec("/bin/llama-server", "/m/a.gguf", 9090, ModelParams{Args: []string{"--threads", "8"}}),
		"vllm":   vllmSpec("/bin/vllm", "/m/hf-model", 8000, "", ModelParams{Args: []string{"--dtype", "auto"}}),
		"kobold": mmprojKoboldSpec("/bin/koboldcpp", "/m/a.gguf", "/m/mmproj.gguf", 5001, ModelParams{Args: []string{"--gpulayers", "99"}}),
		"ollama": {backend: models.BackendOllama, bin: "/bin/ollama", host: "127.0.0.1:11434", modelPath: "qwen:latest"},
	}
	for name, spec := range specs {
		t.Run(name, func(t *testing.T) {
			want := launchArgValues(spec.launchArgv()[1:])
			if got := spec.directArgs(); !slices.Equal(got, want) {
				t.Fatalf("directArgs = %v, want launch argv tail %v", got, want)
			}
		})
	}
}

func TestLlamaDirectArgsMatchDisplayedModelFlag(t *testing.T) {
	spec := llamaSpec("/bin/llama-server", "/m/a.gguf", 9090, ModelParams{})
	if got := spec.directArgs(); len(got) < 2 || got[0] != "--model" || got[1] != "/m/a.gguf" {
		t.Fatalf("directArgs = %v, want --model followed by model path", got)
	}
	if words := spec.commandWords(); len(words) < 3 || words[1] != "--model" || words[2] != "'/m/a.gguf'" {
		t.Fatalf("commandWords = %v, want displayed --model followed by quoted path", words)
	}
}

func TestFormatLlamaServerInvocation(t *testing.T) {
	got := llamaSpec("/bin/llama-server", "/m/a.gguf", 9090, ModelParams{}).invocationEcho()
	want := "" +
		"+ '/bin/llama-server' \\\n" +
		"  --model '/m/a.gguf' \\\n" +
		"  --alias 'a.gguf' \\\n" +
		"  --host 127.0.0.1 \\\n" +
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
		"  --host 127.0.0.1 \\\n" +
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
		"  --host 127.0.0.1 \\\n" +
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

// --- mmproj auto-injection tests ---

// mmprojSpec builds a llama serverSpec with a pre-set mmprojPath (bypassing filesystem).
func mmprojLlamaSpec(bin, modelPath, mmprojPath string, port int, params ModelParams) serverSpec {
	return serverSpec{
		backend:    models.BackendLlama,
		bin:        bin,
		host:       "127.0.0.1",
		port:       port,
		modelPath:  modelPath,
		params:     params,
		mmprojPath: mmprojPath,
	}
}

func mmprojKoboldSpec(bin, modelPath, mmprojPath string, port int, params ModelParams) serverSpec {
	return serverSpec{
		backend:    models.BackendKobold,
		bin:        bin,
		port:       port,
		modelPath:  modelPath,
		params:     params,
		mmprojPath: mmprojPath,
	}
}

func TestLlamaDirectArgs_mmprojectInjected(t *testing.T) {
	got := mmprojLlamaSpec("/bin/llama-server", "/m/model.gguf", "/m/mmproj-BF16.gguf", 8080, ModelParams{}).directArgs()
	// Expect: -m <model> --alias <alias> --host <h> --port <p> --mmproj <mmproj>
	found := false
	for i, a := range got {
		if a == "--mmproj" && i+1 < len(got) && got[i+1] == "/m/mmproj-BF16.gguf" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("--mmproj not injected into directArgs: %v", got)
	}
}

func TestLlamaCommandWords_mmprojectInjected(t *testing.T) {
	got := mmprojLlamaSpec("/bin/llama-server", "/m/model.gguf", "/m/mmproj-BF16.gguf", 8080, ModelParams{}).commandWords()
	found := false
	for i, a := range got {
		if a == "--mmproj" && i+1 < len(got) {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("--mmproj not in commandWords: %v", got)
	}
}

func TestKoboldDirectArgs_mmprojectInjected(t *testing.T) {
	got := mmprojKoboldSpec("/bin/koboldcpp", "/m/model.gguf", "/m/mmproj-BF16.gguf", 5001, ModelParams{}).directArgs()
	found := false
	for i, a := range got {
		if a == "--mmproj" && i+1 < len(got) && got[i+1] == "/m/mmproj-BF16.gguf" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("kobold --mmproj not injected: %v", got)
	}
}

func TestLlamaDirectArgs_noMmprojWhenEmpty(t *testing.T) {
	// No mmprojPath → no --mmproj token in args.
	got := llamaSpec("/bin/llama-server", "/m/model.gguf", 8080, ModelParams{}).directArgs()
	for _, a := range got {
		if a == "--mmproj" {
			t.Fatalf("unexpected --mmproj in args with no mmprojPath: %v", got)
		}
	}
}

func TestVLLMDirectArgs_noMmprojInjection(t *testing.T) {
	// vLLM never injects --mmproj regardless of mmprojPath on the spec.
	s := vllmSpec("/bin/vllm", "/m/model", 8000, "", ModelParams{})
	// Even if someone forcibly set mmprojPath (not possible via buildServerSpec for vllm),
	// the directArgs for vllm branch does not read mmprojPath.
	for _, a := range s.directArgs() {
		if a == "--mmproj" {
			t.Fatalf("--mmproj should never appear for vLLM: %v", s.directArgs())
		}
	}
}

func TestBuildServerSpec_mmprojInjectedFromSibling(t *testing.T) {
	dir := t.TempDir()
	// Write a model GGUF and a sibling mmproj GGUF.
	modelPath := filepath.Join(dir, "gemma-4-Q4.gguf")
	mmprojPath := filepath.Join(dir, "mmproj-BF16.gguf")
	for _, p := range []string{modelPath, mmprojPath} {
		if err := os.WriteFile(p, []byte{}, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	rt := models.RuntimeInfo{LlamaServerPath: "/bin/llama-server"}
	imageParams := ModelParams{UseCase: profiles.UseCaseMetadata{Tags: []string{"image"}}}
	spec, err := buildServerSpec(models.BackendLlama, modelPath, imageParams, rt, false)
	if err != nil {
		t.Fatal(err)
	}
	if spec.mmprojPath != mmprojPath {
		t.Fatalf("got mmprojPath=%q want %q", spec.mmprojPath, mmprojPath)
	}
	if len(spec.mmprojCandidates) != 0 {
		t.Fatalf("expected no candidates, got %v", spec.mmprojCandidates)
	}
	// directArgs must contain --mmproj followed by the mmproj path.
	da := spec.directArgs()
	found := false
	for i, a := range da {
		if a == "--mmproj" && i+1 < len(da) && da[i+1] == mmprojPath {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("--mmproj not in directArgs: %v", da)
	}
}

func TestBuildServerSpec_mmprojSkippedWhenProfileHasIt(t *testing.T) {
	dir := t.TempDir()
	modelPath := filepath.Join(dir, "model.gguf")
	if err := os.WriteFile(modelPath, []byte{}, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "mmproj-BF16.gguf"), []byte{}, 0o600); err != nil {
		t.Fatal(err)
	}
	// Profile already has --mmproj in its args.
	params := ModelParams{Args: []string{"--mmproj", "/explicit/mmproj.gguf"}}
	rt := models.RuntimeInfo{LlamaServerPath: "/bin/llama-server"}
	spec, err := buildServerSpec(models.BackendLlama, modelPath, params, rt, false)
	if err != nil {
		t.Fatal(err)
	}
	if spec.mmprojPath != "" {
		t.Fatalf("expected empty mmprojPath when profile already has --mmproj, got %q", spec.mmprojPath)
	}
}

func TestBuildServerSpec_mmprojAmbiguous(t *testing.T) {
	dir := t.TempDir()
	modelPath := filepath.Join(dir, "gemma-4-Q4.gguf")
	for _, name := range []string{"gemma-4-Q4.gguf", "mmproj-BF16.gguf", "mmproj-F16.gguf"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte{}, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	rt := models.RuntimeInfo{LlamaServerPath: "/bin/llama-server"}
	imageParams := ModelParams{UseCase: profiles.UseCaseMetadata{Tags: []string{"image"}}}
	spec, err := buildServerSpec(models.BackendLlama, modelPath, imageParams, rt, false)
	if err != nil {
		t.Fatal(err)
	}
	if spec.mmprojPath != "" {
		t.Fatalf("expected empty mmprojPath for ambiguous, got %q", spec.mmprojPath)
	}
	if len(spec.mmprojCandidates) != 2 {
		t.Fatalf("expected 2 candidates, got %v", spec.mmprojCandidates)
	}
}

func TestBuildServerSpec_mmprojKoboldInjected(t *testing.T) {
	dir := t.TempDir()
	modelPath := filepath.Join(dir, "model.gguf")
	mmprojPath := filepath.Join(dir, "mmproj-BF16.gguf")
	for _, p := range []string{modelPath, mmprojPath} {
		if err := os.WriteFile(p, []byte{}, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	rt := models.RuntimeInfo{KoboldCppPath: "/bin/koboldcpp"}
	imageParams := ModelParams{UseCase: profiles.UseCaseMetadata{Tags: []string{"image"}}}
	spec, err := buildServerSpec(models.BackendKobold, modelPath, imageParams, rt, false)
	if err != nil {
		t.Fatal(err)
	}
	if spec.mmprojPath != mmprojPath {
		t.Fatalf("kobold mmprojPath: got %q want %q", spec.mmprojPath, mmprojPath)
	}
}

func TestBuildServerSpec_mmprojNotInjectedForVLLM(t *testing.T) {
	dir := t.TempDir()
	modelPath := filepath.Join(dir, "model")
	if err := os.Mkdir(modelPath, 0o755); err != nil {
		t.Fatal(err)
	}
	// No mmproj sibling for vllm models — but even if there were, vllm branch ignores it.
	rt := models.RuntimeInfo{VLLMPath: "/bin/vllm"}
	spec, err := buildServerSpec(models.BackendVLLM, modelPath, ModelParams{}, rt, false)
	if err != nil {
		t.Fatal(err)
	}
	if spec.mmprojPath != "" || len(spec.mmprojCandidates) != 0 {
		t.Fatalf("vllm should not have mmproj fields set: path=%q cands=%v", spec.mmprojPath, spec.mmprojCandidates)
	}
}

func TestBuildServerSpec_mmprojSkippedWhenProfileHasShortFlag(t *testing.T) {
	dir := t.TempDir()
	modelPath := filepath.Join(dir, "model.gguf")
	if err := os.WriteFile(modelPath, []byte{}, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "mmproj-BF16.gguf"), []byte{}, 0o600); err != nil {
		t.Fatal(err)
	}
	// Profile uses -mm short form — auto-detection should be skipped.
	params := ModelParams{Args: []string{"-mm", "/explicit/mmproj.gguf"}}
	rt := models.RuntimeInfo{LlamaServerPath: "/bin/llama-server"}
	spec, err := buildServerSpec(models.BackendLlama, modelPath, params, rt, false)
	if err != nil {
		t.Fatal(err)
	}
	if spec.mmprojPath != "" {
		t.Fatalf("expected empty mmprojPath when profile already has -mm, got %q", spec.mmprojPath)
	}
}

func TestHandleKey_mmprojAmbiguousWarningAdded(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv("HOME", dir)
	t.Setenv("AppData", dir)

	modelPath := filepath.Join(dir, "model.gguf")
	for _, name := range []string{"model.gguf", "mmproj-BF16.gguf", "mmproj-F16.gguf"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte{}, 0o600); err != nil {
			t.Fatal(err)
		}
	}

	// Save an image-tagged profile so loadModelParamsForRun returns UseCase with image tag.
	ent := modelEntry{
		Profiles: []ParameterProfile{
			{Name: "default", UseCase: profiles.UseCaseMetadata{Tags: []string{"image"}}},
		},
		ActiveIndex: 0,
	}
	if err := saveModelEntry(modelPath, ent); err != nil {
		t.Fatal(err)
	}

	m := New()
	m.loading = false
	m.table.files = []models.ModelFile{
		{Backend: models.BackendLlama, Path: modelPath, Name: "model", Size: 1},
	}
	m.runtime = models.RuntimeInfo{LlamaServerPath: "/bin/llama-server"}
	m.table.tbl.SetRows([]btable.Row{{"model", "model", "llama.cpp", "1 B", "", modelPath}})
	m.table.tbl.SetCursor(0)

	rMsg := tea.KeyPressMsg(tea.Key{Code: 'R', Text: "R"})
	got, _ := m.handleKey(rMsg)

	result := got.(Model)

	var found bool
	for _, e := range result.alerts.history {
		if e.severity == alertSeverityWarn && e.source == "mmproj" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected mmproj warn alert in history, got: %v", result.alerts.history)
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

func TestBuildServerSpec_mmprojMissingWhenImageTagged(t *testing.T) {
	dir := t.TempDir()
	modelPath := filepath.Join(dir, "gemma-4-Q4.gguf")
	if err := os.WriteFile(modelPath, []byte{}, 0o600); err != nil {
		t.Fatal(err)
	}
	// No mmproj sibling — but profile has image tag.
	rt := models.RuntimeInfo{LlamaServerPath: "/bin/llama-server"}
	imageParams := ModelParams{UseCase: profiles.UseCaseMetadata{Tags: []string{"image"}}}
	spec, err := buildServerSpec(models.BackendLlama, modelPath, imageParams, rt, false)
	if err != nil {
		t.Fatal(err)
	}
	if spec.mmprojPath != "" {
		t.Fatalf("expected empty mmprojPath, got %q", spec.mmprojPath)
	}
	if !spec.mmprojMissing {
		t.Fatal("expected mmprojMissing=true when image tagged with no sibling")
	}
}

func TestBuildServerSpec_mmprojNotInjectedWithoutTag(t *testing.T) {
	dir := t.TempDir()
	modelPath := filepath.Join(dir, "model.gguf")
	if err := os.WriteFile(modelPath, []byte{}, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "mmproj-BF16.gguf"), []byte{}, 0o600); err != nil {
		t.Fatal(err)
	}
	// Sibling mmproj exists but no image/audio tag — no injection.
	rt := models.RuntimeInfo{LlamaServerPath: "/bin/llama-server"}
	spec, err := buildServerSpec(models.BackendLlama, modelPath, ModelParams{}, rt, false)
	if err != nil {
		t.Fatal(err)
	}
	if spec.mmprojPath != "" {
		t.Fatalf("expected no injection without tag, got mmprojPath=%q", spec.mmprojPath)
	}
	if spec.mmprojMissing {
		t.Fatal("expected mmprojMissing=false when no image/audio tag")
	}
}

func TestBuildServerSpec_mmprojInjectedForAudioTag(t *testing.T) {
	dir := t.TempDir()
	modelPath := filepath.Join(dir, "audio-model.gguf")
	mmprojPath := filepath.Join(dir, "mmproj-BF16.gguf")
	for _, p := range []string{modelPath, mmprojPath} {
		if err := os.WriteFile(p, []byte{}, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	rt := models.RuntimeInfo{LlamaServerPath: "/bin/llama-server"}
	audioParams := ModelParams{UseCase: profiles.UseCaseMetadata{Tags: []string{"audio"}}}
	spec, err := buildServerSpec(models.BackendLlama, modelPath, audioParams, rt, false)
	if err != nil {
		t.Fatal(err)
	}
	if spec.mmprojPath != mmprojPath {
		t.Fatalf("audio tag: got mmprojPath=%q want %q", spec.mmprojPath, mmprojPath)
	}
}

func TestHandleKey_mmprojMissingWarningAdded(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv("HOME", dir)
	t.Setenv("AppData", dir)

	// Image-tagged profile but no mmproj sibling.
	modelPath := filepath.Join(dir, "model.gguf")
	if err := os.WriteFile(modelPath, []byte{}, 0o600); err != nil {
		t.Fatal(err)
	}

	ent := modelEntry{
		Profiles: []ParameterProfile{
			{Name: "default", UseCase: profiles.UseCaseMetadata{Tags: []string{"image"}}},
		},
		ActiveIndex: 0,
	}
	if err := saveModelEntry(modelPath, ent); err != nil {
		t.Fatal(err)
	}

	m := New()
	m.loading = false
	m.table.files = []models.ModelFile{
		{Backend: models.BackendLlama, Path: modelPath, Name: "model", Size: 1},
	}
	m.runtime = models.RuntimeInfo{LlamaServerPath: "/bin/llama-server"}
	m.table.tbl.SetRows([]btable.Row{{"model", "model", "llama.cpp", "1 B", "", modelPath}})
	m.table.tbl.SetCursor(0)

	rMsg := tea.KeyPressMsg(tea.Key{Code: 'R', Text: "R"})
	got, _ := m.handleKey(rMsg)

	result := got.(Model)

	var found bool
	for _, e := range result.alerts.history {
		if e.severity == alertSeverityWarn && e.source == "mmproj" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected missing-mmproj warn alert, got: %v", result.alerts.history)
	}
}

func TestServerSpec_mmprojNote_missing(t *testing.T) {
	s := serverSpec{mmprojMissing: true}
	note := s.mmprojNote()
	if note == "" {
		t.Fatal("expected non-empty note for mmprojMissing=true")
	}
	if !strings.Contains(note, "image/audio") {
		t.Fatalf("note should mention image/audio, got %q", note)
	}
}

func TestServerSpec_mmprojNote_ambiguous(t *testing.T) {
	s := serverSpec{mmprojCandidates: []string{"/a/mmproj-BF16.gguf", "/a/mmproj-F16.gguf"}}
	note := s.mmprojNote()
	if note == "" {
		t.Fatal("expected non-empty note for ambiguous candidates")
	}
	if !strings.Contains(note, "multiple") {
		t.Fatalf("note should mention multiple, got %q", note)
	}
}

func TestServerSpec_mmprojNote_clean(t *testing.T) {
	s := serverSpec{mmprojPath: "/a/mmproj-BF16.gguf"}
	if note := s.mmprojNote(); note != "" {
		t.Fatalf("expected empty note for clean spec, got %q", note)
	}
}
