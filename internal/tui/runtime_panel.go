package tui

import (
	"path/filepath"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/flyingnobita/llml/internal/models"
	"github.com/flyingnobita/llml/internal/settings"
)

// Runtime status marks in the runtime panel list, and the legend that explains them.
const (
	runtimeMarkRunning = "●"
	runtimeMarkFound   = "○"
	runtimeMarkMissing = "✗"
	// runtimeMarkOff replaces the status mark of a Disabled Runtime, whose
	// detection result would be stale: it is not probed.
	runtimeMarkOff = "off"
	// runtimeStatusPendingWord heads the detail pane of a Runtime ticked on
	// but not saved yet, which detection skipped: it has no status to show
	// until saving re-runs detection. Its list line shows no mark.
	runtimeStatusPendingWord = "checked on save"
)

// vllmVenvInUse returns the venv root vLLM launches with: the configured root
// when set, otherwise the root inferred by the same rules as vLLM activation
// (adjacent bin layout, $VLLM_PATH/.venv, dirname(vllm)/.venv), or "" when
// none applies.
func vllmVenvInUse(r models.RuntimeInfo) string {
	if v := strings.TrimSpace(r.VLLMVenv); v != "" {
		return v
	}
	vllmBin := r.Status(models.BackendVLLM).Path
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
	body = lipgloss.PlaceVertical(m.runtimePanelBodyHeight(), lipgloss.Top, body)

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

// runtimePanelBodyHeight is the fixed height of the panel's list and detail
// panes: room for the longer of the list and the tallest Runtime's detail
// pane, plus [runtimePanelSpareRows]. Every Runtime counts, not only the ones
// this platform lists, so the panel is the same height on every platform and
// a Runtime with more fields cannot make it jump.
func (m Model) runtimePanelBodyHeight() int {
	h := lipgloss.Height(m.runtimeListPane())
	for _, rt := range runtimeTable {
		h = max(h, runtimeDetailPaneHeight(rt))
	}
	return h + runtimePanelSpareRows
}

// runtimeDetailPaneHeight is how many lines [Model.runtimeDetailPane] draws
// for rt: a header, then a blank line, an input line, and an in-use line per field.
func runtimeDetailPaneHeight(rt runtimeDef) int {
	const linesPerField = 3
	return 1 + linesPerField*len(rt.fields)
}

// runtimeListPane renders the supported Runtimes under the Model Format each
// one runs. A format no supported Runtime runs gets no label.
func (m Model) runtimeListPane() string {
	var lines []string
	rts := m.panelRuntimes()
	for i, rt := range rts {
		if i == 0 || rt.format != rts[i-1].format {
			lines = append(lines, m.ui.styles.runtimeGroupLabel.Render(rt.format.String()))
		}
		lines = append(lines, m.runtimeListRow(rt))
	}
	return m.ui.styles.runtimeListPane.Width(runtimeListPaneWidth).Render(strings.Join(lines, "\n"))
}

// focusMarker is the "›" that marks the one line holding keyboard focus, or
// blank space of the same width.
func focusMarker(focused bool) string {
	if focused {
		return "› "
	}
	return "  "
}

// runtimeListRow renders one Runtime's list line: a focus marker when the list
// has keyboard focus, its checkbox, its name (emphasized when it is the
// highlighted Runtime), and its status mark, or "off" for a Disabled Runtime.
// Names are padded to one width so the marks line up.
func (m Model) runtimeListRow(rt runtimeDef) string {
	highlighted := rt.backend == m.rc.selected
	nameStyle := m.ui.styles.body
	if highlighted {
		nameStyle = m.ui.styles.runtimeListSelected
	}
	name := nameStyle.Render(rt.name) + strings.Repeat(" ", runtimeNameWidth()-lipgloss.Width(rt.name))
	on := m.panelRuntimeEnabled(rt.backend)
	box := checkbox(on)
	var status string
	switch {
	case !on:
		status = " " + m.ui.styles.runtimeMarkOff.Render(runtimeMarkOff)
	case m.runtimeStatusPending(rt.backend):
		// No mark: detection skipped it, so any mark would be stale.
	default:
		sv := m.runtimeStatusView(rt.status(m.runtime))
		status = " " + sv.style.Render(sv.mark)
	}
	return runtimeListRowIndent + focusMarker(highlighted && m.rc.focus == runtimeFieldNone) +
		m.ui.styles.runtimeCheckbox.Render(box) + " " + name + status
}

// runtimeStatusPending reports whether Runtime b, shown on in the panel, was
// skipped by the last detection because it was off. It got no probe, so its
// status is unknown until saving re-runs detection with it on.
func (m Model) runtimeStatusPending(b models.ModelBackend) bool {
	return m.runtime.Skipped.Has(b)
}

// runtimeNameWidth is the display width of the longest Runtime name.
func runtimeNameWidth() int {
	w := 0
	for _, rt := range runtimeTable {
		w = max(w, lipgloss.Width(rt.name))
	}
	return w
}

// runtimeStatusView is how the panel shows one detection status: the list's
// mark, the word the legend and detail header use, and the mark's style.
type runtimeStatusView struct {
	mark  string
	word  string
	style lipgloss.Style
}

// runtimeStatusView returns how the panel shows st.
func (m Model) runtimeStatusView(st runtimeStatus) runtimeStatusView {
	switch {
	case st.running:
		return runtimeStatusView{runtimeMarkRunning, "running", m.ui.styles.runtimeMarkRunning}
	case st.found:
		return runtimeStatusView{runtimeMarkFound, "found", m.ui.styles.runtimeMarkFound}
	default:
		return runtimeStatusView{runtimeMarkMissing, "not found", m.ui.styles.runtimeMarkMissing}
	}
}

// runtimeLegend explains the list's status marks, or says detection is still running.
func (m Model) runtimeLegend() string {
	if !m.runtimeScanned && m.loading {
		return m.ui.styles.runtimeLegend.Render("Detecting runtimes…")
	}
	var parts []string
	for _, st := range []runtimeStatus{{running: true}, {found: true}, {}} {
		sv := m.runtimeStatusView(st)
		parts = append(parts, sv.style.Render(sv.mark)+m.ui.styles.runtimeLegend.Render(" "+sv.word))
	}
	return strings.Join(parts, m.ui.styles.runtimeLegend.Render("  "))
}

// runtimeDetailPane renders the highlighted Runtime's header and fields in
// width columns. Each field is an input line and a dimmed in-use line.
func (m Model) runtimeDetailPane(width int) string {
	rt := runtimeFor(m.rc.selected)
	header := m.ui.styles.bodyBold.Render(rt.name) + m.ui.styles.runtimeInUse.Render(" · ")
	switch {
	case !m.panelRuntimeEnabled(rt.backend):
		// A Disabled Runtime's fields stay editable; the header says editing
		// them does not turn it on.
		header += m.ui.styles.runtimeOffHeader.Render(runtimeMarkOff)
	case m.runtimeStatusPending(rt.backend):
		header += m.ui.styles.runtimeInUse.Render(runtimeStatusPendingWord)
	default:
		header += m.ui.styles.runtimeInUse.Render(m.runtimeStatusView(rt.status(m.runtime)).word)
	}
	lines := []string{header}
	for _, d := range rt.fields {
		lines = append(lines, "", m.runtimeFieldInputLine(d, width), m.runtimeFieldInUseLine(d, width))
	}
	return strings.Join(lines, "\n")
}

// runtimeFieldInputLine renders a field's focus marker, label, and input.
func (m Model) runtimeFieldInputLine(d runtimeFieldDef, width int) string {
	focused := m.rc.focus == d.field
	prefix := focusMarker(focused)
	labelStyle := m.ui.styles.runtimeFieldLabel
	if focused {
		labelStyle = m.ui.styles.runtimeFieldLabelFocused
	}
	in := m.rc.inputs[d.field]
	if d.port == nil {
		// textinput.View adds the prompt and one cell for the cursor.
		const cursorCell = 1
		in.SetWidth(max(width-lipgloss.Width(prefix)-runtimeFieldLabelWidth-lipgloss.Width(in.Prompt)-cursorCell, 1))
	}
	return m.ui.styles.body.Render(prefix) + labelStyle.Width(runtimeFieldLabelWidth).Render(d.label) + in.View()
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
	value = truncateLeft(value, max(room, 0))
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
// from. A program found anywhere but the configured path reads "detected".
func (m Model) runtimeFieldInUse(d runtimeFieldDef) (string, fieldSource) {
	src := m.settings.Source(d.setting)
	out := fieldSource{label: src.String(), env: src.Origin == settings.OriginEnv}
	if d.inUse == nil {
		return d.value(m.settings), out
	}
	p := d.inUse(m.runtime)
	if p == "" {
		return "not found", out
	}
	if !pathWithin(p, *d.str(&m.settings)) {
		out.label = "detected"
	}
	return FormatPathDisplay(p, m.layout.homeDir), out
}

// pathWithin reports whether p is root or lies under it. An empty root
// contains nothing.
func pathWithin(p, root string) bool {
	if root == "" {
		return false
	}
	return p == root || strings.HasPrefix(p, strings.TrimSuffix(root, string(filepath.Separator))+string(filepath.Separator))
}

// truncateLeft shortens s to w display columns by dropping its start, so a
// long path keeps the program name at its end.
func truncateLeft(s string, w int) string {
	over := lipgloss.Width(s) - w
	if over <= 0 {
		return s
	}
	return ansi.TruncateLeft(s, over+1, "…")
}
