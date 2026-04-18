package tui

import (
	"fmt"
	"strings"
	"time"

	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/flyingnobita/llml/internal/models"
)

// serverLogPaneView renders the bordered server log viewport and, when vertical
// scrolling is possible, a text-mode scroll track beside it (█/░).
func (m Model) serverLogPaneView() string {
	vp := m.server.viewport.View()
	if m.server.viewport.TotalLineCount() <= m.server.viewport.VisibleLineCount() {
		return vp
	}
	trackH := lipgloss.Height(vp)
	if trackH < 1 {
		trackH = 1
	}
	col := verticalScrollBarColumn(viewportVerticalScrollPercent(m.server.viewport), trackH)
	col = m.ui.styles.scrollBarColumn.Render(col)
	return lipgloss.JoinHorizontal(lipgloss.Top, vp, col)
}

// viewportVerticalScrollPercent returns [0,1] for vertical scroll position. The
// upstream [viewport.Model.ScrollPercent] compares outer Height to total line count
// and divides by (total−Height), which is wrong for bordered viewports (the maximum
// Y offset uses total−Height+frameSize) and breaks when SoftWrap inflates total.
func viewportVerticalScrollPercent(vp viewport.Model) float64 {
	total := vp.TotalLineCount()
	if total == 0 {
		return 0
	}
	vs := vp.Style.GetVerticalFrameSize()
	maxY := total - vp.Height() + vs
	if maxY <= 0 {
		return 0
	}
	y := float64(vp.YOffset())
	p := y / float64(maxY)
	if p < 0 {
		return 0
	}
	if p > 1 {
		return 1
	}
	return p
}

// verticalScrollBarColumn renders a single-column scroll indicator: filled cells
// from the top grow with scroll position ([viewportVerticalScrollPercent]).
func verticalScrollBarColumn(pct float64, trackH int) string {
	if trackH < 2 {
		return ""
	}
	if pct < 0 {
		pct = 0
	}
	if pct > 1 {
		pct = 1
	}
	filled := int(pct * float64(trackH))
	if filled > trackH {
		filled = trackH
	}
	var b strings.Builder
	for i := 0; i < trackH; i++ {
		if i > 0 {
			b.WriteByte('\n')
		}
		if i < filled {
			b.WriteString("█")
		} else {
			b.WriteString("░")
		}
	}
	return b.String()
}

// launchPreviewVisible is true when the main table lists models and a launch preview can be shown.
func launchPreviewVisible(m Model) bool {
	if m.loading || m.loadErr != nil || len(m.table.files) == 0 {
		return false
	}
	return true
}

// launchPreviewScrollable is true when the launch command has more lines than the fixed preview height.
func launchPreviewScrollable(m Model) bool {
	return launchPreviewVisible(m) &&
		m.preview.viewport.TotalLineCount() > m.preview.viewport.VisibleLineCount()
}

// launchPreviewPaneView renders the bordered, scrollable launch command viewport or "".
func (m Model) launchPreviewPaneView() string {
	if !launchPreviewVisible(m) {
		return ""
	}
	vp := m.preview.viewport.View()
	if !launchPreviewScrollable(m) {
		return m.ui.styles.launchPreview.Render(vp)
	}
	trackH := lipgloss.Height(vp)
	if trackH < 1 {
		trackH = 1
	}
	col := verticalScrollBarColumn(viewportVerticalScrollPercent(m.preview.viewport), trackH)
	col = m.ui.styles.scrollBarColumn.Render(col)
	row := lipgloss.JoinHorizontal(lipgloss.Top, vp, col)
	return m.ui.styles.launchPreview.Render(row)
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
	inner := "Runtimes\n" + block
	return m.ui.styles.runtimePanel.Width(contentWidth).Render(inner)
}

const appTitle = "LLM Launcher"

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
				"%s · %s · %s · %d×%d",
				FooterHintTabSections, stopOrDismiss, FooterNavHint, m.layout.width, m.layout.height,
			)
		}
		// Table focused: same global shortcuts as the idle view except run (R / ctrl+R) while a server is up.
		parts := []string{
			FooterHintRefresh,
			FooterHintRescan,
			FooterNavHint,
			FooterHintConfigPort,
			FooterHintParameters,
			FooterHintSort,
			FooterHintToggleTheme,
			FooterHintCopyPath,
			FooterHintTabSections,
			stopOrDismiss,
		}
		if launchPreviewScrollable(m) {
			parts = append(parts, FooterHintLaunchPreviewScroll)
		}
		parts = append(parts, fmt.Sprintf("%d×%d", m.layout.width, m.layout.height))
		return strings.Join(parts, FooterHintSep)
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
// (title, subtitle, scroll bars, footer). needsTableHBar and needsLogHBar should
// match whether each horizontal track is shown.
func mainChromeLines(m Model, needsTableHBar bool, needsLogHBar bool) int {
	iw := m.innerWidth()
	n := lipgloss.Height(m.appTitleBlock(iw))
	n += lipgloss.Height(m.ui.styles.subtitle.Render(appSubtitle))
	n += 1

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

	n += 1
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

// mainAppPlacedView renders the primary UI (title, model table, server log when
// running, footer, …) as a full-width, full-height string. Used for the normal
// view and as the backdrop when a centered modal (parameters, runtime config) is open.
func (m Model) mainAppPlacedView() string {
	iw := m.innerWidth()

	title := m.appTitleBlock(iw)
	sub := m.ui.styles.subtitle.Render(appSubtitle)

	var body string
	switch {
	case m.loading:
		body = m.ui.styles.body.Render("Scanning for models…")
	case m.loadErr != nil:
		body = m.ui.styles.errLine.Render("Error: " + m.loadErr.Error())
	case len(m.table.files) == 0:
		body = m.ui.styles.body.Render(fmt.Sprintf("No GGUF or safetensors models found. Press '%s' to add search paths, or place models under ~/models, ~/.cache/huggingface/hub, etc.", FooterKeyModelPaths))
	default:
		m.table.hscroll.SetContent(m.table.tbl.View())
		th := m.layout.tableBodyH
		if th < 1 {
			th = defaultTableHeight
		}
		m.table.hscroll.SetWidth(iw)
		m.table.hscroll.SetHeight(th)
		preview := m.launchPreviewPaneView()
		var parts []string
		parts = append(parts, m.table.hscroll.View())
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
	}

	footer := m.ui.styles.footer.Render(footerHelpLine(m))

	rows := []string{title, sub, "", body, "", footer}
	if m.lastRunNote != "" {
		rows = append(rows, m.lastRunNoteView())
	}
	block := lipgloss.JoinVertical(lipgloss.Left, rows...)
	framed := m.ui.styles.app.Render(block)

	placed := lipgloss.Place(m.layout.width, m.layout.height, lipgloss.Center, lipgloss.Top, framed)
	return clampRenderedHeightKeepTopBottom(placed, m.layout.height)
}

func clampRenderedHeightKeepTopBottom(s string, maxH int) string {
	if maxH <= 0 {
		return s
	}
	lines := strings.Split(s, "\n")
	if len(lines) <= maxH {
		return s
	}
	topKeep := maxH / 2
	if topKeep < 1 {
		topKeep = 1
	}
	bottomKeep := maxH - topKeep
	if bottomKeep < 1 {
		bottomKeep = 1
	}
	out := append([]string{}, lines[:topKeep]...)
	out = append(out, lines[len(lines)-bottomKeep:]...)
	return strings.Join(out, "\n")
}

// View implements tea.Model.
func (m Model) View() tea.View {
	if m.layout.width == 0 {
		return tea.NewView("\n  Initializing…\n")
	}
	if m.params.open {
		s := overlayCentered(m.mainAppPlacedView(), m.paramPanelModalBlock(), m.layout.width, m.layout.height)
		v := tea.NewView(s)
		v.AltScreen = true
		return v
	}
	if m.helpOpen {
		s := overlayCentered(m.mainAppPlacedView(), m.helpPanelModalBlock(), m.layout.width, m.layout.height)
		v := tea.NewView(s)
		v.AltScreen = true
		return v
	}
	if m.rc.open {
		s := overlayCentered(m.mainAppPlacedView(), m.runtimeConfigModalBlock(), m.layout.width, m.layout.height)
		v := tea.NewView(s)
		v.AltScreen = true
		return v
	}
	if m.discovery.open {
		s := overlayCentered(m.mainAppPlacedView(), m.discoveryPathsModalBlock(), m.layout.width, m.layout.height)
		v := tea.NewView(s)
		v.AltScreen = true
		return v
	}

	v := tea.NewView(m.mainAppPlacedView())
	v.AltScreen = true
	return v
}

// discoveryPathsModalBlock returns the framed discovery paths configuration panel.
func (m Model) discoveryPathsModalBlock() string {
	cw := m.paramPanelContentWidth()
	rows := []string{
		m.modalTitleRow(cw, m.ui.styles.portConfigTitle, "Model paths"),
		m.ui.styles.subtitle.Width(cw).Render(discoveryPathsModalSubtitle),
		"",
	}

	for i, p := range m.discovery.paths {
		prefix := "  "
		if i == m.discovery.cursor {
			prefix = "› "
		}
		if m.discovery.editOpen && i == m.discovery.cursor {
			rows = append(rows, m.ui.styles.body.Render(prefix)+m.discovery.editInput.View())
		} else {
			style := m.ui.styles.body
			if i == m.discovery.cursor {
				style = m.ui.styles.bodyBold
			}
			rows = append(rows, style.Render(prefix+p))
		}
	}

	if m.discovery.editOpen && m.discovery.cursor == len(m.discovery.paths) {
		rows = append(rows, m.ui.styles.body.Render("› ")+m.discovery.editInput.View())
	}

	if len(m.discovery.paths) == 0 && !m.discovery.editOpen {
		rows = append(rows, m.ui.styles.bodyDim.Render("  (No extra paths configured)"))
	}

	rows = append(rows, "")
	rows = append(rows, m.ui.styles.body.Render("Defaults (read-only):"))

	for _, p := range models.DefaultSearchRoots() {
		rows = append(rows, m.ui.styles.bodyDim.Render("  "+p))
	}

	rows = append(rows, "")
	rows = append(rows, m.ui.styles.footer.Render(FooterDiscoveryPathsHints))

	block := lipgloss.JoinVertical(lipgloss.Left, rows...)
	return m.ui.styles.portConfigBox.Render(block)
}

// runtimeConfigModalBlock returns the framed runtime configuration panel only
// (no full-screen placement). Composed over the main view via [overlayCentered].
// [runtimePanelView] is shown under the title.
func (m Model) runtimeConfigModalBlock() string {
	label := func(focused bool, name string) string {
		prefix := "  "
		if focused {
			prefix = "› "
		}
		return m.ui.styles.body.Render(prefix + name)
	}
	cw := m.paramPanelContentWidth()
	rows := []string{
		m.modalTitleRow(cw, m.ui.styles.portConfigTitle, "Runtime environment"),
		runtimePanelView(m, cw),
		m.ui.styles.subtitle.Width(cw).Render(runtimeConfigModalSubtitle),
		"",
		label(m.rc.focus == runtimeFieldLlamaCppPath, models.EnvLlamaCppPath),
		m.rc.inputs[runtimeFieldLlamaCppPath].View(),
		"",
		label(m.rc.focus == runtimeFieldVLLMPath, models.EnvVLLMPath),
		m.rc.inputs[runtimeFieldVLLMPath].View(),
		"",
		label(m.rc.focus == runtimeFieldVLLMVenv, runtimeConfigLabelVLLMVenv),
		m.rc.inputs[runtimeFieldVLLMVenv].View(),
		"",
		label(m.rc.focus == runtimeFieldLlamaPort, models.EnvLlamaServerPort),
		m.rc.inputs[runtimeFieldLlamaPort].View(),
		"",
		label(m.rc.focus == runtimeFieldVLLMPort, models.EnvVLLMServerPort),
		m.rc.inputs[runtimeFieldVLLMPort].View(),
		"",
		m.ui.styles.footer.Render(FooterRuntimeConfigHints),
	}
	block := lipgloss.JoinVertical(lipgloss.Left, rows...)
	return m.ui.styles.portConfigBox.Render(block)
}

// SelectedModel returns the filesystem path and backend for the highlighted row.
func (m Model) SelectedModel() (path string, backend models.ModelBackend) {
	if len(m.table.tbl.Rows()) == 0 || m.table.tbl.Cursor() < 0 {
		return "", models.BackendLlama
	}
	i := m.table.tbl.Cursor()
	if i < 0 || i >= len(m.table.files) {
		return "", models.BackendLlama
	}
	return m.table.files[i].Path, m.table.files[i].Backend
}

// SelectedPath returns the full path of the highlighted row, or empty if none.
func (m Model) SelectedPath() string {
	p, _ := m.SelectedModel()
	return p
}

// horizontalScrollBarLine renders a filled track (█) and remainder (░) for horizontal scroll position.
func horizontalScrollBarLine(pct float64, maxWidth int) string {
	if maxWidth < 14 {
		return ""
	}
	inner := maxWidth - 4
	if inner < 8 {
		return ""
	}
	filled := int(pct * float64(inner))
	if filled > inner {
		filled = inner
	}
	if filled < 0 {
		filled = 0
	}
	return "  " + strings.Repeat("█", filled) + strings.Repeat("░", inner-filled) + "  "
}
