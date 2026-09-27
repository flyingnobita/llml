package userdata

import (
	"os"
	"path/filepath"
)

// LlmlDir returns {UserConfigDir}/llml.
func LlmlDir() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "llml"), nil
}

// ConfigTomlPath returns the path to config.toml (same layout as [config.ConfigPath]).
func ConfigTomlPath() (string, error) {
	d, err := LlmlDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(d, "config.toml"), nil
}

// ModelParamsPath returns the path to model-params.json.
func ModelParamsPath() (string, error) {
	d, err := LlmlDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(d, "model-params.json"), nil
}

// RuntimeStatesPath returns the path to runtimes.toml, the runtime state store
// that records which Runtimes are on.
func RuntimeStatesPath() (string, error) {
	d, err := LlmlDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(d, "runtimes.toml"), nil
}
