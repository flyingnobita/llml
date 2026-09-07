package tui

import (
	"time"

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
	ts := m.table.lastScan
	if ts.IsZero() && prevPtr != nil {
		ts = prevPtr.Discovery.LastScan
	}
	if ts.IsZero() {
		ts = time.Now()
	}
	disc := config.DiscoveryConfigForWrite(prevPtr, m.settings, ts)
	return config.WriteFile(config.BuildConfig(config.RuntimeConfigFromSettings(m.settings), disc, m.table.files))
}
