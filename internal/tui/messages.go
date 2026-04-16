package tui

import (
	"os/exec"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/flyingnobita/llml/internal/llamacpp"
)

type runtimeReadyMsg struct {
	runtime llamacpp.RuntimeInfo
}

// modelsLoadedMsg is used in tests to simulate a completed filesystem scan.
type modelsLoadedMsg struct {
	files []llamacpp.ModelFile
}

// startupNeedFullScanMsg triggers a full runtime probe and model discovery (writes config.toml).
type startupNeedFullScanMsg struct{}

// startupCacheHitMsg loads models from config.toml cache (no filesystem walk).
type startupCacheHitMsg struct {
	runtime  llamacpp.RuntimeInfo
	files    []llamacpp.ModelFile
	lastScan time.Time
}

// fullScanDoneMsg completes a full discovery pass (startup or refresh-all path).
type fullScanDoneMsg struct {
	runtime  llamacpp.RuntimeInfo
	files    []llamacpp.ModelFile
	writeErr error
	lastScan time.Time
}

// modelRescanDoneMsg completes an S-key model-only re-scan.
type modelRescanDoneMsg struct {
	files    []llamacpp.ModelFile
	writeErr error
	lastScan time.Time
}

// runtimeReloadErrMsg reports failure to reload [runtime] from config.toml (r key).
type runtimeReloadErrMsg struct {
	err error
}

type modelsErrMsg struct {
	err error
}

type runServerErrMsg struct {
	err error
}

type llamaServerExitedMsg struct {
	err error
}

// serverLogMsg carries one line of stdout/stderr from a split-pane server process.
type serverLogMsg struct {
	line string
}

// serverSplitReadyMsg is sent when a split-pane server subprocess has started and log streaming begins.
type serverSplitReadyMsg struct {
	cmd *exec.Cmd
	ch  chan tea.Msg
}

// splitInterruptMsg is sent when SIGINT arrives while a split-pane server is running, so we stop the server instead of exiting the TUI.
type splitInterruptMsg struct{}

// themeToastClearMsg clears the transient theme toast (after tea.Tick).
type themeToastClearMsg struct{}
