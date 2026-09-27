package tui

import (
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/flyingnobita/llml/internal/models"
	"github.com/flyingnobita/llml/internal/settings"
)

// Runtime status marks in the runtime panel list, and the legend that explains them.
const (
	runtimeMarkRunning = "●"
	runtimeMarkFound   = "○"
	runtimeMarkMissing = "✗"
)

// vllmVenvInUse returns the venv root vLLM launches with: the configured root
// when set, otherwise the root inferred by the same rules as vLLM activation
// (adjacent bin layout, $VLLM_PATH/.venv, dirname(vllm)/.venv), or "" when
// none applies.
func vllmVenvInUse(r models.RuntimeInfo) string {
	if v := strings.TrimSpace(r.VLLMVenv); v != "" {
		return v
	}
	vllmBin := models.ResolveVLLMPath(r)
	act := models.ResolveVLLMActivateScript(vllmBin, r.VLLMVenv, r.VLLMConfiguredPath)
	return models.VenvRootFromActivateScript(act)
}

// panelRuntimes returns the Runtimes this platform supports, in list order.
func (m Model) panelRuntimes() []runtimeDef {
	var out []runtimeDef
	for _, rt := range runtimeTable {
		if rt.supported(m.runtime.Platform) {
			out = append(out, rt)
		}
	}
	return out
}

// runtimeConfigModalBlock returns the framed runtime panel only (no
// full-screen placement). Composed over the main view via [overlayCentered].
func (m Model) runtimeConfigModalBlock() string {
	if m.rc.discardConfirm {
		return m.runtimeConfigDiscardConfirmBlock()
	}
	cw := m.paramPanelContentWidth()
	list := m.runtimeListPane()
	gap := strings.Repeat(" ", runtimePanelPaneGap)
	detail := m.runtimeDetailPane(cw - runtimeListPaneWidth - runtimePanelPaneGap)
	body := lipgloss.JoinHorizontal(lipgloss.Top, list, gap, detail)

	rows := []string{
		m.modalTitleRow(cw, m.ui.styles.portConfigTitle, "Runtime Environment"),
		"",
		body,
		"",
		m.runtimeLegend(),
		m.ui.styles.subtitle.Width(cw).Render(runtimeConfigModalSubtitle),
		m.renderFooterHints(FooterRuntimeConfigHints),
	}
	return m.ui.styles.portConfigBox.Render(lipgloss.JoinVertical(lipgloss.Left, rows...))
}

// runtimeListPane renders the supported Runtimes under the Model Format each
// one runs. A format no supported Runtime runs gets no label.
func (m Model) runtimeListPane() string {
	var lines []string
	var group modelFormat = -1
	for _, rt := range m.panelRuntimes() {
		if rt.format != group {
			group = rt.format
			lines = append(lines, m.ui.styles.runtimeGroupLabel.Render(rt.format.String()))
		}
		lines = append(lines, m.runtimeListRow(rt))
	}
	return m.ui.styles.runtimeListPane.Width(runtimeListPaneWidth).Render(strings.Join(lines, "\n"))
}

// runtimeListRow renders one Runtime's list line: a focus marker when the list
// has keyboard focus, its status mark, and its name, emphasized when it is the
// highlighted Runtime.
func (m Model) runtimeListRow(rt runtimeDef) string {
	prefix := "  "
	highlighted := rt.backend == m.rc.selected
	if highlighted && m.rc.focus == runtimeFieldNone {
		prefix = "› "
	}
	name := m.ui.styles.body.Render(rt.name)
	if highlighted {
		name = m.ui.styles.runtimeListSelected.Render(rt.name)
	}
	return " " + prefix + m.runtimeStatusMark(rt.status(m.runtime)) + " " + name
}

// runtimeStatusMark renders the mark for what detection learned.
func (m Model) runtimeStatusMark(st runtimeStatus) string {
	switch {
	case st.running:
		return m.ui.styles.runtimeMarkRunning.Render(runtimeMarkRunning)
	case st.found:
		return m.ui.styles.runtimeMarkFound.Render(runtimeMarkFound)
	default:
		return m.ui.styles.runtimeMarkMissing.Render(runtimeMarkMissing)
	}
}

// runtimeStatusWord names a status in the detail pane header.
func runtimeStatusWord(st runtimeStatus) string {
	switch {
	case st.running:
		return "running"
	case st.found:
		return "found"
	default:
		return "not found"
	}
}

// runtimeLegend explains the list's status marks, or says detection is still running.
func (m Model) runtimeLegend() string {
	if !m.runtimeScanned && m.loading {
		return m.ui.styles.runtimeLegend.Render("Detecting runtimes…")
	}
	st := m.ui.styles
	return st.runtimeMarkRunning.Render(runtimeMarkRunning) + st.runtimeLegend.Render(" running  ") +
		st.runtimeMarkFound.Render(runtimeMarkFound) + st.runtimeLegend.Render(" found  ") +
		st.runtimeMarkMissing.Render(runtimeMarkMissing) + st.runtimeLegend.Render(" not found")
}

// runtimeDetailPane renders the highlighted Runtime's header and fields in
// width columns. Each field is an input line and a dimmed in-use line.
func (m Model) runtimeDetailPane(width int) string {
	rt := runtimeFor(m.rc.selected)
	header := m.ui.styles.bodyBold.Render(rt.name) +
		m.ui.styles.runtimeInUse.Render(" · "+runtimeStatusWord(rt.status(m.runtime)))
	lines := []string{header}
	for _, d := range rt.fields {
		lines = append(lines, "", m.runtimeFieldInputLine(d, width), m.runtimeFieldInUseLine(d, width))
	}
	return strings.Join(lines, "\n")
}

// runtimeFieldInputLine renders a field's focus marker, label, and input.
func (m Model) runtimeFieldInputLine(d runtimeFieldDef, width int) string {
	focused := m.rc.focus == d.field
	prefix := "  "
	label := m.ui.styles.runtimeFieldLabel.Render(padRight(d.label, runtimeFieldLabelWidth))
	if focused {
		prefix = "› "
		label = m.ui.styles.bodyBold.Render(padRight(d.label, runtimeFieldLabelWidth))
	}
	in := m.rc.inputs[d.field]
	if d.port == nil {
		// textinput.View adds the prompt and one cell for the cursor.
		in.SetWidth(max(width-len(prefix)-runtimeFieldLabelWidth-lipgloss.Width(in.Prompt)-1, 1))
	}
	return m.ui.styles.body.Render(prefix) + label + in.View()
}

// runtimeFieldInUseLine renders "in use: <value> (<source>)" for a field. The
// value is truncated to fit width, and the source is never cut; an
// environment source is shown in the warning colour, since a saved value
// does not take effect while the variable is set.
func (m Model) runtimeFieldInUseLine(d runtimeFieldDef, width int) string {
	value, src := m.runtimeFieldInUse(d)
	indent := strings.Repeat(" ", runtimeInUseIndent)
	tag := " (" + src.label + ")"
	lead := "in use: "
	room := width - runtimeInUseIndent - len(lead) - lipgloss.Width(tag)
	value = truncateLeft(value, room)
	tagStyle := m.ui.styles.runtimeInUse
	if src.env {
		tagStyle = m.ui.styles.runtimeEnvSource
	}
	return indent + m.ui.styles.runtimeInUse.Render(lead+value) + tagStyle.Render(tag)
}

// fieldSource is where a field's in-use value came from, as the panel names it.
type fieldSource struct {
	label string
	env   bool
}

// runtimeFieldInUse returns the value llml runs with for d and where it came
// from. A path found by detection with nothing configured reads "detected".
func (m Model) runtimeFieldInUse(d runtimeFieldDef) (string, fieldSource) {
	src := m.settings.Source(d.setting)
	out := fieldSource{label: src.String(), env: src.Origin == settings.OriginEnv}
	if d.inUse == nil {
		return d.value(m.settings), out
	}
	p := d.inUse(m.runtime)
	if p == "" {
		return "—", out
	}
	if src.Origin == settings.OriginDefault {
		out.label = "detected"
	}
	return FormatPathDisplay(p, m.layout.homeDir), out
}

// padRight pads s with spaces to w display columns.
func padRight(s string, w int) string {
	if n := lipgloss.Width(s); n < w {
		return s + strings.Repeat(" ", w-n)
	}
	return s
}

// truncateLeft shortens s to w display columns by dropping its start, so a
// long path keeps the program name at its end.
func truncateLeft(s string, w int) string {
	if lipgloss.Width(s) <= w {
		return s
	}
	if w < 2 {
		return ""
	}
	r := []rune(s)
	for len(r) > 0 && lipgloss.Width(string(r))+1 > w {
		r = r[1:]
	}
	return "…" + string(r)
}
