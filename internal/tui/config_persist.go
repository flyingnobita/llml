package tui

import (
	"github.com/flyingnobita/llml/internal/config"
)

// writeConfigFromModel writes the model's resolved settings, model list, and
// discovery metadata to config.toml.
func writeConfigFromModel(m Model) error {
	prev, err := config.ReadFile()
	var prevPtr *config.Config
	if err == nil {
		prevPtr = &prev
	}
	disc := config.DiscoveryConfigForWrite(prevPtr, m.settings)
	return config.WriteFile(config.BuildConfig(config.RuntimeConfigFromSettings(m.settings), disc))
}
