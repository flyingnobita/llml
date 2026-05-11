package tui

import "time"

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
	// Padding(0, appPaddingH): no vertical padding; 2 columns horizontal each side).
	appPaddingH = 2

	// hScrollStep is the number of columns scrolled per arrow/key press.
	hScrollStep = 4

	// appTitle is the primary application name rendered in the title bar.
	appTitle = "LLM Launcher"

	// appSubtitle is the optional subtitle line below the app title (empty = hidden).
	appSubtitle = ""

	// paramPanelMaxInnerWidth caps the parameters and runtime-environment modal inner
	// width on wide terminals so panels do not stretch edge-to-edge.
	paramPanelMaxInnerWidth = 88

	// maxServerLogLines is the rolling cap for split-pane server log lines.
	maxServerLogLines = 1000

	// defaultServerLogAlignWidth is the pad used for unprefixed progress lines (tqdm) until a
	// structured vLLM log line has been seen to measure the real prefix width.
	defaultServerLogAlignWidth = 56

	// serverLogAlignPadMax caps padding width to avoid runaway indentation if prefix detection misbehaves.
	serverLogAlignPadMax = 256

	// serverLogSeparatorLines is extra body rows between the table and log in split
	// mode (0 = panes are adjacent).
	serverLogSeparatorLines = 0

	// launchPreviewVisibleLines is the number of visible text rows inside the launch
	// command preview (the bordered viewport outer height adds the frame; see syncLaunchPreviewViewport).
	launchPreviewVisibleLines = 4

	// mainPaneTitleLines is the caption row above each titled pane. Lip Gloss v2 does not
	// expose border-embedded titles; see launchPreviewPaneView and serverLogPaneView.
	mainPaneTitleLines = 1

	// shellDisplayArgIndent is the leading spaces for multiline shell display lines after
	// the first argv line (launch preview and clipboard; split-pane log uses "+ " instead).
	shellDisplayArgIndent = "  "

	// lastRunNoteVisibleDuration is how long the status line below the footer
	// stays visible before clearing (e.g. copy feedback, scan results).
	lastRunNoteVisibleDuration = 5 * time.Second

	// alertPaneVisibleLines is the number of visible rows inside the alert-history viewport.
	alertPaneVisibleLines = 6

	// DefaultViewportWidth is the fallback viewport width used before the first
	// WindowSizeMsg arrives.
	DefaultViewportWidth = 96

	// MinTextDisplayWidth is the minimum content width for viewport text after
	// frame subtraction. Narrower terminals get this floor.
	MinTextDisplayWidth = 8

	// MinModalInnerWidth is the minimum width for modal content panels
	// (runtime config, parameter profiles).
	MinModalInnerWidth = 24

	// ParamEditCharLimit is the textinput.CharLimit for parameter env/arg/value editing.
	ParamEditCharLimit = 4096

	// PathInputCharLimit is the textinput.CharLimit for path text inputs.
	PathInputCharLimit = 2048

	// PathTextInputWidth is the textinput width for path fields.
	PathTextInputWidth = 38

	// FilterInputCharLimit is the max characters for the export filter text input.
	FilterInputCharLimit = 128

	// FilterTextInputWidth is the textinput width for the export filter.
	FilterTextInputWidth = 30

	// MinParamEditInnerWidth is the minimum inner width for a parameter edit text input.
	MinParamEditInnerWidth = 32
)

// Server and process constants.
const (
	// ServerSplitChannelBuffer is the buffer size for the server stdout/stderr channel
	// in split-pane mode.
	ServerSplitChannelBuffer = 64

	// OllamaStartupTimeout is how long to wait for ollama serve to accept connections.
	OllamaStartupTimeout = 8 * time.Second

	// OllamaPollInterval is the delay between /api/tags health probes during startup.
	OllamaPollInterval = 200 * time.Millisecond
)

// Main view pane captions (Title Case). Shown above the launch preview and server log only.
const (
	MainPaneTitleLaunchCommand = "Launch Command"
	MainPaneTitleServerOutput  = "Server Output"
	MainPaneTitleAlerts        = "Alert History"
)

// Column-width defaults for the model table.
const (
	defaultFileNameColW = 36
	defaultIDColW       = 28
	maxIDColW           = 56
	runtimeColW         = 11 // "llama.cpp", "vllm"
	sizeColW            = 9
	modTimeColW         = 17
	maxFileNameColW     = 72
	minPathColW         = 14
	maxPathColW         = 400
	colPaddingExtra     = 10 // extra padding bubbles/table adds across 6 columns
)

// Footer hints: keyboard shortcut bar fragments ("key: description") joined with
// [FooterHintSep]. Keys and descriptions mirror [DefaultKeyMap] so help text stays
// aligned across the TUI and key bindings.
const (
	FooterHintSep = " · "

	// Main view (idle).
	FooterKeyRefresh  = "r"
	FooterDescRefresh = "reload runtime"
	FooterHintRefresh = FooterKeyRefresh + ": " + FooterDescRefresh

	FooterKeyRescan  = "S"
	FooterDescRescan = "rescan models"
	FooterHintRescan = FooterKeyRescan + ": " + FooterDescRescan

	FooterKeyRunSplit  = "R"
	FooterDescRunSplit = "run (split)"
	FooterHintRunSplit = FooterKeyRunSplit + ": " + FooterDescRunSplit

	FooterHintRunFullscreen = "ctrl+R: full terminal"

	FooterKeyConfigPort  = "c"
	FooterDescConfigPort = "Runtime Environment"
	FooterHintConfigPort = FooterKeyConfigPort + ": " + FooterDescConfigPort

	FooterKeyParameters  = "p"
	FooterDescParameters = "Parameter Profiles"
	FooterHintParameters = FooterKeyParameters + ": " + FooterDescParameters

	FooterKeyModelPaths  = "m"
	FooterDescModelPaths = "Model Paths"
	FooterHintModelPaths = FooterKeyModelPaths + ": " + FooterDescModelPaths

	FooterKeyAlerts  = "a"
	FooterDescAlerts = "alerts"
	FooterHintAlerts = FooterKeyAlerts + ": " + FooterDescAlerts

	FooterKeyToggleTheme  = "t"
	FooterDescToggleTheme = "theme"
	FooterHintToggleTheme = FooterKeyToggleTheme + ": " + FooterDescToggleTheme

	FooterKeyQuit  = "q"
	FooterDescQuit = "quit"
	FooterHintQuit = FooterKeyQuit + ": " + FooterDescQuit

	FooterHintHelp = "?: more"

	FooterKeyExport  = "E"
	FooterDescExport = "export"
	FooterHintExport = FooterKeyExport + ": " + FooterDescExport

	FooterKeyImport  = "I"
	FooterDescImport = "import"
	FooterHintImport = FooterKeyImport + ": " + FooterDescImport

	FooterKeyCopyPath  = "enter"
	FooterDescCopyPath = "copy cmd"
	FooterHintCopyPath = FooterKeyCopyPath + ": " + FooterDescCopyPath

	// CopyCommandFeedback* are shown below the footer after Enter copies the launch command.
	CopyCommandFeedbackSuccess = "Command copied to clipboard"
	CopyCommandFeedbackFailure = "Command failed to copy to clipboard"

	// Missing-runtime footer lines after model scan (see maybeSetMissingRuntimeFooterNote).
	MissingLlamaServerFooterNote = "llama-server not found - press " + FooterKeyConfigPort + " to set path manually"
	MissingVLLMFooterNote        = "vllm not found - press " + FooterKeyConfigPort + " to set path manually"
	MissingOllamaFooterNote      = "ollama not found or not reachable - press " + FooterKeyConfigPort + " to set path or host"
	MissingKoboldCppFooterNote   = "koboldcpp not found - press " + FooterKeyConfigPort + " to set path manually"

	FooterKeySortColumn   = ","
	FooterDescSortColumn  = "sort"
	FooterKeySortReverse  = "."
	FooterDescSortReverse = "reverse"
	FooterHintSort        = FooterKeySortColumn + "/" + FooterKeySortReverse + ": " + FooterDescSortColumn

	FooterKeyNav  = "hjkl/↑↓←→"
	FooterDescNav = "nav"
	// FooterNavHint is the navigation keys fragment used in main footers (table,
	// split server, parameters modal).
	FooterNavHint = FooterKeyNav + ": " + FooterDescNav

	// Split server view (while running).
	FooterSplitStopServer = "esc/q: stop server"
	FooterSplitDismiss    = "enter/esc/q: close"

	// splitPanePressEnterToClose is appended to the split log after the server process exits.
	splitPanePressEnterToClose = "Press Enter to close..."
	// splitServerStoppedWithHint is shown on clean exit before the user dismisses the pane.
	splitServerStoppedWithHint = "Server stopped. Press Enter to close..."

	// Runtime config modal.
	FooterRuntimeConfigHints = "tab: fields · enter: save · esc: back"
	// runtimeConfigModalSubtitle appears below the modal title (values here override startup discovery).
	runtimeConfigModalSubtitle      = "Overrides saved to config.toml. Shell environment variables take precedence."
	runtimeConfigHeaderLlama        = "Llama.cpp"
	runtimeConfigHeaderVLLM         = "vLLM"
	runtimeConfigHeaderOllama       = "Ollama"
	runtimeConfigHeaderKoboldCpp    = "KoboldCpp"
	runtimeConfigLabelLlamaCppPath  = "Path (llama-cli / llama-server)"
	runtimeConfigLabelVLLMPath      = "Path (vllm binary)"
	runtimeConfigLabelVLLMVenv      = "Venv Root (Optional)"
	runtimeConfigLabelLlamaPort     = "Server Port"
	runtimeConfigLabelVLLMPort      = "Server Port"
	runtimeConfigLabelOllamaPath    = "Path (ollama binary)"
	runtimeConfigLabelOllamaHost    = "Host"
	runtimeConfigLabelKoboldCppPath = "Path (koboldcpp binary)"
	runtimeConfigLabelKoboldCppPort = "Server Port"

	// FooterHintTabSections is the shared "tab: sections" fragment used by split-pane
	// and parameter modal footers.
	FooterHintTabSections = "tab: section"
	FooterParamConfirmYN  = "y: yes · n: no"

	// Alphabetical by name; footer lines use the same middle action order where modes
	// overlap: create (n/a) → delete → rename or edit → back.
	FooterParamHintAdd = "a: add"

	// Discovery paths modal.
	FooterDiscoveryPathsHints   = FooterNavHint + FooterHintSep + FooterParamHintAdd + FooterHintSep + "enter: edit · d: delete · s: save · esc: back"
	discoveryPathsModalSubtitle = "These paths are saved to config.toml and scanned in addition to the defaults below."
	FooterParamHintBack         = "esc: back"
	FooterParamHintDelete       = "d: delete"
	FooterParamHintEnterEdit    = "enter: edit"
	FooterParamHintClone        = "c: clone"
	FooterParamHintRename       = "r: rename"
	FooterParamHintCycle        = "←/→: cycle"

	FooterParamFooterProfiles    = FooterHintTabSections + FooterHintSep + FooterNavHint + FooterHintSep + FooterParamHintAdd + FooterHintSep + FooterParamHintClone + FooterHintSep + FooterParamHintDelete + FooterHintSep + FooterParamHintRename + FooterHintSep + FooterParamHintBack
	FooterParamFooterMetadata    = FooterHintTabSections + FooterHintSep + FooterNavHint + FooterHintSep + FooterParamHintCycle + FooterHintSep + FooterParamHintEnterEdit + FooterHintSep + FooterParamHintBack
	FooterParamFooterDetailEmpty = FooterHintTabSections + FooterHintSep + FooterNavHint + FooterHintSep + FooterParamHintAdd + FooterHintSep + FooterParamHintDelete + FooterHintSep + FooterParamHintBack
	FooterParamFooterDetailRows  = FooterHintTabSections + FooterHintSep + FooterNavHint + FooterHintSep + FooterParamHintAdd + FooterHintSep + FooterParamHintDelete + FooterHintSep + FooterParamHintEnterEdit + FooterHintSep + FooterParamHintBack

	// Export modal.
	FooterExportHints    = FooterHintExport + FooterHintSep + "tab: focus · /: filter · space: toggle · ctrl+u/d: page · a: select all · A: select none · enter: export · esc: back"
	FooterExportNoMatch  = "(no matches)"
	FooterCollisionHints = "o: overwrite · n: new name · esc: cancel"

	// Import modal.
	FooterImportHintsPath = FooterHintImport + FooterHintSep + "enter: parse · esc: back"
	FooterImportHintsList = FooterHintImport + FooterHintSep + "space: toggle · enter: import · esc: back"
)
