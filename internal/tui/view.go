package tui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/flyingnobita/llml/internal/llamacpp"
)

// serverLogPaneView renders the bordered server log viewport and, when vertical
// scrolling is possible, a text-mode scroll track beside it (█/░).
func (m Model) serverLogPaneView() string {
	vp := m.serverViewport.View()
	if m.serverViewport.TotalLineCount() <= m.serverViewport.VisibleLineCount() {
		return vp
	}
	trackH := lipgloss.Height(vp)
	if trackH < 1 {
		trackH = 1
	}
	col := verticalScrollBarColumn(m.serverViewport.ScrollPercent(), trackH)
	col = m.styles.footer.Render(col)
	return lipgloss.JoinHorizontal(lipgloss.Top, vp, col)
}

// verticalScrollBarColumn renders a single-column scroll indicator: filled cells
// from the top grow with scroll position (see [viewport.Model.ScrollPercent]).
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

// runtimePanelView renders the bottom runtime env summary (env var = value per line).
func runtimePanelView(m Model, contentWidth int) string {
	if m.width == 0 {
		return ""
	}
	if contentWidth < 24 {
		contentWidth = 24
	}
	var block string
	if !m.runtimeScanned && m.loading {
		block = "Detecting runtimes…"
	} else {
		lines := llamacpp.RuntimePanelLines(contentWidth, m.runtime)
		block = strings.Join(lines, "\n")
	}
	inner := "Runtimes\n" + block
	return m.styles.runtimePanel.Width(contentWidth).Render(inner)
}

const appTitle = "LLM Launcher"

// footerHelpLine is the keyboard hint line (shared with layout height math).
// Each binding uses "key: description"; bindings are separated by " · ".
// The same convention is used for modal hint bars (runtime config, parameters).
func footerHelpLine(m Model) string {
	if m.serverRunning {
		if m.splitLogFocused {
			return fmt.Sprintf(
				"%s · %s · %s · %d×%d",
				FooterSplitTabToTable, FooterSplitStopServer, FooterNavHint, m.width, m.height,
			)
		}
		return fmt.Sprintf(
			"%s · %s · %s · %d×%d",
			FooterSplitTabToLog, FooterSplitStopServer, FooterNavHint, m.width, m.height,
		)
	}
	parts := []string{
		FooterHintRefresh,
		FooterHintRunSplit,
		FooterHintRunFullscreen,
		FooterHintConfigPort,
		FooterHintParameters,
		FooterHintToggleTheme,
		FooterHintQuit,
		FooterNavHint,
		FooterHintCopyPath,
		fmt.Sprintf("%d×%d", m.width, m.height),
	}
	return strings.Join(parts, FooterHintSep)
}

// mainChromeLines counts rows in the main view block excluding the table body
// (title, subtitle, scroll bars, runtime panel, footer). needsTableHBar and
// needsLogHBar should match whether each horizontal track is shown.
func mainChromeLines(m Model, needsTableHBar bool, needsLogHBar bool) int {
	iw := m.innerWidth()
	n := lipgloss.Height(m.appTitleBlock(iw))
	n += lipgloss.Height(m.styles.subtitle.Render(appSubtitle))
	n += 1

	if needsTableHBar && len(m.files) > 0 {
		if bar := horizontalScrollBarLine(0, iw); bar != "" {
			n += lipgloss.Height(m.styles.footer.Render(bar))
		}
	}

	if needsLogHBar && m.serverRunning {
		if bar := horizontalScrollBarLine(0, iw); bar != "" {
			n += lipgloss.Height(m.styles.footer.Render(bar))
		}
	}

	if m.width > 0 {
		if rp := runtimePanelView(m, iw); rp != "" {
			n += lipgloss.Height(rp)
		}
	}

	n += 1
	n += lipgloss.Height(m.styles.footer.Render(footerHelpLine(m)))

	if m.lastRunNote != "" {
		n += lipgloss.Height(m.styles.errLine.Render(m.lastRunNote))
	}
	return n
}

// portConfigContentWidth is the maximum text width inside the runtime/param modal box.
func (m Model) portConfigContentWidth() int {
	if m.width <= 0 {
		return minInnerWidth
	}
	w := m.width - m.styles.portConfigBox.GetHorizontalFrameSize()
	if w < minInnerWidth {
		return minInnerWidth
	}
	return w
}

// paramPanelContentWidth is the inner width for the parameters modal only. It
// matches portConfigContentWidth on narrow terminals but is capped on wide ones.
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
	if maxW < 4 || m.themeToast == "" {
		return ""
	}
	runes := []rune(m.themeToast)
	for len(runes) > 0 {
		s := string(runes)
		rendered := m.styles.themeToastInline.Render(s)
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
	if m.themeToast == "" {
		return m.styles.title.Render(appTitle)
	}
	left := m.styles.titleBoldLeft.Render(appTitle)
	if lipgloss.Width(left) >= innerW {
		return m.styles.title.Render(appTitle)
	}
	line := m.joinLeftAndToast(innerW, left)
	if line == left {
		return m.styles.title.Render(appTitle)
	}
	return m.styles.titleToastRowWrap.Render(line)
}

// modalTitleRow renders a one-line modal title with an optional same-row theme toast.
func (m Model) modalTitleRow(innerW int, titleStyle lipgloss.Style, plain string) string {
	if m.themeToast == "" {
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
	sub := m.styles.subtitle.Render(appSubtitle)

	var body string
	switch {
	case m.loading:
		body = m.styles.body.Render("Scanning for models…")
	case m.loadErr != nil:
		body = m.styles.errLine.Render("Error: " + m.loadErr.Error())
	case len(m.files) == 0:
		body = m.styles.body.Render("No GGUF or safetensors models found. Set HUGGINGFACE_HUB_CACHE or HF_HOME if your Hub cache is non-default; add paths via LLM_LAUNCH_LLAMACPP_PATHS or place models under ~/models, ~/.cache/huggingface/hub, etc.")
	default:
		m.hscroll.SetContent(m.tbl.View())
		th := m.tableBodyH
		if th < 1 {
			th = defaultTableHeight
		}
		m.hscroll.SetWidth(iw)
		m.hscroll.SetHeight(th)
		if m.serverRunning {
			if m.serverViewportH > 0 {
				m.serverViewport.SetHeight(m.serverViewportH)
			}
			body = lipgloss.JoinVertical(lipgloss.Left, m.hscroll.View(), m.serverLogPaneView())
		} else {
			body = m.hscroll.View()
		}
	}

	var logHBar string
	if m.serverRunning && m.serverLogNeedsHorizontalScroll() {
		if line := horizontalScrollBarLine(m.serverViewport.HorizontalScrollPercent(), iw); line != "" {
			logHBar = m.styles.footer.Render(line)
		}
	}

	var hBar string
	if len(m.files) > 0 && m.tableLineWidth > 0 && m.tableLineWidth > iw {
		pct := m.hscroll.HorizontalScrollPercent()
		hBar = m.styles.footer.Render(horizontalScrollBarLine(pct, iw))
	}

	footer := m.styles.footer.Render(footerHelpLine(m))

	runtimePanel := runtimePanelView(m, iw)

	rows := []string{title, sub, "", body}
	if logHBar != "" {
		rows = append(rows, logHBar)
	}
	if hBar != "" {
		rows = append(rows, hBar)
	}
	if runtimePanel != "" {
		rows = append(rows, runtimePanel)
	}
	rows = append(rows, "", footer)
	if m.lastRunNote != "" {
		rows = append(rows, m.styles.errLine.Render(m.lastRunNote))
	}
	block := lipgloss.JoinVertical(lipgloss.Left, rows...)
	framed := m.styles.app.Render(block)

	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Top, framed)
}

// View implements tea.Model.
func (m Model) View() tea.View {
	if m.width == 0 {
		return tea.NewView("\n  Initializing…\n")
	}
	if m.paramPanelOpen {
		s := overlayCentered(m.mainAppPlacedView(), m.paramPanelModalBlock(), m.width, m.height)
		v := tea.NewView(s)
		v.AltScreen = true
		return v
	}
	if m.runtimeConfigOpen {
		s := overlayCentered(m.mainAppPlacedView(), m.runtimeConfigModalBlock(), m.width, m.height)
		v := tea.NewView(s)
		v.AltScreen = true
		return v
	}

	v := tea.NewView(m.mainAppPlacedView())
	v.AltScreen = true
	return v
}

// runtimeConfigModalBlock returns the framed runtime configuration panel only
// (no full-screen placement). Composed over the main view via [overlayCentered].
func (m Model) runtimeConfigModalBlock() string {
	label := func(focused bool, name string) string {
		prefix := "  "
		if focused {
			prefix = "› "
		}
		return m.styles.body.Render(prefix + name)
	}
	cw := m.portConfigContentWidth()
	rows := []string{
		m.modalTitleRow(cw, m.styles.portConfigTitle, "Runtime environment"),
		"",
		label(m.runtimeFocus == runtimeFieldLlamaCppPath, llamacpp.EnvLlamaCppPath),
		m.runtimeInputs[runtimeFieldLlamaCppPath].View(),
		"",
		label(m.runtimeFocus == runtimeFieldVLLMPath, llamacpp.EnvVLLMPath),
		m.runtimeInputs[runtimeFieldVLLMPath].View(),
		"",
		label(m.runtimeFocus == runtimeFieldVLLMVenv, llamacpp.EnvVLLMVenv),
		m.runtimeInputs[runtimeFieldVLLMVenv].View(),
		"",
		label(m.runtimeFocus == runtimeFieldLlamaPort, llamacpp.EnvLlamaServerPort),
		m.runtimeInputs[runtimeFieldLlamaPort].View(),
		"",
		label(m.runtimeFocus == runtimeFieldVLLMPort, llamacpp.EnvVLLMServerPort),
		m.runtimeInputs[runtimeFieldVLLMPort].View(),
		"",
		m.styles.footer.Render(FooterRuntimeConfigHints),
	}
	block := lipgloss.JoinVertical(lipgloss.Left, rows...)
	if m.lastRunNote != "" {
		block = lipgloss.JoinVertical(lipgloss.Left, block, "", m.styles.errLine.Render(m.lastRunNote))
	}
	return m.styles.portConfigBox.Render(block)
}

// SelectedModel returns the filesystem path and backend for the highlighted row.
func (m Model) SelectedModel() (path string, backend llamacpp.ModelBackend) {
	if len(m.tbl.Rows()) == 0 || m.tbl.Cursor() < 0 {
		return "", llamacpp.BackendLlama
	}
	i := m.tbl.Cursor()
	if i < 0 || i >= len(m.files) {
		return "", llamacpp.BackendLlama
	}
	return m.files[i].Path, m.files[i].Backend
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
