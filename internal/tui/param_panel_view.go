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
		if m.params.editor.index >= 0 && m.params.editor.index < len(m.params.editor.profiles) {
			pName = m.params.editor.profiles[m.params.editor.index].Name
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
		if m.params.editor.envCursor >= 0 && m.params.editor.envCursor < m.paramEnvLen() {
			line = formatEnvVar(m.params.editor.env[m.params.editor.envCursor])
		}
		confirmRows = []string{
			m.ui.styles.body.Render("Delete This Environment Variable Line?"),
			m.ui.styles.body.Render("  " + truncateParamLine(line, max(confirmInner-2, 8))),
		}
	case paramConfirmArgRow:
		line := ""
		if m.params.editor.argsCursor >= 0 && m.params.editor.argsCursor < m.paramArgsLen() {
			line = m.params.editor.args[m.params.editor.argsCursor]
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
	for i := range m.params.editor.profiles {
		name := m.params.editor.profiles[i].Name
		if name == "" {
			name = "(unnamed)"
		}
		activeRow := i == m.params.editor.index
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
	if len(m.params.editor.profiles) == 0 {
		rows = append(rows, m.ui.styles.body.Render("  (none)"))
	}
	return secBox.Width(cw).Render(lipgloss.JoinVertical(lipgloss.Left, rows...))
}

// renderLabelPart builds the styled label column for a metadata row.
// The label text is padded to paramMetadataLabelWidth with muted color;
// prefix carries the focus indicator.
func (m Model) renderLabelPart(prefix, label string) string {
	labelFmt := fmt.Sprintf("%-*s: ", paramMetadataLabelWidth, label)
	return m.ui.styles.paramMetadataLabel.Render(prefix + labelFmt)
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
	const sepW = 2   // visual width of sep (two ASCII spaces)

	labelPart := m.renderLabelPart(prefix, label)
	labelPartW := lipgloss.Width(labelPart)
	contIndent := labelPartW + sepW // where chips start on line 1; continuation aligns there
	avail := width - contIndent

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
		needed := chipW
		if len(lineChips) > 0 {
			needed += sepW
		}

		if len(lineChips) > 0 && lineUsed+needed > avail {
			chipsStr := strings.Join(lineChips, sep)
			if isFirst {
				rows = append(rows, labelPart+sep+chipsStr)
			} else {
				rows = append(rows, strings.Repeat(" ", contIndent)+chipsStr)
			}
			lineChips = nil
			lineUsed = 0
			isFirst = false
			needed = chipW
		}

		lineChips = append(lineChips, chipRendered)
		lineUsed += needed
	}

	if len(lineChips) > 0 {
		chipsStr := strings.Join(lineChips, sep)
		if isFirst {
			rows = append(rows, labelPart+sep+chipsStr)
		} else {
			rows = append(rows, strings.Repeat(" ", contIndent)+chipsStr)
		}
	}

	return rows
}

// renderRadioRow renders a label and ( )/(•) chips for a single-select field.
// selectedIndex is the index of the currently chosen value (-1 = none selected).
// cursor is the navigation cursor position. optLabels are the display strings for each option.
// Returns one rendered string per terminal line.
func (m Model) renderRadioRow(
	label string,
	optLabels []string,
	selectedIndex int,
	cursor int,
	focused bool,
	width int,
) []string {
	prefix := "  "
	if focused {
		prefix = "› "
	}
	const sep = "  "
	sepW := lipgloss.Width(sep)

	labelPart := m.renderLabelPart(prefix, label)
	labelPartW := lipgloss.Width(labelPart)
	contIndent := labelPartW + sepW
	avail := width - contIndent

	var rows []string
	var lineChips []string
	lineUsed := 0
	isFirst := true

	for i, optLabel := range optLabels {
		radio := "( )"
		if i == selectedIndex {
			radio = "(•)"
		}
		chipText := radio + " " + optLabel
		chipW := lipgloss.Width(chipText)

		var chipRendered string
		if focused && i == cursor {
			chipRendered = m.ui.styles.paramTagSelected.Render(chipText)
		} else {
			chipRendered = m.ui.styles.paramDetailContent.Render(chipText)
		}

		needed := chipW
		if len(lineChips) > 0 {
			needed += sepW
		}

		if len(lineChips) > 0 && lineUsed+needed > avail {
			chipsStr := strings.Join(lineChips, sep)
			if isFirst {
				rows = append(rows, labelPart+sep+chipsStr)
			} else {
				rows = append(rows, strings.Repeat(" ", contIndent)+chipsStr)
			}
			lineChips = nil
			lineUsed = 0
			isFirst = false
			needed = chipW
		}

		lineChips = append(lineChips, chipRendered)
		lineUsed += needed
	}

	if len(lineChips) > 0 {
		chipsStr := strings.Join(lineChips, sep)
		if isFirst {
			rows = append(rows, labelPart+sep+chipsStr)
		} else {
			rows = append(rows, strings.Repeat(" ", contIndent)+chipsStr)
		}
	}

	return rows
}

// wrapTextToLines word-wraps text to fit within maxW visible columns, returning one
// element per output line. Words longer than maxW are kept on their own line untruncated
// (the caller can truncate at render time if needed).
func wrapTextToLines(text string, maxW int) []string {
	if maxW < 1 {
		maxW = 1
	}
	words := strings.Fields(text)
	if len(words) == 0 {
		return nil
	}
	var lines []string
	var cur []string
	curW := 0
	for _, word := range words {
		ww := lipgloss.Width(word)
		switch {
		case len(cur) == 0:
			cur = append(cur, word)
			curW = ww
		case curW+1+ww <= maxW:
			cur = append(cur, word)
			curW += 1 + ww
		default:
			lines = append(lines, strings.Join(cur, " "))
			cur = []string{word}
			curW = ww
		}
	}
	if len(cur) > 0 {
		lines = append(lines, strings.Join(cur, " "))
	}
	return lines
}

// radioOptionLabel returns a display label for a raw option value (empty string → "(none)").
func radioOptionLabel(v string) string {
	if v == "" {
		return "(none)"
	}
	return v
}

func (m Model) renderMetadataSection(cw, maxSec int, secBox lipgloss.Style) string {
	rows := []string{
		lipgloss.JoinHorizontal(lipgloss.Top,
			m.ui.styles.body.Render("  "),
			m.ui.styles.paramSectionHeading.Render(truncateParamLine("Profile Metadata", maxSec-2)),
		),
		"",
	}
	if len(m.params.editor.profiles) > 0 {
		p := m.params.editor.ActiveProfile()
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
			prefix := "  "
			if focused {
				prefix = "› "
			}
			switch field {
			case paramMetadataBackend:
				opts := m.paramBackendOptionsForModel()
				optLabels := make([]string, len(opts))
				for i, o := range opts {
					optLabels[i] = radioOptionLabel(o)
				}
				selIdx := -1
				for i, o := range opts {
					if o == p.Backend {
						selIdx = i
						break
					}
				}
				rows = append(rows, m.renderRadioRow(
					"Backend", optLabels, selIdx,
					m.params.backendCursor, focused, maxSec,
				)...)
			case paramMetadataHardwareClass:
				hwLabels := make([]string, len(paramHardwareClassOptions))
				for i, o := range paramHardwareClassOptions {
					hwLabels[i] = radioOptionLabel(string(o))
				}
				selIdx := -1
				for i, o := range paramHardwareClassOptions {
					if o == p.Hardware.Class {
						selIdx = i
						break
					}
				}
				rows = append(rows, m.renderRadioRow(
					"Hardware Class", hwLabels, selIdx,
					m.params.hardwareClassCursor, focused, maxSec,
				)...)
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
			case paramMetadataHardwareNotes:
				if focused && m.params.editKind == paramEditMetadataValue {
					labelPart := m.renderLabelPart(prefix, paramMetadataFieldLabels[field])
					labelPartW := lipgloss.Width(labelPart)
					indent := strings.Repeat(" ", labelPartW)
					// Split textarea view into lines; prepend label to first, indent the rest.
					taLines := strings.Split(m.params.notesInput.View(), "\n")
					for i, taLine := range taLines {
						if taLine == "" && i == len(taLines)-1 {
							break // skip trailing empty line from textarea View
						}
						if i == 0 {
							rows = append(rows, labelPart+taLine)
						} else {
							rows = append(rows, indent+taLine)
						}
					}
					continue
				}
				labelPart := m.renderLabelPart(prefix, paramMetadataFieldLabels[field])
				labelPartW := lipgloss.Width(labelPart)
				valueW := max(maxSec-labelPartW, 8)
				value := m.metadataFieldValue(field)

				var wrapped []string
				if value == "" {
					wrapped = []string{"unspecified"}
				} else {
					wrapped = wrapTextToLines(value, valueW)
					if len(wrapped) == 0 {
						wrapped = []string{"unspecified"}
					}
				}

				displayLines := min(notesMaxLines, len(wrapped))
				hasScroll := len(wrapped) > notesMaxLines
				displayW := valueW
				thumbLines := 0
				if hasScroll {
					displayW = max(valueW-2, 4)
					thumbLines = max(1, notesMaxLines*notesMaxLines/len(wrapped))
				}

				for i := range displayLines {
					line := truncateParamLine(wrapped[i], displayW)
					var lineStr string
					if hasScroll {
						pad := displayW - lipgloss.Width(line)
						padded := line
						if pad > 0 {
							padded = line + strings.Repeat(" ", pad)
						}
						scrollChar := "░"
						if i < thumbLines {
							scrollChar = "█"
						}
						lineStr = m.ui.styles.paramDetailContent.Render(padded) +
							" " + m.ui.styles.scrollBarColumn.Render(scrollChar)
					} else {
						lineStr = m.ui.styles.paramDetailContent.Render(line)
					}
					if i == 0 {
						rows = append(rows, labelPart+lineStr)
					} else {
						rows = append(rows, strings.Repeat(" ", labelPartW)+lineStr)
					}
				}
			default:
				if focused && m.params.editKind == paramEditMetadataValue {
					labelPart := m.renderLabelPart(prefix, paramMetadataFieldLabels[field])
					rows = append(rows, lipgloss.JoinHorizontal(lipgloss.Top,
						labelPart,
						m.params.editInput.View(),
					))
					continue
				}
				// Split label (muted) and value (bright) with aligned column.
				labelPart := m.renderLabelPart(prefix, paramMetadataFieldLabels[field])
				labelPartW := lipgloss.Width(labelPart)
				valueW := max(maxSec-labelPartW, 8)
				value := m.metadataFieldValue(field)
				if value == "" {
					value = "unspecified"
				}
				valuePart := m.ui.styles.paramDetailContent.Render(truncateParamLine(value, valueW))
				rows = append(rows, labelPart+valuePart)
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
	envItems := make([]string, len(m.params.editor.env))
	for i, e := range m.params.editor.env {
		envItems[i] = formatEnvVar(e)
	}
	rows = append(rows, m.renderEditableListItems(envItems, paramFocusEnv, m.params.editor.envCursor, paramEditEnvLine, maxSec)...)
	rows = append(rows, "", sectionLine("Extra Arguments (e.g. --max-model-len 131072)"), "")
	rows = append(rows, m.renderEditableListItems(m.params.editor.args, paramFocusArgs, m.params.editor.argsCursor, paramEditArgLine, maxSec)...)
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
