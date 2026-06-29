package tui

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"sync"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/atotto/clipboard"

	"github.com/flyingnobita/llml/internal/models"
)

// serverSpec holds the resolved parameters needed to build server commands for one launch.
type serverSpec struct {
	backend          models.ModelBackend
	bin              string
	port             int
	modelPath        string
	host             string
	params           ModelParams
	activateScript   string   // vLLM only: path to venv activate script
	mmprojPath       string   // auto-detected mmproj sidecar (llama/kobold only); empty when none found or profile already specifies one
	mmprojCandidates []string // non-empty when multiple mmproj siblings exist and disambiguation failed (mmprojPath is empty)
	mmprojMissing    bool     // image/audio tagged but no mmproj file found; launch proceeds without multimodal support
}

type launchArg struct {
	value       string
	quoteAlways bool
}

func rawLaunchArg(value string) launchArg {
	return launchArg{value: value}
}

func quotedLaunchArg(value string) launchArg {
	return launchArg{value: value, quoteAlways: true}
}

func (a launchArg) shellWord() string {
	if a.quoteAlways {
		return shellSingleQuoted(a.value)
	}
	return shellWord(a.value)
}

type launchBackend interface {
	args(serverSpec) []launchArg
}

type llamaLaunchBackend struct{}
type vllmLaunchBackend struct{}
type koboldLaunchBackend struct{}
type ollamaLaunchBackend struct{}

func (llamaLaunchBackend) args(s serverSpec) []launchArg {
	args := []launchArg{
		rawLaunchArg("--model"), quotedLaunchArg(s.modelPath),
		rawLaunchArg("--alias"), quotedLaunchArg(llamaServerAlias(s.modelPath)),
		rawLaunchArg("--host"), rawLaunchArg(s.host),
		rawLaunchArg("--port"), rawLaunchArg(fmt.Sprintf("%d", s.port)),
	}
	if s.mmprojPath != "" {
		args = append(args, rawLaunchArg("--mmproj"), quotedLaunchArg(s.mmprojPath))
	}
	return args
}

func (vllmLaunchBackend) args(s serverSpec) []launchArg {
	return []launchArg{
		rawLaunchArg("serve"), quotedLaunchArg(s.modelPath),
		rawLaunchArg("--served-model-name"), quotedLaunchArg(models.InferModelID(s.modelPath)),
		rawLaunchArg("--host"), rawLaunchArg(s.host),
		rawLaunchArg("--port"), rawLaunchArg(fmt.Sprintf("%d", s.port)),
	}
}

func (koboldLaunchBackend) args(s serverSpec) []launchArg {
	args := []launchArg{
		quotedLaunchArg(s.modelPath),
		rawLaunchArg("--port"), rawLaunchArg(fmt.Sprintf("%d", s.port)),
	}
	if s.mmprojPath != "" {
		args = append(args, rawLaunchArg("--mmproj"), quotedLaunchArg(s.mmprojPath))
	}
	return args
}

func (ollamaLaunchBackend) args(serverSpec) []launchArg {
	return []launchArg{rawLaunchArg("serve")}
}

func (s serverSpec) launchBackend() launchBackend {
	switch s.backend {
	case models.BackendOllama:
		return ollamaLaunchBackend{}
	case models.BackendVLLM:
		return vllmLaunchBackend{}
	case models.BackendKobold:
		return koboldLaunchBackend{}
	default:
		return llamaLaunchBackend{}
	}
}

func (s serverSpec) launchArgv() []launchArg {
	args := []launchArg{quotedLaunchArg(s.bin)}
	args = append(args, s.launchBackend().args(s)...)
	for _, a := range s.params.Args {
		args = append(args, rawLaunchArg(a))
	}
	return args
}

func launchArgValues(args []launchArg) []string {
	out := make([]string, len(args))
	for i, a := range args {
		out[i] = a.value
	}
	return out
}

// profileHasMMProj reports whether params already contains a --mmproj / -mm flag token,
// meaning the user has manually specified the projector and auto-injection should be skipped.
// Matches both space-separated form ("--mmproj /path") and equals form ("--mmproj=/path").
func profileHasMMProj(params ModelParams) bool {
	for _, a := range params.Args {
		if a == "--mmproj" || a == "-mm" ||
			strings.HasPrefix(a, "--mmproj=") || strings.HasPrefix(a, "-mm=") {
			return true
		}
	}
	return false
}

// profileWantsMMProj reports whether the active profile opts into mmproj injection
// via an "image" or "audio" use_case tag.
func profileWantsMMProj(params ModelParams) bool {
	for _, t := range params.UseCase.Tags {
		tl := strings.ToLower(strings.TrimSpace(t))
		if tl == "image" || tl == "audio" {
			return true
		}
	}
	return false
}

// resolveMMProjForSpec returns (mmprojPath, mmprojCandidates, mmprojMissing).
// Injection is opt-in: only resolves when the profile has an image/audio use_case tag.
// Returns ("", nil, false) when the profile already carries --mmproj or has no image/audio tag.
func resolveMMProjForSpec(modelPath string, params ModelParams) (string, []string, bool) {
	if profileHasMMProj(params) {
		return "", nil, false
	}
	if !profileWantsMMProj(params) {
		return "", nil, false
	}
	chosen, candidates := models.ResolveMMProj(modelPath)
	if chosen != "" {
		return chosen, nil, false
	}
	if len(candidates) > 0 {
		return "", candidates, false
	}
	return "", nil, true
}

// buildServerSpec resolves the binary, port, and venv for launching a server.
// When strict is true it returns an error if the binary is missing; when false it substitutes
// a placeholder name so display functions show a plausible command even before the runtime is configured.
func buildServerSpec(backend models.ModelBackend, modelPath string, params ModelParams, rt models.RuntimeInfo, strict bool) (serverSpec, error) {
	switch backend {
	case models.BackendOllama:
		bin := models.ResolveOllamaPath(rt)
		host := rt.OllamaHost
		if strings.TrimSpace(host) == "" {
			host = models.OllamaHost()
		}
		if strict && bin == "" && !rt.OllamaRunning {
			return serverSpec{}, errors.New(MissingOllamaFooterNote)
		}
		if bin == "" {
			bin = "ollama"
		}
		return serverSpec{
			backend:   models.BackendOllama,
			bin:       bin,
			host:      host,
			modelPath: modelPath,
			params:    params,
		}, nil
	case models.BackendVLLM:
		bin := models.ResolveVLLMPath(rt)
		activate := models.ResolveVLLMActivateScript(bin)
		host := rt.VLLMServerHost
		if strings.TrimSpace(host) == "" {
			host = models.VllmServerHost()
		}
		if strict {
			if bin == "" {
				return serverSpec{}, errors.New(MissingVLLMFooterNote)
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
			host:           host,
			port:           models.VLLMPort(),
			modelPath:      modelPath,
			params:         params,
			activateScript: activate,
		}, nil
	case models.BackendKobold:
		bin := models.ResolveKoboldCppPath(rt)
		if strict && bin == "" {
			return serverSpec{}, errors.New(MissingKoboldCppFooterNote)
		}
		if bin == "" {
			bin = "koboldcpp"
		}
		mmprojPath, mmprojCandidates, mmprojMissing := resolveMMProjForSpec(modelPath, params)
		return serverSpec{
			backend:          models.BackendKobold,
			bin:              bin,
			port:             models.KoboldCppPort(),
			modelPath:        modelPath,
			params:           params,
			mmprojPath:       mmprojPath,
			mmprojCandidates: mmprojCandidates,
			mmprojMissing:    mmprojMissing,
		}, nil
	default: // BackendLlama
		bin := models.ResolveLlamaServerPath(rt)
		host := rt.LlamaServerHost
		if strings.TrimSpace(host) == "" {
			host = models.LlamaServerHost()
		}
		if strict && bin == "" {
			return serverSpec{}, errors.New(MissingLlamaServerFooterNote)
		}
		if bin == "" {
			bin = "llama-server"
		}
		mmprojPath, mmprojCandidates, mmprojMissing := resolveMMProjForSpec(modelPath, params)
		return serverSpec{
			backend:          models.BackendLlama,
			bin:              bin,
			host:             host,
			port:             models.ListenPort(),
			modelPath:        modelPath,
			params:           params,
			mmprojPath:       mmprojPath,
			mmprojCandidates: mmprojCandidates,
			mmprojMissing:    mmprojMissing,
		}, nil
	}
}

// mmprojNote returns a one-line warning string when mmproj state is abnormal for this spec.
// Empty string means no warning needed.
func (s serverSpec) mmprojNote() string {
	if s.mmprojMissing {
		return "⚠ image/audio profile — no mmproj file found; launching without multimodal support"
	}
	if len(s.mmprojCandidates) > 0 {
		return "⚠ multiple mmproj files found; add --mmproj to profile args to select one"
	}
	return ""
}

// commandWords returns the escaped shell tokens for the server invocation (same order as directArgs).
func (s serverSpec) commandWords() []string {
	argv := s.launchArgv()
	words := make([]string, len(argv))
	for i, a := range argv {
		words[i] = a.shellWord()
	}
	return words
}

// commandLine returns the single-line shell form of the invocation (env prefix + argv).
func (s serverSpec) commandLine() string {
	return strings.TrimSpace(shellEnvPrefix(s.params.Env) + strings.Join(s.commandWords(), " "))
}

// directArgs builds the argv slice for direct binary execution (no sh wrapper).
func (s serverSpec) directArgs() []string {
	return launchArgValues(s.launchArgv()[1:])
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
// G204: intentional subprocess launch — llml's purpose is launching model servers.
func (s serverSpec) foregroundCmd() *exec.Cmd { //nolint:gosec
	if runtime.GOOS == "windows" {
		c := exec.Command(s.bin, s.directArgs()...) //nolint:gosec //nolint:gosec
		c.Env = mergeEnv(os.Environ(), s.params.Env)
		return c
	}
	return exec.Command("sh", "-c", s.unixForegroundScript()) //nolint:gosec
}

// splitCmd returns an *exec.Cmd for split-pane log streaming.
// Unix vLLM uses sh -c with 2>&1 (stderr merge) and also sets Env via mergeEnv — double propagation
// is intentional and matches the original per-backend split paths.
// All other cases run the binary directly with merged env.
//
//nolint:gosec // G204: intentional subprocess launch.
func (s serverSpec) splitCmd() *exec.Cmd {
	if s.backend == models.BackendVLLM && runtime.GOOS != "windows" {
		c := exec.Command("sh", "-c", s.unixSplitScript()) //nolint:gosec
		c.Env = mergeEnv(os.Environ(), s.params.Env)
		return c
	}
	c := exec.Command(s.bin, s.directArgs()...) //nolint:gosec
	c.Env = mergeEnv(os.Environ(), s.params.Env)
	return c
}

// invocationEcho returns the multi-line "+ ..." display string for the split-pane log header.
func (s serverSpec) invocationEcho() string {
	if s.backend == models.BackendOllama {
		lines := []string{
			"+ " + strings.TrimSpace(shellEnvPrefix(s.params.Env)+shellSingleQuoted(s.bin)+" serve"),
			"+ preload " + s.modelPath + " on " + s.host + " (keep_alive=-1)",
		}
		return strings.Join(lines, "\n")
	}
	return shellCommandDisplayMultiline(true, s.activateScript, s.params.Env, s.commandWords())
}

// previewLine returns the multi-line command for the launch preview and clipboard
// (no "+ " prefix, no activate wrapper — shows the raw executable invocation).
func (s serverSpec) previewLine() string {
	if s.backend == models.BackendOllama {
		return strings.Join([]string{
			strings.TrimSpace(shellEnvPrefix(s.params.Env) + shellSingleQuoted(s.bin) + " serve"),
			fmt.Sprintf("curl http://%s/api/generate -d '{\"model\":\"%s\",\"keep_alive\":-1,\"stream\":false}'", s.host, s.modelPath),
		}, "\n")
	}
	return shellCommandDisplayMultiline(false, "", s.params.Env, s.commandWords())
}

// splitServerInvocationEcho returns the first line written to the split-pane log when R is pressed.
// It uses the selected model, active parameter profile, and runtime info exactly as runSplitServerCmd.
func splitServerInvocationEcho(m Model) string {
	modelPath, _ := m.SelectedModel()
	if modelPath == "" {
		return ""
	}
	params, ok := modelParamsForLaunchPreview(m)
	if !ok {
		return ""
	}
	be := m.resolveEffectiveBackend()
	spec, _ := buildServerSpec(be, modelPath, params, m.runtime, false)
	return spec.invocationEcho()
}

// launchPreviewCommandLine returns the shell form of the server command for the table preview and
// clipboard: same tokens as the split-pane subprocess, formatted on multiple lines, but without the
// "+ " log marker or the ". /path/activate &&" venv wrapper used when launching vLLM.
func launchPreviewCommandLine(m Model) string {
	modelPath, _ := m.SelectedModel()
	if modelPath == "" {
		return ""
	}
	params, ok := modelParamsForLaunchPreview(m)
	if !ok {
		return ""
	}
	be := m.resolveEffectiveBackend()
	spec, _ := buildServerSpec(be, modelPath, params, m.runtime, false)
	return spec.previewLine()
}

// launchPreviewCmdAndNote returns both the preview command line and the mmproj note from a single
// buildServerSpec call, avoiding the double os.ReadDir that occurs when the two are fetched
// independently via launchPreviewCommandLine + launchPreviewMMProjNote.
func launchPreviewCmdAndNote(m Model) (string, string) {
	modelPath, _ := m.SelectedModel()
	if modelPath == "" {
		return "", ""
	}
	params, ok := modelParamsForLaunchPreview(m)
	if !ok {
		return "", ""
	}
	be := m.resolveEffectiveBackend()
	spec, _ := buildServerSpec(be, modelPath, params, m.runtime, false)
	return spec.previewLine(), spec.mmprojNote()
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
		ch := make(chan tea.Msg, ServerSplitChannelBuffer)
		inv := spec.invocationEcho()
		go func() {
			ch <- serverLogMsg{line: inv}
			streamSplitServerCmd(cmd, ch)
		}()
		return serverSplitReadyMsg{cmd: cmd, ch: ch}
	}
}

func startOllamaDaemon(spec serverSpec) error {
	cmd := exec.Command(spec.bin, "serve") //nolint:gosec
	cmd.Env = mergeEnv(os.Environ(), spec.params.Env)
	devNull, err := os.OpenFile(os.DevNull, os.O_WRONLY, 0)
	if err != nil {
		return err
	}
	defer func() { _ = devNull.Close() }()
	cmd.Stdout = devNull
	cmd.Stderr = devNull
	applyBackgroundCmdSysProcAttr(cmd)
	return cmd.Start()
}

var (
	startOllamaDaemonFn = startOllamaDaemon
	waitForOllamaFn     = waitForOllama
	probeOllamaFn       = models.ProbeOllama
	preloadOllamaFn     = models.PreloadOllamaModel
)

func waitForOllama() bool {
	deadline := time.Now().Add(OllamaStartupTimeout)
	for time.Now().Before(deadline) {
		if probeOllamaFn() {
			return true
		}
		time.Sleep(OllamaPollInterval)
	}
	return probeOllamaFn()
}

type ollamaReadyResult struct {
	Started bool
}

func ensureOllamaReady(spec serverSpec) (ollamaReadyResult, error) {
	debugf("ensureOllamaReady: probe start bin=%q host=%q", spec.bin, spec.host)
	if probeOllamaFn() {
		debugf("ensureOllamaReady: Ollama already reachable")
		return ollamaReadyResult{}, nil
	}
	if err := startOllamaDaemonFn(spec); err != nil {
		debugf("ensureOllamaReady: start failed: %v", err)
		return ollamaReadyResult{}, err
	}
	if !waitForOllamaFn() {
		debugf("ensureOllamaReady: waitForOllama timed out")
		return ollamaReadyResult{}, fmt.Errorf("ollama did not become ready on %s", spec.host)
	}
	debugf("ensureOllamaReady: Ollama became ready")
	return ollamaReadyResult{Started: true}, nil
}

func discoveryOllamaSpec(rt models.RuntimeInfo) serverSpec {
	host := rt.OllamaHost
	if strings.TrimSpace(host) == "" {
		host = models.OllamaHost()
	}
	return serverSpec{
		backend: models.BackendOllama,
		bin:     models.ResolveOllamaPath(rt),
		host:    host,
	}
}

func runOllamaLaunchCmd(spec serverSpec) tea.Cmd {
	startNote := fmt.Sprintf("Loading %s into Ollama on %s...", spec.modelPath, spec.host)
	return tea.Batch(
		func() tea.Msg { return ollamaLaunchStartedMsg{note: startNote} },
		func() tea.Msg {
			ready, err := ensureOllamaReady(spec)
			if err != nil {
				return ollamaLaunchDoneMsg{err: err}
			}
			if err := preloadOllamaFn(spec.modelPath); err != nil {
				return ollamaLaunchDoneMsg{err: err}
			}
			note := fmt.Sprintf("Loaded %s into Ollama on %s", spec.modelPath, spec.host)
			if ready.Started {
				note = fmt.Sprintf("Started Ollama and loaded %s on %s", spec.modelPath, spec.host)
			}
			return ollamaLaunchDoneMsg{note: note}
		},
	)
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
