package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/flyingnobita/llml/internal/models"
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

func TestApplyRuntimeFromConfig_envWins(t *testing.T) {
	t.Setenv(models.EnvLlamaCppPath, "/from-env")
	t.Cleanup(func() { _ = os.Unsetenv(models.EnvLlamaCppPath) })

	ApplyRuntimeFromConfig(&RuntimeConfig{DefaultLlamaCppPath: "/from-toml"})
	if os.Getenv(models.EnvLlamaCppPath) != "/from-env" {
		t.Fatalf("env should win, got %q", os.Getenv(models.EnvLlamaCppPath))
	}
}

func TestApplyRuntimeFromConfig_tomlFallback(t *testing.T) {
	_ = os.Unsetenv(models.EnvLlamaCppPath)
	t.Cleanup(func() { _ = os.Unsetenv(models.EnvLlamaCppPath) })

	ApplyRuntimeFromConfig(&RuntimeConfig{DefaultLlamaCppPath: "/from-toml"})
	got := os.Getenv(models.EnvLlamaCppPath)
	if got == "" {
		t.Fatal("expected TOML path applied")
	}
	if !filepath.IsAbs(got) {
		t.Fatalf("want absolute path, got %q", got)
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
	}
	out := FilterExistingPaths(files)
	if len(out) != 1 || out[0].Path != gguf {
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

func TestDiscoveryConfigForWrite_merge(t *testing.T) {
	prev := &Config{
		Discovery: DiscoveryConfig{
			ExtraModelPaths: []string{"/a"},
			LastScan:        time.Unix(100, 0).UTC(),
		},
	}
	t.Setenv(models.EnvModelPaths, "/b")
	t.Cleanup(func() { _ = os.Unsetenv(models.EnvModelPaths) })
	d := DiscoveryConfigForWrite(prev, time.Unix(200, 0).UTC())
	if len(d.ExtraModelPaths) != 2 {
		t.Fatalf("paths %v", d.ExtraModelPaths)
	}
	if !d.LastScan.Equal(time.Unix(200, 0).UTC()) {
		t.Fatalf("last scan %v", d.LastScan)
	}
}

func TestDiscoveryConfigFromInputs(t *testing.T) {
	t.Setenv(models.EnvModelPaths, "/env/ignored") // should not be used
	t.Cleanup(func() { _ = os.Unsetenv(models.EnvModelPaths) })

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
