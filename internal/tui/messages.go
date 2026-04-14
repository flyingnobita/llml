package tui

import (
	"os/exec"

	tea "charm.land/bubbletea/v2"
	"github.com/flyingnobita/llml/internal/llamacpp"
)

type runtimeReadyMsg struct {
	runtime llamacpp.RuntimeInfo
}

type modelsLoadedMsg struct {
	files []llamacpp.ModelFile
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
