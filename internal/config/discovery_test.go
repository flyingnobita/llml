package config

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/flyingnobita/llml/internal/models"
)

func TestFilterByBackend_SingleMatch(t *testing.T) {
	t.Parallel()
	files := []models.ModelFile{
		{Backend: models.BackendLlama, Path: "/m1.gguf", Name: "m1.gguf"},
		{Backend: models.BackendVLLM, Path: "/m2", Name: "m2"},
		{Backend: models.BackendOllama, ID: "qwen:latest", Name: "qwen:latest"},
	}
	got := FilterByBackend(files, []string{"llama"})
	if len(got) != 1 || got[0].Name != "m1.gguf" {
		t.Fatalf("got %+v", got)
	}
}

func TestFilterByBackend_MultipleMatch(t *testing.T) {
	t.Parallel()
	files := []models.ModelFile{
		{Backend: models.BackendLlama, Path: "/m1.gguf", Name: "m1.gguf"},
		{Backend: models.BackendLlama, Path: "/m2.gguf", Name: "m2.gguf"},
		{Backend: models.BackendVLLM, Path: "/m3", Name: "m3"},
	}
	got := FilterByBackend(files, []string{"llama"})
	if len(got) != 2 {
		t.Fatalf("got %d models, want 2", len(got))
	}
}

func TestFilterByBackend_MultipleBackends(t *testing.T) {
	t.Parallel()
	files := []models.ModelFile{
		{Backend: models.BackendLlama, Path: "/m1.gguf", Name: "m1.gguf"},
		{Backend: models.BackendVLLM, Path: "/m2", Name: "m2"},
		{Backend: models.BackendOllama, ID: "qwen:latest", Name: "qwen:latest"},
	}
	got := FilterByBackend(files, []string{"llama", "vllm"})
	if len(got) != 2 {
		t.Fatalf("got %d models, want 2", len(got))
	}
}

func TestFilterByBackend_NoMatch(t *testing.T) {
	t.Parallel()
	files := []models.ModelFile{
		{Backend: models.BackendLlama, Path: "/m1.gguf", Name: "m1.gguf"},
	}
	got := FilterByBackend(files, []string{"vllm"})
	if len(got) != 0 {
		t.Fatalf("got %+v, want empty", got)
	}
}

func TestFilterByBackend_UnknownBackend(t *testing.T) {
	t.Parallel()
	files := []models.ModelFile{
		{Backend: models.BackendLlama, Path: "/m1.gguf", Name: "m1.gguf"},
	}
	got := FilterByBackend(files, []string{"nonexistent"})
	if len(got) != 0 {
		t.Fatalf("unknown backend should return empty, got %+v", got)
	}
}

func TestFilterByBackend_EmptyBackends(t *testing.T) {
	t.Parallel()
	files := []models.ModelFile{
		{Backend: models.BackendLlama, Path: "/m1.gguf", Name: "m1.gguf"},
	}
	got := FilterByBackend(files, nil)
	if len(got) != 0 {
		t.Fatalf("empty backends should return nil, got %+v", got)
	}
}

func TestFilterByBackend_KoboldCpp(t *testing.T) {
	t.Parallel()
	files := []models.ModelFile{
		{Backend: models.BackendKobold, Path: "/m1.gguf", Name: "m1.gguf"},
		{Backend: models.BackendLlama, Path: "/m2.gguf", Name: "m2.gguf"},
	}
	got := FilterByBackend(files, []string{"koboldcpp"})
	if len(got) != 1 || got[0].Backend != models.BackendKobold {
		t.Fatalf("got %+v", got)
	}
}

func TestModelBackends(t *testing.T) {
	t.Parallel()
	files := []models.ModelFile{
		{Backend: models.BackendLlama, Path: "/m1.gguf", Name: "m1.gguf"},
		{Backend: models.BackendLlama, Path: "/m2.gguf", Name: "m2.gguf"},
		{Backend: models.BackendOllama, ID: "qwen:latest", Name: "qwen:latest"},
	}
	got := ModelBackends(files)
	if len(got) != 2 {
		t.Fatalf("got %v, want 2 backends", got)
	}
	if got[0] != "llama" || got[1] != "ollama" {
		t.Fatalf("got %v, want [llama ollama]", got)
	}
}

func TestCachedModels_NoConfig(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv("HOME", dir)
	t.Setenv("AppData", dir)

	models, err := CachedModels()
	if err != nil {
		t.Fatal(err)
	}
	if models != nil {
		t.Fatal("expected nil models when no config exists")
	}
}

func TestCachedModels_Valid(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv("HOME", dir)
	t.Setenv("AppData", dir)

	c := Config{
		SchemaVersion: SchemaVersion,
		Discovery: DiscoveryConfig{
			LastScan: time.Now(),
		},
		Models: []ModelEntry{
			{Backend: "llama", Path: "/m.gguf", Name: "m.gguf", Size: 100, ModTime: time.Now()},
		},
	}
	if err := WriteFile(c); err != nil {
		t.Fatal(err)
	}

	models, err := CachedModels()
	if err != nil {
		t.Fatal(err)
	}
	if len(models) != 1 || models[0].Name != "m.gguf" {
		t.Fatalf("got %+v", models)
	}
}

func TestCachedModels_Stale(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv("HOME", dir)
	t.Setenv("AppData", dir)

	c := Config{
		SchemaVersion: SchemaVersion,
		Discovery: DiscoveryConfig{
			LastScan: time.Now().Add(-48 * time.Hour),
		},
		Models: []ModelEntry{
			{Backend: "llama", Path: "/m.gguf", Name: "m.gguf", Size: 100, ModTime: time.Now()},
		},
	}
	if err := WriteFile(c); err != nil {
		t.Fatal(err)
	}

	models, err := CachedModels()
	if err == nil {
		t.Fatal("expected stale error")
	}
	var stale *CacheStaleError
	if !errors.As(err, &stale) {
		t.Fatalf("expected CacheStaleError, got %T: %v", err, err)
	}
	if len(models) != 1 {
		t.Fatalf("stale cache should still return models, got %d", len(models))
	}
}

func TestCachedModels_EmptyModels(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv("HOME", dir)
	t.Setenv("AppData", dir)

	c := Config{
		SchemaVersion: SchemaVersion,
		Discovery: DiscoveryConfig{
			LastScan: time.Now(),
		},
		Models: nil,
	}
	if err := WriteFile(c); err != nil {
		t.Fatal(err)
	}

	models, err := CachedModels()
	if err != nil {
		t.Fatal(err)
	}
	if models != nil {
		t.Fatal("expected nil models for empty cache")
	}
}

func TestCachedModels_WrongSchema(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv("HOME", dir)
	t.Setenv("AppData", dir)

	// Write raw TOML with wrong schema_version (WriteFile normalizes to current).
	cfgPath, err := ConfigPath()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(cfgPath), 0o755); err != nil {
		t.Fatal(err)
	}
	raw := "schema_version = 1\n[discovery]\nlast_scan = 2026-05-12T00:00:00Z\n[[models]]\nbackend = \"llama\"\npath = \"/m.gguf\"\nname = \"m.gguf\"\nsize = 100\n"
	if err := os.WriteFile(cfgPath, []byte(raw), 0o644); err != nil {
		t.Fatal(err)
	}

	models, err := CachedModels()
	if err != nil {
		t.Fatal(err)
	}
	if models != nil {
		t.Fatal("expected nil models for wrong schema")
	}
}

func TestRunDiscovery_WritesConfig(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv("HOME", dir)
	t.Setenv("AppData", dir)
	t.Setenv("LLML_MODEL_PATHS", "")

	// Create a fake .gguf file so discovery finds something.
	modelsDir := filepath.Join(dir, "models")
	if err := os.MkdirAll(modelsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(modelsDir, "test.gguf"), []byte("fake"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("XDG_DATA_HOME", dir)
	t.Setenv("HOME", dir)

	t.Setenv(models.EnvModelPaths, modelsDir)

	models, err := RunDiscovery()
	if err != nil {
		t.Fatal(err)
	}
	if len(models) == 0 {
		t.Fatal("expected at least one discovered model")
	}

	// Verify config.toml was written.
	cfg, err := ReadFile()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.SchemaVersion != SchemaVersion {
		t.Fatalf("schema %d", cfg.SchemaVersion)
	}
	if !cfg.ValidForCache() {
		t.Fatal("config should be valid for cache after discovery")
	}
}

func TestCacheStaleError_Error(t *testing.T) {
	ts := time.Date(2026, 5, 12, 10, 30, 0, 0, time.UTC)
	e := &CacheStaleError{LastScan: ts}
	msg := e.Error()
	if !strings.Contains(msg, "2026-05-12T10:30:00Z") {
		t.Fatalf("unexpected error message: %s", msg)
	}
}
