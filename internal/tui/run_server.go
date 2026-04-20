package tui

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"sync"

	tea "charm.land/bubbletea/v2"
	"github.com/atotto/clipboard"
	"github.com/flyingnobita/llml/internal/models"
)

// serverSpec holds the resolved parameters needed to build server commands for one launch.
type serverSpec struct {
	backend        models.ModelBackend
	bin            string
	port           int
	modelPath      string
	params         ModelParams
	activateScript string // vLLM only: path to venv activate script
}

// buildServerSpec resolves the binary, port, and venv for launching a server.
// When strict is true it returns an error if the binary is missing; when false it substitutes
// a placeholder name so display functions show a plausible command even before the runtime is configured.
func buildServerSpec(backend models.ModelBackend, modelPath string, params ModelParams, rt models.RuntimeInfo, strict bool) (serverSpec, error) {
	switch backend {
	case models.BackendVLLM:
		bin := models.ResolveVLLMPath(rt)
		activate := models.ResolveVLLMActivateScript(bin)
		if strict {
			if bin == "" {
				return serverSpec{}, fmt.Errorf(MissingVLLMFooterNote)
			}
			if runtime.GOOS == "windows" && activate != "" {
				return serverSpec{}, fmt.Errorf("vLLM venv activation is not supported on Windows from this app; run vllm from an activated shell or add vllm to PATH (detected %s)", activate)
			}
		} else if bin == "" {
			bin = "vllm"
		}
		return serverSpec{
			backend:        models.BackendVLLM,
			bin:            bin,
			port:           models.VLLMPort(),
			modelPath:      modelPath,
			params:         params,
			activateScript: activate,
		}, nil
	default: // BackendLlama
		bin := models.ResolveLlamaServerPath(rt)
		if strict && bin == "" {
			return serverSpec{}, fmt.Errorf(MissingLlamaServerFooterNote)
		}
		if bin == "" {
			bin = "llama-server"
		}
		return serverSpec{
			backend:   models.BackendLlama,
			bin:       bin,
			port:      models.ListenPort(),
			modelPath: modelPath,
			params:    params,
		}, nil
	}
}

// commandWords returns the escaped shell tokens for the server invocation (same order as directArgs).
func (s serverSpec) commandWords() []string {
	var words []string
	switch s.backend {
	case models.BackendVLLM:
		words = []string{
			shellSingleQuoted(s.bin), "serve",
			shellSingleQuoted(s.modelPath),
			"--served-model-name", shellSingleQuoted(models.InferModelID(s.modelPath)),
			"--port", fmt.Sprintf("%d", s.port),
		}
	default:
		words = []string{
			shellSingleQuoted(s.bin),
			"--model", shellSingleQuoted(s.modelPath),
			"--alias", shellSingleQuoted(llamaServerAlias(s.modelPath)),
			"--port", fmt.Sprintf("%d", s.port),
		}
	}
	for _, a := range s.params.Args {
		words = append(words, shellWord(a))
	}
	return words
}

// commandLine returns the single-line shell form of the invocation (env prefix + argv).
func (s serverSpec) commandLine() string {
	return strings.TrimSpace(shellEnvPrefix(s.params.Env) + strings.Join(s.commandWords(), " "))
}

// directArgs builds the argv slice for direct binary execution (no sh wrapper).
func (s serverSpec) directArgs() []string {
	var args []string
	switch s.backend {
	case models.BackendVLLM:
		args = []string{
			"serve", s.modelPath,
			"--served-model-name", models.InferModelID(s.modelPath),
			"--port", fmt.Sprintf("%d", s.port),
		}
	default:
		args = []string{
			"-m", s.modelPath,
			"--alias", llamaServerAlias(s.modelPath),
			"--port", fmt.Sprintf("%d", s.port),
		}
	}
	return append(args, s.params.Args...)
}

// unixForegroundScript returns the sh -c script used for foreground launch on Unix:
// echoes the invocation, runs the server, then pauses for Enter.
func (s serverSpec) unixForegroundScript() string {
	inv := s.invocationEcho()
	var runLine string
	if s.backend == models.BackendVLLM && s.activateScript != "" {
		runLine = fmt.Sprintf(". %s\n", shellSingleQuoted(s.activateScript))
	}
	runLine += s.commandLine()
	return fmt.Sprintf("printf '%%s\n' %s\n%s\necho\necho 'Press Enter to return to LLM Launcher...'\nread -r _ </dev/tty || read -r _\n",
		shellSingleQuoted(inv), runLine)
}

// unixSplitScript returns the sh -c script for split-pane log streaming on Unix (vLLM only;
// llama-server uses directArgs). Merges stderr and sources the venv activate script if present.
func (s serverSpec) unixSplitScript() string {
	var runLine string
	if s.activateScript != "" {
		runLine = fmt.Sprintf(". %s && ", shellSingleQuoted(s.activateScript))
	}
	return runLine + s.commandLine() + " 2>&1"
}

// foregroundCmd returns an *exec.Cmd for tea.ExecProcess (TUI suspends while server runs).
// On Unix, wraps in sh -c with printf echo and read-pause so logs stay readable before the TUI redraws.
// On Windows, runs the binary directly with merged env (no pause support).
func (s serverSpec) foregroundCmd() *exec.Cmd {
	if runtime.GOOS == "windows" {
		c := exec.Command(s.bin, s.directArgs()...)
		c.Env = mergeEnv(os.Environ(), s.params.Env)
		return c
	}
	return exec.Command("sh", "-c", s.unixForegroundScript())
}

// splitCmd returns an *exec.Cmd for split-pane log streaming.
// Unix vLLM uses sh -c with 2>&1 (stderr merge) and also sets Env via mergeEnv — double propagation
// is intentional and matches the original per-backend split paths.
// All other cases run the binary directly with merged env.
func (s serverSpec) splitCmd() *exec.Cmd {
	if s.backend == models.BackendVLLM && runtime.GOOS != "windows" {
		c := exec.Command("sh", "-c", s.unixSplitScript())
		c.Env = mergeEnv(os.Environ(), s.params.Env)
		return c
	}
	c := exec.Command(s.bin, s.directArgs()...)
	c.Env = mergeEnv(os.Environ(), s.params.Env)
	return c
}

// invocationEcho returns the multi-line "+ ..." display string for the split-pane log header.
func (s serverSpec) invocationEcho() string {
	return shellCommandDisplayMultiline(true, s.activateScript, s.params.Env, s.commandWords())
}

// previewLine returns the multi-line command for the launch preview and clipboard
// (no "+ " prefix, no activate wrapper — shows the raw executable invocation).
func (s serverSpec) previewLine() string {
	return shellCommandDisplayMultiline(false, "", s.params.Env, s.commandWords())
}

// splitServerInvocationEcho returns the first line written to the split-pane log when R is pressed.
// It uses the selected model, active parameter profile, and runtime info exactly as runSplitServerCmd.
func splitServerInvocationEcho(m Model) string {
	modelPath, be := m.SelectedModel()
	if modelPath == "" {
		return ""
	}
	params, ok := modelParamsForLaunchPreview(m)
	if !ok {
		return ""
	}
	spec, _ := buildServerSpec(be, modelPath, params, m.runtime, false)
	return spec.invocationEcho()
}

// launchPreviewCommandLine returns the shell form of the server command for the table preview and
// clipboard: same tokens as the split-pane subprocess, formatted on multiple lines, but without the
// "+ " log marker or the ". /path/activate &&" venv wrapper used when launching vLLM.
func launchPreviewCommandLine(m Model) string {
	modelPath, be := m.SelectedModel()
	if modelPath == "" {
		return ""
	}
	params, ok := modelParamsForLaunchPreview(m)
	if !ok {
		return ""
	}
	spec, _ := buildServerSpec(be, modelPath, params, m.runtime, false)
	return spec.previewLine()
}

func scanReaderLines(r io.Reader, ch chan<- tea.Msg, wg *sync.WaitGroup) {
	defer wg.Done()
	sc := bufio.NewScanner(r)
	for sc.Scan() {
		ch <- serverLogMsg{line: sc.Text()}
	}
}

func sendErrAndClose(ch chan tea.Msg, err error) {
	ch <- llamaServerExitedMsg{err: err}
	close(ch)
}

// streamSplitServerCmd starts cmd with stdout/stderr pipes, streams lines as [serverLogMsg], then sends [llamaServerExitedMsg] and closes ch.
func streamSplitServerCmd(cmd *exec.Cmd, ch chan tea.Msg) {
	applySplitCmdSysProcAttr(cmd)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		sendErrAndClose(ch, err)
		return
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		sendErrAndClose(ch, err)
		return
	}
	if err := cmd.Start(); err != nil {
		sendErrAndClose(ch, err)
		return
	}
	var wg sync.WaitGroup
	wg.Add(2)
	go scanReaderLines(stdout, ch, &wg)
	go scanReaderLines(stderr, ch, &wg)
	wg.Wait()
	err = cmd.Wait()
	ch <- llamaServerExitedMsg{err: err}
	close(ch)
}

// runForegroundServerCmd runs the server in the foreground (TUI suspends) via tea.ExecProcess.
func runForegroundServerCmd(spec serverSpec) tea.Cmd {
	return tea.ExecProcess(spec.foregroundCmd(), func(err error) tea.Msg {
		return llamaServerExitedMsg{err: err}
	})
}

// runSplitServerCmd starts the server in split-pane mode (logs stream into the TUI).
func runSplitServerCmd(spec serverSpec) tea.Cmd {
	return func() tea.Msg {
		cmd := spec.splitCmd()
		ch := make(chan tea.Msg, 64)
		inv := spec.invocationEcho()
		go func() {
			ch <- serverLogMsg{line: inv}
			streamSplitServerCmd(cmd, ch)
		}()
		return serverSplitReadyMsg{cmd: cmd, ch: ch}
	}
}

// readNextServerMsg blocks for the next message from a split-pane log channel (call from a tea.Cmd).
func readNextServerMsg(ch chan tea.Msg) tea.Cmd {
	return func() tea.Msg {
		msg, ok := <-ch
		if !ok {
			return llamaServerExitedMsg{err: nil}
		}
		return msg
	}
}

// copyLaunchCommandToClipboard writes the launch preview command and sets lastRunNote feedback.
func copyLaunchCommandToClipboard(m Model) (Model, tea.Cmd) {
	cmd := launchPreviewCommandLine(m)
	if cmd == "" {
		return m.flashError(CopyCommandFeedbackFailure)
	}
	if err := clipboard.WriteAll(cmd); err != nil {
		return m.flashError(CopyCommandFeedbackFailure)
	}
	return m.flashSuccess(CopyCommandFeedbackSuccess)
}
