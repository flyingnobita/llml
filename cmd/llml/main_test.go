package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/flyingnobita/llml/internal/config"
	"github.com/flyingnobita/llml/internal/models"
	"github.com/flyingnobita/llml/internal/profiles"
)

func TestUniqueBackendsFromPortable(t *testing.T) {
	t.Parallel()
	pp := []profiles.PortableProfile{
		{Backend: "llama"},
		{Backend: "vllm"},
		{Backend: "llama"},
		{Backend: ""}, // empty normalizes to "llama", already in set
	}
	got := uniqueBackendsFromPortable(pp)
	if len(got) != 2 {
		t.Fatalf("got %v, want 2 backends (empty normalizes to llama)", got)
	}
}

func TestUniqueBackendsFromPortable_EmptyDefaultsToLlama(t *testing.T) {
	t.Parallel()
	pp := []profiles.PortableProfile{
		{Backend: ""},
	}
	got := uniqueBackendsFromPortable(pp)
	if len(got) != 1 || got[0] != "llama" {
		t.Fatalf("got %v, want [llama]", got)
	}
}

func TestUniqueBackendsFromPortable_WhitespaceTrimmed(t *testing.T) {
	t.Parallel()
	pp := []profiles.PortableProfile{
		{Backend: "  vllm  "},
	}
	got := uniqueBackendsFromPortable(pp)
	if len(got) != 1 || got[0] != "vllm" {
		t.Fatalf("got %v, want [vllm]", got)
	}
}

func TestIsTerminal(t *testing.T) {
	// In test environment, stdin is typically not a TTY.
	if isTerminal() {
		t.Log("stdin is a TTY (unusual in tests)")
	}
}

func TestPresentModelPicker_SelectFirst(t *testing.T) {
	mdls := []models.ModelFile{
		{Backend: models.BackendLlama, Path: "/m1.gguf", Name: "m1.gguf"},
		{Backend: models.BackendLlama, Path: "/m2.gguf", Name: "m2.gguf"},
	}
	r := strings.NewReader("1\n")
	got, err := presentModelPicker(mdls, r)
	if err != nil {
		t.Fatal(err)
	}
	if got != "/m1.gguf" {
		t.Fatalf("got %q, want /m1.gguf", got)
	}
}

func TestPresentModelPicker_Cancel(t *testing.T) {
	mdls := []models.ModelFile{
		{Backend: models.BackendLlama, Path: "/m1.gguf", Name: "m1.gguf"},
	}
	r := strings.NewReader("q\n")
	_, err := presentModelPicker(mdls, r)
	if err == nil {
		t.Fatal("expected cancel error")
	}
	if !strings.Contains(err.Error(), "cancelled") {
		t.Fatalf("got %v", err)
	}
}

func TestPresentModelPicker_InvalidThenSelect(t *testing.T) {
	mdls := []models.ModelFile{
		{Backend: models.BackendLlama, Path: "/m1.gguf", Name: "m1.gguf"},
	}
	r := strings.NewReader("abc\n5\n1\n")
	got, err := presentModelPicker(mdls, r)
	if err != nil {
		t.Fatal(err)
	}
	if got != "/m1.gguf" {
		t.Fatalf("got %q", got)
	}
}

func TestPresentModelPicker_OutOfRange(t *testing.T) {
	mdls := []models.ModelFile{
		{Backend: models.BackendLlama, Path: "/m1.gguf", Name: "m1.gguf"},
	}
	r := strings.NewReader("0\n2\n")
	_, err := presentModelPicker(mdls, r)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestPresentModelPicker_EOF(t *testing.T) {
	mdls := []models.ModelFile{
		{Backend: models.BackendLlama, Path: "/m1.gguf", Name: "m1.gguf"},
	}
	r := strings.NewReader("")
	_, err := presentModelPicker(mdls, r)
	if err == nil {
		t.Fatal("expected cancel error on EOF")
	}
}

func TestPresentModelPicker_OllamaIdentity(t *testing.T) {
	mdls := []models.ModelFile{
		{Backend: models.BackendOllama, ID: "qwen3.5:latest", Location: "ollama://qwen3.5:latest", Name: "qwen3.5:latest"},
	}
	r := strings.NewReader("1\n")
	got, err := presentModelPicker(mdls, r)
	if err != nil {
		t.Fatal(err)
	}
	if got != "qwen3.5:latest" {
		t.Fatalf("got %q, want qwen3.5:latest", got)
	}
}

// pickTargetModel tests — these mock isTerminal to return false so we test
// error paths without hitting stdin. Ollama API is disabled via dead host.

func disableOllama(t *testing.T) {
	t.Helper()
	t.Setenv("OLLAMA_HOST", "http://127.0.0.1:1")
}

func disableTerminal(t *testing.T) {
	t.Helper()
	old := isTerminal
	isTerminal = func() bool { return false }
	t.Cleanup(func() { isTerminal = old })
}

func setupConfigDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv("HOME", dir)
	t.Setenv("AppData", dir)
	t.Setenv("LLML_MODEL_PATHS", "")
	return dir
}

func TestPickTargetModel_NoCachedModels(t *testing.T) {
	setupConfigDir(t)
	disableOllama(t)
	disableTerminal(t)

	// No config.toml — CachedModels returns nil,nil.
	// Auto-runs discovery, finds nothing → "no local model files found".
	pp := []profiles.PortableProfile{{Backend: "llama"}}
	_, err := pickTargetModel(pp, false)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "no local model files found") {
		t.Fatalf("got %v", err)
	}
}

func TestPickTargetModel_NoCompatibleBackend(t *testing.T) {
	dir := setupConfigDir(t)
	disableOllama(t)
	disableTerminal(t)

	// Create a fake .gguf file so discovery finds a llama model.
	modelsDir := filepath.Join(dir, "models")
	if err := os.MkdirAll(modelsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(modelsDir, "test.gguf"), []byte("fake"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv(models.EnvModelPaths, modelsDir)

	// Profile requests vllm backend, but only llama models are on disk.
	pp := []profiles.PortableProfile{{Backend: "vllm"}}
	_, err := pickTargetModel(pp, false)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "no compatible local models") {
		t.Fatalf("got %v", err)
	}
	if !strings.Contains(err.Error(), "backend: vllm") {
		t.Fatalf("missing backend info: %v", err)
	}
	if !strings.Contains(err.Error(), "discovered: llama") {
		t.Fatalf("missing discovered info: %v", err)
	}
}

func TestPickTargetModel_NonTTY(t *testing.T) {
	_ = setupConfigDir(t)
	disableOllama(t)
	disableTerminal(t)

	// Write a valid cache with a llama model.
	c := config.Config{
		SchemaVersion: config.SchemaVersion,
		Discovery: config.DiscoveryConfig{
			LastScan: time.Now(),
		},
		Models: []config.ModelEntry{
			{Backend: "llama", Path: "/m.gguf", Name: "m.gguf", Size: 100, ModTime: time.Now()},
		},
	}
	if err := config.WriteFile(c); err != nil {
		t.Fatal(err)
	}

	pp := []profiles.PortableProfile{{Backend: "llama"}}
	_, err := pickTargetModel(pp, false)
	if err == nil {
		t.Fatal("expected non-TTY error")
	}
	if !strings.Contains(err.Error(), "not a terminal") {
		t.Fatalf("got %v", err)
	}
}

func TestPickTargetModel_StaleCache(t *testing.T) {
	dir := setupConfigDir(t)
	disableOllama(t)
	disableTerminal(t)

	// Create a fake .gguf file so rescan finds something.
	modelsDir := filepath.Join(dir, "models")
	if err := os.MkdirAll(modelsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(modelsDir, "test.gguf"), []byte("fake"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv(models.EnvModelPaths, modelsDir)

	// Write a stale cache.
	c := config.Config{
		SchemaVersion: config.SchemaVersion,
		Discovery: config.DiscoveryConfig{
			LastScan: time.Now().Add(-48 * time.Hour),
		},
		Models: []config.ModelEntry{
			{Backend: "llama", Path: "/old.gguf", Name: "old.gguf", Size: 100, ModTime: time.Now()},
		},
	}
	if err := config.WriteFile(c); err != nil {
		t.Fatal(err)
	}

	pp := []profiles.PortableProfile{{Backend: "llama"}}
	_, err := pickTargetModel(pp, false)
	// Stale cache → rescan → finds model → non-TTY error (mocked).
	if err == nil {
		t.Fatal("expected error after rescan")
	}
	if !strings.Contains(err.Error(), "not a terminal") {
		t.Fatalf("got %v", err)
	}
}

func TestPickTargetModel_Rescan(t *testing.T) {
	dir := setupConfigDir(t)
	disableOllama(t)
	disableTerminal(t)

	// Create a fake .gguf file so rescan finds something.
	modelsDir := filepath.Join(dir, "models")
	if err := os.MkdirAll(modelsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(modelsDir, "test.gguf"), []byte("fake"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv(models.EnvModelPaths, modelsDir)

	pp := []profiles.PortableProfile{{Backend: "llama"}}
	_, err := pickTargetModel(pp, true) // --rescan
	if err == nil {
		t.Fatal("expected non-TTY error after rescan")
	}
	if !strings.Contains(err.Error(), "not a terminal") {
		t.Fatalf("got %v", err)
	}
}

func TestPickTargetModel_EmptyDiscovery(t *testing.T) {
	setupConfigDir(t)
	disableOllama(t)
	disableTerminal(t)

	pp := []profiles.PortableProfile{{Backend: "llama"}}
	_, err := pickTargetModel(pp, false)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "no local model files found") {
		t.Fatalf("got %v", err)
	}
}

// validateTargetBackend tests

func TestValidateTargetBackend_Compatible(t *testing.T) {
	setupConfigDir(t)
	disableOllama(t)

	c := config.Config{
		SchemaVersion: config.SchemaVersion,
		Discovery:     config.DiscoveryConfig{LastScan: time.Now()},
		Models: []config.ModelEntry{
			{Backend: "llama", Path: "/m.gguf", Name: "m.gguf", Size: 100, ModTime: time.Now()},
		},
	}
	if err := config.WriteFile(c); err != nil {
		t.Fatal(err)
	}

	pp := []profiles.PortableProfile{{Backend: "llama"}}
	err := validateTargetBackend("/m.gguf", pp, false)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestValidateTargetBackend_Incompatible(t *testing.T) {
	setupConfigDir(t)
	disableOllama(t)

	c := config.Config{
		SchemaVersion: config.SchemaVersion,
		Discovery:     config.DiscoveryConfig{LastScan: time.Now()},
		Models: []config.ModelEntry{
			{Backend: "llama", Path: "/m.gguf", Name: "m.gguf", Size: 100, ModTime: time.Now()},
		},
	}
	if err := config.WriteFile(c); err != nil {
		t.Fatal(err)
	}

	pp := []profiles.PortableProfile{{Backend: "vllm"}}
	err := validateTargetBackend("/m.gguf", pp, false)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "no compatible local models") {
		t.Fatalf("got %v", err)
	}
}

func TestValidateTargetBackend_TargetNotInCache(t *testing.T) {
	setupConfigDir(t)
	disableOllama(t)

	c := config.Config{
		SchemaVersion: config.SchemaVersion,
		Discovery:     config.DiscoveryConfig{LastScan: time.Now()},
		Models: []config.ModelEntry{
			{Backend: "llama", Path: "/other.gguf", Name: "other.gguf", Size: 100, ModTime: time.Now()},
		},
	}
	if err := config.WriteFile(c); err != nil {
		t.Fatal(err)
	}

	pp := []profiles.PortableProfile{{Backend: "llama"}}
	// Target not in discovered models — should pass (user knows better).
	err := validateTargetBackend("/m.gguf", pp, false)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestValidateTargetBackend_EmptyCache(t *testing.T) {
	setupConfigDir(t)
	disableOllama(t)

	pp := []profiles.PortableProfile{{Backend: "llama"}}
	// No cache, no models on disk — resolveModels returns empty, validateTargetBackend
	// returns nil (lets user proceed).
	err := validateTargetBackend("/m.gguf", pp, false)
	if err != nil {
		t.Fatalf("expected no error on empty cache, got %v", err)
	}
}

func TestValidateTargetBackend_OllamaTarget(t *testing.T) {
	setupConfigDir(t)
	disableOllama(t) // disable real Ollama API, but use a cached Ollama entry

	c := config.Config{
		SchemaVersion: config.SchemaVersion,
		Discovery:     config.DiscoveryConfig{LastScan: time.Now()},
		Models: []config.ModelEntry{
			{Backend: "ollama", ID: "qwen3.5:latest", Location: "ollama://qwen3.5:latest", Name: "qwen3.5:latest", Size: 100, ModTime: time.Now()},
		},
	}
	if err := config.WriteFile(c); err != nil {
		t.Fatal(err)
	}

	pp := []profiles.PortableProfile{{Backend: "ollama"}}
	err := validateTargetBackend("qwen3.5:latest", pp, false)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestValidateTargetBackend_Rescan(t *testing.T) {
	dir := setupConfigDir(t)
	disableOllama(t)

	// Create a fake .gguf so rescan finds a llama model.
	modelsDir := filepath.Join(dir, "models")
	if err := os.MkdirAll(modelsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(modelsDir, "test.gguf"), []byte("fake"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv(models.EnvModelPaths, modelsDir)

	pp := []profiles.PortableProfile{{Backend: "vllm"}}
	// Rescan finds llama models, but profile wants vllm → incompatible.
	err := validateTargetBackend(filepath.Join(modelsDir, "test.gguf"), pp, true)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "no compatible local models") {
		t.Fatalf("got %v", err)
	}
}
