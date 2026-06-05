package tui

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"

	profilepkg "github.com/flyingnobita/llml/internal/profiles"
)

// renderEditableListItems renders the rows for one editable param section (env vars or extra args).
// It uses "› " prefix for the focused row, shows the inline edit input when that row is being edited,
// and renders "(none)" when the list is empty and not in an active-append edit.
func (m Model) renderEditableListItems(items []string, sectionFocus paramFocus, cursor int, sectionEditKind paramEditKind, maxSec int) []string {
	if len(items) == 0 && (m.params.focus != sectionFocus || m.params.editKind != sectionEditKind) {
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
	confirmInner := max(cw-confirmBox.GetHorizontalFrameSize(), MinModalInnerWidth)
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
	confirmRows = append(confirmRows, "", m.renderFooterHints(FooterParamConfirmYN))
	return confirmBox.Width(cw).Render(lipgloss.JoinVertical(lipgloss.Left, confirmRows...))
}

func (m Model) renderProfileSection(cw, maxSec int, secBox lipgloss.Style) string {
	rows := []string{
		lipgloss.JoinHorizontal(lipgloss.Top,
			m.ui.styles.body.Render("  "),
			m.ui.styles.paramSectionHeading.Render(truncateParamLine("Parameter Profiles", maxSec-2)),
		),
		"",
	}
	for i := range m.params.profiles {
		name := m.params.profiles[i].Name
		if name == "" {
			name = "(unnamed)"
		}
		activeRow := i == m.params.profileIndex
		focused := m.params.focus == paramFocusProfiles && activeRow
		if focused && m.params.editKind == paramEditProfileName {
			rows = append(rows, m.params.editInput.View())
			continue
		}
		prefix := "  "
		if activeRow {
			prefix = "› "
		}
		pw := lipgloss.Width(prefix)
		nameW := maxSec - pw
		if nameW < 8 {
			nameW = maxSec
		}
		displayName := name
		if activeRow {
			displayName = "(active) " + name
		}
		nameStyle := m.ui.styles.paramProfileInactive
		if activeRow {
			nameStyle = m.ui.styles.paramProfileName
		}
		rows = append(rows, lipgloss.JoinHorizontal(lipgloss.Top,
			m.ui.styles.body.Render(prefix),
			nameStyle.Render(truncateParamLine(displayName, nameW)),
		))
	}
	if len(m.params.profiles) == 0 {
		rows = append(rows, m.ui.styles.body.Render("  (none)"))
	}
	return secBox.Width(cw).Render(lipgloss.JoinVertical(lipgloss.Left, rows...))
}

func formatMetadataValue(v string) string {
	v = truncateParamLine(v, 120)
	if v == "" {
		return "unspecified"
	}
	return v
}

func formatMetadataFieldLine(p ParameterProfile, field paramMetadataField) string {
	label := paramMetadataFieldLabels[field]
	switch field {
	case paramMetadataBackend:
		return fmt.Sprintf("%s: %s", label, formatMetadataValue(p.Backend))
	case paramMetadataHardwareClass:
		return fmt.Sprintf("%s: %s", label, formatMetadataValue(string(p.Hardware.Class)))
	case paramMetadataHardwareGPUCount:
		return fmt.Sprintf("%s: %s", label, formatMetadataValue(formatOptionalInt(p.Hardware.GPUCount)))
	case paramMetadataHardwareMinVRAM:
		return fmt.Sprintf("%s: %s", label, formatMetadataValue(formatOptionalInt(p.Hardware.MinVRAMGB)))
	case paramMetadataHardwareMaxVRAM:
		return fmt.Sprintf("%s: %s", label, formatMetadataValue(formatOptionalInt(p.Hardware.MaxVRAMGB)))
	case paramMetadataHardwareNotes:
		return fmt.Sprintf("%s: %s", label, formatMetadataValue(p.Hardware.Notes))
	default:
		return label + ": unspecified"
	}
}

// renderCheckboxRow renders a label and [ ]/[✓] chips packed into lines that each fit
// within width, so lipgloss never splits a chip across lines.
// Returns one rendered string per terminal line (no newlines within each string).
func (m Model) renderCheckboxRow(
	label string,
	options []string,
	selected []string,
	cursor int,
	focused bool,
	width int,
) []string {
	prefix := "  "
	if focused {
		prefix = "› "
	}
	const sep = "  " // 2-space gap between chips
	sepW := lipgloss.Width(sep)

	labelPart := m.ui.styles.paramDetailContent.Render(prefix + label + ":")
	labelPartW := lipgloss.Width(labelPart)

	// Available chip-area widths.
	firstAvail := width - labelPartW - sepW
	contAvail := width - lipgloss.Width(prefix)

	var rows []string
	var lineChips []string
	lineUsed := 0
	isFirst := true

	for i, opt := range options {
		check := "[ ]"
		if hasTag(selected, opt) {
			check = "[✓]"
		}
		chipText := check + " " + opt
		chipW := lipgloss.Width(chipText)

		var chipRendered string
		if focused && i == cursor {
			chipRendered = m.ui.styles.paramTagSelected.Render(chipText)
		} else {
			chipRendered = m.ui.styles.paramDetailContent.Render(chipText)
		}

		avail := firstAvail
		if !isFirst {
			avail = contAvail
		}
		needed := chipW
		if len(lineChips) > 0 {
			needed += sepW
		}

		if len(lineChips) > 0 && lineUsed+needed > avail {
			// Flush current line.
			chipsStr := strings.Join(lineChips, sep)
			if isFirst {
				rows = append(rows, labelPart+sep+chipsStr)
			} else {
				rows = append(rows, strings.Repeat(" ", lipgloss.Width(prefix))+chipsStr)
			}
			lineChips = nil
			lineUsed = 0
			isFirst = false
			needed = chipW // no separator at start of new line
		}

		lineChips = append(lineChips, chipRendered)
		lineUsed += needed
	}

	if len(lineChips) > 0 {
		chipsStr := strings.Join(lineChips, sep)
		if isFirst {
			rows = append(rows, labelPart+sep+chipsStr)
		} else {
			rows = append(rows, strings.Repeat(" ", lipgloss.Width(prefix))+chipsStr)
		}
	}

	return rows
}

func (m Model) renderMetadataSection(cw, maxSec int, secBox lipgloss.Style) string {
	rows := []string{
		lipgloss.JoinHorizontal(lipgloss.Top,
			m.ui.styles.body.Render("  "),
			m.ui.styles.paramSectionHeading.Render(truncateParamLine("Profile Metadata", maxSec-2)),
		),
		"",
	}
	if m.params.profileIndex >= 0 && m.params.profileIndex < len(m.params.profiles) {
		p := m.params.profiles[m.params.profileIndex]
		// Build []string slices for checkbox rows.
		primaryStrs := make([]string, len(p.UseCase.Primary))
		for i, v := range p.UseCase.Primary {
			primaryStrs[i] = string(v)
		}
		canonicalPrimaryStrs := make([]string, len(profilepkg.CanonicalPrimaries))
		for i, v := range profilepkg.CanonicalPrimaries {
			canonicalPrimaryStrs[i] = string(v)
		}
		for field := paramMetadataField(0); field < paramMetadataFieldCount; field++ {
			focused := m.params.focus == paramFocusMetadata && m.params.metadataCursor == int(field)
			switch field {
			case paramMetadataUseCasePrimary:
				rows = append(rows, m.renderCheckboxRow(
					"Use Case Primary", canonicalPrimaryStrs, primaryStrs,
					m.params.primaryCursor, focused, maxSec,
				)...)
			case paramMetadataUseCaseTags:
				rows = append(rows, m.renderCheckboxRow(
					"Tags", profilepkg.CanonicalTags, p.UseCase.Tags,
					m.params.tagCursor, focused, maxSec,
				)...)
			default:
				prefix := "  "
				if focused {
					prefix = "› "
				}
				if focused && m.params.editKind == paramEditMetadataValue {
					label := paramMetadataFieldLabels[field] + ": "
					rows = append(rows, lipgloss.JoinHorizontal(lipgloss.Top,
						m.ui.styles.paramDetailContent.Render(prefix+label),
						m.params.editInput.View(),
					))
					continue
				}
				rows = append(rows, m.ui.styles.paramDetailContent.Render(prefix+truncateParamLine(formatMetadataFieldLine(p, field), maxSec)))
			}
		}
	} else {
		rows = append(rows, m.ui.styles.paramDetailContent.Render("  unspecified"))
	}
	return secBox.Width(cw).Render(lipgloss.JoinVertical(lipgloss.Left, rows...))
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
	panelBox := m.ui.styles.paramPanelBox
	profilesBox := m.ui.styles.paramSectionBox
	metaBox := m.ui.styles.paramSectionBox
	detailBox := m.ui.styles.paramSectionBox
	if m.params.focus == paramFocusProfiles {
		profilesBox = m.ui.styles.paramSectionBoxFocused
	}
	if m.params.focus == paramFocusMetadata {
		metaBox = m.ui.styles.paramSectionBoxFocused
	}
	if m.params.focus == paramFocusEnv || m.params.focus == paramFocusArgs {
		detailBox = m.ui.styles.paramSectionBoxFocused
	}
	maxSec := max(cw-detailBox.GetHorizontalFrameSize(), MinModalInnerWidth)

	rows := []string{m.modalTitleRow(cw, m.ui.styles.portConfigTitle, "Parameter Profiles — "+m.params.modelDisplayName)}
	if block := m.renderConfirmBlock(cw); block != "" {
		rows = append(rows, "", block)
	}
	rows = append(rows,
		"",
		m.renderProfileSection(cw, max(cw-profilesBox.GetHorizontalFrameSize(), MinModalInnerWidth), profilesBox),
		m.renderMetadataSection(cw, max(cw-metaBox.GetHorizontalFrameSize(), MinModalInnerWidth), metaBox),
		m.renderDetailSections(cw, maxSec, detailBox),
	)

	var footerHelp string
	switch m.params.focus {
	case paramFocusProfiles:
		footerHelp = FooterParamFooterProfiles
	case paramFocusMetadata:
		footerHelp = FooterParamFooterMetadata
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
		rows = append(rows, m.renderFooterHints(footerHelp))
	}
	block := lipgloss.JoinVertical(lipgloss.Left, rows...)
	if m.lastRunNote != "" {
		block = lipgloss.JoinVertical(lipgloss.Left, block, "", m.lastRunNoteView())
	}
	return panelBox.Render(block)
}
