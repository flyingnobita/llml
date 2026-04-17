package tui

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"

	tea "charm.land/bubbletea/v2"
	"github.com/flyingnobita/llml/internal/llamacpp"
)

// shellSingleQuoted returns s wrapped in single quotes for POSIX sh (safe for paths with spaces).
func shellSingleQuoted(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'"'"'`) + "'"
}

// llamaServerAlias returns the API model id alias: leaf name of the GGUF path (matches File Name column).
func llamaServerAlias(modelPath string) string {
	return filepath.Base(filepath.Clean(modelPath))
}

// llamaCommandWords returns escaped shell tokens for llama-server (same order as the executed argv).
func llamaCommandWords(bin, modelPath string, port int, params ModelParams) []string {
	alias := llamaServerAlias(modelPath)
	words := []string{
		shellSingleQuoted(bin),
		"-m", shellSingleQuoted(modelPath),
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
	served := llamacpp.InferModelID(modelDir)
	words := []string{
		shellSingleQuoted(bin),
		"serve",
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
// [llamacpp.InferModelID] (same as the Model ID column), --port, then profile argv.
func vllmCommandLine(bin, modelDir string, port int, params ModelParams) string {
	envP := shellEnvPrefix(params.Env)
	words := vllmCommandWords(bin, modelDir, port, params)
	return strings.TrimSpace(envP + strings.Join(words, " "))
}

// errVLLMNotFound is returned when ResolveVLLMPath finds no vllm binary.
func errVLLMNotFound() error {
	return fmt.Errorf("vllm not found; set %s (project dir; we use vllm or .venv/bin/vllm) or %s (venv root), or install vllm on PATH", llamacpp.EnvVLLMPath, llamacpp.EnvVLLMVenv)
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

// splitServerInvocationEcho returns the same string as the first message written to the split-pane
// log when R is pressed (the multi-line "+ ..." echo). It uses the selected row, active parameter
// profile, and [llamacpp.RuntimeInfo] the same way as [runLlamaServerSplitCmd] / [runVLLMServerSplitCmd].
func splitServerInvocationEcho(m Model) string {
	modelPath, be := m.SelectedModel()
	if modelPath == "" {
		return ""
	}
	params, ok := modelParamsForLaunchPreview(m)
	if !ok {
		return ""
	}
	rt := m.runtime
	switch be {
	case llamacpp.BackendVLLM:
		bin := llamacpp.ResolveVLLMPath(rt)
		activate := llamacpp.ResolveVLLMActivateScript(bin)
		if bin == "" {
			bin = "vllm"
		}
		return formatVLLMServerInvocation(bin, modelPath, llamacpp.VLLMPort(), activate, params)
	default:
		bin := llamacpp.ResolveLlamaServerPath(rt)
		if bin == "" {
			bin = "llama-server"
		}
		return formatLlamaServerInvocation(bin, modelPath, llamacpp.ListenPort(), params)
	}
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
	rt := m.runtime
	switch be {
	case llamacpp.BackendVLLM:
		bin := llamacpp.ResolveVLLMPath(rt)
		if bin == "" {
			bin = "vllm"
		}
		return shellCommandDisplayMultiline(false, "", params.Env, vllmCommandWords(bin, modelPath, llamacpp.VLLMPort(), params))
	default:
		bin := llamacpp.ResolveLlamaServerPath(rt)
		if bin == "" {
			bin = "llama-server"
		}
		return shellCommandDisplayMultiline(false, "", params.Env, llamaCommandWords(bin, modelPath, llamacpp.ListenPort(), params))
	}
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

// runLlamaServerCmd runs llama-server for the selected GGUF in the foreground with the TUI suspended.
// Stdout and stderr go to the terminal (see tea.ExecProcess). Port matches LLAMA_SERVER_PORT / ListenPort.
//
// On Unix, the command is run under `sh -c` with a trailing `read` so the shell stays on the main screen
// until you press Enter after the server exits; then Bubble Tea restores the alternate screen.
// Read from /dev/tty first: some servers (e.g. vLLM) leave stdin unusable (EIO), but the controlling tty still works.
// Windows runs llama-server directly (no pause); use scrollback or an external terminal if needed.
func runLlamaServerCmd(modelPath string, rt llamacpp.RuntimeInfo, params ModelParams) tea.Cmd {
	bin := llamacpp.ResolveLlamaServerPath(rt)
	if bin == "" {
		return func() tea.Msg {
			return runServerErrMsg{
				err: fmt.Errorf("llama-server not found; set %s or install on PATH", llamacpp.EnvLlamaCppPath),
			}
		}
	}
	port := llamacpp.ListenPort()
	var c *exec.Cmd
	if runtime.GOOS == "windows" {
		args := []string{
			"-m", modelPath,
			"--alias", llamaServerAlias(modelPath),
			"--port", fmt.Sprintf("%d", port),
		}
		args = append(args, params.Args...)
		c = exec.Command(bin, args...)
		c.Env = mergeEnv(os.Environ(), params.Env)
	} else {
		c = exec.Command("sh", "-c", unixLlamaServerScript(bin, modelPath, port, params))
	}
	return tea.ExecProcess(c, func(err error) tea.Msg {
		return llamaServerExitedMsg{err: err}
	})
}

func newLlamaSplitCmd(modelPath string, rt llamacpp.RuntimeInfo, params ModelParams) (*exec.Cmd, error) {
	bin := llamacpp.ResolveLlamaServerPath(rt)
	if bin == "" {
		return nil, fmt.Errorf("llama-server not found; set %s or install on PATH", llamacpp.EnvLlamaCppPath)
	}
	port := llamacpp.ListenPort()
	args := []string{
		"-m", modelPath,
		"--alias", llamaServerAlias(modelPath),
		"--port", fmt.Sprintf("%d", port),
	}
	args = append(args, params.Args...)
	c := exec.Command(bin, args...)
	c.Env = mergeEnv(os.Environ(), params.Env)
	return c, nil
}

func newVLLMSplitCmd(modelDir string, rt llamacpp.RuntimeInfo, params ModelParams) (*exec.Cmd, error) {
	bin := llamacpp.ResolveVLLMPath(rt)
	if bin == "" {
		return nil, errVLLMNotFound()
	}
	port := llamacpp.VLLMPort()
	activate := llamacpp.ResolveVLLMActivateScript(bin)
	if runtime.GOOS == "windows" {
		if activate != "" {
			return nil, fmt.Errorf("vLLM venv activation is not supported on Windows from this app; run vllm from an activated shell or add vllm to PATH (detected %s)", activate)
		}
		args := []string{"serve", modelDir, "--port", fmt.Sprintf("%d", port)}
		args = append(args, params.Args...)
		c := exec.Command(bin, args...)
		c.Env = mergeEnv(os.Environ(), params.Env)
		return c, nil
	}
	c := exec.Command("sh", "-c", unixVLLMSplitScript(bin, modelDir, port, activate, params))
	c.Env = mergeEnv(os.Environ(), params.Env)
	return c, nil
}

func scanReaderLines(r io.Reader, ch chan<- tea.Msg, wg *sync.WaitGroup) {
	defer wg.Done()
	sc := bufio.NewScanner(r)
	for sc.Scan() {
		ch <- serverLogMsg{line: sc.Text()}
	}
}

// streamSplitServerCmd starts cmd with stdout/stderr pipes, streams lines as [serverLogMsg], then sends [llamaServerExitedMsg] and closes ch.
func streamSplitServerCmd(cmd *exec.Cmd, ch chan tea.Msg) {
	applySplitCmdSysProcAttr(cmd)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		ch <- llamaServerExitedMsg{err: err}
		close(ch)
		return
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		ch <- llamaServerExitedMsg{err: err}
		close(ch)
		return
	}
	if err := cmd.Start(); err != nil {
		ch <- llamaServerExitedMsg{err: err}
		close(ch)
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

// runLlamaServerSplitCmd starts llama-server in split-pane mode (log streaming into the TUI).
func runLlamaServerSplitCmd(modelPath string, rt llamacpp.RuntimeInfo, params ModelParams) tea.Cmd {
	return func() tea.Msg {
		cmd, err := newLlamaSplitCmd(modelPath, rt, params)
		if err != nil {
			return runServerErrMsg{err: err}
		}
		ch := make(chan tea.Msg, 64)
		bin := llamacpp.ResolveLlamaServerPath(rt)
		inv := formatLlamaServerInvocation(bin, modelPath, llamacpp.ListenPort(), params)
		go func() {
			ch <- serverLogMsg{line: inv}
			streamSplitServerCmd(cmd, ch)
		}()
		return serverSplitReadyMsg{cmd: cmd, ch: ch}
	}
}

// runVLLMServerSplitCmd starts vllm serve in split-pane mode.
func runVLLMServerSplitCmd(modelDir string, rt llamacpp.RuntimeInfo, params ModelParams) tea.Cmd {
	return func() tea.Msg {
		cmd, err := newVLLMSplitCmd(modelDir, rt, params)
		if err != nil {
			return runServerErrMsg{err: err}
		}
		ch := make(chan tea.Msg, 64)
		bin := llamacpp.ResolveVLLMPath(rt)
		activate := llamacpp.ResolveVLLMActivateScript(bin)
		inv := formatVLLMServerInvocation(bin, modelDir, llamacpp.VLLMPort(), activate, params)
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

// runVLLMServerCmd runs `vllm serve` for a Hugging Face-style model directory.
// Port matches VLLM_SERVER_PORT / VLLMPort (default 8000).
func runVLLMServerCmd(modelDir string, rt llamacpp.RuntimeInfo, params ModelParams) tea.Cmd {
	bin := llamacpp.ResolveVLLMPath(rt)
	if bin == "" {
		return func() tea.Msg {
			return runServerErrMsg{err: errVLLMNotFound()}
		}
	}
	port := llamacpp.VLLMPort()
	activate := llamacpp.ResolveVLLMActivateScript(bin)
	var c *exec.Cmd
	if runtime.GOOS == "windows" {
		if activate != "" {
			return func() tea.Msg {
				return runServerErrMsg{
					err: fmt.Errorf("vLLM venv activation is not supported on Windows from this app; run vllm from an activated shell or add vllm to PATH (detected %s)", activate),
				}
			}
		}
		args := []string{"serve", modelDir, "--port", fmt.Sprintf("%d", port)}
		args = append(args, params.Args...)
		c = exec.Command(bin, args...)
		c.Env = mergeEnv(os.Environ(), params.Env)
	} else {
		c = exec.Command("sh", "-c", unixVLLMServerScript(bin, modelDir, port, activate, params))
	}
	return tea.ExecProcess(c, func(err error) tea.Msg {
		return llamaServerExitedMsg{err: err}
	})
}
