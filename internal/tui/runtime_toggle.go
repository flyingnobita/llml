package tui

import (
	"strings"

	"github.com/flyingnobita/llml/internal/models"
)

// adoptRuntimeStates makes r the model's stored on/off state, then decides
// every Runtime rt shows for the first time (see [Model.decideFirstSeen]), and
// redraws the rows, which dim by the result. A failed read leaves every
// Runtime on, is reported in the alert history, and decides nothing, so the
// unreadable file is not overwritten.
func (m Model) adoptRuntimeStates(r runtimeStatesRead, rt models.RuntimeInfo) Model {
	m.runtimeStates = r.states
	if r.err != nil {
		m = m.addAlert(alertSeverityWarn, "Runtimes", "Could not read runtimes.toml, treating every Runtime as on: "+r.err.Error())
	} else {
		m = m.decideFirstSeen(rt)
	}
	// After decideFirstSeen, which may change runtimeStates, so first-seen
	// Runtimes that start off dim their rows at once.
	return m.layoutTable()
}

// decideFirstSeen gives each Runtime rt's platform supports, and that has no
// stored state yet, its first state: on if it is a Detected Runtime, off
// otherwise. The result is written once. When any Runtime was turned off this
// way, one warning names them, so rows that dim after an upgrade do not look
// like a bug.
//
// A Runtime already stored is never re-decided: after this, only the user's
// toggles change it, even if it is installed later.
func (m Model) decideFirstSeen(rt models.RuntimeInfo) Model {
	states := m.runtimeStates
	var decided bool
	var off []string
	for _, def := range runtimeTable {
		if !def.supported(rt.Platform) {
			continue
		}
		if _, seen := states.Lookup(def.backend); seen {
			continue
		}
		on := rt.Detected(def.backend)
		states = states.With(def.backend, on)
		decided = true
		if !on {
			off = append(off, def.name)
		}
	}
	if !decided {
		return m
	}
	// Adopted even when the write fails, so this session honours the decision.
	m.runtimeStates = states
	if err := m.svc.writeRuntimeStates(states); err != nil {
		m = m.addAlert(alertSeverityWarn, "Runtimes", "Could not save runtimes.toml: "+err.Error())
	}
	if len(off) > 0 {
		m = m.addAlert(alertSeverityWarn, "Runtimes",
			"Runtimes not found are now off: "+strings.Join(off, ", ")+". Turn them on in "+FooterKeyConfigPort+".")
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
	m = m.layoutTable()
	if err := m.svc.writeRuntimeStates(m.runtimeStates); err != nil {
		m = m.addAlert(alertSeverityWarn, "Runtimes", "Could not save runtimes.toml: "+err.Error())
	}
	return m
}
