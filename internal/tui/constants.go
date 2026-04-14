package tui

// Layout constants used across model, view, and table_layout.
const (
	// minTerminalWidth is the minimum terminal width we attempt to render into.
	minTerminalWidth = 56

	// minInnerWidth is the minimum inner body width (after app padding).
	minInnerWidth = 40

	// defaultTableHeight is the fallback table row-area height before the first
	// WindowSizeMsg arrives.
	defaultTableHeight = 18

	// appPaddingH is the Lip Gloss horizontal padding per side (app style uses
	// Padding(1, 2), so 2 on each side = 4 total consumed columns).
	appPaddingH = 2

	// hScrollStep is the number of columns scrolled per arrow/key press.
	hScrollStep = 4

	// appSubtitle is the subtitle line shown below the app title.
	appSubtitle = "llama.cpp (GGUF) · vLLM (config.json + safetensors) — filesystem scan · Last modified = file mtime"

	// paramPanelMaxInnerWidth caps the parameters modal inner width on wide
	// terminals so the panel does not stretch edge-to-edge.
	paramPanelMaxInnerWidth = 88

	// maxServerLogLines is the rolling cap for split-pane server log lines.
	maxServerLogLines = 1000

	// serverLogSeparatorLines is extra body rows between the table and log in split
	// mode (0 = panes are adjacent).
	serverLogSeparatorLines = 0
)

// Column-width defaults for the model table.
const (
	defaultNameColW = 36
	runtimeColW     = 11 // "llama.cpp", "vllm"
	sizeColW        = 9
	modTimeColW     = 17
	maxNameColW     = 72
	minPathColW     = 14
	maxPathColW     = 400
	colPaddingExtra = 8 // extra padding bubbles/table adds across 5 columns
)

// Footer hints: keyboard shortcut bar fragments ("key: description") joined with
// [FooterHintSep]. Keys and descriptions mirror [DefaultKeyMap] so help text stays
// aligned across the TUI and key bindings.
const (
	FooterHintSep = " · "

	// Main view (idle).
	FooterKeyRefresh  = "r"
	FooterDescRefresh = "refresh"
	FooterHintRefresh = FooterKeyRefresh + ": " + FooterDescRefresh

	FooterKeyRunSplit  = "R"
	FooterDescRunSplit = "run (split)"
	FooterHintRunSplit = FooterKeyRunSplit + ": " + FooterDescRunSplit

	FooterHintRunFullscreen = "ctrl+R: full terminal"

	FooterKeyConfigPort  = "c"
	FooterDescConfigPort = "runtime env"
	FooterHintConfigPort = FooterKeyConfigPort + ": " + FooterDescConfigPort

	FooterKeyParameters  = "p"
	FooterDescParameters = "param profiles"
	FooterHintParameters = FooterKeyParameters + ": " + FooterDescParameters

	FooterKeyToggleTheme  = "t"
	FooterDescToggleTheme = "theme"
	FooterHintToggleTheme = FooterKeyToggleTheme + ": " + FooterDescToggleTheme

	FooterKeyQuit  = "q"
	FooterDescQuit = "quit"
	FooterHintQuit = FooterKeyQuit + ": " + FooterDescQuit

	FooterKeyCopyPath  = "enter"
	FooterDescCopyPath = "copy path"
	FooterHintCopyPath = FooterKeyCopyPath + ": " + FooterDescCopyPath

	FooterKeyNav  = "hjkl/↑↓←→"
	FooterDescNav = "nav"
	// FooterNavHint is the navigation keys fragment used in main footers (table,
	// split server, parameters modal).
	FooterNavHint = FooterKeyNav + ": " + FooterDescNav

	// Split server view (while running).
	FooterSplitTabToTable = "tab: model table"
	FooterSplitTabToLog   = "tab: server log"
	FooterSplitStopServer = "esc/q: stop server"

	// Runtime config modal.
	FooterRuntimeConfigHints = "tab: next · shift+tab: prev · enter: save · esc: cancel"

	// Parameters modal (per-key fragments, then full footers composed with [FooterHintSep]).
	FooterParamTabSections = "tab: sections"
	FooterParamConfirmYN   = "y: yes · n: no"

	// Alphabetical by name; footer lines use the same middle action order where modes
	// overlap: create (n/a) → delete → rename or edit → back.
	FooterParamHintAddRow    = "a: add row"
	FooterParamHintBack      = "esc/q: back"
	FooterParamHintDelete    = "d: delete"
	FooterParamHintEnterEdit = "enter: edit"
	FooterParamHintNew       = "n: new"
	FooterParamHintRename    = "r: rename"

	FooterParamFooterProfiles = FooterParamTabSections + FooterHintSep + FooterNavHint + FooterHintSep +
		FooterParamHintNew + FooterHintSep + FooterParamHintDelete + FooterHintSep + FooterParamHintRename + FooterHintSep + FooterParamHintBack
	FooterParamFooterDetailEmpty = FooterParamTabSections + FooterHintSep + FooterNavHint + FooterHintSep +
		FooterParamHintAddRow + FooterHintSep + FooterParamHintDelete + FooterHintSep + FooterParamHintBack
	FooterParamFooterDetailRows = FooterParamTabSections + FooterHintSep + FooterNavHint + FooterHintSep +
		FooterParamHintAddRow + FooterHintSep + FooterParamHintDelete + FooterHintSep + FooterParamHintEnterEdit + FooterHintSep + FooterParamHintBack
)
