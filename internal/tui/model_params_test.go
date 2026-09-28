package tui

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/flyingnobita/llml/internal/config"
	"github.com/flyingnobita/llml/internal/models"
	"github.com/flyingnobita/llml/internal/profiles"
)

func TestLoadSaveModelEntry_roundTrip(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv("HOME", dir)
	t.Setenv("AppData", dir)
	modelPath := filepath.Join(t.TempDir(), "m", "model.gguf")
	ent := profiles.Entry{
		Profiles: []profiles.Profile{
			{Name: "cuda", Env: []profiles.EnvVar{{Key: "PYTORCH_CUDA_ALLOC_CONF", Value: "expandable_segments:True"}}, Args: []string{"--max-model-len", "4096"}},
			{Name: "cpu", Env: nil, Args: []string{"--device", "cpu"}},
		},
		ActiveIndex: 0,
	}
	if err := profiles.SaveEntry(modelPath, ent); err != nil {
		t.Fatal(err)
	}
	got, err := profiles.LoadEntry(modelPath)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Profiles) != 2 || got.Profiles[0].Name != "cuda" {
		t.Fatalf("%+v", got.Profiles)
	}
	run, err := profiles.LoadParamsForRun(modelPath)
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
	ent := profiles.Entry{
		Profiles: []profiles.Profile{
			{Name: "a", Args: []string{"--a"}},
			{Name: "b", Args: []string{"--b", "2"}},
		},
		ActiveIndex: 1,
	}
	if err := profiles.SaveEntry(modelPath, ent); err != nil {
		t.Fatal(err)
	}
	p, err := profiles.LoadParamsForRun(modelPath)
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Args) != 2 || p.Args[0] != "--b" {
		t.Fatalf("%v", p.Args)
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
	flat := profiles.FlattenArgLines(lines)
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
	got := shellCommandDisplayMultiline(false, "", []profiles.EnvVar{{Key: "FOO", Value: "bar"}}, []string{
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

func TestLoadModelParamsForRun_missingFile(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv("HOME", dir)
	t.Setenv("AppData", dir)
	p, err := profiles.LoadParamsForRun("/nonexistent/model.gguf")
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
	if err := profiles.SaveEntry(a, profiles.Entry{
		Profiles:    []profiles.Profile{{Name: "default", Args: []string{"x"}}},
		ActiveIndex: 0,
	}); err != nil {
		t.Fatal(err)
	}
	if err := profiles.SaveEntry(b, profiles.Entry{
		Profiles:    []profiles.Profile{{Name: "default", Args: []string{"y"}}},
		ActiveIndex: 0,
	}); err != nil {
		t.Fatal(err)
	}
	pa, err := profiles.LoadParamsForRun(a)
	if err != nil {
		t.Fatal(err)
	}
	if len(pa.Args) != 1 || pa.Args[0] != "x" {
		t.Fatalf("%v", pa.Args)
	}
	pb, err := profiles.LoadParamsForRun(b)
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
	e, err := profiles.LoadEntry(modelPath)
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
	_, err := profiles.LoadParamsForRun("/x/y.gguf")
	if err != nil {
		t.Fatal(err)
	}
}

// Opening the p panel and closing it with esc writes nothing when nothing
// changed: a model with no stored entry still has none, and a stored entry's
// file is left byte for byte. A real change is still saved.
func TestParamPanel_escWithoutChangeWritesNothing(t *testing.T) {
	dir := useTempConfigDir(t)
	fresh := testRow(models.BackendOMLX, filepath.Join(dir, ".omlx", "models", "fresh"))
	stored := testRow(models.BackendSplash, filepath.Join(dir, "splash", "stored"))
	saveProfiles(t, stored.Path, profiles.Entry{Profiles: []profiles.Profile{{Name: "mine", Args: []string{"--x"}}}})
	cfg, err := profiles.ConfigPath()
	if err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(cfg)
	if err != nil {
		t.Fatal(err)
	}

	m := dimModel(t, newLaunchFakes().services, macPlatform, config.RuntimeStates{}, fresh, stored)
	m.runtime.SplashPath = ""
	for _, row := range []models.ModelFile{fresh, stored} {
		m = selectRow(t, m, row)
		m = press(t, m, keyText("p"), keyEsc)
		if m.params.open {
			t.Fatalf("esc should close the panel for %s", row.Path)
		}
		// Opening the panel clears the footer; closing it restores the note.
		if !strings.Contains(m.lastRunNote, MissingSplashFooterNote) {
			t.Errorf("closing p should restore the missing-Runtime footer, note %q", m.lastRunNote)
		}
	}
	after, err := os.ReadFile(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if string(after) != string(before) {
		t.Fatalf("p then esc rewrote model-params.json:\nbefore %s\nafter  %s", before, after)
	}

	m = selectRow(t, m, fresh)
	m = press(t, m, keyText("p"), keyText("c"), keyEsc) // c duplicates the profile
	ent, err := profiles.LoadEntry(profiles.ModelParamsKey(fresh.Path))
	if err != nil || len(ent.Profiles) != 2 {
		t.Errorf("a duplicated profile should be saved, got %+v (err %v)", ent, err)
	}

	// Every key in the panel saves its own edit, so an edit left for esc to
	// save is made through the editor directly.
	m = press(t, m, keyText("p"))
	m.params.editor.AddEnvRow()
	m.params.editor.SetEnvRow(len(m.params.editor.EnvRows())-1, profiles.EnvVar{Key: "UNSAVED", Value: "1"})
	m = press(t, m, keyEsc)
	ent, err = profiles.LoadEntry(profiles.ModelParamsKey(fresh.Path))
	if err != nil || !slices.ContainsFunc(ent.Profiles[ent.ActiveIndex].Env, func(e profiles.EnvVar) bool { return e.Key == "UNSAVED" }) {
		t.Errorf("esc should save an edit not yet saved, got %+v (err %v)", ent, err)
	}
}
