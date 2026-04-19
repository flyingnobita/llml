package tui

import (
	"charm.land/lipgloss/v2"
)

// renderEditableListItems renders the rows for one editable param section (env vars or extra args).
// It uses "› " prefix for the focused row, shows the inline edit input when that row is being edited,
// and renders "(none)" when the list is empty and not in an active-append edit.
func (m Model) renderEditableListItems(items []string, sectionFocus paramFocus, cursor int, sectionEditKind paramEditKind, maxSec int) []string {
	if len(items) == 0 && !(m.params.focus == sectionFocus && m.params.editKind == sectionEditKind) {
		prefix := "  "
		if m.params.focus == sectionFocus {
			prefix = "› "
		}
		return []string{m.ui.styles.paramDetailContent.Render(prefix + "(none)")}
	}
	rows := make([]string, 0, len(items))
	for i, line := range items {
		focused := m.params.focus == sectionFocus && cursor == i
		if focused && m.params.editKind == sectionEditKind {
			rows = append(rows, m.params.editInput.View())
		} else {
			prefix := "  "
			if focused {
				prefix = "› "
			}
			rows = append(rows, m.ui.styles.paramDetailContent.Render(prefix+truncateParamLine(line, maxSec)))
		}
	}
	return rows
}

func (m Model) paramPanelModalBlock() string {
	cw := m.paramPanelContentWidth()
	maxLine := cw
	if maxLine < 24 {
		maxLine = 24
	}
	secBox := m.ui.styles.paramSectionBox
	if m.params.focus == paramFocusEnv || m.params.focus == paramFocusArgs {
		secBox = m.ui.styles.paramSectionBoxFocused
	}
	maxSec := cw - secBox.GetHorizontalFrameSize()
	if maxSec < 24 {
		maxSec = 24
	}

	title := m.modalTitleRow(cw, m.ui.styles.portConfigTitle, "Parameters — "+m.params.modelDisplayName)
	rows := []string{title, ""}

	if k := m.params.confirmDelete; k != paramConfirmNone {
		confirmBox := m.ui.styles.paramConfirmDialog
		confirmInner := cw - confirmBox.GetHorizontalFrameSize()
		if confirmInner < 24 {
			confirmInner = 24
		}
		var confirmRows []string
		switch k {
		case paramConfirmProfile:
			pName := ""
			if m.params.profileIndex >= 0 && m.params.profileIndex < len(m.params.profiles) {
				pName = m.params.profiles[m.params.profileIndex].Name
			}
			if pName == "" {
				pName = "(unnamed)"
			}
			nameLine := lipgloss.JoinHorizontal(lipgloss.Top,
				m.ui.styles.body.Render("  "),
				m.ui.styles.paramProfileName.Render(truncateParamLine(pName, confirmInner-2)),
			)
			confirmRows = []string{
				m.ui.styles.body.Render("Delete this parameter profile?"),
				nameLine,
			}
		case paramConfirmEnvRow:
			line := ""
			if m.params.envCursor >= 0 && m.params.envCursor < m.paramEnvLen() {
				line = formatEnvVar(m.params.env[m.params.envCursor])
			}
			confirmRows = []string{
				m.ui.styles.body.Render("Delete this environment variable line?"),
				m.ui.styles.body.Render("  " + truncateParamLine(line, max(confirmInner-2, 8))),
			}
		case paramConfirmArgRow:
			line := ""
			if m.params.argsCursor >= 0 && m.params.argsCursor < m.paramArgsLen() {
				line = m.params.args[m.params.argsCursor]
			}
			confirmRows = []string{
				m.ui.styles.body.Render("Delete this extra argument line?"),
				m.ui.styles.body.Render("  " + truncateParamLine(line, max(confirmInner-2, 8))),
			}
		}
		if len(confirmRows) > 0 {
			confirmRows = append(confirmRows, "",
				m.ui.styles.footer.Render(FooterParamConfirmYN),
			)
			rows = append(rows, confirmBox.Width(cw).Render(lipgloss.JoinVertical(lipgloss.Left, confirmRows...)))
			rows = append(rows, "")
		}
	}

	rows = append(rows, m.ui.styles.body.Render("  Profiles"))
	rows = append(rows, "")
	for i := range m.params.profiles {
		name := m.params.profiles[i].Name
		if name == "" {
			name = "(unnamed)"
		}
		activeRow := i == m.params.profileIndex
		focused := m.params.focus == paramFocusProfiles && activeRow
		switch {
		case focused && m.params.editKind == paramEditProfileName:
			rows = append(rows, m.params.editInput.View())
		default:
			prefix := "  "
			if activeRow {
				prefix = "› "
			}
			pw := lipgloss.Width(prefix)
			nameW := maxLine - pw
			if nameW < 8 {
				nameW = maxLine
			}
			displayName := name
			if activeRow {
				displayName = "(active) " + name
			}
			nameStyle := m.ui.styles.paramProfileInactive
			if activeRow {
				nameStyle = m.ui.styles.paramProfileName
			}
			row := lipgloss.JoinHorizontal(lipgloss.Top,
				m.ui.styles.body.Render(prefix),
				nameStyle.Render(truncateParamLine(displayName, nameW)),
			)
			rows = append(rows, row)
		}
	}
	if len(m.params.profiles) == 0 {
		rows = append(rows, m.ui.styles.body.Render("  (none)"))
	}

	rows = append(rows, "")
	var detailRows []string
	const sectionHeadingIndent = "  "
	envHeading := "Environment Variables (e.g. PYTORCH_CUDA_ALLOC_CONF=expandable_segments:True)"
	detailRows = append(detailRows, lipgloss.JoinHorizontal(lipgloss.Top,
		m.ui.styles.body.Render(sectionHeadingIndent),
		m.ui.styles.paramSectionHeading.Render(truncateParamLine(envHeading, maxSec-lipgloss.Width(sectionHeadingIndent))),
	))
	detailRows = append(detailRows, "")
	envItems := make([]string, len(m.params.env))
	for i, e := range m.params.env {
		envItems[i] = formatEnvVar(e)
	}
	detailRows = append(detailRows, m.renderEditableListItems(envItems, paramFocusEnv, m.params.envCursor, paramEditEnvLine, maxSec)...)

	detailRows = append(detailRows, "")

	argHeading := "Extra arguments (e.g. --max-model-len 131072)"
	detailRows = append(detailRows, lipgloss.JoinHorizontal(lipgloss.Top,
		m.ui.styles.body.Render(sectionHeadingIndent),
		m.ui.styles.paramSectionHeading.Render(truncateParamLine(argHeading, maxSec-lipgloss.Width(sectionHeadingIndent))),
	))
	detailRows = append(detailRows, "")
	detailRows = append(detailRows, m.renderEditableListItems(m.params.args, paramFocusArgs, m.params.argsCursor, paramEditArgLine, maxSec)...)
	rows = append(rows, secBox.Width(cw).Render(lipgloss.JoinVertical(lipgloss.Left, detailRows...)))

	var footerHelp string
	switch m.params.focus {
	case paramFocusProfiles:
		footerHelp = FooterParamFooterProfiles
	case paramFocusEnv:
		if m.paramEnvLen() == 0 {
			footerHelp = FooterParamFooterDetailEmpty
		} else {
			footerHelp = FooterParamFooterDetailRows
		}
	case paramFocusArgs:
		if m.paramArgsLen() == 0 {
			footerHelp = FooterParamFooterDetailEmpty
		} else {
			footerHelp = FooterParamFooterDetailRows
		}
	}
	if m.params.confirmDelete == paramConfirmNone {
		rows = append(rows, "", m.ui.styles.footer.Render(footerHelp))
	}
	block := lipgloss.JoinVertical(lipgloss.Left, rows...)
	if m.lastRunNote != "" {
		block = lipgloss.JoinVertical(lipgloss.Left, block, "", m.lastRunNoteView())
	}
	return m.ui.styles.portConfigBox.Render(block)
}
