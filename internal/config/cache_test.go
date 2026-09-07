package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/flyingnobita/llml/internal/models"
	"github.com/flyingnobita/llml/internal/settings"
)

// isolatedConfigDir points os.UserConfigDir at a temp directory.
func isolatedConfigDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv("HOME", dir)
	t.Setenv("AppData", dir)
	return dir
}

func TestCacheRoundTrip(t *testing.T) {
	isolatedConfigDir(t)

	want := CacheFromFiles([]models.ModelFile{
		{Backend: models.BackendLlama, Path: "/m/a.gguf", Name: "a.gguf", Size: 3, ModTime: time.Unix(10, 0).UTC()},
	}, time.Unix(99, 0).UTC())

	if err := WriteCache(want); err != nil {
		t.Fatal(err)
	}
	got, err := ReadCache()
	if err != nil {
		t.Fatal(err)
	}
	if !got.ValidForCache() {
		t.Fatalf("round-tripped cache should be valid: %+v", got)
	}
	if !got.LastScan.Equal(want.LastScan) {
		t.Errorf("LastScan = %v, want %v", got.LastScan, want.LastScan)
	}
	if len(got.Models) != 1 || got.Models[0].Name != "a.gguf" {
		t.Errorf("models = %+v", got.Models)
	}
}

func TestReadCache_missingFileIsNotFatal(t *testing.T) {
	isolatedConfigDir(t)

	got, err := ReadCache()
	if err == nil {
		t.Fatal("expected an error for a missing cache file")
	}
	if got.ValidForCache() {
		t.Error("a missing cache must not be valid")
	}
}

// A version 3 file carries its models inline. Reading it must move them to the
// cache and rewrite config.toml at version 4 without the [[models]] table.
func TestReadFile_migratesV3CacheOutOfConfig(t *testing.T) {
	isolatedConfigDir(t)

	path, err := ConfigPath()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	v3 := `schema_version = 3

[runtime]
  default_llama_cpp_path = "/opt/llama"

[discovery]
  extra_model_paths = ["/roots/a"]
  last_scan = 2026-01-02T03:04:05Z

[[models]]
  backend = "llama"
  id = "/m/a.gguf"
  path = "/m/a.gguf"
  name = "a.gguf"
  size = 3
  mod_time = 2026-01-02T03:04:05Z
  parameters = "7B"
`
	if err := os.WriteFile(path, []byte(v3), 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := ReadFile()
	if err != nil {
		t.Fatal(err)
	}
	if got.SchemaVersion != SchemaVersion {
		t.Errorf("SchemaVersion = %d, want %d", got.SchemaVersion, SchemaVersion)
	}
	if got.LegacyModels != nil {
		t.Errorf("LegacyModels should be drained, got %+v", got.LegacyModels)
	}
	if got.Runtime.DefaultLlamaCppPath != "/opt/llama" {
		t.Errorf("runtime lost in migration: %+v", got.Runtime)
	}
	if len(got.Discovery.ExtraModelPaths) != 1 {
		t.Errorf("discovery paths lost in migration: %+v", got.Discovery)
	}

	// The models moved to the cache, with the scan time that was in [discovery].
	cached, err := ReadCache()
	if err != nil {
		t.Fatal(err)
	}
	if len(cached.Models) != 1 || cached.Models[0].Name != "a.gguf" {
		t.Fatalf("cache models = %+v", cached.Models)
	}
	// The old scan time carries over, so the migrated cache is not instantly stale.
	if want := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC); !cached.LastScan.Equal(want) {
		t.Errorf("cache LastScan = %v, want %v", cached.LastScan, want)
	}
	if got.Discovery.LegacyLastScan != (time.Time{}) {
		t.Errorf("LegacyLastScan should be drained, got %v", got.Discovery.LegacyLastScan)
	}

	// config.toml no longer carries the cache.
	onDisk, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(onDisk), "[[models]]") {
		t.Errorf("config.toml still has [[models]]:\n%s", onDisk)
	}

	// The pre-migration file is recoverable from backups/.
	backups := filepath.Join(filepath.Dir(path), "backups")
	entries, err := os.ReadDir(backups)
	if err != nil || len(entries) == 0 {
		t.Errorf("expected a backup of the pre-migration config, got %v (%v)", entries, err)
	}
}

// Migration must be idempotent: a second read changes nothing.
func TestReadFile_migrationIsIdempotent(t *testing.T) {
	isolatedConfigDir(t)

	path, _ := ConfigPath()
	_ = os.MkdirAll(filepath.Dir(path), 0o700)
	if err := os.WriteFile(path, []byte("schema_version = 3\n\n[[models]]\n  backend = \"llama\"\n  path = \"/m/a.gguf\"\n  name = \"a.gguf\"\n  size = 1\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := ReadFile(); err != nil {
		t.Fatal(err)
	}
	first, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ReadFile(); err != nil {
		t.Fatal(err)
	}
	second, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(first) != string(second) {
		t.Errorf("second read rewrote config.toml:\n%s\nvs\n%s", first, second)
	}
}

// The point of the split: a scan writes the cache and leaves config.toml alone.
func TestRunDiscovery_LeavesConfigByteIdentical(t *testing.T) {
	dir := isolatedConfigDir(t)

	modelsDir := filepath.Join(dir, "models")
	if err := os.MkdirAll(modelsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(modelsDir, "test.gguf"), []byte("fake"), 0o644); err != nil {
		t.Fatal(err)
	}

	path, _ := ConfigPath()
	_ = os.MkdirAll(filepath.Dir(path), 0o700)
	handEdited := "# my notes, please keep\nschema_version = 4\n\n[runtime]\n  default_llama_cpp_path = \"/opt/llama\"\n"
	if err := os.WriteFile(path, []byte(handEdited), 0o644); err != nil {
		t.Fatal(err)
	}

	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := RunDiscovery(t.Context(), settingsForRoots(modelsDir)); err != nil {
		t.Fatal(err)
	}
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(before) != string(after) {
		t.Errorf("a scan rewrote config.toml:\nbefore:\n%s\nafter:\n%s", before, after)
	}

	cached, err := ReadCache()
	if err != nil {
		t.Fatal(err)
	}
	if !cached.ValidForCache() {
		t.Errorf("the scan should have populated the cache, got %+v", cached)
	}
}

// settingsForRoots builds settings whose only extra search root is dir.
func settingsForRoots(dir string) settings.Settings {
	return settings.Resolve(settings.Layer{ExtraModelPaths: []string{dir}}, settings.Defaults())
}
