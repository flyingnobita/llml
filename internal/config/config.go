// Package config persists llml runtime and discovery cache in human-readable TOML.
// Parameter profiles remain in model-params.json; see [github.com/flyingnobita/llml/internal/tui].
package config

import (
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/BurntSushi/toml"

	"github.com/flyingnobita/llml/internal/fsutil"
	"github.com/flyingnobita/llml/internal/models"
	"github.com/flyingnobita/llml/internal/userdata"
)

// SchemaVersion is the current on-disk format for config.toml.
// When bumping this, migrate after backing up (WriteFile already snapshots the
// previous file under backups/ before overwrite).
const SchemaVersion = 3

// Config is the root document stored at [ConfigPath].
type Config struct {
	SchemaVersion int             `toml:"schema_version"`
	Runtime       RuntimeConfig   `toml:"runtime"`
	Discovery     DiscoveryConfig `toml:"discovery"`
	Models        []ModelEntry    `toml:"models"`
}

// RuntimeConfig mirrors env vars LLAMA_CPP_PATH, VLLM_PATH, VLLM_VENV, and server ports.
// Empty strings mean unset; ports use pointers so zero can mean "omit default in file".
type RuntimeConfig struct {
	DefaultLlamaCppPath    string `toml:"default_llama_cpp_path"`
	DefaultLlamaServerHost string `toml:"default_llama_server_host"`
	DefaultVLLMPath        string `toml:"default_vllm_path"`
	DefaultVLLMServerHost  string `toml:"default_vllm_server_host"`
	DefaultVLLMVenv        string `toml:"default_vllm_venv"`
	DefaultOllamaPath      string `toml:"default_ollama_path"`
	DefaultOllamaHost      string `toml:"default_ollama_host"`
	DefaultKoboldCppPath   string `toml:"default_koboldcpp_path"`
	DefaultLlamaServerPort *int   `toml:"default_llama_server_port,omitempty"`
	DefaultVLLMServerPort  *int   `toml:"default_vllm_server_port,omitempty"`
	DefaultKoboldCppPort   *int   `toml:"default_koboldcpp_port,omitempty"`
}

// DiscoveryConfig holds extra search roots and the last full filesystem scan time.
type DiscoveryConfig struct {
	ExtraModelPaths []string  `toml:"extra_model_paths"`
	LastScan        time.Time `toml:"last_scan"`
}

// ModelEntry is one cached model row from discovery.
type ModelEntry struct {
	Backend    string    `toml:"backend"`
	ID         string    `toml:"id,omitempty"`
	Path       string    `toml:"path"`
	Location   string    `toml:"location,omitempty"`
	Name       string    `toml:"name"`
	Size       int64     `toml:"size"`
	ModTime    time.Time `toml:"mod_time"`
	Parameters string    `toml:"parameters"`
}

// ConfigPath returns {UserConfigDir}/llml/config.toml.
func ConfigPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "llml", "config.toml"), nil
}

// ReadFile reads and parses config.toml, or returns an empty error if the file is missing.
//
//nolint:gosec // G304: path from ConfigPath() using os.UserConfigDir — trusted source.
func ReadFile() (Config, error) {
	path, err := ConfigPath()
	if err != nil {
		return Config{}, err
	}
	b, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return Config{}, err
		}
		return Config{}, err
	}
	var c Config
	if _, err := toml.Decode(string(b), &c); err != nil {
		return Config{}, err
	}
	return c, nil
}

// ValidForCache reports whether the file is usable for instant startup (skip filesystem walk).
func (c Config) ValidForCache() bool {
	if c.SchemaVersion != SchemaVersion {
		return false
	}
	if len(c.Models) < 1 {
		return false
	}
	return true
}

// ApplyRuntimeFromConfig sets process environment from [runtime] only where the
// corresponding env var is currently unset (env wins over TOML).
func ApplyRuntimeFromConfig(r *RuntimeConfig) {
	if r == nil {
		return
	}
	applyPathIfUnset(models.EnvLlamaCppPath, r.DefaultLlamaCppPath)
	applyPathIfUnset(models.EnvVLLMPath, r.DefaultVLLMPath)
	applyPathIfUnset(models.EnvVLLMVenv, r.DefaultVLLMVenv)
	applyPathIfUnset(models.EnvOllamaPath, r.DefaultOllamaPath)
	applyPathIfUnset(models.EnvKoboldCppPath, r.DefaultKoboldCppPath)
	if v := strings.TrimSpace(r.DefaultLlamaServerHost); v != "" && os.Getenv(models.EnvLlamaServerHost) == "" {
		_ = os.Setenv(models.EnvLlamaServerHost, v)
	}
	if v := strings.TrimSpace(r.DefaultVLLMServerHost); v != "" && os.Getenv(models.EnvVLLMServerHost) == "" {
		_ = os.Setenv(models.EnvVLLMServerHost, v)
	}
	if v := strings.TrimSpace(r.DefaultOllamaHost); v != "" && os.Getenv(models.EnvOllamaHost) == "" {
		_ = os.Setenv(models.EnvOllamaHost, v)
	}
	if r.DefaultLlamaServerPort != nil && os.Getenv(models.EnvLlamaServerPort) == "" {
		_ = os.Setenv(models.EnvLlamaServerPort, strconv.Itoa(*r.DefaultLlamaServerPort))
	}
	if r.DefaultVLLMServerPort != nil && os.Getenv(models.EnvVLLMServerPort) == "" {
		_ = os.Setenv(models.EnvVLLMServerPort, strconv.Itoa(*r.DefaultVLLMServerPort))
	}
	if r.DefaultKoboldCppPort != nil && os.Getenv(models.EnvKoboldCppPort) == "" {
		_ = os.Setenv(models.EnvKoboldCppPort, strconv.Itoa(*r.DefaultKoboldCppPort))
	}
}

// normalizePath trims, expands tilde, and cleans a filesystem path string.
// Returns "" for empty or whitespace-only inputs.
func normalizePath(v string) string {
	if v = strings.TrimSpace(v); v == "" {
		return ""
	}
	return filepath.Clean(models.ExpandTildePath(v))
}

func applyPathIfUnset(key, value string) {
	v := normalizePath(value)
	if v == "" || v == "." || os.Getenv(key) != "" {
		return
	}
	_ = os.Setenv(key, v)
}

// RuntimeFromEnv builds a RuntimeConfig from the current process environment (for writing).
func RuntimeFromEnv() RuntimeConfig {
	var r RuntimeConfig
	if v := normalizePath(os.Getenv(models.EnvLlamaCppPath)); v != "" {
		r.DefaultLlamaCppPath = v
	}
	if v := normalizePath(os.Getenv(models.EnvVLLMPath)); v != "" {
		r.DefaultVLLMPath = v
	}
	if v := normalizePath(os.Getenv(models.EnvVLLMVenv)); v != "" {
		r.DefaultVLLMVenv = v
	}
	if v := normalizePath(os.Getenv(models.EnvOllamaPath)); v != "" {
		r.DefaultOllamaPath = v
	}
	if v := normalizePath(os.Getenv(models.EnvKoboldCppPath)); v != "" {
		r.DefaultKoboldCppPath = v
	}
	if v := strings.TrimSpace(os.Getenv(models.EnvLlamaServerHost)); v != "" {
		r.DefaultLlamaServerHost = v
	} else {
		r.DefaultLlamaServerHost = models.LlamaServerHost()
	}
	if v := strings.TrimSpace(os.Getenv(models.EnvVLLMServerHost)); v != "" {
		r.DefaultVLLMServerHost = v
	} else {
		r.DefaultVLLMServerHost = models.VllmServerHost()
	}
	if v := strings.TrimSpace(os.Getenv(models.EnvOllamaHost)); v != "" {
		r.DefaultOllamaHost = v
	} else {
		r.DefaultOllamaHost = models.OllamaHost()
	}
	if v := strings.TrimSpace(os.Getenv(models.EnvLlamaServerPort)); v != "" {
		if p, err := strconv.Atoi(v); err == nil && p > 0 && p <= 65535 {
			r.DefaultLlamaServerPort = &p
		}
	} else {
		p := models.ListenPort()
		r.DefaultLlamaServerPort = &p
	}
	if v := strings.TrimSpace(os.Getenv(models.EnvVLLMServerPort)); v != "" {
		if p, err := strconv.Atoi(v); err == nil && p > 0 && p <= 65535 {
			r.DefaultVLLMServerPort = &p
		}
	} else {
		p := models.VLLMPort()
		r.DefaultVLLMServerPort = &p
	}
	if v := strings.TrimSpace(os.Getenv(models.EnvKoboldCppPort)); v != "" {
		if p, err := strconv.Atoi(v); err == nil && p > 0 && p <= 65535 {
			r.DefaultKoboldCppPort = &p
		}
	} else {
		p := models.KoboldCppPort()
		r.DefaultKoboldCppPort = &p
	}
	return r
}

// ExtraModelPathsFromEnv returns comma-separated LLML_MODEL_PATHS entries.
func ExtraModelPathsFromEnv() []string {
	v := strings.TrimSpace(os.Getenv(models.EnvModelPaths))
	if v == "" {
		return nil
	}
	var out []string
	for part := range strings.SplitSeq(v, ",") {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}

// MergeExtraRoots combines discovery extra paths from config with env-only extras for Discover options.
// Config file paths are merged with env in [models.MergeSearchRoots] via Options.ExtraRoots.
func MergeExtraRoots(discoveryExtra, envExtra []string) []string {
	ps := models.NewPathSet()
	for _, p := range discoveryExtra {
		ps.Add(p)
	}
	for _, p := range envExtra {
		ps.Add(p)
	}
	return ps.Slice()
}

// BuildConfig builds a full Config for writing from runtime, discovery, and models.
func BuildConfig(runtime RuntimeConfig, discovery DiscoveryConfig, files []models.ModelFile) Config {
	c := Config{
		SchemaVersion: SchemaVersion,
		Runtime:       runtime,
		Discovery:     discovery,
	}
	for _, f := range files {
		c.Models = append(c.Models, ModelEntryFromFile(f))
	}
	return c
}

// DiscoveryConfigFromInputs builds a DiscoveryConfig from explicit config-owned paths plus lastScan.
// It normalizes and deduplicates paths without merging environment variables.
func DiscoveryConfigFromInputs(configPaths []string, lastScan time.Time) DiscoveryConfig {
	return DiscoveryConfig{
		ExtraModelPaths: MergeExtraRoots(configPaths, nil),
		LastScan:        lastScan,
	}
}

// DiscoveryConfigForWrite merges extra model paths from a previous on-disk config with
// current LLML_MODEL_PATHS so hand-edited TOML entries are preserved across writes.
func DiscoveryConfigForWrite(prev *Config, lastScan time.Time) DiscoveryConfig {
	var fromFile []string
	if prev != nil {
		fromFile = prev.Discovery.ExtraModelPaths
	}
	return DiscoveryConfig{
		ExtraModelPaths: MergeExtraRoots(fromFile, ExtraModelPathsFromEnv()),
		LastScan:        lastScan,
	}
}

// WriteFile writes config.toml atomically (write temp + rename).
// It best-effort copies the previous file into backups/ before overwrite.
func WriteFile(c Config) error {
	path, err := ConfigPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	_ = userdata.BackupFileIfExists(path)
	c.SchemaVersion = SchemaVersion
	var buf strings.Builder
	if err := toml.NewEncoder(&buf).Encode(c); err != nil {
		return err
	}
	return fsutil.WriteFileAtomic(path, []byte(buf.String()), 0o644)
}
