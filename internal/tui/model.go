package tui

import (
	"os"
	"os/exec"
	"strings"
	"time"

	btable "charm.land/bubbles/v2/table"
	"charm.land/bubbles/v2/textinput"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"charm.land/lipgloss/v2/compat"
	"github.com/charmbracelet/x/ansi"
	"github.com/flyingnobita/llml/internal/models"
)

// layoutState holds terminal geometry and derived table dimensions.
type layoutState struct {
	width             int
	height            int
	bodyInnerW        int
	tableBodyH        int
	tableLineWidth    int
	tableNeedsHScroll bool // true when tableContentMinWidth exceeds inner body width
	homeDir           string
}

// themeState holds visual theme, style set, and transient toast text.
type themeState struct {
	theme      Theme
	themePick  int
	themeToast string
	styles     styles
}

// tableState holds the file list, sort state, table component, and scroll viewport.
type tableState struct {
	tbl      btable.Model
	hscroll  viewport.Model
	files    []models.ModelFile
	sortCol  tableSortCol // default Path ascending matches models.Discover order
	sortDesc bool         // false = ascending
	lastScan time.Time
}

// runtimeConfigState holds the runtime-config modal's open/focus/input state.
type runtimeConfigState struct {
	open   bool
	focus  runtimeField
	inputs [runtimeFieldCount]textinput.Model
}

// paramsState holds the parameter-profiles panel's state.
type paramsState struct {
	open             bool
	confirmDelete    paramConfirm
	modelPath        string
	modelDisplayName string
	focus            paramFocus
	profileIndex     int
	profiles         []ParameterProfile
	envCursor        int
	argsCursor       int
	env              []EnvVar
	args             []string
	editKind         paramEditKind
	editInput        textinput.Model
}

// serverPaneState holds the split-pane server subprocess and log viewport.
type serverPaneState struct {
	running       bool
	exited        bool // true after the process exits; split pane stays until dismissSplitServer.
	cmd           *exec.Cmd
	msgCh         chan tea.Msg
	log           []string
	logAlignWidth int // measured prefix width for split-pane log alignment (vLLM vs tqdm)
	viewport      viewport.Model
	viewportH     int
	splitFocused  bool // true: keys scroll log; false: keys use model table (Tab toggles).
}

// launchPreviewState holds the launch-command preview viewport below the table.
type launchPreviewState struct {
	viewport viewport.Model
	focused  bool   // idle only: Tab toggles with table whenever the preview is visible
	lastCmd  string // resets scroll when the displayed command changes
}

// Model is the root Bubble Tea model.
type Model struct {
	layout  layoutState
	ui      themeState
	table   tableState
	rc      runtimeConfigState
	params  paramsState
	server  serverPaneState
	preview launchPreviewState

	keys               KeyMap
	runtime            models.RuntimeInfo
	runtimeScanned     bool
	lastRunNote        string
	lastRunNoteSuccess bool // true: lastRunNote is non-error feedback (e.g. copy confirmation)
	loading            bool
	loadErr            error
	helpOpen           bool // keyboard shortcuts popup
}

// New returns a model with default key bindings and an empty table; Init triggers discovery.
func New() Model {
	homeDir, _ := os.UserHomeDir()
	pick := initialThemePick()
	th := themeFromPick(pick, compat.HasDarkBackground)
	st := newStyles(th)
	t := btable.New(
		btable.WithColumns(tableColumns(100, nil, homeDir, defaultSortCol, false)),
		btable.WithRows(nil),
		btable.WithFocused(true),
		btable.WithStyles(st.table),
		btable.WithWidth(96),
		btable.WithHeight(defaultTableHeight),
	)
	hv := viewport.New(viewport.WithWidth(96), viewport.WithHeight(defaultTableHeight))
	hv.SetHorizontalStep(hScrollStep)
	hv.Style = st.splitPaneChromeFocused
	sv := viewport.New(viewport.WithWidth(96), viewport.WithHeight(1))
	sv.MouseWheelEnabled = true
	sv.Style = st.serverLogViewport
	lpvOuter := launchPreviewVisibleLines + st.launchPreviewViewport.GetVerticalFrameSize()
	lpv := viewport.New(viewport.WithWidth(96), viewport.WithHeight(lpvOuter))
	lpv.MouseWheelEnabled = true
	lpv.MouseWheelDelta = 1
	lpv.SoftWrap = true
	lpv.Style = st.launchPreviewViewport
	return Model{
		layout: layoutState{
			homeDir: homeDir,
		},
		ui: themeState{
			theme:     th,
			themePick: pick,
			styles:    st,
		},
		table: tableState{
			sortCol: defaultSortCol,
			tbl:     t,
			hscroll: hv,
		},
		server: serverPaneState{
			viewport: sv,
		},
		preview: launchPreviewState{
			viewport: lpv,
		},
		rc: runtimeConfigState{
			inputs: [runtimeFieldCount]textinput.Model{
				newPathTextInput(),
				newPathTextInput(),
				newPathTextInput(),
				newPortTextInput(),
				newPortTextInput(),
			},
		},
		params: paramsState{
			editInput: newParamLineTextInput(),
		},
		keys:    DefaultKeyMap(),
		loading: true,
	}
}

// Init implements tea.Model.
func (m Model) Init() tea.Cmd {
	return startupCmd()
}

// innerWidth returns the usable inner body width for rendering. It falls back
// to a computed value when bodyInnerW has not yet been set by layoutTable.
func (m Model) innerWidth() int {
	if m.layout.bodyInnerW >= 1 {
		return m.layout.bodyInnerW
	}
	if m.layout.width > 0 {
		return max(m.layout.width-appPaddingH*2, minInnerWidth)
	}
	return minInnerWidth
}

func maxAnsiLineWidth(lines []string) int {
	max := 0
	for _, line := range lines {
		if w := ansi.StringWidth(line); w > max {
			max = w
		}
	}
	return max
}

// serverLogNeedsHorizontalScroll reports whether any log line is wider than the
// viewport's inner content width (after border and optional vertical track).
func (m Model) serverLogNeedsHorizontalScroll() bool {
	if !m.server.running || len(m.server.log) == 0 {
		return false
	}
	inner := m.server.viewport.Width() - m.server.viewport.Style.GetHorizontalFrameSize()
	if inner < 1 {
		return false
	}
	return maxAnsiLineWidth(m.server.log) > inner
}

// splitServerBodyHeights divides total body rows between the model table (top) and server log viewport (bottom).
func splitServerBodyHeights(total int) (tableH, logH int) {
	sep := serverLogSeparatorLines
	if total <= sep {
		return 1, 1
	}
	rest := total - sep
	tableH = rest / 2
	logH = rest - tableH
	if tableH < 1 {
		tableH = 1
	}
	if logH < 1 {
		logH = 1
	}
	return tableH, logH
}

func tableRowAreaHeight(contentAreaH int) int {
	if contentAreaH <= 1 {
		return 1
	}
	// bubbles/table adds a header row above body rows.
	return contentAreaH - 1
}

func (m Model) layoutTable() Model {
	w := m.layout.width
	if w < minTerminalWidth {
		w = minTerminalWidth
	}
	innerW := m.layout.width - appPaddingH*2
	if innerW < minInnerWidth {
		innerW = w - appPaddingH*2
	}
	m.layout.bodyInnerW = innerW
	// Column widths must use the same budget as the table viewport (inner body
	// width). Using full terminal width here made rows ~4 cells wider than
	// innerW and triggered empty horizontal scrolling.
	cols := tableColumns(innerW, m.table.files, m.layout.homeDir, m.table.sortCol, m.table.sortDesc)
	m.table.tbl.SetColumns(cols)
	m.table.tbl.SetStyles(m.ui.styles.table)
	minW := tableContentMinWidth(cols)
	m.table.tbl.SetWidth(max(minW, innerW))
	// Column widths do not change when only header labels (sort indicators) change; using
	// minW keeps the horizontal scroll bar row and table body height stable.
	m.layout.tableNeedsHScroll = len(m.table.files) > 0 && minW > innerW

	// Determine log h-bar without a heuristic: content wider than viewport inner width → bar shown.
	// Uses the style frame size directly so no second pass is needed.
	logFrameH := m.server.viewport.Style.GetHorizontalFrameSize()
	needsLogHBar := m.server.running && maxAnsiLineWidth(m.server.log) > max(1, innerW-logFrameH)

	previewH := m.launchPreviewPaneLayoutHeight()
	h := m.computeBodyHeight(needsLogHBar)
	m = m.applyTableAndLogHeights(h, innerW, previewH)

	m.table.tbl.SetRows(buildTableRows(m.table.files, cols, m.layout.homeDir))
	tview := m.table.tbl.View()
	m.layout.tableBodyH = max(1, strings.Count(tview, "\n")+1)
	lines := strings.Split(tview, "\n")
	if len(lines) > 0 {
		m.layout.tableLineWidth = lipgloss.Width(lines[0])
	} else {
		m.layout.tableLineWidth = 0
	}

	m.table.hscroll.SetContent(tview)
	m.table.hscroll.SetWidth(innerW)
	m.table.hscroll.SetHeight(m.layout.tableBodyH)

	m = m.syncLaunchPreviewViewport(innerW)
	m = m.applyMainPaneFocusStyles()
	return m
}

// computeBodyHeight returns the total body rows available for the table + log panes.
func (m Model) computeBodyHeight(needsLogHBar bool) int {
	if m.layout.height <= 0 {
		return defaultTableHeight
	}
	// Bubble Tea keeps only the bottom m.layout.height lines if the view is taller;
	// size the table so framed (padding + chrome + body) fits.
	appPad := m.ui.styles.app.GetVerticalFrameSize()
	innerMax := m.layout.height - appPad
	if innerMax < 1 {
		innerMax = 1
	}
	h := innerMax - mainChromeLines(m, m.layout.tableNeedsHScroll, needsLogHBar)
	if h < 1 {
		h = 1
	}
	return h
}

// applyTableAndLogHeights sets table and server-log viewport dimensions from bodyH.
func (m Model) applyTableAndLogHeights(bodyH, innerW, previewH int) Model {
	tableFrameV := m.table.hscroll.Style.GetVerticalFrameSize()
	logFrameV := m.server.viewport.Style.GetVerticalFrameSize()
	if m.server.running {
		rest := bodyH - previewH
		if rest < 2 {
			// Need at least one line each for table and log; may exceed bodyH on tiny terminals.
			rest = 2
		}
		tablePaneH, logPaneH := splitServerBodyHeights(rest)
		tableContentH := tablePaneH - tableFrameV
		if tableContentH < 1 {
			tableContentH = 1
		}
		logContentH := logPaneH - logFrameV
		if logContentH < 1 {
			logContentH = 1
		}
		m.table.tbl.SetHeight(tableRowAreaHeight(tableContentH))
		m.server.viewport.SetHeight(logContentH)
		m.server.viewport.SetWidth(innerW)
		if m.server.viewport.TotalLineCount() > m.server.viewport.VisibleLineCount() {
			m.server.viewport.SetWidth(innerW - 1)
		}
		m.server.viewportH = logContentH
	} else {
		tablePaneH := bodyH - previewH
		if tablePaneH < 1 {
			tablePaneH = 1
		}
		tableContentH := tablePaneH - tableFrameV
		if tableContentH < 1 {
			tableContentH = 1
		}
		m.table.tbl.SetHeight(tableRowAreaHeight(tableContentH))
		m.server.viewport.SetWidth(innerW)
		m.server.viewport.SetHeight(1)
		m.server.viewportH = 0
	}
	return m
}

// launchPreviewPaneLayoutHeight returns vertical rows consumed by the launch command preview
// (margin + bordered viewport) when models are listed.
func (m Model) launchPreviewPaneLayoutHeight() int {
	if !launchPreviewVisible(m) {
		return 0
	}
	// MarginTop(1) on [styles.launchPreview] plus the fixed-height bordered viewport.
	return m.ui.styles.launchPreview.GetMarginTop() +
		m.ui.styles.launchPreviewViewport.GetVerticalFrameSize() +
		launchPreviewVisibleLines
}

// syncLaunchPreviewViewport sets viewport dimensions and wrapped content from the selected row.
func (m Model) syncLaunchPreviewViewport(innerW int) Model {
	if innerW < minInnerWidth {
		innerW = minInnerWidth
	}
	if !launchPreviewVisible(m) {
		m.preview.viewport.SetContent("")
		m.preview.lastCmd = ""
		return m
	}
	cmd := launchPreviewCommandLine(m)
	if cmd != m.preview.lastCmd {
		m.preview.viewport.GotoTop()
		m.preview.lastCmd = cmd
	}
	fr := m.preview.viewport.Style.GetHorizontalFrameSize()
	textW := innerW - fr
	if textW < 8 {
		textW = 8
	}
	pvFrV := m.preview.viewport.Style.GetVerticalFrameSize()
	outerH := launchPreviewVisibleLines + pvFrV

	m.preview.viewport.SetWidth(innerW)
	rendered := m.ui.styles.launchPreviewContent.Width(textW).Render(cmd)
	m.preview.viewport.SetContent(rendered)
	m.preview.viewport.SetHeight(outerH)
	if m.preview.viewport.TotalLineCount() > m.preview.viewport.VisibleLineCount() {
		m.preview.viewport.SetWidth(innerW - 1)
		textW = innerW - 1 - fr
		if textW < 8 {
			textW = 8
		}
		rendered = m.ui.styles.launchPreviewContent.Width(textW).Render(cmd)
		m.preview.viewport.SetContent(rendered)
		m.preview.viewport.SetHeight(outerH)
	}
	return m
}

// withLaunchPreviewSynced refreshes the launch preview after table input without a full layout pass.
func (m Model) withLaunchPreviewSynced() Model {
	iw := m.layout.bodyInnerW
	if iw < 1 {
		iw = m.innerWidth()
	}
	return m.syncLaunchPreviewViewport(iw)
}

// applyMainPaneFocusStyles sets table vs launch-preview chrome when idle, or delegates to
// [Model.applySplitPaneFocusStyles] when a split-pane server is running.
func (m Model) applyMainPaneFocusStyles() Model {
	if m.server.running {
		m = m.applySplitPaneFocusStyles()
		if m.preview.focused {
			m.preview.viewport.Style = m.ui.styles.splitPaneChromeFocused
		} else {
			m.preview.viewport.Style = m.ui.styles.launchPreviewViewport
		}
		return m
	}
	if m.preview.focused {
		m.table.hscroll.Style = m.ui.styles.splitPaneChromeDim
		m.preview.viewport.Style = m.ui.styles.splitPaneChromeFocused
	} else {
		m.table.hscroll.Style = m.ui.styles.splitPaneChromeFocused
		m.preview.viewport.Style = m.ui.styles.launchPreviewViewport
	}
	return m
}

// applySplitPaneFocusStyles sets rounded borders on the table scroll viewport and
// the server log viewport. When the server is not running, the table uses focused
// chrome (single main pane); the idle log strip uses the default serverLogViewport
// style. When the server is running, the keyboard-focused split pane uses
// SplitPaneBorderFocused and the other SplitPaneBorderDim.
func (m Model) applySplitPaneFocusStyles() Model {
	if !m.server.running {
		m.table.hscroll.Style = m.ui.styles.splitPaneChromeFocused
		m.server.viewport.Style = m.ui.styles.serverLogViewport
		return m
	}
	if m.server.splitFocused {
		m.table.hscroll.Style = m.ui.styles.splitPaneChromeDim
		m.server.viewport.Style = m.ui.styles.splitPaneChromeFocused
	} else if m.preview.focused {
		m.table.hscroll.Style = m.ui.styles.splitPaneChromeDim
		m.server.viewport.Style = m.ui.styles.splitPaneChromeDim
	} else {
		m.table.hscroll.Style = m.ui.styles.splitPaneChromeFocused
		m.server.viewport.Style = m.ui.styles.splitPaneChromeDim
	}
	return m
}

// appendServerLogLine appends a log line for split-pane server output and refreshes the log viewport.
func (m Model) appendServerLogLine(line string) Model {
	align := m.server.logAlignWidth
	line = normalizeSplitServerLogLine(line, &align)
	m.server.logAlignWidth = align
	m.server.log = append(m.server.log, line)
	if len(m.server.log) > maxServerLogLines {
		m.server.log = m.server.log[len(m.server.log)-maxServerLogLines:]
	}
	m.server.viewport.SetContent(strings.Join(m.server.log, "\n"))
	m.server.viewport.GotoBottom()
	return m
}

// cycleTheme advances dark → light → auto → dark, rebuilds lipgloss styles, and
// shows a short toast on the title row naming the active mode.
func (m Model) cycleTheme() (Model, tea.Cmd) {
	m.ui.themePick = (m.ui.themePick + 1) % themePickCount
	m.ui.theme = themeFromPick(m.ui.themePick, compat.HasDarkBackground)
	m.ui.styles = newStyles(m.ui.theme)
	m.ui.themeToast = themeToastText(m.ui.themePick, m.ui.theme)
	m.preview.viewport.Style = m.ui.styles.launchPreviewViewport
	m = m.layoutTable()
	return m, clearThemeToastAfterCmd()
}

// withLastRunError sets a red status line below the footer (see lastRunNoteView).
func (m Model) withLastRunError(msg string) Model {
	m.lastRunNote = msg
	m.lastRunNoteSuccess = false
	return m
}

// withLastRunSuccess sets a non-error status line below the footer.
func (m Model) withLastRunSuccess(msg string) Model {
	m.lastRunNote = msg
	m.lastRunNoteSuccess = true
	return m
}

// withLastRunCleared removes the footer status line.
func (m Model) withLastRunCleared() Model {
	m.lastRunNote = ""
	m.lastRunNoteSuccess = false
	return m
}

// dismissSplitServer clears split-pane server state after the user dismisses the
// log (enter/esc/q) or tears down the UI after a non-split llamaServerExitedMsg.
func (m Model) dismissSplitServer() Model {
	m.server.running = false
	m.server.exited = false
	m.server.splitFocused = false
	m.preview.focused = false
	m.server.cmd = nil
	m.server.msgCh = nil
	m.server.log = nil
	m.server.logAlignWidth = 0
	m.server.viewport.SetContent("")
	m.table.tbl.Focus()
	m = m.layoutTable()
	return m
}
