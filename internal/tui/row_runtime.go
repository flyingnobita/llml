package tui

import (
	"fmt"

	tea "charm.land/bubbletea/v2"

	"github.com/flyingnobita/llml/internal/models"
	"github.com/flyingnobita/llml/internal/profiles"
)

// runtimeOffSuffix follows the Runtime name of a dimmed row, and a Disabled
// Runtime's option in the p panel.
const runtimeOffSuffix = " (off)"

// rowRuntime returns the Runtime row f launches on. For a GGUF row it is the
// Active Profile's choice between llama.cpp and KoboldCpp, cached in
// effectiveBackends; for any other row it is the row's own Runtime, so a row
// in oMLX's model folders stays oMLX.
func rowRuntime(f models.ModelFile, effectiveBackends map[string]models.ModelBackend) models.ModelBackend {
	if f.Backend != models.BackendLlama {
		return f.Backend
	}
	if b, ok := effectiveBackends[profiles.ModelParamsKey(f.Identity())]; ok {
		return b
	}
	return models.BackendLlama
}

// rowDimmed reports whether row f is on a Disabled Runtime: listed, dimmed,
// and marked (off), but not launchable.
func (m Model) rowDimmed(f models.ModelFile) bool {
	return !m.runtimeEnabled(rowRuntime(f, m.table.effectiveBackends))
}

// anyRowDimmed reports whether any row is dimmed, which widens the Runtime
// column to fit the (off) marker.
func (m Model) anyRowDimmed() bool {
	for _, f := range m.table.files {
		if m.rowDimmed(f) {
			return true
		}
	}
	return false
}

// runtimeOffNote returns the launch preview's note for a row on Runtime b when
// b is off, or "" when it is on. The preview still shows the full command, so
// it can be copied; the note says why R will not run it.
func (m Model) runtimeOffNote(b models.ModelBackend) string {
	if m.runtimeEnabled(b) {
		return ""
	}
	return fmt.Sprintf(runtimeOffPreviewNote, runtimeFor(b).name)
}

// blockLaunchOnDisabledRuntime refuses to launch on Runtime b, which is off,
// and tells the user where to turn it on.
func (m Model) blockLaunchOnDisabledRuntime(b models.ModelBackend) (Model, tea.Cmd) {
	msg := fmt.Sprintf("%s is off. Turn it on in the runtime panel (c) to launch this model.", runtimeFor(b).name)
	m = m.addAlert(alertSeverityWarn, "Runtimes", msg)
	return m.flashError(msg)
}
