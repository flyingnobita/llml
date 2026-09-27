package config

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/flyingnobita/llml/internal/settings"
)

// omlxAppSettings is the part of oMLX's settings.json that llml reads.
type omlxAppSettings struct {
	Model struct {
		ModelDirs []string `json:"model_dirs"`
		ModelDir  string   `json:"model_dir"`
	} `json:"model"`
}

// OMLXAppLayer reads the model directories configured in the oMLX app
// ({home}/.omlx/settings.json), so llml finds models wherever oMLX serves them
// from without a second copy of that setting. It returns an empty layer when
// the file is missing or unreadable; the settings package then falls back to
// oMLX's default directory.
func OMLXAppLayer(home string) settings.Layer {
	if home == "" {
		return settings.Layer{}
	}
	data, err := os.ReadFile(filepath.Join(home, ".omlx", "settings.json")) //nolint:gosec // G304: fixed path under the user's home.
	if err != nil {
		return settings.Layer{}
	}
	var s omlxAppSettings
	if err := json.Unmarshal(data, &s); err != nil {
		return settings.Layer{}
	}
	dirs := s.Model.ModelDirs
	if len(dirs) == 0 && s.Model.ModelDir != "" {
		dirs = []string{s.Model.ModelDir}
	}
	return settings.Layer{OMLXModelDirs: dirs}
}
