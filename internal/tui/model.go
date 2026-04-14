package tui

import (
	"os"
	"os/exec"
	"strings"

	btable "charm.land/bubbles/v2/table"
	"charm.land/bubbles/v2/textinput"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"charm.land/lipgloss/v2/compat"
	"github.com/charmbracelet/x/ansi"
	"github.com/flyingnobita/llml/internal/llamacpp"
)

// Model is the root Bubble Tea model.
type Model struct {
	width             int
	height            int
	bodyInnerW        int
	tableBodyH        int
	tableLineWidth    int
	theme             Theme
	themePick         int
	themeToast        string
	styles            styles
	keys              KeyMap
	tbl               btable.Model
	hscroll           viewport.Model
	files             []llamacpp.ModelFile
	runtime           llamacpp.RuntimeInfo
	runtimeScanned    bool
	lastRunNote       string
	loading           bool
	loadErr           error
	runtimeConfigOpen bool
	runtimeFocus      int
	runtimeInputs     [runtimeFieldCount]textinput.Model

	paramPanelOpen        bool
	paramConfirmDelete    int // paramConfirm* (see param_panel.go); 0 = none
	paramModelPath        string
	paramModelDisplayName string
	paramFocus            int
	paramProfileIndex     int
	paramProfiles         []ParameterProfile
	paramEnvCursor        int
	paramArgsCursor       int
	paramEnv              []EnvVar
	paramArgs             []string
	paramEditKind         int
	paramEditInput        textinput.Model

	homeDir string // from [os.UserHomeDir] at startup; used for path display (~/)

	// Split-pane server (R): subprocess logs in lower half; see run_server.go.
	serverRunning   bool
	serverCmd       *exec.Cmd
	serverMsgCh     chan tea.Msg
	serverLog       []string
	serverViewport  viewport.Model
	serverViewportH int
	splitLogFocused bool // true: keys scroll log; false: keys use model table (Tab toggles).
}

// New returns a model with default key bindings and an empty table; Init triggers discovery.
func New() Model {
	homeDir, _ := os.UserHomeDir()
	pick := initialThemePick()
	th := themeFromPick(pick, compat.HasDarkBackground)
	st := newStyles(th)
	t := btable.New(
		btable.WithColumns(tableColumns(100, nil, homeDir)),
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
	return Model{
		homeDir:        homeDir,
		theme:          th,
		themePick:      pick,
		styles:         st,
		keys:           DefaultKeyMap(),
		tbl:            t,
		hscroll:        hv,
		serverViewport: sv,
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
	cols := tableColumns(innerW, m.files, m.homeDir)
	m.tbl.SetColumns(cols)
	m.tbl.SetStyles(m.styles.table)
	minW := tableContentMinWidth(cols)
	m.tbl.SetWidth(max(minW, innerW))

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
		needsHBarGuess := len(m.files) > 0 && minW > innerW
		needsLogHBarGuess := m.serverRunning && maxAnsiLineWidth(m.serverLog) > max(1, innerW-8)
		static := mainChromeLines(m, needsHBarGuess, needsLogHBarGuess)
		h = innerMax - static
		if h < 1 {
			h = 1
		}
	}

	setHeights := func(bodyH int) {
		if m.serverRunning {
			tableH, logH := splitServerBodyHeights(bodyH)
			m.tbl.SetHeight(tableH)
			m.serverViewport.SetHeight(logH)
			m.serverViewport.SetWidth(innerW)
			if m.serverViewport.TotalLineCount() > m.serverViewport.VisibleLineCount() {
				m.serverViewport.SetWidth(innerW - 1)
			}
			m.serverViewportH = logH
		} else {
			m.tbl.SetHeight(bodyH)
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

	// Second pass if scroll bar visibility differs from min-width estimate.
	if m.height > 0 {
		needsHBar := len(m.files) > 0 && m.tableLineWidth > 0 && m.tableLineWidth > innerW
		needsHBarGuess := len(m.files) > 0 && minW > innerW
		needsLogHBar := m.serverRunning && m.serverLogNeedsHorizontalScroll()
		needsLogHBarGuess := m.serverRunning && maxAnsiLineWidth(m.serverLog) > max(1, innerW-8)
		if needsHBar != needsHBarGuess || needsLogHBar != needsLogHBarGuess {
			appPad := m.styles.app.GetVerticalFrameSize()
			innerMax := m.height - appPad
			if innerMax < 1 {
				innerMax = 1
			}
			static := mainChromeLines(m, needsHBar, needsLogHBar)
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

	m = m.applySplitPaneFocusStyles()
	m.hscroll.SetContent(tview)
	m.hscroll.SetWidth(innerW)
	m.hscroll.SetHeight(m.tableBodyH)
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
	m = m.layoutTable()
	return m, clearThemeToastAfterCmd()
}
