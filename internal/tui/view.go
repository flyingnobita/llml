package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/flyingnobita/llm-launch/internal/llamacpp"
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
		lines := llamacpp.RuntimePanelLines(contentWidth)
		block = strings.Join(lines, "\n")
	}
	inner := "Runtimes\n" + block
	return runtimePanelStyle.Width(contentWidth).Render(inner)
}

const appTitle = "llm-launch"

// View implements tea.Model.
func (m Model) View() string {
	if m.width == 0 {
		return "\n  Initializing…\n"
	}
	if m.paramPanelOpen {
		return m.paramPanelView()
	}
	if m.runtimeConfigOpen {
		return m.runtimeConfigView()
	}

	title := titleStyle.Render(appTitle)
	sub := subtitleStyle.Render(appSubtitle)
	iw := m.innerWidth()

	var body string
	switch {
	case m.loading:
		body = bodyStyle.Render("Scanning for models…")
	case m.loadErr != nil:
		body = errorStyle.Render("Error: " + m.loadErr.Error())
	case len(m.files) == 0:
		body = bodyStyle.Render("No GGUF or safetensors models found. Set HUGGINGFACE_HUB_CACHE or HF_HOME if your Hub cache is non-default; add paths via LLM_LAUNCH_LLAMACPP_PATHS or place models under ~/models, ~/.cache/huggingface/hub, etc.")
	default:
		m.hscroll.SetContent(m.tbl.View())
		th := m.tableBodyH
		if th < 1 {
			th = defaultTableHeight
		}
		m.hscroll.Width = iw
		m.hscroll.Height = th
		body = m.hscroll.View()
	}

	var hBar string
	if len(m.files) > 0 && m.tableLineWidth > 0 && m.tableLineWidth > iw {
		pct := m.hscroll.HorizontalScrollPercent()
		hBar = footerStyle.Render(horizontalScrollBarLine(pct, iw))
	}

	help := fmt.Sprintf(
		"%s %s · %s %s · %s %s · %s %s · %s %s · ↑/↓ · %s %s · %s %s · %d×%d",
		m.keys.Refresh.Help().Key, m.keys.Refresh.Help().Desc,
		m.keys.RunServer.Help().Key, m.keys.RunServer.Help().Desc,
		m.keys.ConfigPort.Help().Key, m.keys.ConfigPort.Help().Desc,
		m.keys.Parameters.Help().Key, m.keys.Parameters.Help().Desc,
		m.keys.Quit.Help().Key, m.keys.Quit.Help().Desc,
		m.keys.CopyPath.Help().Key, m.keys.CopyPath.Help().Desc,
		m.keys.ScrollLeft.Help().Key, m.keys.ScrollLeft.Help().Desc,
		m.width, m.height,
	)
	footer := footerStyle.Render(help)

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
		rows = append(rows, errorStyle.Render(m.lastRunNote))
	}
	block := lipgloss.JoinVertical(lipgloss.Left, rows...)
	framed := app.Render(block)

	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, framed)
}

func (m Model) runtimeConfigView() string {
	label := func(focused bool, name string) string {
		prefix := "  "
		if focused {
			prefix = "› "
		}
		return bodyStyle.Render(prefix + name)
	}
	rows := []string{
		portConfigTitleStyle.Render("Runtime environment"),
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
		footerStyle.Render("tab next · shift+tab prev · ⏎ save · esc cancel · ctrl+c quit"),
	}
	block := lipgloss.JoinVertical(lipgloss.Left, rows...)
	if m.lastRunNote != "" {
		block = lipgloss.JoinVertical(lipgloss.Left, block, "", errorStyle.Render(m.lastRunNote))
	}
	framed := portConfigBoxStyle.Render(block)
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
