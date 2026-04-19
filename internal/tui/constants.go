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
	// Padding(1, 2), so 2 on each side = 4 total consumed columns).
	appPaddingH = 2

	// hScrollStep is the number of columns scrolled per arrow/key press.
	hScrollStep = 4

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

	// shellDisplayArgIndent is the leading spaces for multiline shell display lines after
	// the first argv line (launch preview and clipboard; split-pane log uses "+ " instead).
	shellDisplayArgIndent = "  "

	// lastRunNoteVisibleDuration is how long the status line below the footer
	// stays visible before clearing (e.g. copy feedback, scan results).
	lastRunNoteVisibleDuration = 5 * time.Second
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
	FooterDescConfigPort = "runtime env"
	FooterHintConfigPort = FooterKeyConfigPort + ": " + FooterDescConfigPort

	FooterKeyParameters  = "p"
	FooterDescParameters = "profiles"
	FooterHintParameters = FooterKeyParameters + ": " + FooterDescParameters

	FooterKeyModelPaths  = "m"
	FooterDescModelPaths = "model paths"
	FooterHintModelPaths = FooterKeyModelPaths + ": " + FooterDescModelPaths

	FooterKeyToggleTheme  = "t"
	FooterDescToggleTheme = "theme"
	FooterHintToggleTheme = FooterKeyToggleTheme + ": " + FooterDescToggleTheme

	FooterKeyQuit  = "q"
	FooterDescQuit = "quit"
	FooterHintQuit = FooterKeyQuit + ": " + FooterDescQuit

	FooterHintHelp = "?: more"

	FooterKeyCopyPath  = "enter"
	FooterDescCopyPath = "copy cmd"
	FooterHintCopyPath = FooterKeyCopyPath + ": " + FooterDescCopyPath

	// CopyCommandFeedback* are shown below the footer after Enter copies the launch command.
	CopyCommandFeedbackSuccess = "Command copied to clipboard"
	CopyCommandFeedbackFailure = "Command failed to copy to clipboard"

	// Missing-runtime footer lines after model scan (see maybeSetMissingRuntimeFooterNote).
	MissingLlamaServerFooterNote = "llama-server not found - press " + FooterKeyConfigPort + " to set path manually"
	MissingVLLMFooterNote        = "vllm not found - press " + FooterKeyConfigPort + " to set path manually"

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
	FooterRuntimeConfigHints = "tab: fields · enter: save · esc: cancel"
	// runtimeConfigModalSubtitle appears below the modal title (values here override startup discovery).
	runtimeConfigModalSubtitle     = "Overrides saved to config.toml. Shell environment variables take precedence."
	runtimeConfigHeaderLlama       = "LLAMA.CPP"
	runtimeConfigHeaderVLLM        = "VLLM"
	runtimeConfigLabelLlamaCppPath = "Path (llama-cli / llama-server)"
	runtimeConfigLabelVLLMPath     = "Path (vllm binary)"
	runtimeConfigLabelVLLMVenv     = "Venv Root (Optional)"
	runtimeConfigLabelLlamaPort    = "Server Port"
	runtimeConfigLabelVLLMPort     = "Server Port"

	// Discovery paths modal.
	FooterDiscoveryPathsHints   = FooterNavHint + FooterHintSep + "n: add · enter: edit · d: delete · s: save · esc/q: cancel"
	discoveryPathsModalSubtitle = "These paths are saved to config.toml and scanned in addition to the defaults below."

	// FooterHintTabSections is the shared "tab: sections" fragment used by split-pane
	// and parameter modal footers.
	FooterHintTabSections = "tab: sections"
	FooterParamConfirmYN  = "y: yes · n: no"

	// Alphabetical by name; footer lines use the same middle action order where modes
	// overlap: create (n/a) → delete → rename or edit → back.
	FooterParamHintAddRow    = "a: add"
	FooterParamHintBack      = "esc/q: back"
	FooterParamHintDelete    = "d: delete"
	FooterParamHintEnterEdit = "enter: edit"
	FooterParamHintNew       = "n: new"
	FooterParamHintClone     = "c: clone"
	FooterParamHintRename    = "r: rename"

	FooterParamFooterProfiles    = FooterHintTabSections + FooterHintSep + FooterNavHint + FooterHintSep + FooterParamHintNew + FooterHintSep + FooterParamHintClone + FooterHintSep + FooterParamHintDelete + FooterHintSep + FooterParamHintRename + FooterHintSep + FooterParamHintBack
	FooterParamFooterDetailEmpty = FooterHintTabSections + FooterHintSep + FooterNavHint + FooterHintSep + FooterParamHintAddRow + FooterHintSep + FooterParamHintDelete + FooterHintSep + FooterParamHintBack
	FooterParamFooterDetailRows  = FooterHintTabSections + FooterHintSep + FooterNavHint + FooterHintSep + FooterParamHintAddRow + FooterHintSep + FooterParamHintDelete + FooterHintSep + FooterParamHintEnterEdit + FooterHintSep + FooterParamHintBack
)
