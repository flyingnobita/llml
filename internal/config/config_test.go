package config

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/flyingnobita/llml/internal/models"
	"github.com/flyingnobita/llml/internal/settings"
)

func TestConfigRoundTrip(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv("HOME", dir)
	t.Setenv("AppData", dir)

	p1 := 8080
	p2 := 8000
	c := Config{
		SchemaVersion: SchemaVersion,
		Runtime: RuntimeConfig{
			DefaultLlamaCppPath:    "/opt/llama",
			DefaultLlamaServerPort: &p1,
			DefaultVLLMServerPort:  &p2,
		},
		Discovery: DiscoveryConfig{
			ExtraModelPaths: []string{"/extra/models"},
		},
	}
	if err := WriteFile(c); err != nil {
		t.Fatal(err)
	}
	got, err := ReadFile()
	if err != nil {
		t.Fatal(err)
	}
	if got.SchemaVersion != SchemaVersion {
		t.Fatalf("schema %d", got.SchemaVersion)
	}
	if got.Runtime.DefaultLlamaCppPath != c.Runtime.DefaultLlamaCppPath {
		t.Fatalf("runtime path %q", got.Runtime.DefaultLlamaCppPath)
	}
}

func TestValidForCache(t *testing.T) {
	t.Parallel()
	if (CacheFile{SchemaVersion: 0, Models: []ModelEntry{{Path: "/x"}}}).ValidForCache() {
		t.Fatal("wrong schema should not validate")
	}
	if (CacheFile{SchemaVersion: CacheSchemaVersion}).ValidForCache() {
		t.Fatal("empty models should not validate")
	}
	if !(CacheFile{SchemaVersion: CacheSchemaVersion, Models: []ModelEntry{{Path: "/x"}}}).ValidForCache() {
		t.Fatal("valid cache should validate")
	}
}

func TestFilterExistingPaths(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	gguf := filepath.Join(dir, "m.gguf")
	if err := os.WriteFile(gguf, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	files := []models.ModelFile{
		{Path: gguf, Name: "m.gguf"},
		{Path: filepath.Join(dir, "missing.gguf"), Name: "missing.gguf"},
		{Backend: models.BackendOllama, ID: "qwen3.5:latest", Location: "ollama://qwen3.5:latest", Name: "qwen3.5:latest"},
	}
	out := FilterExistingPaths(files)
	if len(out) != 2 || out[0].Path != gguf || out[1].Backend != models.BackendOllama {
		t.Fatalf("got %+v", out)
	}
}

func TestModelEntryToModelFile(t *testing.T) {
	t.Parallel()
	e := ModelEntry{Backend: "vllm", Path: "/m", Name: "m", Size: 1, ModTime: time.Unix(1, 0).UTC(), Parameters: "p"}
	f, err := e.ToModelFile()
	if err != nil {
		t.Fatal(err)
	}
	if f.Backend != models.BackendVLLM {
		t.Fatalf("backend %v", f.Backend)
	}
}

func TestModelEntryToModelFile_Ollama(t *testing.T) {
	t.Parallel()
	e := ModelEntry{Backend: "ollama", ID: "qwen3.5:latest", Location: "ollama://qwen3.5:latest", Name: "qwen3.5:latest", Size: 1, ModTime: time.Unix(1, 0).UTC(), Parameters: "ollama · qwen"}
	f, err := e.ToModelFile()
	if err != nil {
		t.Fatal(err)
	}
	if f.Backend != models.BackendOllama || f.Identity() != "qwen3.5:latest" {
		t.Fatalf("file %+v", f)
	}
}

func TestDiscoveryConfigForWrite_merge(t *testing.T) {
	t.Parallel()
	prev := &Config{
		Discovery: DiscoveryConfig{
			ExtraModelPaths: []string{"/a"},
		},
	}
	s := settings.Settings{ExtraModelPaths: []string{"/b"}}
	d := DiscoveryConfigForWrite(prev, s)
	if len(d.ExtraModelPaths) != 2 {
		t.Fatalf("paths %v", d.ExtraModelPaths)
	}
}

func TestConfigRoundTrip_koboldCpp(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv("HOME", dir)
	t.Setenv("AppData", dir)

	p1 := 8080
	p2 := 8000
	kp := 5001
	c := Config{
		SchemaVersion: SchemaVersion,
		Runtime: RuntimeConfig{
			DefaultLlamaCppPath:    "/opt/llama",
			DefaultKoboldCppPath:   "/opt/koboldcpp",
			DefaultLlamaServerPort: &p1,
			DefaultVLLMServerPort:  &p2,
			DefaultKoboldCppPort:   &kp,
		},
		Discovery: DiscoveryConfig{
			ExtraModelPaths: []string{"/extra/models"},
		},
	}
	if err := WriteFile(c); err != nil {
		t.Fatal(err)
	}
	got, err := ReadFile()
	if err != nil {
		t.Fatal(err)
	}
	if got.Runtime.DefaultKoboldCppPath != "/opt/koboldcpp" {
		t.Fatalf("koboldcpp path: got %q", got.Runtime.DefaultKoboldCppPath)
	}
	if got.Runtime.DefaultKoboldCppPort == nil || *got.Runtime.DefaultKoboldCppPort != 5001 {
		t.Fatalf("koboldcpp port: got %v", got.Runtime.DefaultKoboldCppPort)
	}
}

func TestDiscoveryConfigFromInputs(t *testing.T) {
	t.Parallel()

	paths := []string{" /a ", "  ", ".", "/b/../c", "/a"}
	d := DiscoveryConfigFromInputs(paths)

	if len(d.ExtraModelPaths) != 2 {
		t.Fatalf("want 2 paths, got %v", d.ExtraModelPaths)
	}
	if filepath.ToSlash(d.ExtraModelPaths[0]) != "/a" {
		t.Errorf("got %q", d.ExtraModelPaths[0])
	}
	if filepath.ToSlash(d.ExtraModelPaths[1]) != "/c" {
		t.Errorf("got %q", d.ExtraModelPaths[1])
	}
}

// fakeEnv backs settings resolution with a map, so precedence can be exercised
// without mutating the process environment.
func fakeEnv(m map[string]string) settings.Getenv {
	return func(k string) string { return m[k] }
}

func TestRuntimeConfigLayer_envWinsOverFile(t *testing.T) {
	t.Parallel()

	rc := RuntimeConfig{DefaultLlamaCppPath: "/from-toml", DefaultKoboldCppPath: "/from-toml-kobold"}
	s := settings.Resolve(
		settings.FromEnv(fakeEnv(map[string]string{settings.EnvLlamaCppPath: "/from-env"})),
		rc.Layer(),
		settings.Defaults(),
	)
	if s.LlamaCppPath != "/from-env" {
		t.Errorf("env should win: got %q", s.LlamaCppPath)
	}
	if s.KoboldCppPath != "/from-toml-kobold" {
		t.Errorf("file should supply what the env does not: got %q", s.KoboldCppPath)
	}
}

func TestRuntimeConfigLayer_fileWinsOverDefault(t *testing.T) {
	t.Parallel()

	port := 6000
	rc := RuntimeConfig{
		DefaultKoboldCppPort:   &port,
		DefaultLlamaServerHost: "0.0.0.0",
		DefaultOllamaHost:      "http://box:11434/",
	}
	s := settings.Resolve(settings.FromEnv(fakeEnv(nil)), rc.Layer(), settings.Defaults())

	if s.KoboldCppPort != 6000 {
		t.Errorf("KoboldCppPort = %d, want 6000", s.KoboldCppPort)
	}
	if s.LlamaServerHost != "0.0.0.0" {
		t.Errorf("LlamaServerHost = %q", s.LlamaServerHost)
	}
	// A host written with a scheme is normalized on the way in.
	if s.OllamaHost != "box:11434" {
		t.Errorf("OllamaHost = %q, want box:11434", s.OllamaHost)
	}
	if s.VLLMServerPort != settings.DefaultVLLMServerPort {
		t.Errorf("VLLMServerPort should fall back to the default, got %d", s.VLLMServerPort)
	}
}

func TestRuntimeConfigLayer_expandsAndCleansPaths(t *testing.T) {
	t.Parallel()

	rc := RuntimeConfig{DefaultVLLMPath: "  /opt/vllm/bin/..  ", DefaultOllamaPath: "   "}
	s := settings.Resolve(settings.FromEnv(fakeEnv(nil)), rc.Layer(), settings.Defaults())

	if s.VLLMPath != "/opt/vllm" {
		t.Errorf("VLLMPath = %q, want /opt/vllm", s.VLLMPath)
	}
	if s.OllamaPath != "" {
		t.Errorf("a whitespace-only path should stay unset, got %q", s.OllamaPath)
	}
}

func TestRuntimeConfigLayer_reportsConfigAsSource(t *testing.T) {
	t.Parallel()

	port := 6000
	rc := RuntimeConfig{
		DefaultLlamaCppPath:  "/from-toml",
		DefaultKoboldCppPort: &port,
		DefaultOllamaHost:    "http://box:11434/",
		DefaultVLLMPath:      "/from-toml-vllm",
	}
	s := settings.Resolve(
		settings.FromEnv(fakeEnv(map[string]string{settings.EnvVLLMPath: "/from-env"})),
		rc.Layer(),
		settings.Defaults(),
	)

	tests := []struct {
		field settings.Field
		want  string
	}{
		{settings.FieldLlamaCppPath, "config"},
		{settings.FieldKoboldCppPort, "config"},
		{settings.FieldOllamaHost, "config"},
		{settings.FieldVLLMPath, settings.EnvVLLMPath},
		{settings.FieldLlamaServerPort, "default"},
	}
	for _, tt := range tests {
		if got := s.Source(tt.field).String(); got != tt.want {
			t.Errorf("Source(%s) = %q, want %q", tt.field.EnvVar(), got, tt.want)
		}
	}
}

// RuntimeConfigFromSettings must round-trip through Layer unchanged, so writing
// the file and reading it back does not shift the resolved values.
func TestRuntimeConfigFromSettings_roundTrips(t *testing.T) {
	t.Parallel()

	want := settings.Resolve(settings.FromEnv(fakeEnv(map[string]string{
		settings.EnvLlamaCppPath:     "/opt/llama",
		settings.EnvKoboldCppPort:    "6000",
		settings.EnvOllamaHost:       "box:11434",
		settings.EnvVLLMServerHost:   "0.0.0.0",
		settings.EnvNInferPath:       "/opt/ninfer",
		settings.EnvNInferServerPort: "18181",
		settings.EnvNInferServerHost: "0.0.0.0",
		settings.EnvOMLXPath:         "/Users/u/.omlx",
		settings.EnvOMLXPort:         "8100",
		settings.EnvSplashPath:       "/opt/homebrew/bin/splash",
		settings.EnvSplashHost:       "0.0.0.0",
	})), settings.Defaults())

	rc := RuntimeConfigFromSettings(want)
	got := settings.Resolve(settings.FromEnv(fakeEnv(nil)), rc.Layer(), settings.Defaults())

	got.ExtraModelPaths = want.ExtraModelPaths // not part of the [runtime] table
	if !sameValues(got, want) {
		t.Errorf("round trip changed settings:\n got %+v\nwant %+v", got, want)
	}
}

// sameValues reports whether a and b resolved to the same values. It ignores
// where each value came from: a round trip through the file turns environment
// sources into config sources by design.
func sameValues(a, b settings.Settings) bool {
	va, vb := reflect.ValueOf(a), reflect.ValueOf(b)
	for i := range va.NumField() {
		if !va.Type().Field(i).IsExported() {
			continue
		}
		if !reflect.DeepEqual(va.Field(i).Interface(), vb.Field(i).Interface()) {
			return false
		}
	}
	return true
}

func TestOMLXAppLayer(t *testing.T) {
	t.Parallel()

	home := t.TempDir()
	if l := OMLXAppLayer(home); l.OMLXModelDirs != nil {
		t.Fatalf("no oMLX install should give an empty layer, got %v", l.OMLXModelDirs)
	}
	write := func(body string) {
		t.Helper()
		dir := filepath.Join(home, ".omlx")
		if err := os.MkdirAll(dir, 0o750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "settings.json"), []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	write(`{"model":{"model_dirs":["/a","/b"],"model_dir":"/a"}}`)
	if got := OMLXAppLayer(home).OMLXModelDirs; !reflect.DeepEqual(got, []string{"/a", "/b"}) {
		t.Errorf("model_dirs = %v", got)
	}
	// Older settings files carry only the single model_dir.
	write(`{"model":{"model_dir":"/only"}}`)
	if got := OMLXAppLayer(home).OMLXModelDirs; !reflect.DeepEqual(got, []string{"/only"}) {
		t.Errorf("model_dir fallback = %v", got)
	}
	write(`not json`)
	if got := OMLXAppLayer(home).OMLXModelDirs; got != nil {
		t.Errorf("unreadable settings should give an empty layer, got %v", got)
	}

	// A base path moved in the app is recorded in its bootstrap file.
	moved := t.TempDir()
	if err := os.WriteFile(filepath.Join(moved, "settings.json"), []byte(`{"model":{"model_dirs":["/moved"]}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	bootstrap := filepath.Join(home, "Library", "Application Support", "oMLX")
	if err := os.MkdirAll(bootstrap, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(bootstrap, "base-path"), []byte(moved+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if got := OMLXAppLayer(home).OMLXModelDirs; !reflect.DeepEqual(got, []string{"/moved"}) {
		t.Errorf("bootstrap base path = %v", got)
	}
}
