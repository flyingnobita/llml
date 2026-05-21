package tui

import (
	"os/exec"
	"strings"
	"time"

	"charm.land/bubbles/v2/filepicker"
	btable "charm.land/bubbles/v2/table"
	"charm.land/bubbles/v2/textinput"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"charm.land/lipgloss/v2/compat"
	"github.com/charmbracelet/x/ansi"

	"github.com/flyingnobita/llml/internal/models"
	"github.com/flyingnobita/llml/internal/profiles"
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
	tbl               btable.Model
	hscroll           viewport.Model
	files             []models.ModelFile
	sortCol           tableSortCol // default Runtime ascending
	sortDesc          bool         // false = ascending
	lastScan          time.Time
	effectiveBackends map[string]models.ModelBackend // keyed by model identity
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
	metadataCursor   int
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
	viewport          viewport.Model
	focused           bool   // idle only: Tab toggles with table whenever the preview is visible
	lastCmd           string // resets scroll when the displayed command changes
	activeProfileName string // cached active profile name for the title line
}

// alertsState holds persistent current status plus a toggleable history pane.
type alertsState struct {
	open        bool
	unread      int
	current     string
	currentSrc  string
	history     []alertEntry
	viewport    viewport.Model
	lastContent string
}

// discoveryPathsState holds the model-discovery paths modal's state.
type discoveryPathsState struct {
	open      bool
	cursor    int
	paths     []string
	editOpen  bool
	editInput textinput.Model
}

// exportViewState holds the profile export modal state.
type exportViewState struct {
	open            bool
	focus           exportFocus
	items           []exportProfileItem
	cursor          int
	scrollOffset    int
	outputPath      string
	pathInput       textinput.Model
	filterInput     textinput.Model
	filteredIndices []int
}

// collisionState holds the file-collision sub-modal state.
type collisionState struct {
	open       bool
	dest       string
	suffixPath string
}

// importViewState holds the profile import modal state.
type importViewState struct {
	open         bool
	focus        importFocus
	filePath     string
	pathInput    textinput.Model
	picker       filepicker.Model
	groups       []importGroup
	cursor       int
	scrollOffset int
	parseError   string
}

// importGroup is one model_hint group in the import modal.
type importGroup struct {
	modelHint      string
	matchedKey     string
	matchedDisplay string
	profiles       []profiles.PortableProfile
	checked        bool
}

type importFocus int

const (
	importFocusPath importFocus = iota
	importFocusList
	importFocusPicker
)

type exportFocus int

const (
	exportFocusList exportFocus = iota
	exportFocusFilter
	exportFocusPath
)

type exportItemKind int

const (
	exportItemProfile exportItemKind = iota
	exportItemHeader
)

// exportProfileItem is one profile entry in the export checkbox list.
type exportProfileItem struct {
	kind         exportItemKind
	modelKey     string
	modelDisplay string
	backend      string
	profileName  string
	checked      bool
	pp           profiles.PortableProfile // full portable profile for export
}

// mainPaneFocusSnap stores keyboard focus among the model table, launch preview, and
// split-pane server log before a full-screen modal opens. At most one modal uses it at a time.
type mainPaneFocusSnap struct {
	valid           bool
	previewFocused  bool
	splitLogFocused bool
}

// Model is the root Bubble Tea model.
type Model struct {
	layout    layoutState
	ui        themeState
	table     tableState
	rc        runtimeConfigState
	params    paramsState
	server    serverPaneState
	preview   launchPreviewState
	alerts    alertsState
	discovery discoveryPathsState
	export    exportViewState
	import_   importViewState
	collision collisionState
	paneFocus mainPaneFocusSnap

	keys               KeyMap
	runtime            models.RuntimeInfo
	runtimeScanned     bool
	lastRunNote        string
	lastRunNoteSuccess bool // true: lastRunNote is non-error feedback (e.g. copy confirmation)
	loading            bool
	loadErr            error
	helpOpen           bool // keyboard shortcuts popup
}

func newTableViewport(st styles, homeDir string) (btable.Model, viewport.Model) {
	t := btable.New(
		btable.WithColumns(tableColumns(100, nil, homeDir, defaultSortCol, false)),
		btable.WithRows(nil),
		btable.WithFocused(true),
		btable.WithStyles(st.table),
		btable.WithWidth(DefaultViewportWidth),
		btable.WithHeight(defaultTableHeight),
	)
	hv := viewport.New(viewport.WithWidth(DefaultViewportWidth), viewport.WithHeight(defaultTableHeight))
	hv.SetHorizontalStep(hScrollStep)
	// Vertical wheel must not scroll this wrapper; it desyncs from the bubbles table cursor.
	// Horizontal panning uses keys (see keymap); Shift+wheel could be added later if desired.
	hv.MouseWheelEnabled = false
	hv.Style = st.splitPaneChromeFocused
	return t, hv
}

func newServerLogViewport(st styles) viewport.Model {
	sv := viewport.New(viewport.WithWidth(DefaultViewportWidth), viewport.WithHeight(1))
	sv.MouseWheelEnabled = true
	sv.Style = st.serverLogViewport
	return sv
}

func newLaunchPreviewViewport(st styles) viewport.Model {
	lpvOuter := launchPreviewVisibleLines + st.launchPreviewViewport.GetVerticalFrameSize()
	lpv := viewport.New(viewport.WithWidth(DefaultViewportWidth), viewport.WithHeight(lpvOuter))
	lpv.MouseWheelEnabled = true
	lpv.MouseWheelDelta = 1
	lpv.SoftWrap = true
	lpv.Style = st.launchPreviewViewport
	return lpv
}

func newAlertViewport(st styles) viewport.Model {
	avOuter := alertPaneVisibleLines + st.alertPaneViewport.GetVerticalFrameSize()
	av := viewport.New(viewport.WithWidth(DefaultViewportWidth), viewport.WithHeight(avOuter))
	av.MouseWheelEnabled = true
	av.MouseWheelDelta = 1
	av.SoftWrap = true
	av.Style = st.alertPaneViewport
	return av
}

func newRuntimeConfigInputs() [runtimeFieldCount]textinput.Model {
	return [runtimeFieldCount]textinput.Model{
		newPathTextInput(),
		newPortTextInput(),
		newPathTextInput(),
		newPathTextInput(),
		newPortTextInput(),
		newPathTextInput(),
		newPathTextInput(),
		newPathTextInput(),
		newPortTextInput(),
	}
}

// New returns a model with default key bindings and an empty table; Init triggers discovery.
func New() Model {
	homeDir := models.HomeDir()
	pick := initialThemePick()
	th := resolveTheme()
	st := newStyles(th)
	t, hv := newTableViewport(st, homeDir)
	return Model{
		layout:    layoutState{homeDir: homeDir},
		ui:        themeState{theme: th, themePick: pick, styles: st},
		table:     tableState{sortCol: defaultSortCol, tbl: t, hscroll: hv, effectiveBackends: make(map[string]models.ModelBackend)},
		server:    serverPaneState{viewport: newServerLogViewport(st)},
		preview:   launchPreviewState{viewport: newLaunchPreviewViewport(st)},
		alerts:    alertsState{viewport: newAlertViewport(st)},
		rc:        runtimeConfigState{inputs: newRuntimeConfigInputs()},
		params:    paramsState{editInput: newParamLineTextInput()},
		discovery: discoveryPathsState{editInput: newPathTextInput()},
		export:    exportViewState{pathInput: newPathTextInput(), filterInput: newFilterTextInput()},
		import_:   importViewState{pathInput: newPathTextInput(), picker: filepicker.New()},
		keys:      DefaultKeyMap(),
		loading:   true,
	}
}

// Init implements tea.Model.
func (m Model) Init() tea.Cmd {
	return startupCmd()
}

// SelectedModelFile returns the highlighted model row.
func (m Model) SelectedModelFile() (models.ModelFile, bool) {
	if len(m.table.tbl.Rows()) == 0 || m.table.tbl.Cursor() < 0 {
		return models.ModelFile{}, false
	}
	i := m.table.tbl.Cursor()
	if i < 0 || i >= len(m.table.files) {
		return models.ModelFile{}, false
	}
	return m.table.files[i], true
}

// SelectedModel returns the backend-specific launch target and backend for the highlighted row.
func (m Model) SelectedModel() (target string, backend models.ModelBackend) {
	f, ok := m.SelectedModelFile()
	if !ok {
		return "", models.BackendLlama
	}
	return f.LaunchTarget(), f.Backend
}

// resolveEffectiveBackend returns the launch backend for the selected row, factoring in
// the active profile's backend override for GGUF rows.
//
//	GGUF row
//	  + profile=koboldcpp -> koboldcpp
//	  + profile=llama/""  -> llama-server
//
//	vllm row   -> vllm
//	ollama row -> ollama
func (m Model) resolveEffectiveBackend() models.ModelBackend {
	_, rowBackend := m.SelectedModel()
	if rowBackend != models.BackendLlama {
		return rowBackend
	}
	// When the param panel is open, read from in-memory state so unsaved
	// edits affect the launch preview immediately.
	if m.params.open {
		profileBackend := m.activeProfileBackendForSelected()
		if profileBackend == models.BackendKobold {
			return models.BackendKobold
		}
		return models.BackendLlama
	}
	// Otherwise use the cached effective-backend map (no disk I/O per cursor move).
	key := modelParamsKey(m.SelectedPath())
	if b, ok := m.table.effectiveBackends[key]; ok {
		return b
	}
	// Cache miss (e.g. before first scan): fall back to disk read.
	profileBackend := m.activeProfileBackendForSelected()
	if profileBackend == models.BackendKobold {
		return models.BackendKobold
	}
	return models.BackendLlama
}

// activeProfileBackendForSelected returns the backend stored in the active profile for
// the selected model, or BackendLlama when no profile backend is set.
func (m Model) activeProfileBackendForSelected() models.ModelBackend {
	sel := m.SelectedPath()
	if sel == "" {
		return models.BackendLlama
	}
	if m.params.open {
		if modelParamsKey(m.params.modelPath) == modelParamsKey(sel) {
			if m.params.profileIndex >= 0 && m.params.profileIndex < len(m.params.profiles) {
				b, _ := models.ParseBackend(m.params.profiles[m.params.profileIndex].Backend)
				return b
			}
		}
	}
	ent, err := loadModelEntry(modelParamsKey(sel))
	if err != nil || len(ent.Profiles) == 0 {
		return models.BackendLlama
	}
	idx := clampInt(ent.ActiveIndex, 0, len(ent.Profiles)-1)
	b, _ := models.ParseBackend(ent.Profiles[idx].Backend)
	return b
}

// populateEffectiveBackends reloads every model's active-profile backend from disk
// into m.table.effectiveBackends.
func (m Model) populateEffectiveBackends() Model {
	for _, f := range m.table.files {
		if f.Backend != models.BackendLlama {
			continue
		}
		m = m.loadEffectiveBackendForIdentity(f.Identity())
	}
	return m
}

// updateEffectiveBackendForPath updates or removes the cached effective backend for
// the given model identity after a profile save.
func (m Model) updateEffectiveBackendForPath(modelPath string) Model {
	return m.loadEffectiveBackendForIdentity(modelPath)
}

// loadEffectiveBackendForIdentity reads the active profile for identity and
// sets or deletes the effectiveBackends map entry accordingly.
func (m Model) loadEffectiveBackendForIdentity(identity string) Model {
	key := modelParamsKey(identity)
	ent, err := loadModelEntry(key)
	if err != nil || len(ent.Profiles) == 0 {
		delete(m.table.effectiveBackends, key)
		return m
	}
	idx := clampInt(ent.ActiveIndex, 0, len(ent.Profiles)-1)
	b, _ := models.ParseBackend(ent.Profiles[idx].Backend)
	if b != models.BackendLlama {
		m.table.effectiveBackends[key] = b
	} else {
		delete(m.table.effectiveBackends, key)
	}
	return m
}

// refreshTableRows rebuilds the table rows from m.table.files and the effective
// backend cache. It recalculates columns from current files and cache. It does not relayout body heights — use layoutTable
// for a full relayout (e.g. after terminal resize or sort change).
func (m Model) refreshTableRows() Model {
	cols := tableColumns(m.innerWidth(), m.table.files, m.layout.homeDir, m.table.sortCol, m.table.sortDesc)
	m.table.tbl.SetRows(buildTableRows(m.table.files, cols, m.layout.homeDir, m.table.effectiveBackends))
	return m
}

// SelectedPath returns the stable identity of the highlighted row, or empty if none.
func (m Model) SelectedPath() string {
	f, ok := m.SelectedModelFile()
	if !ok {
		return ""
	}
	return f.Identity()
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

// layoutTableAtInnerW builds columns, body height, and hscroll for a given inner body width.
func (m Model) layoutTableAtInnerW(innerW int) Model {
	m.layout.bodyInnerW = innerW
	cols := tableColumns(innerW, m.table.files, m.layout.homeDir, m.table.sortCol, m.table.sortDesc)
	m.table.tbl.SetColumns(cols)
	m.table.tbl.SetStyles(m.ui.styles.table)
	minW := tableContentMinWidth(cols)
	m.table.tbl.SetWidth(max(minW, innerW))
	m.layout.tableNeedsHScroll = len(m.table.files) > 0 && minW > innerW

	logFrameH := m.server.viewport.Style.GetHorizontalFrameSize()
	needsLogHBar := m.server.running && maxAnsiLineWidth(m.server.log) > max(1, innerW-logFrameH)

	previewH := m.launchPreviewPaneLayoutHeight()
	h := m.computeBodyHeight(needsLogHBar)
	m = m.applyTableAndLogHeights(h, innerW, previewH)

	m.table.tbl.SetRows(buildTableRows(m.table.files, cols, m.layout.homeDir, m.table.effectiveBackends))
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
	return m
}

// tablePaneShowsVerticalIndicator is true when the model table needs a █/░ track: inner row
// overflow (more files than fit in the bubbles table body) or outer viewport vertical overflow.
func (m Model) tablePaneShowsVerticalIndicator() bool {
	if len(m.table.files) == 0 {
		return false
	}
	if len(m.table.files) > m.table.tbl.Height() {
		return true
	}
	return m.table.hscroll.TotalLineCount() > m.table.hscroll.VisibleLineCount()
}

func (m Model) layoutTable() Model {
	w := m.layout.width
	if w < minTerminalWidth {
		w = minTerminalWidth
	}
	innerBase := m.layout.width - appPaddingH*2
	if innerBase < minInnerWidth {
		innerBase = w - appPaddingH*2
	}

	m = m.layoutTableAtInnerW(innerBase)
	m.table.hscroll.SetYOffset(0)

	if len(m.table.files) > 0 && innerBase > minInnerWidth && m.tablePaneShowsVerticalIndicator() {
		m = m.layoutTableAtInnerW(innerBase - 1)
		m.table.hscroll.SetYOffset(0)
	}

	m = m.syncLaunchPreviewViewport(m.layout.bodyInnerW)
	m = m.syncAlertViewport()
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
		logContentH := logPaneH - logFrameV - mainPaneTitleLines
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
// (margin + pane title + bordered viewport) when models are listed.
func (m Model) launchPreviewPaneLayoutHeight() int {
	if !m.launchPreviewVisible() {
		return 0
	}
	// MarginTop(1) on [styles.launchPreview], caption row, then the fixed-height bordered viewport.
	return m.ui.styles.launchPreview.GetMarginTop() +
		mainPaneTitleLines +
		m.ui.styles.launchPreviewViewport.GetVerticalFrameSize() +
		launchPreviewVisibleLines
}

func (m Model) alertPaneLayoutHeight() int {
	if !m.alerts.open {
		return 0
	}
	return m.ui.styles.alertPane.GetMarginTop() +
		mainPaneTitleLines +
		m.alerts.viewport.Style.GetVerticalFrameSize() +
		alertPaneVisibleLines
}

// syncLaunchPreviewViewport sets viewport dimensions and wrapped content from the selected row.
func (m Model) syncLaunchPreviewViewport(innerW int) Model {
	if innerW < minInnerWidth {
		innerW = minInnerWidth
	}
	if !m.launchPreviewVisible() {
		m.preview.viewport.SetContent("")
		m.preview.lastCmd = ""
		return m
	}
	cmd := launchPreviewCommandLine(m)
	if cmd != m.preview.lastCmd {
		m.preview.viewport.GotoTop()
		m.preview.lastCmd = cmd
	}
	m.preview.activeProfileName = activeProfileNameForPreview(m)
	fr := m.preview.viewport.Style.GetHorizontalFrameSize()
	textW := innerW - fr
	if textW < MinTextDisplayWidth {
		textW = MinTextDisplayWidth
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
		if textW < MinTextDisplayWidth {
			textW = MinTextDisplayWidth
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

// saveMainPaneFocusForModal snapshots which main pane had keyboard focus, then clears
// launch-preview focus so modal routing matches existing behavior.
func (m Model) saveMainPaneFocusForModal() Model {
	m.paneFocus.valid = true
	m.paneFocus.previewFocused = m.preview.focused
	m.paneFocus.splitLogFocused = m.server.splitFocused
	m.preview.focused = false
	return m
}

// restoreMainPaneFocusAfterModal restores focus saved by [Model.saveMainPaneFocusForModal].
func (m Model) restoreMainPaneFocusAfterModal() Model {
	if !m.paneFocus.valid {
		return m
	}
	m.preview.focused = m.paneFocus.previewFocused
	m.server.splitFocused = m.paneFocus.splitLogFocused
	m.paneFocus.valid = false
	if m.preview.focused || m.server.splitFocused {
		m.table.tbl.Blur()
	} else {
		m.table.tbl.Focus()
	}
	return m.applyMainPaneFocusStyles()
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
		m.alerts.viewport.Style = m.ui.styles.alertPaneViewport
		return m
	}
	switch {
	case m.server.splitFocused:
		m.table.hscroll.Style = m.ui.styles.splitPaneChromeDim
		m.server.viewport.Style = m.ui.styles.splitPaneChromeFocused
	case m.preview.focused:
		m.table.hscroll.Style = m.ui.styles.splitPaneChromeDim
		m.server.viewport.Style = m.ui.styles.splitPaneChromeDim
	default:
		m.table.hscroll.Style = m.ui.styles.splitPaneChromeFocused
		m.server.viewport.Style = m.ui.styles.splitPaneChromeDim
	}
	m.alerts.viewport.Style = m.ui.styles.alertPaneViewport
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
	m.alerts.viewport.Style = m.ui.styles.alertPaneViewport
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

// flashError sets an error status note and returns the cmd to clear it after a delay.
func (m Model) flashError(msg string) (Model, tea.Cmd) {
	return m.withLastRunError(msg), clearLastRunNoteAfterCmd()
}

// flashSuccess sets a non-error status note and returns the cmd to clear it after a delay.
func (m Model) flashSuccess(msg string) (Model, tea.Cmd) {
	return m.withLastRunSuccess(msg), clearLastRunNoteAfterCmd()
}

func (m Model) setCurrentStatus(source, msg string) Model {
	m.alerts.currentSrc = strings.TrimSpace(source)
	m.alerts.current = strings.TrimSpace(msg)
	return m
}

func (m Model) clearCurrentStatus() Model {
	m.alerts.currentSrc = ""
	m.alerts.current = ""
	return m
}

func (m Model) addAlert(severity alertSeverity, source, msg string) Model {
	msg = strings.TrimSpace(msg)
	if msg == "" {
		return m
	}
	entry := alertEntry{
		at:       time.Now(),
		severity: severity,
		source:   strings.TrimSpace(source),
		message:  msg,
	}
	m.alerts.history = append(m.alerts.history, entry)
	if !m.alerts.open {
		m.alerts.unread++
	}
	return m.syncAlertViewport()
}

func (m Model) syncAlertViewport() Model {
	iw := m.layout.bodyInnerW
	if iw < 1 {
		iw = m.innerWidth()
	}
	if iw < minInnerWidth {
		iw = minInnerWidth
	}
	fr := m.alerts.viewport.Style.GetHorizontalFrameSize()
	textW := max(iw-fr, MinTextDisplayWidth)
	outerH := alertPaneVisibleLines + m.alerts.viewport.Style.GetVerticalFrameSize()
	content := m.renderAlertHistoryContent(textW)
	if content != m.alerts.lastContent {
		m.alerts.lastContent = content
		m.alerts.viewport.SetContent(content)
		m.alerts.viewport.GotoBottom()
	} else {
		m.alerts.viewport.SetContent(content)
	}
	m.alerts.viewport.SetWidth(iw)
	m.alerts.viewport.SetHeight(outerH)
	if m.alerts.viewport.TotalLineCount() > m.alerts.viewport.VisibleLineCount() {
		m.alerts.viewport.SetWidth(iw - 1)
		textW = max(iw-1-fr, MinTextDisplayWidth)
		content = m.renderAlertHistoryContent(textW)
		m.alerts.lastContent = content
		m.alerts.viewport.SetContent(content)
		m.alerts.viewport.SetHeight(outerH)
	}
	return m
}

func (m Model) toggleAlerts() Model {
	m.alerts.open = !m.alerts.open
	if m.alerts.open {
		m.alerts.unread = 0
	}
	m = m.layoutTable()
	return m.syncAlertViewport()
}

// applyScanResult applies the result of a model scan to the model.
// runtime may be nil (model-only rescan); when non-nil it replaces m.runtime.
// firstLoad resets the table cursor to 0; otherwise cursor is only adjusted if out of range.
func (m Model) applyScanResult(runtime *models.RuntimeInfo, files []models.ModelFile, lastScan time.Time, configPaths []string, writeErr error, firstLoad bool) (Model, tea.Cmd) {
	m.loading = false
	m.loadErr = nil
	if runtime != nil {
		m.runtime = *runtime
		m.runtimeScanned = true
	}
	m.table.files = files
	m = m.populateEffectiveBackends()
	m.table.lastScan = lastScan
	m.discovery.paths = configPaths
	sortModelFiles(m.table.files, m.table.sortCol, m.table.sortDesc)
	m = m.layoutTable()
	m.table.hscroll.SetXOffset(0)
	if len(m.table.files) > 0 {
		if firstLoad {
			m.table.tbl.SetCursor(0)
			m = m.withLaunchPreviewSynced()
		} else if m.table.tbl.Cursor() >= len(m.table.files) {
			m.table.tbl.SetCursor(len(m.table.files) - 1)
			m = m.withLaunchPreviewSynced()
		}
	}
	if writeErr != nil {
		m = m.withLastRunError("Could not save config: " + writeErr.Error())
		m = m.addAlert(alertSeverityWarn, "Config", "Could not save config: "+writeErr.Error())
		return m.maybeSetMissingRuntimeFooterNoteBatch(clearLastRunNoteAfterCmd())
	}
	m = m.withLastRunCleared()
	return m.maybeSetMissingRuntimeFooterNote()
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
	m = m.syncAlertViewport()
	return m
}
