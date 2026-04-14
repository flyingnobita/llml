package tui

import (
	"charm.land/lipgloss/v2"
)

func (m Model) paramPanelView() string {
	cw := m.paramPanelContentWidth()
	maxLine := cw
	if maxLine < 24 {
		maxLine = 24
	}
	secBox := m.styles.paramSectionBox
	maxSec := cw - secBox.GetHorizontalFrameSize()
	if maxSec < 24 {
		maxSec = 24
	}

	title := m.modalTitleRow(cw, m.styles.portConfigTitle, "Parameters — "+m.paramModelDisplayName)
	rows := []string{title, ""}

	if k := m.paramConfirmDelete; k != paramConfirmNone {
		confirmBox := m.styles.paramConfirmDialog
		confirmInner := cw - confirmBox.GetHorizontalFrameSize()
		if confirmInner < 24 {
			confirmInner = 24
		}
		var confirmRows []string
		switch k {
		case paramConfirmProfile:
			pName := ""
			if m.paramProfileIndex >= 0 && m.paramProfileIndex < len(m.paramProfiles) {
				pName = m.paramProfiles[m.paramProfileIndex].Name
			}
			if pName == "" {
				pName = "(unnamed)"
			}
			nameLine := lipgloss.JoinHorizontal(lipgloss.Top,
				m.styles.body.Render("  "),
				m.styles.paramProfileName.Render(truncateParamLine(pName, confirmInner-2)),
			)
			confirmRows = []string{
				m.styles.body.Render("Delete this parameter profile?"),
				nameLine,
			}
		case paramConfirmEnvRow:
			line := ""
			if m.paramEnvCursor >= 0 && m.paramEnvCursor < m.paramEnvLen() {
				line = formatEnvVar(m.paramEnv[m.paramEnvCursor])
			}
			confirmRows = []string{
				m.styles.body.Render("Delete this environment variable line?"),
				m.styles.body.Render("  " + truncateParamLine(line, max(confirmInner-2, 8))),
			}
		case paramConfirmArgRow:
			line := ""
			if m.paramArgsCursor >= 0 && m.paramArgsCursor < m.paramArgsLen() {
				line = m.paramArgs[m.paramArgsCursor]
			}
			confirmRows = []string{
				m.styles.body.Render("Delete this extra argument line?"),
				m.styles.body.Render("  " + truncateParamLine(line, max(confirmInner-2, 8))),
			}
		}
		if len(confirmRows) > 0 {
			confirmRows = append(confirmRows, "",
				m.styles.footer.Render("y: yes · n: no"),
			)
			rows = append(rows, confirmBox.Width(cw).Render(lipgloss.JoinVertical(lipgloss.Left, confirmRows...)))
			rows = append(rows, "")
		}
	}

	rows = append(rows, m.styles.body.Render("  Profiles"))
	rows = append(rows, "")
	for i := range m.paramProfiles {
		name := m.paramProfiles[i].Name
		if name == "" {
			name = "(unnamed)"
		}
		focused := m.paramFocus == paramFocusProfiles && i == m.paramProfileIndex
		switch {
		case focused && m.paramEditKind == paramEditProfileName:
			rows = append(rows, m.paramEditInput.View())
		default:
			prefix := "  "
			if focused {
				prefix = "› "
			}
			pw := lipgloss.Width(prefix)
			nameW := maxLine - pw
			if nameW < 8 {
				nameW = maxLine
			}
			row := lipgloss.JoinHorizontal(lipgloss.Top,
				m.styles.body.Render(prefix),
				m.styles.paramProfileName.Render(truncateParamLine(name, nameW)),
			)
			rows = append(rows, row)
		}
	}
	if len(m.paramProfiles) == 0 {
		rows = append(rows, m.styles.body.Render("  (none)"))
	}

	rows = append(rows, "")
	var detailRows []string
	const sectionHeadingIndent = "  "
	envHeading := "Environment Variables (e.g. PYTORCH_CUDA_ALLOC_CONF=expandable_segments:True)"
	detailRows = append(detailRows, lipgloss.JoinHorizontal(lipgloss.Top,
		m.styles.body.Render(sectionHeadingIndent),
		m.styles.paramSectionHeading.Render(truncateParamLine(envHeading, maxSec-lipgloss.Width(sectionHeadingIndent))),
	))
	detailRows = append(detailRows, "")
	if m.paramEnvLen() == 0 && !(m.paramFocus == paramFocusEnv && m.paramEditKind == paramEditEnvLine) {
		prefix := "  "
		if m.paramFocus == paramFocusEnv {
			prefix = "› "
		}
		detailRows = append(detailRows, m.styles.body.Render(prefix+"(none)"))
	}
	for i := range m.paramEnv {
		line := formatEnvVar(m.paramEnv[i])
		focused := m.paramFocus == paramFocusEnv && m.paramEnvCursor == i
		switch {
		case focused && m.paramEditKind == paramEditEnvLine:
			detailRows = append(detailRows, m.paramEditInput.View())
		default:
			prefix := "  "
			if focused {
				prefix = "› "
			}
			detailRows = append(detailRows, m.styles.body.Render(prefix+truncateParamLine(line, maxSec)))
		}
	}

	detailRows = append(detailRows, "")

	argHeading := "Extra arguments (e.g. --max-model-len 131072)"
	detailRows = append(detailRows, lipgloss.JoinHorizontal(lipgloss.Top,
		m.styles.body.Render(sectionHeadingIndent),
		m.styles.paramSectionHeading.Render(truncateParamLine(argHeading, maxSec-lipgloss.Width(sectionHeadingIndent))),
	))
	detailRows = append(detailRows, "")
	if m.paramArgsLen() == 0 && !(m.paramFocus == paramFocusArgs && m.paramEditKind == paramEditArgLine) {
		prefix := "  "
		if m.paramFocus == paramFocusArgs {
			prefix = "› "
		}
		detailRows = append(detailRows, m.styles.body.Render(prefix+"(none)"))
	}
	for i := range m.paramArgs {
		line := m.paramArgs[i]
		focused := m.paramFocus == paramFocusArgs && m.paramArgsCursor == i
		switch {
		case focused && m.paramEditKind == paramEditArgLine:
			detailRows = append(detailRows, m.paramEditInput.View())
		default:
			prefix := "  "
			if focused {
				prefix = "› "
			}
			detailRows = append(detailRows, m.styles.body.Render(prefix+truncateParamLine(line, maxSec)))
		}
	}
	rows = append(rows, secBox.Width(cw).Render(lipgloss.JoinVertical(lipgloss.Left, detailRows...)))

	var footerHelp string
	switch m.paramFocus {
	case paramFocusProfiles:
		footerHelp = "tab: sections · hjkl: nav · n: new · d: delete · r: rename · esc/q: back"
	case paramFocusEnv:
		if m.paramEnvLen() == 0 {
			footerHelp = "tab: sections · hjkl: nav · a: add row · d: delete · esc/q: back"
		} else {
			footerHelp = "tab: sections · hjkl: nav · enter: edit · a: add row · d: delete · esc/q: back"
		}
	case paramFocusArgs:
		if m.paramArgsLen() == 0 {
			footerHelp = "tab: sections · hjkl: nav · a: add row · d: delete · esc/q: back"
		} else {
			footerHelp = "tab: sections · hjkl: nav · enter: edit · a: add row · d: delete · esc/q: back"
		}
	}
	if m.paramConfirmDelete == paramConfirmNone {
		rows = append(rows, "", m.styles.footer.Render(footerHelp))
	}
	block := lipgloss.JoinVertical(lipgloss.Left, rows...)
	if m.lastRunNote != "" {
		block = lipgloss.JoinVertical(lipgloss.Left, block, "", m.styles.errLine.Render(m.lastRunNote))
	}
	framed := m.styles.portConfigBox.Render(block)
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, framed)
}
