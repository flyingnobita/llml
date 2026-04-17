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

// Model is the root Bubble Tea model.
type Model struct {
	width              int
	height             int
	bodyInnerW         int
	tableBodyH         int
	tableLineWidth     int
	tableNeedsHScroll  bool // true when [tableContentMinWidth] exceeds inner body width; used for chrome + View (not rendered line width, so header glyphs cannot shift layout).
	theme              Theme
	themePick          int
	themeToast         string
	styles             styles
	keys               KeyMap
	tbl                btable.Model
	hscroll            viewport.Model
	files              []models.ModelFile
	sortCol            tableSortCol // default Path ascending matches [models.Discover] order
	sortDesc           bool         // false = ascending
	runtime            models.RuntimeInfo
	runtimeScanned     bool
	lastRunNote        string
	lastRunNoteSuccess bool // true: lastRunNote is non-error feedback (e.g. copy confirmation)
	loading            bool
	loadErr            error
	runtimeConfigOpen  bool
	runtimeFocus       runtimeField
	runtimeInputs      [runtimeFieldCount]textinput.Model

	paramPanelOpen        bool
	paramConfirmDelete    paramConfirm
	paramModelPath        string
	paramModelDisplayName string
	paramFocus            paramFocus
	paramProfileIndex     int
	paramProfiles         []ParameterProfile
	paramEnvCursor        int
	paramArgsCursor       int
	paramEnv              []EnvVar
	paramArgs             []string
	paramEditKind         paramEditKind
	paramEditInput        textinput.Model

	homeDir string // from [os.UserHomeDir] at startup; used for path display (~/)

	// lastScan is the timestamp of the last full model filesystem scan written to config.toml.
	lastScan time.Time

	// Split-pane server (R): subprocess logs in lower half; see run_server.go.
	serverRunning       bool
	serverExited        bool // true after the process exits; split pane stays until [dismissSplitServer].
	serverCmd           *exec.Cmd
	serverMsgCh         chan tea.Msg
	serverLog           []string
	serverLogAlignWidth int // measured prefix width for split-pane log alignment (vLLM vs tqdm)
	serverViewport      viewport.Model
	serverViewportH     int
	splitLogFocused     bool // true: keys scroll log; false: keys use model table (Tab toggles).

	// Launch command preview below the table: fixed-height scrollable viewport; see view.go.
	launchPreviewViewport viewport.Model
	launchPreviewFocused  bool   // idle only: Tab toggles with table when preview is scrollable
	launchPreviewLastCmd  string // resets scroll when the displayed command changes
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
		homeDir:               homeDir,
		theme:                 th,
		themePick:             pick,
		styles:                st,
		sortCol:               defaultSortCol,
		keys:                  DefaultKeyMap(),
		tbl:                   t,
		hscroll:               hv,
		serverViewport:        sv,
		launchPreviewViewport: lpv,
		runtimeInputs: [runtimeFieldCount]textinput.Model{
			newPathTextInput(),
			newPathTextInput(),
			newPathTextInput(),
			newPortTextInput(),
			newPortTextInput(),
		},
		paramEditInput: newParamLineTextInput(),
		loading:        true,
	}
}

// Init implements tea.Model.
func (m Model) Init() tea.Cmd {
	return startupCmd()
}

// innerWidth returns the usable inner body width for rendering. It falls back
// to a computed value when bodyInnerW has not yet been set by layoutTable.
func (m Model) innerWidth() int {
	if m.bodyInnerW >= 1 {
		return m.bodyInnerW
	}
	if m.width > 0 {
		return max(m.width-appPaddingH*2, minInnerWidth)
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
	if !m.serverRunning || len(m.serverLog) == 0 {
		return false
	}
	inner := m.serverViewport.Width() - m.serverViewport.Style.GetHorizontalFrameSize()
	if inner < 1 {
		return false
	}
	return maxAnsiLineWidth(m.serverLog) > inner
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
	w := m.width
	if w < minTerminalWidth {
		w = minTerminalWidth
	}
	innerW := m.width - appPaddingH*2
	if innerW < minInnerWidth {
		innerW = w - appPaddingH*2
	}
	m.bodyInnerW = innerW
	// Column widths must use the same budget as the table viewport (inner body
	// width). Using full terminal width here made rows ~4 cells wider than
	// innerW and triggered empty horizontal scrolling.
	cols := tableColumns(innerW, m.files, m.homeDir, m.sortCol, m.sortDesc)
	m.tbl.SetColumns(cols)
	m.tbl.SetStyles(m.styles.table)
	minW := tableContentMinWidth(cols)
	m.tbl.SetWidth(max(minW, innerW))
	// Column widths do not change when only header labels (sort indicators) change; using
	// minW keeps the horizontal scroll bar row and table body height stable. Measuring
	// lipgloss.Width of the rendered header row can disagree with minW when glyphs differ.
	m.tableNeedsHScroll = len(m.files) > 0 && minW > innerW

	var h int
	if m.height <= 0 {
		h = defaultTableHeight
	} else {
		// Bubble Tea keeps only the bottom m.height lines if the view is taller;
		// size the table so framed (padding + chrome + body) fits.
		appPad := m.styles.app.GetVerticalFrameSize()
		innerMax := m.height - appPad
		if innerMax < 1 {
			innerMax = 1
		}
		needsLogHBarGuess := m.serverRunning && maxAnsiLineWidth(m.serverLog) > max(1, innerW-8)
		static := mainChromeLines(m, m.tableNeedsHScroll, needsLogHBarGuess)
		h = innerMax - static
		if h < 1 {
			h = 1
		}
	}

	previewH := m.launchPreviewPaneLayoutHeight()

	setHeights := func(bodyH int) {
		tableFrameV := m.hscroll.Style.GetVerticalFrameSize()
		logFrameV := m.serverViewport.Style.GetVerticalFrameSize()
		if m.serverRunning {
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
			m.tbl.SetHeight(tableRowAreaHeight(tableContentH))
			m.serverViewport.SetHeight(logContentH)
			m.serverViewport.SetWidth(innerW)
			if m.serverViewport.TotalLineCount() > m.serverViewport.VisibleLineCount() {
				m.serverViewport.SetWidth(innerW - 1)
			}
			m.serverViewportH = logContentH
		} else {
			tablePaneH := bodyH - previewH
			if tablePaneH < 1 {
				tablePaneH = 1
			}
			tableContentH := tablePaneH - tableFrameV
			if tableContentH < 1 {
				tableContentH = 1
			}
			m.tbl.SetHeight(tableRowAreaHeight(tableContentH))
			m.serverViewport.SetWidth(innerW)
			m.serverViewport.SetHeight(1)
			m.serverViewportH = 0
		}
	}
	setHeights(h)

	m.tbl.SetRows(buildTableRows(m.files, cols, m.homeDir))
	tview := m.tbl.View()
	m.tableBodyH = max(1, strings.Count(tview, "\n")+1)
	lines := strings.Split(tview, "\n")
	if len(lines) > 0 {
		m.tableLineWidth = lipgloss.Width(lines[0])
	} else {
		m.tableLineWidth = 0
	}

	// Second pass only when log horizontal scroll bar visibility differs from estimate.
	if m.height > 0 {
		needsLogHBar := m.serverRunning && m.serverLogNeedsHorizontalScroll()
		needsLogHBarGuess := m.serverRunning && maxAnsiLineWidth(m.serverLog) > max(1, innerW-8)
		if needsLogHBar != needsLogHBarGuess {
			appPad := m.styles.app.GetVerticalFrameSize()
			innerMax := m.height - appPad
			if innerMax < 1 {
				innerMax = 1
			}
			static := mainChromeLines(m, m.tableNeedsHScroll, needsLogHBar)
			h2 := innerMax - static
			if h2 < 1 {
				h2 = 1
			}
			if h2 != h {
				h = h2
				setHeights(h)
				m.tbl.SetRows(buildTableRows(m.files, cols, m.homeDir))
				tview = m.tbl.View()
				m.tableBodyH = max(1, strings.Count(tview, "\n")+1)
				lines = strings.Split(tview, "\n")
				if len(lines) > 0 {
					m.tableLineWidth = lipgloss.Width(lines[0])
				}
			}
		}
	}

	m.hscroll.SetContent(tview)
	m.hscroll.SetWidth(innerW)
	m.hscroll.SetHeight(m.tableBodyH)

	m = m.syncLaunchPreviewViewport(innerW)
	m = m.applyMainPaneFocusStyles()
	return m
}

// launchPreviewPaneLayoutHeight returns vertical rows consumed by the launch command preview
// (margin + bordered viewport) when models are listed.
func (m Model) launchPreviewPaneLayoutHeight() int {
	if !launchPreviewVisible(m) {
		return 0
	}
	// MarginTop(1) on [styles.launchPreview] plus the fixed-height bordered viewport.
	return m.styles.launchPreview.GetMarginTop() +
		m.styles.launchPreviewViewport.GetVerticalFrameSize() +
		launchPreviewVisibleLines
}

// syncLaunchPreviewViewport sets viewport dimensions and wrapped content from the selected row.
func (m Model) syncLaunchPreviewViewport(innerW int) Model {
	if innerW < minInnerWidth {
		innerW = minInnerWidth
	}
	if !launchPreviewVisible(m) {
		m.launchPreviewViewport.SetContent("")
		m.launchPreviewLastCmd = ""
		return m
	}
	cmd := launchPreviewCommandLine(m)
	if cmd != m.launchPreviewLastCmd {
		m.launchPreviewViewport.GotoTop()
		m.launchPreviewLastCmd = cmd
	}
	fr := m.launchPreviewViewport.Style.GetHorizontalFrameSize()
	textW := innerW - fr
	if textW < 8 {
		textW = 8
	}
	pvFrV := m.launchPreviewViewport.Style.GetVerticalFrameSize()
	outerH := launchPreviewVisibleLines + pvFrV

	m.launchPreviewViewport.SetWidth(innerW)
	rendered := m.styles.launchPreviewContent.Width(textW).Render(cmd)
	m.launchPreviewViewport.SetContent(rendered)
	m.launchPreviewViewport.SetHeight(outerH)
	if m.launchPreviewViewport.TotalLineCount() > m.launchPreviewViewport.VisibleLineCount() {
		m.launchPreviewViewport.SetWidth(innerW - 1)
		textW = innerW - 1 - fr
		if textW < 8 {
			textW = 8
		}
		rendered = m.styles.launchPreviewContent.Width(textW).Render(cmd)
		m.launchPreviewViewport.SetContent(rendered)
		m.launchPreviewViewport.SetHeight(outerH)
	}
	return m
}

// withLaunchPreviewSynced refreshes the launch preview after table input without a full layout pass.
func (m Model) withLaunchPreviewSynced() Model {
	iw := m.bodyInnerW
	if iw < 1 {
		iw = m.innerWidth()
	}
	return m.syncLaunchPreviewViewport(iw)
}

// applyMainPaneFocusStyles sets table vs launch-preview chrome when idle, or delegates to
// [Model.applySplitPaneFocusStyles] when a split-pane server is running.
func (m Model) applyMainPaneFocusStyles() Model {
	if m.serverRunning {
		m = m.applySplitPaneFocusStyles()
		m.launchPreviewViewport.Style = m.styles.launchPreviewViewport
		return m
	}
	if m.launchPreviewFocused {
		m.hscroll.Style = m.styles.splitPaneChromeDim
		m.launchPreviewViewport.Style = m.styles.splitPaneChromeFocused
	} else {
		m.hscroll.Style = m.styles.splitPaneChromeFocused
		m.launchPreviewViewport.Style = m.styles.launchPreviewViewport
	}
	return m
}

// applySplitPaneFocusStyles sets rounded borders on the table scroll viewport and
// the server log viewport. When the server is not running, the table uses focused
// chrome (single main pane); the idle log strip uses the default serverLogViewport
// style. When the server is running, the keyboard-focused split pane uses
// SplitPaneBorderFocused and the other SplitPaneBorderDim.
func (m Model) applySplitPaneFocusStyles() Model {
	if !m.serverRunning {
		m.hscroll.Style = m.styles.splitPaneChromeFocused
		m.serverViewport.Style = m.styles.serverLogViewport
		return m
	}
	if m.splitLogFocused {
		m.hscroll.Style = m.styles.splitPaneChromeDim
		m.serverViewport.Style = m.styles.splitPaneChromeFocused
	} else {
		m.hscroll.Style = m.styles.splitPaneChromeFocused
		m.serverViewport.Style = m.styles.splitPaneChromeDim
	}
	return m
}

// appendServerLogLine appends a log line for split-pane server output and refreshes the log viewport.
func (m Model) appendServerLogLine(line string) Model {
	align := m.serverLogAlignWidth
	line = normalizeSplitServerLogLine(line, &align)
	m.serverLogAlignWidth = align
	m.serverLog = append(m.serverLog, line)
	if len(m.serverLog) > maxServerLogLines {
		m.serverLog = m.serverLog[len(m.serverLog)-maxServerLogLines:]
	}
	m.serverViewport.SetContent(strings.Join(m.serverLog, "\n"))
	m.serverViewport.GotoBottom()
	return m
}

// cycleTheme advances dark → light → auto → dark, rebuilds lipgloss styles, and
// shows a short toast on the title row naming the active mode.
func (m Model) cycleTheme() (Model, tea.Cmd) {
	m.themePick = (m.themePick + 1) % themePickCount
	m.theme = themeFromPick(m.themePick, compat.HasDarkBackground)
	m.styles = newStyles(m.theme)
	m.themeToast = themeToastText(m.themePick, m.theme)
	m.launchPreviewViewport.Style = m.styles.launchPreviewViewport
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
// log (enter/esc/q) or tears down the UI after a non-split [llamaServerExitedMsg].
func (m Model) dismissSplitServer() Model {
	m.serverRunning = false
	m.serverExited = false
	m.splitLogFocused = false
	m.launchPreviewFocused = false
	m.serverCmd = nil
	m.serverMsgCh = nil
	m.serverLog = nil
	m.serverLogAlignWidth = 0
	m.serverViewport.SetContent("")
	m.tbl.Focus()
	m = m.layoutTable()
	return m
}
