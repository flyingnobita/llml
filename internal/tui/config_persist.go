package tui

import (
	"github.com/flyingnobita/llml/internal/config"
	"github.com/flyingnobita/llml/internal/settings"
)

// writeSettings writes s and the discovery metadata to config.toml. It goes
// through the injected file access, so tests with fake services never touch
// the real file.
func (svc services) writeSettings(s settings.Settings) error {
	prev, err := svc.readConfig()
	var prevPtr *config.Config
	if err == nil {
		prevPtr = &prev
	}
	disc := config.DiscoveryConfigForWrite(prevPtr, s)
	return svc.writeConfig(svc.buildConfig(svc.runtimeConfig(s), disc))
}
