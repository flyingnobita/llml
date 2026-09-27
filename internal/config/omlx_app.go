package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

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
// ({base}/settings.json), so llml finds models wherever oMLX serves them from
// without a second copy of that setting. It returns an empty layer when the
// file is missing or unreadable; the settings package then falls back to
// oMLX's default directory.
func OMLXAppLayer(home string) settings.Layer {
	if home == "" {
		return settings.Layer{}
	}
	data, err := os.ReadFile(filepath.Join(omlxBasePath(home), "settings.json"))
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

// omlxBasePath returns oMLX's base directory the way the app's CLI shim finds
// it: the path stored in its bootstrap file when the user has moved it, else
// ~/.omlx.
func omlxBasePath(home string) string {
	bootstrap := filepath.Join(home, "Library", "Application Support", "oMLX", "base-path")
	if data, err := os.ReadFile(bootstrap); err == nil { //nolint:gosec // G304: oMLX's bootstrap file under the user's home.
		if line, _, _ := strings.Cut(string(data), "\n"); strings.TrimSpace(line) != "" {
			return strings.TrimSpace(line)
		}
	}
	return filepath.Join(home, ".omlx")
}
