package tui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/flyingnobita/llml/internal/llamacpp"
)

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
	return fmt.Sprintf(
		"%s: %s · %s: %s · %s: %s · %s: %s · %s: %s · %s: %s · %s: %s · %s: %s · %d×%d",
		m.keys.Refresh.Help().Key, m.keys.Refresh.Help().Desc,
		m.keys.RunServer.Help().Key, m.keys.RunServer.Help().Desc,
		m.keys.ConfigPort.Help().Key, m.keys.ConfigPort.Help().Desc,
		m.keys.Parameters.Help().Key, m.keys.Parameters.Help().Desc,
		m.keys.ToggleTheme.Help().Key, m.keys.ToggleTheme.Help().Desc,
		m.keys.Quit.Help().Key, m.keys.Quit.Help().Desc,
		m.keys.Nav.Help().Key, m.keys.Nav.Help().Desc,
		m.keys.CopyPath.Help().Key, m.keys.CopyPath.Help().Desc,
		m.width, m.height,
	)
}

// mainChromeLines counts rows in the main view block excluding the table body
// (title, subtitle, scroll bar, runtime panel, footer). needsHBar should match
// whether the horizontal scroll track is shown.
func mainChromeLines(m Model, needsHBar bool) int {
	iw := m.innerWidth()
	n := lipgloss.Height(m.appTitleBlock(iw))
	n += lipgloss.Height(m.styles.subtitle.Render(appSubtitle))
	n += 1

	if needsHBar && len(m.files) > 0 {
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

// View implements tea.Model.
func (m Model) View() tea.View {
	if m.width == 0 {
		return tea.NewView("\n  Initializing…\n")
	}
	if m.paramPanelOpen {
		return tea.NewView(m.paramPanelView())
	}
	if m.runtimeConfigOpen {
		return tea.NewView(m.runtimeConfigView())
	}

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
		body = m.hscroll.View()
	}

	var hBar string
	if len(m.files) > 0 && m.tableLineWidth > 0 && m.tableLineWidth > iw {
		pct := m.hscroll.HorizontalScrollPercent()
		hBar = m.styles.footer.Render(horizontalScrollBarLine(pct, iw))
	}

	footer := m.styles.footer.Render(footerHelpLine(m))

	runtimePanel := runtimePanelView(m, iw)

	rows := []string{title, sub, "", body}
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

	v := tea.NewView(lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Top, framed))
	v.AltScreen = true
	return v
}

func (m Model) runtimeConfigView() string {
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
		m.styles.footer.Render("tab: next · shift+tab: prev · enter: save · esc: cancel"),
	}
	block := lipgloss.JoinVertical(lipgloss.Left, rows...)
	if m.lastRunNote != "" {
		block = lipgloss.JoinVertical(lipgloss.Left, block, "", m.styles.errLine.Render(m.lastRunNote))
	}
	framed := m.styles.portConfigBox.Render(block)
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, framed)
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
