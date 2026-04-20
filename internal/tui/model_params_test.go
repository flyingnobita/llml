package tui

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadSaveModelEntry_roundTrip(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv("HOME", dir)
	t.Setenv("AppData", dir)
	modelPath := filepath.Join(t.TempDir(), "m", "model.gguf")
	ent := modelEntry{
		Profiles: []ParameterProfile{
			{Name: "cuda", Env: []EnvVar{{Key: "PYTORCH_CUDA_ALLOC_CONF", Value: "expandable_segments:True"}}, Args: []string{"--max-model-len", "4096"}},
			{Name: "cpu", Env: nil, Args: []string{"--device", "cpu"}},
		},
		ActiveIndex: 0,
	}
	if err := saveModelEntry(modelPath, ent); err != nil {
		t.Fatal(err)
	}
	got, err := loadModelEntry(modelPath)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Profiles) != 2 || got.Profiles[0].Name != "cuda" {
		t.Fatalf("%+v", got.Profiles)
	}
	run, err := loadModelParamsForRun(modelPath)
	if err != nil {
		t.Fatal(err)
	}
	if len(run.Env) != 1 || run.Env[0].Key != "PYTORCH_CUDA_ALLOC_CONF" {
		t.Fatalf("env: %+v", run.Env)
	}
}

func TestLoadModelParamsForRun_usesActiveProfile(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv("HOME", dir)
	t.Setenv("AppData", dir)
	modelPath := filepath.Join(dir, "m.gguf")
	ent := modelEntry{
		Profiles: []ParameterProfile{
			{Name: "a", Args: []string{"--a"}},
			{Name: "b", Args: []string{"--b", "2"}},
		},
		ActiveIndex: 1,
	}
	if err := saveModelEntry(modelPath, ent); err != nil {
		t.Fatal(err)
	}
	p, err := loadModelParamsForRun(modelPath)
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Args) != 2 || p.Args[0] != "--b" {
		t.Fatalf("%v", p.Args)
	}
}

func TestNormalizeModelParams(t *testing.T) {
	p := ModelParams{
		Env:  []EnvVar{{Key: "  X  ", Value: "1"}, {Key: "   ", Value: "y"}},
		Args: []string{"  ", "ok", " "},
	}
	n := normalizeModelParams(p)
	if len(n.Env) != 1 || n.Env[0].Key != "X" {
		t.Fatalf("%+v", n.Env)
	}
	if len(n.Args) != 1 || n.Args[0] != "ok" {
		t.Fatalf("%v", n.Args)
	}
}

func TestExpandArgLine_flagValuePairs(t *testing.T) {
	tests := []struct {
		in   string
		want []string
	}{
		{"--max-model-len 4096", []string{"--max-model-len", "4096"}},
		{"-m /models/foo bar/model", []string{"-m", "/models/foo bar/model"}},
		{"--gpu-memory-utilization 0.85", []string{"--gpu-memory-utilization", "0.85"}},
		{"--port-only-flag", []string{"--port-only-flag"}},
		{"/abs/path only", []string{"/abs/path only"}},
	}
	for _, tc := range tests {
		got := expandArgLine(tc.in)
		if len(got) != len(tc.want) {
			t.Fatalf("expandArgLine(%q) = %v, want %v", tc.in, got, tc.want)
		}
		for i := range got {
			if got[i] != tc.want[i] {
				t.Fatalf("expandArgLine(%q)[%d] = %q, want %q", tc.in, i, got[i], tc.want[i])
			}
		}
	}
}

func TestNormalizeModelParams_splitsFlagValueLines(t *testing.T) {
	n := normalizeModelParams(ModelParams{
		Args: []string{"  --max-model-len 4096  ", "--max-num-seqs 4"},
	})
	want := []string{"--max-model-len", "4096", "--max-num-seqs", "4"}
	if len(n.Args) != len(want) {
		t.Fatalf("got %v want %v", n.Args, want)
	}
	for i := range want {
		if n.Args[i] != want[i] {
			t.Fatalf("args[%d] = %q want %q", i, n.Args[i], want[i])
		}
	}
}

func TestCollapseArgsForDisplay_and_flattenRoundTrip(t *testing.T) {
	tok := []string{"--max-model-len", "8192", "--max-num-seqs", "4", "--enable-auto-tool-choice", "--tool-call-parser", "gemma4"}
	lines := pairFlagValueForShellDisplay(tok)
	if len(lines) != 4 {
		t.Fatalf("collapsed %v (len %d)", lines, len(lines))
	}
	if lines[0] != "--max-model-len 8192" {
		t.Fatalf("got %q", lines[0])
	}
	flat := flattenArgLines(lines)
	if len(flat) != len(tok) {
		t.Fatalf("flat %v want %v", flat, tok)
	}
	for i := range tok {
		if flat[i] != tok[i] {
			t.Fatalf("[%d] %q vs %q", i, flat[i], tok[i])
		}
	}
}

func TestJoinShellArgv_minimalQuoting(t *testing.T) {
	s := joinShellArgv([]string{"--max-model-len", "4096", "--foo", "bar baz"})
	if strings.Contains(s, "'--max-model-len'") || strings.Contains(s, "'4096'") {
		t.Fatalf("expected unquoted simple tokens: %q", s)
	}
	if !strings.Contains(s, "'bar baz'") {
		t.Fatalf("expected value with space quoted: %q", s)
	}
}

func TestPairFlagValueForShellDisplay(t *testing.T) {
	toks := []string{
		"'/bin/vllm'", "serve", "'/m/model'",
		"--max-model-len", "131072",
		"--max-num-seqs", "4",
		"--gpu-memory-utilization", "0.90",
		"--enable-auto-tool-choice",
		"--tool-call-parser", "gemma4",
		"--reasoning-parser", "gemma4",
	}
	got := pairFlagValueForShellDisplay(toks)
	want := []string{
		"'/bin/vllm'", "serve", "'/m/model'",
		"--max-model-len 131072",
		"--max-num-seqs 4",
		"--gpu-memory-utilization 0.90",
		"--enable-auto-tool-choice",
		"--tool-call-parser gemma4",
		"--reasoning-parser gemma4",
	}
	if len(got) != len(want) {
		t.Fatalf("len %d got %v", len(got), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("[%d] got %q want %q", i, got[i], want[i])
		}
	}
}

func TestShellCommandDisplayMultiline_previewIndentsArgvContinuation(t *testing.T) {
	got := shellCommandDisplayMultiline(false, "", nil, []string{
		"'/bin/llama-server'",
		"-m", "/m/model.gguf",
		"--alias", "a.gguf",
		"--port", "9001",
	})
	want := "" +
		"'/bin/llama-server' \\\n" +
		"  -m /m/model.gguf \\\n" +
		"  --alias a.gguf \\\n" +
		"  --port 9001"
	if got != want {
		t.Fatalf("got:\n%s\nwant:\n%s", got, want)
	}
}

func TestShellCommandDisplayMultiline_envLinesUnindentedArgvIndented(t *testing.T) {
	got := shellCommandDisplayMultiline(false, "", []EnvVar{{Key: "FOO", Value: "bar"}}, []string{
		"'/bin/llama-server'",
		"-m", "/m/a.gguf",
	})
	want := "" +
		"FOO='bar' \\\n" +
		"'/bin/llama-server' \\\n" +
		"  -m /m/a.gguf"
	if got != want {
		t.Fatalf("got:\n%s\nwant:\n%s", got, want)
	}
}

func TestModelParamsConfigPath_respectsXDG(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv("HOME", dir)
	t.Setenv("AppData", dir)
	path, err := modelParamsConfigPath()
	if err != nil {
		t.Fatal(err)
	}
	cfgDir, _ := os.UserConfigDir()
	want := filepath.Join(cfgDir, "llml", "model-params.json")
	if path != want {
		t.Fatalf("got %q want %q", path, want)
	}
}

func TestLoadModelParamsForRun_missingFile(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv("HOME", dir)
	t.Setenv("AppData", dir)
	p, err := loadModelParamsForRun("/nonexistent/model.gguf")
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Env) != 0 || len(p.Args) != 0 {
		t.Fatalf("%+v", p)
	}
}

func TestSaveModelEntry_mergesOtherModels(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv("HOME", dir)
	t.Setenv("AppData", dir)
	a := filepath.Join(dir, "a.gguf")
	b := filepath.Join(dir, "b.gguf")
	if err := saveModelEntry(a, modelEntry{
		Profiles:    []ParameterProfile{{Name: "default", Args: []string{"x"}}},
		ActiveIndex: 0,
	}); err != nil {
		t.Fatal(err)
	}
	if err := saveModelEntry(b, modelEntry{
		Profiles:    []ParameterProfile{{Name: "default", Args: []string{"y"}}},
		ActiveIndex: 0,
	}); err != nil {
		t.Fatal(err)
	}
	pa, err := loadModelParamsForRun(a)
	if err != nil {
		t.Fatal(err)
	}
	if len(pa.Args) != 1 || pa.Args[0] != "x" {
		t.Fatalf("%v", pa.Args)
	}
	pb, err := loadModelParamsForRun(b)
	if err != nil {
		t.Fatal(err)
	}
	if len(pb.Args) != 1 || pb.Args[0] != "y" {
		t.Fatalf("%v", pb.Args)
	}
}

func TestMigrateV1File_toProfiles(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv("HOME", dir)
	t.Setenv("AppData", dir)
	cfgDir, _ := os.UserConfigDir()
	cfg := filepath.Join(cfgDir, "llml", "model-params.json")
	if err := os.MkdirAll(filepath.Dir(cfg), 0o755); err != nil {
		t.Fatal(err)
	}
	modelPath := filepath.Join(dir, "repo", "x.gguf")
	key := filepath.Clean(modelPath)
	payload := map[string]any{
		"version": 1,
		"models": map[string]any{
			key: map[string]any{
				"env":  []any{map[string]any{"key": "K", "value": "V"}},
				"args": []any{"--x"},
			},
		},
	}
	b, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(cfg, b, 0o644); err != nil {
		t.Fatal(err)
	}
	e, err := loadModelEntry(modelPath)
	if err != nil {
		t.Fatal(err)
	}
	if len(e.Profiles) != 1 || e.Profiles[0].Name != "default" {
		t.Fatalf("%+v", e.Profiles)
	}
	if e.Profiles[0].Env[0].Key != "K" || len(e.Profiles[0].Args) != 1 {
		t.Fatalf("%+v", e.Profiles[0])
	}
}

func TestParseModelEntry_v2EmptyProfiles_getsDefault(t *testing.T) {
	raw := []byte(`{"profiles":[],"activeIndex":0}`)
	e, err := parseModelEntry(raw)
	if err != nil {
		t.Fatal(err)
	}
	if len(e.Profiles) != 1 || e.Profiles[0].Name != "default" {
		t.Fatalf("%+v", e.Profiles)
	}
}

func TestModelParamsFile_exists(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv("HOME", dir)
	t.Setenv("AppData", dir)
	cfgDir, _ := os.UserConfigDir()
	path := filepath.Join(cfgDir, "llml", "model-params.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(`{"version":1,"models":{}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := loadModelParamsForRun("/x/y.gguf")
	if err != nil {
		t.Fatal(err)
	}
}
