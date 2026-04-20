package tui

import (
	"fmt"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/flyingnobita/llml/internal/models"
)

// mainPaneCaptionLine renders a full-width pane caption (lipgloss v2 has no border Title API).
func (m Model) mainPaneCaptionLine(title string, titleStyle lipgloss.Style) string {
	iw := m.innerWidth()
	if iw < minInnerWidth {
		iw = minInnerWidth
	}
	return titleStyle.Width(iw).Render(title)
}

func (m Model) launchPreviewPaneTitleStyle() lipgloss.Style {
	st := lipgloss.NewStyle().Bold(true)
	if m.preview.focused {
		return st.Foreground(m.ui.theme.SplitPaneBorderFocused)
	}
	return st.Foreground(m.ui.theme.Border)
}

func (m Model) serverLogPaneTitleStyle() lipgloss.Style {
	st := lipgloss.NewStyle().Bold(true)
	if m.server.splitFocused {
		return st.Foreground(m.ui.theme.SplitPaneBorderFocused)
	}
	return st.Foreground(m.ui.theme.SplitPaneBorderDim)
}

// serverLogPaneView renders the bordered server log viewport and, when vertical
// withVScrollBar appends a vertical scroll-bar column to content at the given scroll percent.
// Returns content unchanged when the bar would be empty (trackH < 2).
func (m Model) withVScrollBar(content string, pct float64) string {
	trackH := lipgloss.Height(content)
	if trackH < 1 {
		trackH = 1
	}
	col := verticalScrollBarColumn(pct, trackH)
	if col == "" {
		return content
	}
	col = m.ui.styles.scrollBarColumn.Render(col)
	return lipgloss.JoinHorizontal(lipgloss.Top, content, col)
}

// scrolling is possible, a text-mode scroll track beside it (█/░).
func (m Model) serverLogPaneView() string {
	title := m.mainPaneCaptionLine(MainPaneTitleServerOutput, m.serverLogPaneTitleStyle())
	vp := m.server.viewport.View()
	if m.server.viewport.TotalLineCount() <= m.server.viewport.VisibleLineCount() {
		return lipgloss.JoinVertical(lipgloss.Left, title, vp)
	}
	row := m.withVScrollBar(vp, viewportVerticalScrollPercent(m.server.viewport))
	return lipgloss.JoinVertical(lipgloss.Left, title, row)
}

// launchPreviewVisible is true when the main table lists models and a launch preview can be shown.
func (m Model) launchPreviewVisible() bool {
	return !m.loading && m.loadErr == nil && len(m.table.files) > 0
}

// launchPreviewScrollable is true when the launch command has more lines than the fixed preview height.
func (m Model) launchPreviewScrollable() bool {
	return m.launchPreviewVisible() &&
		m.preview.viewport.TotalLineCount() > m.preview.viewport.VisibleLineCount()
}

// modelTablePaneView renders the horizontal-scroll viewport around the model table and, when the
// list overflows vertically (inner table body or outer wrapped content), a █/░ column beside it.
func (m Model) modelTablePaneView() string {
	vp := m.table.hscroll.View()
	if len(m.table.files) == 0 {
		return vp
	}
	trackH := lipgloss.Height(vp)
	if trackH < 2 {
		return vp
	}
	var pct float64
	haveOuter := m.table.hscroll.TotalLineCount() > m.table.hscroll.VisibleLineCount()
	if haveOuter {
		pct = viewportVerticalScrollPercent(m.table.hscroll)
	} else if len(m.table.files) > m.table.tbl.Height() {
		n := len(m.table.files)
		if n <= 1 {
			pct = 0
		} else {
			pct = float64(m.table.tbl.Cursor()) / float64(n-1)
		}
	} else {
		return vp
	}
	return m.withVScrollBar(vp, pct)
}

// launchPreviewPaneView renders the bordered, scrollable launch command viewport or "".
func (m Model) launchPreviewPaneView() string {
	if !m.launchPreviewVisible() {
		return ""
	}
	title := m.mainPaneCaptionLine(MainPaneTitleLaunchCommand, m.launchPreviewPaneTitleStyle())
	vp := m.preview.viewport.View()
	var inner string
	if !m.launchPreviewScrollable() {
		inner = vp
	} else {
		inner = m.withVScrollBar(vp, viewportVerticalScrollPercent(m.preview.viewport))
	}
	stack := lipgloss.JoinVertical(lipgloss.Left, title, inner)
	return m.ui.styles.launchPreview.Render(stack)
}

// runtimePanelView renders the runtimes summary (label = value per line) for the runtime
// config modal opened with c.
func runtimePanelView(m Model, contentWidth int) string {
	if m.layout.width == 0 {
		return ""
	}
	if contentWidth < 24 {
		contentWidth = 24
	}
	var block string
	if !m.runtimeScanned && m.loading {
		block = "Detecting runtimes…"
	} else {
		lines := RuntimePanelLines(contentWidth, m.runtime)
		block = strings.Join(lines, "\n")
		if !m.table.lastScan.IsZero() {
			block += "\nLast model scan: " + m.table.lastScan.Local().Format(time.RFC3339)
		}
	}
	inner := m.ui.styles.paramSectionHeading.Render("Active Configuration") + "\n" + block
	return m.ui.styles.runtimePanel.Width(contentWidth).Render(inner)
}

// lastRunNoteView renders lastRunNote as one styled line per newline-separated
// segment below the main footer (not shown inside the runtime-environment modal).
func (m Model) lastRunNoteView() string {
	if m.lastRunNote == "" {
		return ""
	}
	lineStyle := m.ui.styles.errLine
	if m.lastRunNoteSuccess {
		lineStyle = m.ui.styles.body
	}
	parts := strings.Split(m.lastRunNote, "\n")
	var lines []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		lines = append(lines, lineStyle.Render(p))
	}
	if len(lines) == 0 {
		return ""
	}
	return lipgloss.JoinVertical(lipgloss.Left, lines...)
}

// footerHelpLine is the keyboard hint line (shared with layout height math).
// Each binding uses "key: description"; bindings are separated by " · ".
// The same convention is used for modal hint bars (runtime config, parameters).
func footerHelpLine(m Model) string {
	if m.server.running {
		stopOrDismiss := FooterSplitStopServer
		if m.server.exited {
			stopOrDismiss = FooterSplitDismiss
		}
		if m.server.splitFocused {
			return fmt.Sprintf(
				"%s · %s · %s · %s",
				FooterHintTabSections, FooterNavHint, stopOrDismiss, FooterHintHelp,
			)
		}
		if m.preview.focused {
			return fmt.Sprintf(
				"%s · %s · %s · %s · %s",
				FooterHintTabSections, FooterNavHint, FooterHintCopyPath, stopOrDismiss, FooterHintHelp,
			)
		}
		// Table focused: same global shortcuts as the idle view except run (R / ctrl+R) while a server is up.
		parts := []string{
			FooterHintTabSections,
			FooterNavHint,
			stopOrDismiss,
			FooterHintHelp,
		}
		return strings.Join(parts, FooterHintSep)
	}
	if m.preview.focused {
		return fmt.Sprintf(
			"%s · %s · %s · %s · %s · %s",
			FooterHintTabSections, FooterNavHint, FooterHintRunSplit, FooterHintParameters, FooterHintCopyPath, FooterHintHelp,
		)
	}
	parts := []string{
		FooterHintTabSections,
		FooterNavHint,
		FooterHintRunSplit,
		FooterHintParameters,
		FooterHintHelp,
	}
	return strings.Join(parts, FooterHintSep)
}

// mainChromeLines counts rows in the main view block excluding the table body
// (title, optional subtitle, scroll bars, footer). needsTableHBar and needsLogHBar should
// match whether each horizontal track is shown.
func mainChromeLines(m Model, needsTableHBar bool, needsLogHBar bool) int {
	iw := m.innerWidth()
	n := lipgloss.Height(m.appTitleBlock(iw))
	if strings.TrimSpace(appSubtitle) != "" {
		n += lipgloss.Height(m.ui.styles.subtitle.Render(appSubtitle))
	}

	if needsTableHBar && len(m.table.files) > 0 {
		if bar := horizontalScrollBarLine(0, iw); bar != "" {
			n += lipgloss.Height(m.ui.styles.footer.Render(bar))
		}
	}

	if needsLogHBar && m.server.running {
		if bar := horizontalScrollBarLine(0, iw); bar != "" {
			n += lipgloss.Height(m.ui.styles.footer.Render(bar))
		}
	}

	n += lipgloss.Height(m.ui.styles.footer.Render(footerHelpLine(m)))

	if m.lastRunNote != "" {
		n += lipgloss.Height(m.lastRunNoteView())
	}
	return n
}

// portConfigContentWidth is the maximum text width inside modals when uncapped (see
// [Model.paramPanelContentWidth] for the wide-terminal cap used by runtime + parameters UIs).
func (m Model) portConfigContentWidth() int {
	if m.layout.width <= 0 {
		return minInnerWidth
	}
	w := m.layout.width - m.ui.styles.portConfigBox.GetHorizontalFrameSize()
	if w < minInnerWidth {
		return minInnerWidth
	}
	return w
}

// paramPanelContentWidth is the inner width for the parameters and runtime-environment
// modals. It matches portConfigContentWidth on narrow terminals but is capped on wide ones.
func (m Model) paramPanelContentWidth() int {
	w := m.portConfigContentWidth()
	if w > paramPanelMaxInnerWidth {
		w = paramPanelMaxInnerWidth
	}
	if w < minInnerWidth {
		w = minInnerWidth
	}
	return w
}

// fitThemeToastInline renders the transient theme message as a compact reversed chip
// that fits in maxW terminal columns (or returns "" if it cannot).
func (m Model) fitThemeToastInline(maxW int) string {
	if maxW < 4 || m.ui.themeToast == "" {
		return ""
	}
	runes := []rune(m.ui.themeToast)
	for len(runes) > 0 {
		s := string(runes)
		rendered := m.ui.styles.themeToastInline.Render(s)
		if lipgloss.Width(rendered) <= maxW {
			return rendered
		}
		runes = runes[:len(runes)-1]
	}
	return ""
}

// joinLeftAndToast renders left (already styled) plus an optional theme toast on one line.
func (m Model) joinLeftAndToast(innerW int, leftRendered string) string {
	lw := lipgloss.Width(leftRendered)
	if lw >= innerW {
		return leftRendered
	}
	toast := m.fitThemeToastInline(innerW - lw)
	if toast == "" {
		return leftRendered
	}
	gap := innerW - lw - lipgloss.Width(toast)
	if gap < 1 {
		gap = 1
	}
	return lipgloss.JoinHorizontal(lipgloss.Top, leftRendered, strings.Repeat(" ", gap), toast)
}

// appTitleBlock renders the app title with an optional same-row theme toast
// (right-aligned), using the same vertical space as styles.title.
func (m Model) appTitleBlock(innerW int) string {
	if m.ui.themeToast == "" {
		return m.ui.styles.title.Render(appTitle)
	}
	left := m.ui.styles.titleBoldLeft.Render(appTitle)
	if lipgloss.Width(left) >= innerW {
		return m.ui.styles.title.Render(appTitle)
	}
	line := m.joinLeftAndToast(innerW, left)
	if line == left {
		return m.ui.styles.title.Render(appTitle)
	}
	return m.ui.styles.titleToastRowWrap.Render(line)
}

// modalTitleRow renders a one-line modal title with an optional same-row theme toast.
func (m Model) modalTitleRow(innerW int, titleStyle lipgloss.Style, plain string) string {
	if m.ui.themeToast == "" {
		return titleStyle.Render(plain)
	}
	left := titleStyle.Render(plain)
	return m.joinLeftAndToast(innerW, left)
}

// mainAppModelListBody renders the model-table stack (table viewport, optional h-scroll row,
// launch preview, optional server log). minTableScrollH is the outer hscroll viewport height;
// use -1 to size the viewport to the natural height of tbl.View(). Returns the rendered body
// and the hscroll height applied (for absorbing leftover terminal rows inside the table chrome).
func (m Model) mainAppModelListBody(iw int, minTableScrollH int) (body string, appliedScrollH int) {
	tview := m.table.tbl.View()
	th := strings.Count(tview, "\n") + 1
	if th < 1 {
		th = 1
	}
	appliedScrollH = th
	if minTableScrollH > appliedScrollH {
		appliedScrollH = minTableScrollH
	}
	m.table.hscroll.SetContent(tview)
	m.table.hscroll.SetWidth(iw)
	m.table.hscroll.SetHeight(appliedScrollH)
	preview := m.launchPreviewPaneView()
	var parts []string
	parts = append(parts, m.modelTablePaneView())
	if m.layout.tableNeedsHScroll {
		if line := horizontalScrollBarLine(m.table.hscroll.HorizontalScrollPercent(), iw); line != "" {
			parts = append(parts, m.ui.styles.footer.Render(line))
		}
	}
	if preview != "" {
		parts = append(parts, preview)
	}
	if m.server.running {
		if m.server.viewportH > 0 {
			m.server.viewport.SetHeight(m.server.viewportH)
		}
		parts = append(parts, m.serverLogPaneView())
		if m.serverLogNeedsHorizontalScroll() {
			if line := horizontalScrollBarLine(m.server.viewport.HorizontalScrollPercent(), iw); line != "" {
				parts = append(parts, m.ui.styles.footer.Render(line))
			}
		}
		body = lipgloss.JoinVertical(lipgloss.Left, parts...)
	} else {
		if len(parts) == 1 {
			body = parts[0]
		} else {
			body = lipgloss.JoinVertical(lipgloss.Left, parts...)
		}
	}
	return body, appliedScrollH
}

// mainAppPlacedView renders the primary UI (title, model table, server log when
// running, footer, …) as a full-width string whose height matches the terminal.
// Extra vertical slack is absorbed inside the bordered table viewport (not as blank
// rows between title and table or between the preview and footer).
// Used for the normal view and as the backdrop when a centered modal is open.
func (m Model) mainAppPlacedView() string {
	iw := m.innerWidth()

	title := m.appTitleBlock(iw)

	var body string
	tableScrollBase := 0
	switch {
	case m.loading:
		body = m.ui.styles.body.Render("Scanning for models…")
	case m.loadErr != nil:
		body = m.ui.styles.errLine.Render("Error: " + m.loadErr.Error())
	case len(m.table.files) == 0:
		body = m.ui.styles.body.Render(fmt.Sprintf("No GGUF or safetensors models found. Press '%s' to add search paths, or place models under ~/models, ~/.cache/huggingface/hub, etc.", FooterKeyModelPaths))
	default:
		var ash int
		body, ash = m.mainAppModelListBody(iw, -1)
		tableScrollBase = ash
	}

	footer := m.ui.styles.footer.Render(footerHelpLine(m))

	headerParts := []string{title}
	if strings.TrimSpace(appSubtitle) != "" {
		headerParts = append(headerParts, m.ui.styles.subtitle.Render(appSubtitle))
	}
	header := lipgloss.JoinVertical(lipgloss.Left, headerParts...)

	tailParts := []string{body, footer}
	if m.lastRunNote != "" {
		tailParts = append(tailParts, m.lastRunNoteView())
	}
	tail := lipgloss.JoinVertical(lipgloss.Left, tailParts...)

	combined := lipgloss.JoinVertical(lipgloss.Left, header, tail)
	framed := m.ui.styles.app.Render(combined)

	target := m.layout.height
	if target > 0 && tableScrollBase > 0 {
		pad := target - lipgloss.Height(framed)
		if pad > 0 {
			body, _ = m.mainAppModelListBody(iw, tableScrollBase+pad)
			tailParts = []string{body, footer}
			if m.lastRunNote != "" {
				tailParts = append(tailParts, m.lastRunNoteView())
			}
			tail = lipgloss.JoinVertical(lipgloss.Left, tailParts...)
			combined = lipgloss.JoinVertical(lipgloss.Left, header, tail)
			framed = m.ui.styles.app.Render(combined)
		}
	}

	placed := lipgloss.PlaceHorizontal(m.layout.width, lipgloss.Center, framed)
	return clampRenderedHeightKeepTopBottom(placed, target)
}

// modalBlock returns the overlay content for whichever modal is currently open,
// and true when any modal is open.
func (m Model) modalBlock() (string, bool) {
	switch {
	case m.params.open:
		return m.paramPanelModalBlock(), true
	case m.helpOpen:
		return m.helpPanelModalBlock(), true
	case m.rc.open:
		return m.runtimeConfigModalBlock(), true
	case m.discovery.open:
		return m.discoveryPathsModalBlock(), true
	}
	return "", false
}

// View implements tea.Model.
func (m Model) View() tea.View {
	if m.layout.width == 0 {
		return tea.NewView("\n  Initializing…\n")
	}
	base := m.mainAppPlacedView()
	if block, ok := m.modalBlock(); ok {
		base = overlayCentered(base, block, m.layout.width, m.layout.height)
	}
	v := tea.NewView(base)
	v.AltScreen = true
	return v
}

// discoveryRow renders one editable path row (with inline edit input when active).
func (m Model) discoveryRow(i int, p string) string {
	prefix := "  "
	if i == m.discovery.cursor {
		prefix = "› "
	}
	if m.discovery.editOpen && i == m.discovery.cursor {
		return m.ui.styles.body.Render(prefix) + m.discovery.editInput.View()
	}
	style := m.ui.styles.body
	if i == m.discovery.cursor {
		style = m.ui.styles.bodyBold
	}
	return style.Render(prefix + p)
}

// discoveryPathsModalBlock returns the framed discovery paths configuration panel.
func (m Model) discoveryPathsModalBlock() string {
	cw := m.paramPanelContentWidth()
	rows := []string{
		m.modalTitleRow(cw, m.ui.styles.portConfigTitle, "Model Paths"),
		m.ui.styles.subtitle.Width(cw).Render(discoveryPathsModalSubtitle),
		"",
	}
	for i, p := range m.discovery.paths {
		rows = append(rows, m.discoveryRow(i, p))
	}
	if m.discovery.editOpen && m.discovery.cursor == len(m.discovery.paths) {
		rows = append(rows, m.ui.styles.body.Render("› ")+m.discovery.editInput.View())
	}
	if len(m.discovery.paths) == 0 && !m.discovery.editOpen {
		rows = append(rows, m.ui.styles.bodyDim.Render("  (No extra paths configured)"))
	}
	rows = append(rows, "", m.ui.styles.body.Render("Defaults (Read-Only):"))
	for _, p := range models.DefaultSearchRoots() {
		rows = append(rows, m.ui.styles.bodyDim.Render("  "+p))
	}
	rows = append(rows, "", m.ui.styles.footer.Render(FooterDiscoveryPathsHints))
	block := lipgloss.JoinVertical(lipgloss.Left, rows...)
	return m.ui.styles.portConfigBox.Render(block)
}

// runtimeFieldRow renders a label + input pair for one runtime config field.
func (m Model) runtimeFieldRow(fieldID runtimeField, label string) []string {
	prefix := "  "
	if m.rc.focus == fieldID {
		prefix = "› "
	}
	return []string{
		m.ui.styles.body.Render(prefix + label),
		m.rc.inputs[fieldID].View(),
	}
}

// runtimeConfigModalBlock returns the framed runtime configuration panel only
// (no full-screen placement). Composed over the main view via [overlayCentered].
// [runtimePanelView] is shown under the title.
func (m Model) runtimeConfigModalBlock() string {
	cw := m.paramPanelContentWidth()
	header := func(text string) string { return m.ui.styles.bodyBold.Render(text) }

	llamaRows := append([]string{header(runtimeConfigHeaderLlama), ""},
		append(m.runtimeFieldRow(runtimeFieldLlamaCppPath, runtimeConfigLabelLlamaCppPath),
			append([]string{""}, m.runtimeFieldRow(runtimeFieldLlamaPort, runtimeConfigLabelLlamaPort)...)...)...)
	llamaBlock := lipgloss.JoinVertical(lipgloss.Left, llamaRows...)

	vllmRows := append([]string{header(runtimeConfigHeaderVLLM), ""},
		append(m.runtimeFieldRow(runtimeFieldVLLMPath, runtimeConfigLabelVLLMPath),
			append([]string{""},
				append(m.runtimeFieldRow(runtimeFieldVLLMVenv, runtimeConfigLabelVLLMVenv),
					append([]string{""}, m.runtimeFieldRow(runtimeFieldVLLMPort, runtimeConfigLabelVLLMPort)...)...)...)...)...)
	vllmBlock := lipgloss.JoinVertical(lipgloss.Left, vllmRows...)

	var inputBlock string
	if cw >= 80 {
		inputBlock = lipgloss.JoinHorizontal(lipgloss.Top, llamaBlock, m.ui.styles.body.PaddingLeft(4).Render(vllmBlock))
	} else {
		inputBlock = lipgloss.JoinVertical(lipgloss.Left, llamaBlock, "", vllmBlock)
	}

	rows := []string{
		m.modalTitleRow(cw, m.ui.styles.portConfigTitle, "Runtime Environment"),
		runtimePanelView(m, cw),
		"",
		m.ui.styles.paramSectionHeading.Render("Overrides"),
		m.ui.styles.subtitle.Width(cw).Render(runtimeConfigModalSubtitle),
		"",
		inputBlock,
		"",
		m.ui.styles.footer.Render(FooterRuntimeConfigHints),
	}
	block := lipgloss.JoinVertical(lipgloss.Left, rows...)
	return m.ui.styles.portConfigBox.Render(block)
}
