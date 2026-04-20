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

// renderConfirmBlock returns the framed delete-confirmation dialog, or "" if no confirmation is pending.
func (m Model) renderConfirmBlock(cw int) string {
	k := m.params.confirmDelete
	if k == paramConfirmNone {
		return ""
	}
	confirmBox := m.ui.styles.paramConfirmDialog
	confirmInner := max(cw-confirmBox.GetHorizontalFrameSize(), 24)
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
		confirmRows = []string{m.ui.styles.body.Render("Delete This Parameter Profile?"), nameLine}
	case paramConfirmEnvRow:
		line := ""
		if m.params.envCursor >= 0 && m.params.envCursor < m.paramEnvLen() {
			line = formatEnvVar(m.params.env[m.params.envCursor])
		}
		confirmRows = []string{
			m.ui.styles.body.Render("Delete This Environment Variable Line?"),
			m.ui.styles.body.Render("  " + truncateParamLine(line, max(confirmInner-2, 8))),
		}
	case paramConfirmArgRow:
		line := ""
		if m.params.argsCursor >= 0 && m.params.argsCursor < m.paramArgsLen() {
			line = m.params.args[m.params.argsCursor]
		}
		confirmRows = []string{
			m.ui.styles.body.Render("Delete This Extra Argument Line?"),
			m.ui.styles.body.Render("  " + truncateParamLine(line, max(confirmInner-2, 8))),
		}
	}
	if len(confirmRows) == 0 {
		return ""
	}
	confirmRows = append(confirmRows, "", m.ui.styles.footer.Render(FooterParamConfirmYN))
	return confirmBox.Width(cw).Render(lipgloss.JoinVertical(lipgloss.Left, confirmRows...))
}

// renderProfileList appends profile list rows to dst and returns the result.
func (m Model) renderProfileList(dst []string, maxLine int) []string {
	dst = append(dst, m.ui.styles.body.Render("  Parameter Profiles"), "")
	for i := range m.params.profiles {
		name := m.params.profiles[i].Name
		if name == "" {
			name = "(unnamed)"
		}
		activeRow := i == m.params.profileIndex
		focused := m.params.focus == paramFocusProfiles && activeRow
		if focused && m.params.editKind == paramEditProfileName {
			dst = append(dst, m.params.editInput.View())
			continue
		}
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
		dst = append(dst, lipgloss.JoinHorizontal(lipgloss.Top,
			m.ui.styles.body.Render(prefix),
			nameStyle.Render(truncateParamLine(displayName, nameW)),
		))
	}
	if len(m.params.profiles) == 0 {
		dst = append(dst, m.ui.styles.body.Render("  (none)"))
	}
	return dst
}

// renderDetailSections renders the env-vars and extra-args sections into the section box.
func (m Model) renderDetailSections(cw, maxSec int, secBox lipgloss.Style) string {
	const sectionHeadingIndent = "  "
	var rows []string
	sectionLine := func(heading string) string {
		return lipgloss.JoinHorizontal(lipgloss.Top,
			m.ui.styles.body.Render(sectionHeadingIndent),
			m.ui.styles.paramSectionHeading.Render(truncateParamLine(heading, maxSec-lipgloss.Width(sectionHeadingIndent))),
		)
	}
	rows = append(rows, sectionLine("Environment Variables (e.g. PYTORCH_CUDA_ALLOC_CONF=expandable_segments:True)"), "")
	envItems := make([]string, len(m.params.env))
	for i, e := range m.params.env {
		envItems[i] = formatEnvVar(e)
	}
	rows = append(rows, m.renderEditableListItems(envItems, paramFocusEnv, m.params.envCursor, paramEditEnvLine, maxSec)...)
	rows = append(rows, "", sectionLine("Extra Arguments (e.g. --max-model-len 131072)"), "")
	rows = append(rows, m.renderEditableListItems(m.params.args, paramFocusArgs, m.params.argsCursor, paramEditArgLine, maxSec)...)
	return secBox.Width(cw).Render(lipgloss.JoinVertical(lipgloss.Left, rows...))
}

func (m Model) paramPanelModalBlock() string {
	cw := m.paramPanelContentWidth()
	maxLine := max(cw, 24)
	secBox := m.ui.styles.paramSectionBox
	if m.params.focus == paramFocusEnv || m.params.focus == paramFocusArgs {
		secBox = m.ui.styles.paramSectionBoxFocused
	}
	maxSec := max(cw-secBox.GetHorizontalFrameSize(), 24)

	rows := []string{m.modalTitleRow(cw, m.ui.styles.portConfigTitle, "Parameter Profiles — "+m.params.modelDisplayName), ""}
	if block := m.renderConfirmBlock(cw); block != "" {
		rows = append(rows, block, "")
	}
	rows = m.renderProfileList(rows, maxLine)
	rows = append(rows, "", m.renderDetailSections(cw, maxSec, secBox))

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
