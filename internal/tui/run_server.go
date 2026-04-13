package tui

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/flyingnobita/llm-launch/internal/llamacpp"
)

// shellSingleQuoted returns s wrapped in single quotes for POSIX sh (safe for paths with spaces).
func shellSingleQuoted(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'"'"'`) + "'"
}

func llamaCommandLine(bin, modelPath string, port int, params ModelParams) string {
	envP := shellEnvPrefix(params.Env)
	cmdPart := fmt.Sprintf("%s -m %s --port %d", shellSingleQuoted(bin), shellSingleQuoted(modelPath), port)
	if x := joinShellArgv(params.Args); x != "" {
		cmdPart += " " + x
	}
	return strings.TrimSpace(envP + cmdPart)
}

func vllmCommandLine(bin, modelDir string, port int, params ModelParams) string {
	envP := shellEnvPrefix(params.Env)
	cmdPart := fmt.Sprintf("%s serve %s --port %d", shellSingleQuoted(bin), shellSingleQuoted(modelDir), port)
	if x := joinShellArgv(params.Args); x != "" {
		cmdPart += " " + x
	}
	return strings.TrimSpace(envP + cmdPart)
}

// formatLlamaServerInvocation is a copy-paste style one-liner (with a leading "+ ") printed before launch.
func formatLlamaServerInvocation(bin, modelPath string, port int, params ModelParams) string {
	return "+ " + llamaCommandLine(bin, modelPath, port, params)
}

// formatVLLMServerInvocation is a copy-paste style one-liner printed before vllm serve.
// If activateScript is non-empty, the line uses `. activate && ...` (POSIX).
func formatVLLMServerInvocation(bin, modelDir string, port int, activateScript string, params ModelParams) string {
	line := vllmCommandLine(bin, modelDir, port, params)
	if activateScript != "" {
		return "+ . " + shellSingleQuoted(activateScript) + " && " + line
	}
	return "+ " + line
}

// unixLlamaServerScript echoes the invocation, runs llama-server, then waits for Enter so logs stay readable before the TUI redraws.
func unixLlamaServerScript(bin, modelPath string, port int, params ModelParams) string {
	inv := formatLlamaServerInvocation(bin, modelPath, port, params)
	runLine := llamaCommandLine(bin, modelPath, port, params)
	return fmt.Sprintf(`printf '%%s\n' %s
%s
echo
echo 'Press Enter to return to llm-launch...'
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
echo 'Press Enter to return to llm-launch...'
read -r _ </dev/tty || read -r _
`, shellSingleQuoted(inv), runLine)
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
		args := []string{"-m", modelPath, "--port", fmt.Sprintf("%d", port)}
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

// runVLLMServerCmd runs `vllm serve` for a Hugging Face-style model directory.
// Port matches VLLM_SERVER_PORT / VLLMPort (default 8000).
func runVLLMServerCmd(modelDir string, rt llamacpp.RuntimeInfo, params ModelParams) tea.Cmd {
	bin := llamacpp.ResolveVLLMPath(rt)
	if bin == "" {
		return func() tea.Msg {
			return runServerErrMsg{
				err: fmt.Errorf("vllm not found; set %s (project dir; we use vllm or .venv/bin/vllm) or %s (venv root), or install vllm on PATH", llamacpp.EnvVLLMPath, llamacpp.EnvVLLMVenv),
			}
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
