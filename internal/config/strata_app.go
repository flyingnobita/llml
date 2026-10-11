package config

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/flyingnobita/llml/internal/settings"
)

// strataAppSettings is the part of Strata's settings.json that llml reads.
// data_dir is Strata's model/pack storage; installs are the Strata folders the
// app manages, each holding serve/server.py and its engine configs.
type strataAppSettings struct {
	DataDir  string   `json:"data_dir"`
	Installs []string `json:"installs"`
}

// StrataAppLayer reads the install directories from Strata's own
// settings.json so llml finds engine configs wherever Strata put them without
// a second copy of that setting. Strata writes it at
// {UserConfigDir}/strata/settings.json on Linux and
// {UserConfigDir}/Strata/settings.json on Windows, so both spellings are
// tried. It returns an empty layer when no file is readable.
func StrataAppLayer() settings.Layer {
	dir, err := os.UserConfigDir()
	if err != nil {
		return settings.Layer{}
	}
	return readStrataAppLayer(dir)
}

// readStrataAppLayer is [StrataAppLayer] with the configuration directory
// passed in, so a test can point it at a temp directory.
func readStrataAppLayer(configDir string) settings.Layer {
	if configDir == "" {
		return settings.Layer{}
	}
	for _, name := range []string{"strata", "Strata"} {
		data, err := os.ReadFile(filepath.Join(configDir, name, "settings.json")) //nolint:gosec // G304: Strata's own settings file under the user's config dir.
		if err != nil {
			continue
		}
		var s strataAppSettings
		if json.Unmarshal(data, &s) != nil {
			continue
		}
		if len(s.Installs) == 0 {
			continue
		}
		return settings.Layer{StrataInstalls: s.Installs}
	}
	return settings.Layer{}
}
