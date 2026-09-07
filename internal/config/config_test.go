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
		SchemaVersion: 3,
		Runtime: RuntimeConfig{
			DefaultLlamaCppPath:    "/opt/llama",
			DefaultLlamaServerPort: &p1,
			DefaultVLLMServerPort:  &p2,
		},
		Discovery: DiscoveryConfig{
			ExtraModelPaths: []string{"/extra/models"},
			LastScan:        time.Unix(1700000000, 0).UTC(),
		},
		Models: []ModelEntry{
			{
				Backend:    "llama",
				Path:       "/models/a.gguf",
				Name:       "a.gguf",
				Size:       100,
				ModTime:    time.Unix(1600000000, 0).UTC(),
				Parameters: "llama · 4096 ctx",
			},
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
	if len(got.Models) != 1 || got.Models[0].Path != "/models/a.gguf" {
		t.Fatalf("models %+v", got.Models)
	}
}

func TestValidForCache(t *testing.T) {
	t.Parallel()
	if (Config{SchemaVersion: 0}).ValidForCache() {
		t.Fatal("wrong schema should not validate")
	}
	if (Config{SchemaVersion: 3, Models: nil}).ValidForCache() {
		t.Fatal("empty models should not validate")
	}
	if !(Config{SchemaVersion: 3, Models: []ModelEntry{{Path: "/x"}}}).ValidForCache() {
		t.Fatal("valid config should validate")
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
			LastScan:        time.Unix(100, 0).UTC(),
		},
	}
	s := settings.Settings{ExtraModelPaths: []string{"/b"}}
	d := DiscoveryConfigForWrite(prev, s, time.Unix(200, 0).UTC())
	if len(d.ExtraModelPaths) != 2 {
		t.Fatalf("paths %v", d.ExtraModelPaths)
	}
	if !d.LastScan.Equal(time.Unix(200, 0).UTC()) {
		t.Fatalf("last scan %v", d.LastScan)
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
		SchemaVersion: 3,
		Runtime: RuntimeConfig{
			DefaultLlamaCppPath:    "/opt/llama",
			DefaultKoboldCppPath:   "/opt/koboldcpp",
			DefaultLlamaServerPort: &p1,
			DefaultVLLMServerPort:  &p2,
			DefaultKoboldCppPort:   &kp,
		},
		Discovery: DiscoveryConfig{
			ExtraModelPaths: []string{"/extra/models"},
			LastScan:        time.Unix(1700000000, 0).UTC(),
		},
		Models: []ModelEntry{
			{
				Backend:    "llama",
				Path:       "/models/a.gguf",
				Name:       "a.gguf",
				Size:       100,
				ModTime:    time.Unix(1600000000, 0).UTC(),
				Parameters: "llama · 4096 ctx",
			},
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
	lastScan := time.Unix(300, 0).UTC()
	d := DiscoveryConfigFromInputs(paths, lastScan)

	if len(d.ExtraModelPaths) != 2 {
		t.Fatalf("want 2 paths, got %v", d.ExtraModelPaths)
	}
	if filepath.ToSlash(d.ExtraModelPaths[0]) != "/a" {
		t.Errorf("got %q", d.ExtraModelPaths[0])
	}
	if filepath.ToSlash(d.ExtraModelPaths[1]) != "/c" {
		t.Errorf("got %q", d.ExtraModelPaths[1])
	}
	if !d.LastScan.Equal(lastScan) {
		t.Fatalf("last scan %v", d.LastScan)
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

// RuntimeConfigFromSettings must round-trip through Layer unchanged, so writing
// the file and reading it back does not shift the resolved values.
func TestRuntimeConfigFromSettings_roundTrips(t *testing.T) {
	t.Parallel()

	want := settings.Resolve(settings.FromEnv(fakeEnv(map[string]string{
		settings.EnvLlamaCppPath:   "/opt/llama",
		settings.EnvKoboldCppPort:  "6000",
		settings.EnvOllamaHost:     "box:11434",
		settings.EnvVLLMServerHost: "0.0.0.0",
	})), settings.Defaults())

	rc := RuntimeConfigFromSettings(want)
	got := settings.Resolve(settings.FromEnv(fakeEnv(nil)), rc.Layer(), settings.Defaults())

	got.ExtraModelPaths = want.ExtraModelPaths // not part of the [runtime] table
	if !reflect.DeepEqual(got, want) {
		t.Errorf("round trip changed settings:\n got %+v\nwant %+v", got, want)
	}
}
