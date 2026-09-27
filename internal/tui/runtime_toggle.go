package tui

import (
	"github.com/flyingnobita/llml/internal/models"
)

// adoptRuntimeStates makes r the model's stored on/off state. A failed read
// leaves every Runtime on and is reported in the alert history.
func (m Model) adoptRuntimeStates(r runtimeStatesRead) Model {
	m.runtimeStates = r.states
	if r.err != nil {
		m = m.addAlert(alertSeverityWarn, "Runtimes", "Could not read runtimes.toml, treating every Runtime as on: "+r.err.Error())
	}
	return m
}

// runtimeEnabled reports whether Runtime b is on, as stored. A Runtime with no
// stored state is on.
func (m Model) runtimeEnabled(b models.ModelBackend) bool {
	return m.runtimeStates.Enabled(b)
}

// panelRuntimeEnabled reports whether the runtime panel shows Runtime b as on,
// counting toggles not saved yet.
func (m Model) panelRuntimeEnabled(b models.ModelBackend) bool {
	return m.rc.toggles.Enabled(b)
}

// toggleSelectedRuntime flips the highlighted Runtime's checkbox. The change
// is held in the panel until enter saves it.
func (m Model) toggleSelectedRuntime() Model {
	b := m.rc.selected
	m.rc.toggles = m.rc.toggles.With(b, !m.rc.toggles.Enabled(b))
	return m
}

// runtimeTogglesDirty reports whether any Runtime the panel lists is shown on
// where it is stored off, or the reverse.
func (m Model) runtimeTogglesDirty() bool {
	for _, rt := range m.panelRuntimes() {
		if m.panelRuntimeEnabled(rt.backend) != m.runtimeEnabled(rt.backend) {
			return true
		}
	}
	return false
}

// saveRuntimeToggles stores the panel's toggles, when any changed. The model
// adopts them even if the write fails, so this session honours what the user
// ticked; the failure is reported in the alert history.
func (m Model) saveRuntimeToggles() Model {
	if !m.runtimeTogglesDirty() {
		return m
	}
	m.runtimeStates = m.rc.toggles
	if err := m.svc.writeRuntimeStates(m.runtimeStates); err != nil {
		m = m.addAlert(alertSeverityWarn, "Runtimes", "Could not save runtimes.toml: "+err.Error())
	}
	return m
}
