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

	// layoutVerticalReserve is the number of terminal rows consumed by the title,
	// subtitle, blank lines, runtime panel, footer, and outer padding.
	layoutVerticalReserve = 17

	// appPaddingH is the Lip Gloss horizontal padding per side (app style uses
	// Padding(1, 2), so 2 on each side = 4 total consumed columns).
	appPaddingH = 2

	// hScrollStep is the number of columns scrolled per arrow/key press.
	hScrollStep = 4

	// appSubtitle is the subtitle line shown below the app title.
	appSubtitle = "llama.cpp (GGUF) · vLLM (config.json + safetensors) — filesystem scan · Last modified = file mtime"
)

// Column-width defaults for the model table.
const (
	defaultNameColW = 36
	sizeColW        = 9
	modTimeColW     = 17
	paramColW       = 18
	maxNameColW     = 72
	minPathColW     = 14
	maxPathColW     = 400
	colPaddingExtra = 8 // extra padding bubbles/table adds across 5 columns
)
