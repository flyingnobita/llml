package tui

import (
	"fmt"
	"slices"

	tea "charm.land/bubbletea/v2"

	"github.com/flyingnobita/llml/internal/models"
	"github.com/flyingnobita/llml/internal/profiles"
)

// rowRuntimeChoices returns the Runtimes row f may launch on: those that run
// its Model Format on this platform. A GGUF row chooses between llama.cpp and
// KoboldCpp; a Safetensors row between the Safetensors Runtimes, with oMLX
// offered only for rows inside one of its model folders.
func (m Model) rowRuntimeChoices(f models.ModelFile) []models.ModelBackend {
	return runtimeChoices(f.Backend, m.runtime.Platform)
}

// usableRuntime returns chosen when row f may launch on it, and otherwise the
// Runtime discovery gave f. A profile can name a Runtime the row cannot use
// (an imported profile, or no override at all), and that choice is ignored.
func (m Model) usableRuntime(f models.ModelFile, chosen models.ModelBackend) models.ModelBackend {
	if slices.Contains(m.rowRuntimeChoices(f), chosen) {
		return chosen
	}
	return f.Backend
}

// rowHasRuntimeChoice reports whether row f can launch on more than one
// Runtime, so that its Active Profile's backend matters.
func (m Model) rowHasRuntimeChoice(f models.ModelFile) bool {
	return len(m.rowRuntimeChoices(f)) > 1
}

// rowRuntime returns the Runtime row f launches on: its Active Profile's
// choice, cached in effectiveBackends, when the row may use it, and otherwise
// the Runtime discovery gave the row.
func (m Model) rowRuntime(f models.ModelFile) models.ModelBackend {
	b, _ := m.cachedRowRuntime(f)
	return b
}

// cachedRowRuntime returns [Model.rowRuntime] for f and whether its Active
// Profile's choice is cached. A row whose profile makes no choice is not.
func (m Model) cachedRowRuntime(f models.ModelFile) (models.ModelBackend, bool) {
	if b, ok := m.table.effectiveBackends[profiles.ModelParamsKey(f.Identity())]; ok {
		return m.usableRuntime(f, b), true
	}
	return f.Backend, false
}

// rowDimmed reports whether row f is on a Disabled Runtime: listed, dimmed,
// and marked (off), but not launchable.
func (m Model) rowDimmed(f models.ModelFile) bool {
	return !m.runtimeEnabled(m.rowRuntime(f))
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
