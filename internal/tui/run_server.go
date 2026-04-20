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

// llamaCommandWords returns escaped shell tokens for llama-server (same order as the executed argv).
func llamaCommandWords(bin, modelPath string, port int, params ModelParams) []string {
	alias := llamaServerAlias(modelPath)
	words := []string{
		shellSingleQuoted(bin),
		"--model", shellSingleQuoted(modelPath),
		"--alias", shellSingleQuoted(alias),
		"--port", fmt.Sprintf("%d", port),
	}
	for _, a := range params.Args {
		words = append(words, shellWord(a))
	}
	return words
}

func llamaCommandLine(bin, modelPath string, port int, params ModelParams) string {
	envP := shellEnvPrefix(params.Env)
	words := llamaCommandWords(bin, modelPath, port, params)
	return strings.TrimSpace(envP + strings.Join(words, " "))
}

// vllmCommandWords returns escaped shell tokens for vllm serve (same order as the executed argv).
func vllmCommandWords(bin, modelDir string, port int, params ModelParams) []string {
	served := models.InferModelID(modelDir)
	words := []string{
		shellSingleQuoted(bin), "serve",
		shellSingleQuoted(modelDir),
		"--served-model-name",
		shellSingleQuoted(served),
		"--port",
		fmt.Sprintf("%d", port),
	}
	for _, a := range params.Args {
		words = append(words, shellWord(a))
	}
	return words
}

// vllmCommandLine builds the vllm serve invocation: model dir, --served-model-name from
// [models.InferModelID] (same as the Model ID column), --port, then profile argv.
func vllmCommandLine(bin, modelDir string, port int, params ModelParams) string {
	envP := shellEnvPrefix(params.Env)
	words := vllmCommandWords(bin, modelDir, port, params)
	return strings.TrimSpace(envP + strings.Join(words, " "))
}

// errVLLMNotFound is returned when ResolveVLLMPath finds no vllm binary.
func errVLLMNotFound() error {
	return fmt.Errorf(MissingVLLMFooterNote)
}

// formatLlamaServerInvocation is a multi-line, copy-paste safe command (with a leading "+ " on the
// first line) printed before launch and in the split-pane log.
func formatLlamaServerInvocation(bin, modelPath string, port int, params ModelParams) string {
	return shellCommandDisplayMultiline(true, "", params.Env, llamaCommandWords(bin, modelPath, port, params))
}

// formatVLLMServerInvocation is a multi-line command printed before vllm serve. If activateScript is
// non-empty, the block starts with `. '/path/activate' && \` on its own line.
func formatVLLMServerInvocation(bin, modelDir string, port int, activateScript string, params ModelParams) string {
	return shellCommandDisplayMultiline(true, activateScript, params.Env, vllmCommandWords(bin, modelDir, port, params))
}

// unixLlamaServerScript echoes the invocation, runs llama-server, then waits for Enter so logs stay readable before the TUI redraws.
func unixLlamaServerScript(bin, modelPath string, port int, params ModelParams) string {
	inv := formatLlamaServerInvocation(bin, modelPath, port, params)
	runLine := llamaCommandLine(bin, modelPath, port, params)
	return fmt.Sprintf(`printf '%%s\n' %s
%s
echo
echo 'Press Enter to return to LLM Launcher...'
read -r _ </dev/tty || read -r _
`, shellSingleQuoted(inv), runLine)
}

// unixVLLMServerScript echoes the invocation, runs vllm serve, then waits for Enter.
// If activateScript is non-empty, it is sourced with `.` before vllm (typical Python venv).
func unixVLLMServerScript(bin, modelDir string, port int, activateScript string, params ModelParams) string {
	inv := formatVLLMServerInvocation(bin, modelDir, port, activateScript, params)
	var runLine string
	if activateScript != "" {
		runLine = fmt.Sprintf(". %s\n", shellSingleQuoted(activateScript))
	}
	runLine += vllmCommandLine(bin, modelDir, port, params)
	return fmt.Sprintf(`printf '%%s\n' %s
%s
echo
echo 'Press Enter to return to LLM Launcher...'
read -r _ </dev/tty || read -r _
`, shellSingleQuoted(inv), runLine)
}

// unixVLLMSplitScript runs vllm under sh with merged stderr for split-pane log streaming (no pause/read).
func unixVLLMSplitScript(bin, modelDir string, port int, activateScript string, params ModelParams) string {
	var runLine string
	if activateScript != "" {
		runLine = fmt.Sprintf(". %s && ", shellSingleQuoted(activateScript))
	}
	runLine += vllmCommandLine(bin, modelDir, port, params)
	return runLine + " 2>&1"
}

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
// Returns an error if the binary is missing or the platform rejects the config (e.g. Windows + venv).
func buildServerSpec(backend models.ModelBackend, modelPath string, params ModelParams, rt models.RuntimeInfo) (serverSpec, error) {
	switch backend {
	case models.BackendVLLM:
		bin := models.ResolveVLLMPath(rt)
		if bin == "" {
			return serverSpec{}, errVLLMNotFound()
		}
		activate := models.ResolveVLLMActivateScript(bin)
		if runtime.GOOS == "windows" && activate != "" {
			return serverSpec{}, fmt.Errorf("vLLM venv activation is not supported on Windows from this app; run vllm from an activated shell or add vllm to PATH (detected %s)", activate)
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
		if bin == "" {
			return serverSpec{}, fmt.Errorf(MissingLlamaServerFooterNote)
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

// buildPreviewSpec is like buildServerSpec but substitutes placeholder bin names when not found,
// so display functions (launch preview, invocation echo) show a plausible command even before
// the runtime is configured.
func buildPreviewSpec(backend models.ModelBackend, modelPath string, params ModelParams, rt models.RuntimeInfo) serverSpec {
	switch backend {
	case models.BackendVLLM:
		bin := models.ResolveVLLMPath(rt)
		activate := models.ResolveVLLMActivateScript(bin)
		if bin == "" {
			bin = "vllm"
		}
		return serverSpec{
			backend:        models.BackendVLLM,
			bin:            bin,
			port:           models.VLLMPort(),
			modelPath:      modelPath,
			params:         params,
			activateScript: activate,
		}
	default: // BackendLlama
		bin := models.ResolveLlamaServerPath(rt)
		if bin == "" {
			bin = "llama-server"
		}
		return serverSpec{
			backend:   models.BackendLlama,
			bin:       bin,
			port:      models.ListenPort(),
			modelPath: modelPath,
			params:    params,
		}
	}
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

// foregroundCmd returns an *exec.Cmd for tea.ExecProcess (TUI suspends while server runs).
// On Unix, wraps in sh -c with printf echo and read-pause so logs stay readable before the TUI redraws.
// On Windows, runs the binary directly with merged env (no pause support).
func (s serverSpec) foregroundCmd() *exec.Cmd {
	if runtime.GOOS == "windows" {
		c := exec.Command(s.bin, s.directArgs()...)
		c.Env = mergeEnv(os.Environ(), s.params.Env)
		return c
	}
	var script string
	switch s.backend {
	case models.BackendVLLM:
		script = unixVLLMServerScript(s.bin, s.modelPath, s.port, s.activateScript, s.params)
	default:
		script = unixLlamaServerScript(s.bin, s.modelPath, s.port, s.params)
	}
	return exec.Command("sh", "-c", script)
}

// splitCmd returns an *exec.Cmd for split-pane log streaming.
// Unix vLLM uses sh -c with 2>&1 (stderr merge) and also sets Env via mergeEnv — double propagation
// is intentional and matches the original per-backend split paths.
// All other cases run the binary directly with merged env.
func (s serverSpec) splitCmd() *exec.Cmd {
	if s.backend == models.BackendVLLM && runtime.GOOS != "windows" {
		script := unixVLLMSplitScript(s.bin, s.modelPath, s.port, s.activateScript, s.params)
		c := exec.Command("sh", "-c", script)
		c.Env = mergeEnv(os.Environ(), s.params.Env)
		return c
	}
	c := exec.Command(s.bin, s.directArgs()...)
	c.Env = mergeEnv(os.Environ(), s.params.Env)
	return c
}

// invocationEcho returns the multi-line "+ ..." display string for the split-pane log header.
func (s serverSpec) invocationEcho() string {
	switch s.backend {
	case models.BackendVLLM:
		return formatVLLMServerInvocation(s.bin, s.modelPath, s.port, s.activateScript, s.params)
	default:
		return formatLlamaServerInvocation(s.bin, s.modelPath, s.port, s.params)
	}
}

// previewLine returns the multi-line command for the launch preview and clipboard
// (no "+ " prefix, no activate wrapper — shows the raw executable invocation).
func (s serverSpec) previewLine() string {
	switch s.backend {
	case models.BackendVLLM:
		return shellCommandDisplayMultiline(false, "", s.params.Env, vllmCommandWords(s.bin, s.modelPath, s.port, s.params))
	default:
		return shellCommandDisplayMultiline(false, "", s.params.Env, llamaCommandWords(s.bin, s.modelPath, s.port, s.params))
	}
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
	return buildPreviewSpec(be, modelPath, params, m.runtime).invocationEcho()
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
	return buildPreviewSpec(be, modelPath, params, m.runtime).previewLine()
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
