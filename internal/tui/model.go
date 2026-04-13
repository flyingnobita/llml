package tui

import (
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/flyingnobita/llm-launch/internal/llamacpp"
	btable "github.com/flyingnobita/llm-launch/internal/tui/btable"
)

// Model is the root Bubble Tea model.
type Model struct {
	width             int
	height            int
	bodyInnerW        int
	tableBodyH        int
	tableLineWidth    int
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

	paramPanelOpen    bool
	paramModelPath    string
	paramFocus        int
	paramProfileIndex int
	paramProfiles     []ParameterProfile
	paramEnvCursor    int
	paramArgsCursor   int
	paramEnv          []EnvVar
	paramArgs         []string
	paramEditKind     int
	paramEditInput    textinput.Model
}

// New returns a model with default key bindings and an empty table; Init triggers discovery.
func New() Model {
	t := btable.New(
		btable.WithColumns(tableColumns(100, nil)),
		btable.WithRows(nil),
		btable.WithFocused(true),
		btable.WithStyles(DefaultTableStyles()),
		btable.WithWidth(96),
		btable.WithHeight(defaultTableHeight),
	)
	hv := viewport.New(96, defaultTableHeight)
	hv.SetHorizontalStep(hScrollStep)
	return Model{
		keys:    DefaultKeyMap(),
		tbl:     t,
		hscroll: hv,
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

func (m Model) layoutTable() Model {
	w := m.width
	if w < minTerminalWidth {
		w = minTerminalWidth
	}
	cols := tableColumns(w, m.files)
	m.tbl.SetColumns(cols)
	m.tbl.SetStyles(DefaultTableStyles())
	innerW := m.width - appPaddingH*2
	if innerW < minInnerWidth {
		innerW = w - appPaddingH*2
	}
	m.bodyInnerW = innerW
	minW := tableContentMinWidth(cols)
	m.tbl.SetWidth(max(minW, innerW))
	h := m.height - layoutVerticalReserve
	if m.height <= 0 {
		h = defaultTableHeight
	} else if h < 6 {
		h = 6
	}
	m.tbl.SetHeight(h)
	m.tbl.SetRows(buildTableRows(m.files, cols))
	tview := m.tbl.View()
	m.tableBodyH = max(1, strings.Count(tview, "\n")+1)
	lines := strings.Split(tview, "\n")
	if len(lines) > 0 {
		m.tableLineWidth = lipgloss.Width(lines[0])
	} else {
		m.tableLineWidth = 0
	}
	m.hscroll.SetContent(tview)
	m.hscroll.Width = innerW
	m.hscroll.Height = m.tableBodyH
	return m
}
