package tui

import (
	"slices"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/flyingnobita/llml/internal/config"
)

// openDiscoveryPathsModal opens the model discovery paths modal.
func (m Model) openDiscoveryPathsModal() (Model, tea.Cmd) {
	m = m.saveMainPaneFocusForModal()
	m.discovery.open = true
	m.discovery.original = slices.Clone(m.discovery.paths)
	m.discovery.discardConfirm = false
	m = m.withLastRunCleared()
	m.discovery.editOpen = false
	m.discovery.cursor = 0
	m.discovery.editInput.Blur()
	m.discovery.editInput.SetValue("")
	return m, nil
}

// closeDiscoveryPathsModal closes the modal.
func (m Model) closeDiscoveryPathsModal() Model {
	m.discovery.open = false
	m.discovery.original = nil
	m.discovery.discardConfirm = false
	m.discovery.editOpen = false
	m.discovery.editInput.Blur()
	m.discovery.editInput.SetValue("")
	return m.restoreMainPaneFocusAfterModal()
}

// cancelDiscoveryPathsModal closes the modal and restores the pre-open paths snapshot.
func (m Model) cancelDiscoveryPathsModal() Model {
	m.discovery.paths = slices.Clone(m.discovery.original)
	return m.closeDiscoveryPathsModal()
}

func (m Model) discoveryPathsDirty() bool {
	originalNorm := config.MergeExtraRoots(m.discovery.original, nil)
	currentNorm := config.MergeExtraRoots(m.discovery.paths, nil)
	if len(originalNorm) == 0 {
		originalNorm = nil
	}
	if len(currentNorm) == 0 {
		currentNorm = nil
	}
	return !slices.Equal(originalNorm, currentNorm)
}

func (m Model) updateDiscoveryDiscardConfirmKey(msg tea.KeyPressMsg) (Model, tea.Cmd) {
	if isEscapeKey(msg) {
		m.discovery.discardConfirm = false
		return m, nil
	}

	switch strings.ToLower(strings.TrimSpace(msg.String())) {
	case "y":
		return m.cancelDiscoveryPathsModal(), nil
	case "n":
		m.discovery.discardConfirm = false
		return m, nil
	}
	return m, nil
}

// startDiscoveryPathEdit opens the inline text input for editing the current row or a new row.
func (m Model) startDiscoveryPathEdit(isNew bool) (Model, tea.Cmd) {
	m.discovery.editOpen = true
	if isNew {
		m.discovery.paths = append(m.discovery.paths, "")
		m.discovery.cursor = len(m.discovery.paths) - 1
		m.discovery.editInput.SetValue("")
	} else {
		if m.discovery.cursor >= 0 && m.discovery.cursor < len(m.discovery.paths) {
			m.discovery.editInput.SetValue(m.discovery.paths[m.discovery.cursor])
		} else {
			m.discovery.editInput.SetValue("")
		}
	}
	return m, m.discovery.editInput.Focus()
}

// commitDiscoveryPathEdit saves the inline edit to the paths list.
func (m Model) commitDiscoveryPathEdit() Model {
	line := strings.TrimSpace(m.discovery.editInput.Value())
	if line == "" {
		return m.cancelDiscoveryPathEdit()
	}
	m.discovery.editOpen = false
	m.discovery.editInput.Blur()
	m.discovery.editInput.SetValue("")

	if m.discovery.cursor >= 0 && m.discovery.cursor < len(m.discovery.paths) {
		m.discovery.paths[m.discovery.cursor] = line
	}
	return m
}

// cancelDiscoveryPathEdit cancels the inline edit and removes the row if it was empty.
func (m Model) cancelDiscoveryPathEdit() Model {
	m.discovery.editOpen = false
	m.discovery.editInput.Blur()
	m.discovery.editInput.SetValue("")
	if m.discovery.cursor >= 0 && m.discovery.cursor < len(m.discovery.paths) {
		if strings.TrimSpace(m.discovery.paths[m.discovery.cursor]) == "" {
			m = m.deleteDiscoveryPathRow()
		}
	}
	return m
}

// deleteDiscoveryPathRow removes the currently selected path.
func (m Model) deleteDiscoveryPathRow() Model {
	if m.discovery.cursor < 0 || m.discovery.cursor >= len(m.discovery.paths) {
		return m
	}
	m.discovery.paths = append(m.discovery.paths[:m.discovery.cursor], m.discovery.paths[m.discovery.cursor+1:]...)
	if m.discovery.cursor >= len(m.discovery.paths) && len(m.discovery.paths) > 0 {
		m.discovery.cursor = len(m.discovery.paths) - 1
	}
	return m
}

// saveDiscoveryPaths normalizes the paths, checks if they changed compared to config, and either closes or triggers a rescan.
func (m Model) saveDiscoveryPaths() (Model, tea.Cmd) {
	prev, err := config.ReadFile()
	var prevPaths []string
	if err == nil {
		prevPaths = prev.Discovery.ExtraModelPaths
	}

	oldNorm := config.MergeExtraRoots(prevPaths, nil)
	newNorm := config.MergeExtraRoots(m.discovery.paths, nil)

	if len(oldNorm) == 0 {
		oldNorm = nil
	}
	if len(newNorm) == 0 {
		newNorm = nil
	}

	if slices.Equal(oldNorm, newNorm) {
		m = m.closeDiscoveryPathsModal()
		m = m.withLastRunSuccess("Model Paths Unchanged.")
		return m, clearLastRunNoteAfterCmd()
	}

	m = m.closeDiscoveryPathsModal()
	m = m.withLastRunSuccess("Model Paths Saved. Rescanning Models...")
	m.loading = true
	m.loadErr = nil
	return m, tea.Batch(rescanModelsCmd(m.discovery.paths...), clearLastRunNoteAfterCmd())
}

// updateDiscoveryPathsKey handles keys while the discovery paths modal is open.
func (m Model) updateDiscoveryPathsKey(msg tea.KeyPressMsg) (Model, tea.Cmd) {
	if m.discovery.discardConfirm {
		return m.updateDiscoveryDiscardConfirmKey(msg)
	}

	if m.discovery.editOpen {
		switch {
		case isEscapeKey(msg):
			m = m.cancelDiscoveryPathEdit()
			return m, nil
		case isEnterKey(msg):
			m = m.commitDiscoveryPathEdit()
			return m, nil
		default:
			var cmd tea.Cmd
			m.discovery.editInput, cmd = m.discovery.editInput.Update(msg)
			return m, cmd
		}
	}

	if isEscapeKey(msg) {
		if m.discoveryPathsDirty() {
			m.discovery.discardConfirm = true
			return m, nil
		}
		m = m.cancelDiscoveryPathsModal().withLastRunCleared()
		return m, nil
	}
	if isEnterKey(msg) {
		if len(m.discovery.paths) > 0 {
			return m.startDiscoveryPathEdit(false)
		}
		return m, nil
	}

	switch msg.String() {
	case "s":
		return m.saveDiscoveryPaths()
	case "up", "k":
		if m.discovery.cursor > 0 {
			m.discovery.cursor--
		}
		return m, nil
	case "down", "j":
		if m.discovery.cursor < len(m.discovery.paths)-1 {
			m.discovery.cursor++
		}
		return m, nil
	case "a":
		return m.startDiscoveryPathEdit(true)
	case "d":
		m = m.deleteDiscoveryPathRow()
		return m, nil
	default:
		return m, nil
	}
}
