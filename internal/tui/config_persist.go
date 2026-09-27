package tui

import (
	"github.com/flyingnobita/llml/internal/config"
)

// writeConfigFromModel writes the model's resolved settings and discovery
// metadata to config.toml through the model's services, so tests with fake
// services never touch the real file.
func writeConfigFromModel(m Model) error {
	prev, err := m.svc.readConfig()
	var prevPtr *config.Config
	if err == nil {
		prevPtr = &prev
	}
	disc := config.DiscoveryConfigForWrite(prevPtr, m.settings)
	return m.svc.writeConfig(m.svc.buildConfig(m.svc.runtimeConfig(m.settings), disc))
}
