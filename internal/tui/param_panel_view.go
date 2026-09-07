package tui

import (
	"fmt"
	"slices"
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/flyingnobita/llml/internal/profiles"
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

// renderMetadataSection renders the Profile Metadata block. Each field kind has
// its own renderer below; this function only picks between them.
func (m Model) renderMetadataSection(cw, maxSec int, secBox lipgloss.Style) string {
	rows := []string{
		lipgloss.JoinHorizontal(lipgloss.Top,
			m.ui.styles.body.Render("  "),
			m.ui.styles.paramSectionHeading.Render(truncateParamLine("Profile Metadata", maxSec-2)),
		),
		"",
	}

	if len(m.params.editor.profiles) == 0 {
		rows = append(rows, m.ui.styles.paramDetailContent.Render("  unspecified"))
		return secBox.Width(cw).Render(lipgloss.JoinVertical(lipgloss.Left, rows...))
	}

	p := m.params.editor.ActiveProfile()
	for field := paramMetadataField(0); field < paramMetadataFieldCount; field++ {
		focused := m.params.focus == paramFocusMetadata && m.params.metadataCursor == int(field)
		rows = append(rows, m.renderMetadataField(field, p, focused, maxSec)...)
	}
	return secBox.Width(cw).Render(lipgloss.JoinVertical(lipgloss.Left, rows...))
}

// renderMetadataField renders the rows for one metadata field.
func (m Model) renderMetadataField(field paramMetadataField, p profiles.Profile, focused bool, maxSec int) []string {
	switch field {
	case paramMetadataBackend:
		return m.renderBackendRow(p, focused, maxSec)
	case paramMetadataHardwareClass:
		return m.renderHardwareClassRow(p, focused, maxSec)
	case paramMetadataUseCasePrimary:
		return m.renderPrimaryRow(p, focused, maxSec)
	case paramMetadataUseCaseTags:
		return m.renderCheckboxRow("Tags", profiles.CanonicalTags, p.UseCase.Tags,
			m.params.tagCursor, focused, maxSec)
	case paramMetadataHardwareNotes:
		return m.renderNotesRow(field, focused, maxSec)
	default:
		return m.renderTextRow(field, focused, maxSec)
	}
}

// metadataRowPrefix marks the focused row with a cursor.
func metadataRowPrefix(focused bool) string {
	if focused {
		return "› "
	}
	return "  "
}

func (m Model) renderBackendRow(p profiles.Profile, focused bool, maxSec int) []string {
	opts := m.paramBackendOptionsForModel()
	labels := make([]string, len(opts))
	for i, o := range opts {
		labels[i] = radioOptionLabel(o)
	}
	return m.renderRadioRow("Backend", labels, slices.Index(opts, p.Backend),
		m.params.backendCursor, focused, maxSec)
}

func (m Model) renderHardwareClassRow(p profiles.Profile, focused bool, maxSec int) []string {
	labels := make([]string, len(paramHardwareClassOptions))
	for i, o := range paramHardwareClassOptions {
		labels[i] = radioOptionLabel(string(o))
	}
	return m.renderRadioRow("Hardware Class", labels,
		slices.Index(paramHardwareClassOptions, p.Hardware.Class),
		m.params.hardwareClassCursor, focused, maxSec)
}

func (m Model) renderPrimaryRow(p profiles.Profile, focused bool, maxSec int) []string {
	selected := make([]string, len(p.UseCase.Primary))
	for i, v := range p.UseCase.Primary {
		selected[i] = string(v)
	}
	canonical := make([]string, len(profiles.CanonicalPrimaries))
	for i, v := range profiles.CanonicalPrimaries {
		canonical[i] = string(v)
	}
	return m.renderCheckboxRow("Use Case Primary", canonical, selected,
		m.params.primaryCursor, focused, maxSec)
}

// renderNotesRow renders the multi-line Notes field: a textarea while editing,
// otherwise up to notesMaxLines of wrapped text with a scrollbar.
func (m Model) renderNotesRow(field paramMetadataField, focused bool, maxSec int) []string {
	labelPart := m.renderLabelPart(metadataRowPrefix(focused), paramMetadataFieldLabels[field])
	labelPartW := lipgloss.Width(labelPart)

	if focused && m.params.editKind == paramEditMetadataValue {
		indent := strings.Repeat(" ", labelPartW)
		taLines := strings.Split(m.params.notesInput.View(), "\n")
		var rows []string
		for i, taLine := range taLines {
			if taLine == "" && i == len(taLines)-1 {
				break // the textarea view ends with an empty line
			}
			if i == 0 {
				rows = append(rows, labelPart+taLine)
				continue
			}
			rows = append(rows, indent+taLine)
		}
		return rows
	}

	valueW := max(maxSec-labelPartW, 8)
	wrapped := wrapTextToLines(m.metadataFieldValue(field), valueW)
	if len(wrapped) == 0 {
		wrapped = []string{"unspecified"}
	}

	displayLines := min(notesMaxLines, len(wrapped))
	hasScroll := len(wrapped) > notesMaxLines
	displayW := valueW
	thumbLines := 0
	if hasScroll {
		displayW = max(valueW-2, 4)
		thumbLines = max(1, notesMaxLines*notesMaxLines/len(wrapped))
	}

	rows := make([]string, 0, displayLines)
	for i := range displayLines {
		lineStr := m.renderNotesLine(wrapped[i], displayW, i, thumbLines, hasScroll)
		if i == 0 {
			rows = append(rows, labelPart+lineStr)
			continue
		}
		rows = append(rows, strings.Repeat(" ", labelPartW)+lineStr)
	}
	return rows
}

// renderNotesLine renders one wrapped Notes line, with a scrollbar cell when the
// text is taller than the visible window.
func (m Model) renderNotesLine(text string, displayW, i, thumbLines int, hasScroll bool) string {
	line := truncateParamLine(text, displayW)
	if !hasScroll {
		return m.ui.styles.paramDetailContent.Render(line)
	}
	if pad := displayW - lipgloss.Width(line); pad > 0 {
		line += strings.Repeat(" ", pad)
	}
	scrollChar := "░"
	if i < thumbLines {
		scrollChar = "█"
	}
	return m.ui.styles.paramDetailContent.Render(line) + " " + m.ui.styles.scrollBarColumn.Render(scrollChar)
}

// renderTextRow renders a single-line metadata field, as an input while editing.
func (m Model) renderTextRow(field paramMetadataField, focused bool, maxSec int) []string {
	labelPart := m.renderLabelPart(metadataRowPrefix(focused), paramMetadataFieldLabels[field])
	if focused && m.params.editKind == paramEditMetadataValue {
		return []string{lipgloss.JoinHorizontal(lipgloss.Top, labelPart, m.params.editInput.View())}
	}
	// Label muted, value bright, aligned at a common column.
	valueW := max(maxSec-lipgloss.Width(labelPart), 8)
	value := m.metadataFieldValue(field)
	if value == "" {
		value = "unspecified"
	}
	return []string{labelPart + m.ui.styles.paramDetailContent.Render(truncateParamLine(value, valueW))}
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
