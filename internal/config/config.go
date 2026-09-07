// Package config persists llml runtime and discovery cache in human-readable TOML.
// Parameter profiles remain in model-params.json; see [github.com/flyingnobita/llml/internal/tui].
package config

import (
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/BurntSushi/toml"

	"github.com/flyingnobita/llml/internal/fsutil"
	"github.com/flyingnobita/llml/internal/models"
	"github.com/flyingnobita/llml/internal/settings"
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

// Layer converts the persisted [runtime] table into a settings layer. Empty
// strings and nil ports mean "this file says nothing about that value", so a
// higher-precedence layer (the environment) or the built-in defaults supply it.
//
// This is the only direction configuration flows into the running process: the
// file is read into a layer, never applied to the process environment.
func (r RuntimeConfig) Layer() settings.Layer {
	var l settings.Layer
	setLayerPath(&l.LlamaCppPath, r.DefaultLlamaCppPath)
	setLayerPath(&l.VLLMPath, r.DefaultVLLMPath)
	setLayerPath(&l.VLLMVenv, r.DefaultVLLMVenv)
	setLayerPath(&l.OllamaPath, r.DefaultOllamaPath)
	setLayerPath(&l.KoboldCppPath, r.DefaultKoboldCppPath)
	setLayerString(&l.LlamaServerHost, r.DefaultLlamaServerHost)
	setLayerString(&l.VLLMServerHost, r.DefaultVLLMServerHost)
	setLayerString(&l.OllamaHost, settings.NormalizeOllamaHost(r.DefaultOllamaHost))
	setLayerPort(&l.LlamaServerPort, r.DefaultLlamaServerPort)
	setLayerPort(&l.VLLMServerPort, r.DefaultVLLMServerPort)
	setLayerPort(&l.KoboldCppPort, r.DefaultKoboldCppPort)
	return l
}

// Layer converts the persisted [discovery] table into a settings layer.
func (d DiscoveryConfig) Layer() settings.Layer {
	return settings.Layer{ExtraModelPaths: d.ExtraModelPaths}
}

// Layer converts the whole document into one settings layer.
func (c Config) Layer() settings.Layer {
	l := c.Runtime.Layer()
	l.ExtraModelPaths = c.Discovery.ExtraModelPaths
	return l
}

// Resolve reads config.toml and resolves it against the environment and the
// built-in defaults, in that order of precedence. A missing or unreadable file
// is not an error: the result is then the environment over the defaults.
func Resolve(getenv settings.Getenv) settings.Settings {
	c, err := ReadFile()
	if err != nil {
		return settings.Resolve(settings.FromEnv(getenv), settings.Defaults())
	}
	return settings.Resolve(settings.FromEnv(getenv), c.Layer(), settings.Defaults())
}

func setLayerPath(dst **string, value string) {
	if v := fsutil.NormalizePath(value); v != "" {
		*dst = &v
	}
}

func setLayerString(dst **string, value string) {
	if v := strings.TrimSpace(value); v != "" {
		*dst = &v
	}
}

func setLayerPort(dst **int, value *int) {
	if value != nil && *value > 0 && *value <= 65535 {
		v := *value
		*dst = &v
	}
}

// RuntimeConfigFromSettings projects resolved settings back into the [runtime]
// table for writing. Every value is written, so the file always shows what the
// process is actually using.
func RuntimeConfigFromSettings(s settings.Settings) RuntimeConfig {
	llamaPort := s.LlamaServerPort
	vllmPort := s.VLLMServerPort
	koboldPort := s.KoboldCppPort
	return RuntimeConfig{
		DefaultLlamaCppPath:    s.LlamaCppPath,
		DefaultLlamaServerHost: s.LlamaServerHost,
		DefaultVLLMPath:        s.VLLMPath,
		DefaultVLLMServerHost:  s.VLLMServerHost,
		DefaultVLLMVenv:        s.VLLMVenv,
		DefaultOllamaPath:      s.OllamaPath,
		DefaultOllamaHost:      s.OllamaHost,
		DefaultKoboldCppPath:   s.KoboldCppPath,
		DefaultLlamaServerPort: &llamaPort,
		DefaultVLLMServerPort:  &vllmPort,
		DefaultKoboldCppPort:   &koboldPort,
	}
}

// MergeExtraRoots combines extra model roots from several sources into one
// normalized, deduplicated list, preserving order.
func MergeExtraRoots(lists ...[]string) []string {
	var ps fsutil.PathSet
	for _, l := range lists {
		ps.Add(l...)
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
		ExtraModelPaths: MergeExtraRoots(configPaths),
		LastScan:        lastScan,
	}
}

// DiscoveryConfigForWrite merges extra model paths from a previous on-disk config
// with the resolved extra roots, so hand-edited TOML entries survive a write.
func DiscoveryConfigForWrite(prev *Config, s settings.Settings, lastScan time.Time) DiscoveryConfig {
	var fromFile []string
	if prev != nil {
		fromFile = prev.Discovery.ExtraModelPaths
	}
	return DiscoveryConfig{
		ExtraModelPaths: MergeExtraRoots(fromFile, s.ExtraModelPaths),
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
