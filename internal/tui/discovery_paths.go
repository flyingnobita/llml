package tui

import (
	"reflect"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/flyingnobita/llml/internal/config"
)

// openDiscoveryPathsModal opens the model discovery paths modal.
func (m Model) openDiscoveryPathsModal() (Model, tea.Cmd) {
	m = m.saveMainPaneFocusForModal()
	m.discovery.open = true
	m = m.withLastRunCleared()
	m.discovery.editOpen = false
	if len(m.discovery.paths) > 0 {
		m.discovery.cursor = 0
	} else {
		m.discovery.cursor = 0
	}
	m.discovery.editInput.Blur()
	m.discovery.editInput.SetValue("")
	return m, nil
}

// closeDiscoveryPathsModal closes the modal.
func (m Model) closeDiscoveryPathsModal() Model {
	m.discovery.open = false
	m.discovery.editOpen = false
	m.discovery.editInput.Blur()
	m.discovery.editInput.SetValue("")
	return m.restoreMainPaneFocusAfterModal()
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

	if reflect.DeepEqual(oldNorm, newNorm) {
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
	if m.discovery.editOpen {
		switch msg.String() {
		case "esc":
			m = m.cancelDiscoveryPathEdit()
			return m, nil
		case "enter":
			m = m.commitDiscoveryPathEdit()
			return m, nil
		default:
			var cmd tea.Cmd
			m.discovery.editInput, cmd = m.discovery.editInput.Update(msg)
			return m, cmd
		}
	}

	switch msg.String() {
	case "esc":
		m = m.closeDiscoveryPathsModal()
		m = m.withLastRunCleared()
		return m, nil
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
	case "n":
		return m.startDiscoveryPathEdit(true)
	case "enter":
		if len(m.discovery.paths) > 0 {
			return m.startDiscoveryPathEdit(false)
		}
		return m, nil
	case "d":
		m = m.deleteDiscoveryPathRow()
		return m, nil
	default:
		return m, nil
	}
}
