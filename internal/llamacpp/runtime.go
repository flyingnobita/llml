package llamacpp

import (
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
)

// Environment variables for locating llama.cpp binaries and probing a running server.
const (
	EnvLlamaCppPath    = "LLAMA_CPP_PATH"
	EnvLlamaServerPort = "LLAMA_SERVER_PORT"
	// EnvVLLMServerPort is the TCP port for vllm serve (default 8000 when unset or invalid; matches vLLM's typical default).
	EnvVLLMServerPort = "VLLM_SERVER_PORT"
	// EnvVLLMPath is an optional directory containing a `vllm` executable (checked before PATH).
	EnvVLLMPath = "VLLM_PATH"
	// EnvVLLMVenv is an optional Python venv root (directory containing bin/activate on Unix).
	// When set (or when $VLLM_PATH/.venv or dirname(vllm)/.venv exists), R sources activate before vllm serve.
	EnvVLLMVenv = "VLLM_VENV"
)

const defaultLlamaServerPort = 8080

const defaultVLLMServerPort = 8000

// RuntimeInfo describes detected llama-cli / llama-server binaries, optional vLLM CLI, and optional running server.
type RuntimeInfo struct {
	LlamaCLIPath    string
	LlamaServerPath string
	VLLMPath        string
	ServerRunning   bool
	ProbePort       int // port used when ServerRunning is true (0 if not probed)
}

// Available is true if either binary was found, vLLM was found, or a llama-server responded on the health probe.
func (r RuntimeInfo) Available() bool {
	return r.LlamaCLIPath != "" || r.LlamaServerPath != "" || r.VLLMPath != "" || r.ServerRunning
}

// Summary is a single-line status for the TUI (no trailing newline).
func (r RuntimeInfo) Summary() string {
	var base string
	switch {
	case r.LlamaCLIPath != "" && r.LlamaServerPath != "":
		base = fmt.Sprintf("llama.cpp: cli %s · server %s", formatBinLabel(r.LlamaCLIPath), formatBinLabel(r.LlamaServerPath))
	case r.LlamaCLIPath != "":
		base = fmt.Sprintf("llama.cpp: cli %s · server —", formatBinLabel(r.LlamaCLIPath))
	case r.LlamaServerPath != "":
		base = fmt.Sprintf("llama.cpp: cli — · server %s", formatBinLabel(r.LlamaServerPath))
	case r.ServerRunning:
		base = fmt.Sprintf("llama.cpp: binaries not on PATH — server running :%d", r.ProbePort)
	default:
		base = "llama.cpp: not found — set " + EnvLlamaCppPath + " or install to PATH (Homebrew: ensure /opt/homebrew/bin is on PATH)"
	}
	v := "vllm: —"
	if r.VLLMPath != "" {
		v = "vllm: ✓"
	}
	return base + " · " + v
}

func formatBinLabel(abs string) string {
	if abs == "" {
		return "—"
	}
	return "✓"
}

// runtimePanelEnvLabelWidth is the width of the left column (env var names) in RuntimePanelLines.
const runtimePanelEnvLabelWidth = len(EnvLlamaServerPort) // 17; same as len(EnvVLLMServerPort)

// portEnvDisplay returns the env value when set, otherwise the effective TCP port as decimal.
func portEnvDisplay(envKey string, effective int) string {
	if v := strings.TrimSpace(os.Getenv(envKey)); v != "" {
		return v
	}
	return strconv.Itoa(effective)
}

// pathEnvDisplay returns a display value for a path env var, or "—" when unset.
func pathEnvDisplay(envKey string) string {
	v := strings.TrimSpace(os.Getenv(envKey))
	if v == "" {
		return "—"
	}
	return FormatPathDisplay(v)
}

// RuntimePanelLines returns lines for the TUI footer: each row is an environment variable name
// (left) and its current value (right). Path vars use the process environment; port vars use
// the env when set, otherwise the effective default (ListenPort / VLLMPort).
// Lines are truncated to maxWidth display width.
func RuntimePanelLines(maxWidth int) []string {
	if maxWidth < 24 {
		maxWidth = 24
	}
	valW := maxWidth - runtimePanelEnvLabelWidth - 1
	if valW < 8 {
		valW = 8
	}
	line := func(envKey, value string) string {
		v := TruncateRunes(value, valW)
		s := fmt.Sprintf("%-*s %s", runtimePanelEnvLabelWidth, envKey, v)
		return TruncateRunes(s, maxWidth)
	}
	return []string{
		line(EnvLlamaCppPath, pathEnvDisplay(EnvLlamaCppPath)),
		line(EnvVLLMPath, pathEnvDisplay(EnvVLLMPath)),
		line(EnvVLLMVenv, pathEnvDisplay(EnvVLLMVenv)),
		line(EnvLlamaServerPort, portEnvDisplay(EnvLlamaServerPort, ListenPort())),
		line(EnvVLLMServerPort, portEnvDisplay(EnvVLLMServerPort, VLLMPort())),
	}
}

// venvActivateScriptPath returns the shell script path for a Python venv root (the directory
// that contains bin/activate on Unix or Scripts/activate.bat on Windows).
func venvActivateScriptPath(venvRoot string) string {
	venvRoot = filepath.Clean(venvRoot)
	if runtime.GOOS == "windows" {
		return filepath.Join(venvRoot, "Scripts", "activate.bat")
	}
	return filepath.Join(venvRoot, "bin", "activate")
}

func isRegularFile(path string) bool {
	st, err := os.Stat(path)
	return err == nil && !st.IsDir() && st.Mode().IsRegular()
}

// activateAdjacentToVLLM returns the activate script in the same directory as the vllm
// executable (standard layout: .venv/bin/vllm next to .venv/bin/activate).
func activateAdjacentToVLLM(vllmBin string) string {
	if vllmBin == "" {
		return ""
	}
	dir := filepath.Dir(vllmBin)
	if runtime.GOOS == "windows" {
		p := filepath.Join(dir, "activate.bat")
		if isRegularFile(p) {
			return p
		}
		return ""
	}
	p := filepath.Join(dir, "activate")
	if isRegularFile(p) {
		return p
	}
	return ""
}

// ResolveVLLMActivateScript returns an activate script path to source before `vllm serve`, or ""
// when no venv should be activated. Resolution order:
//  1. activate next to vllm in the same bin/ directory (e.g. .venv/bin/activate + .venv/bin/vllm)
//  2. VLLM_VENV (venv root)
//  3. $VLLM_PATH/.venv (when VLLM_PATH is set)
//  4. dirname(vllmBin)/.venv (project-local venv when vllm is a top-level script)
func ResolveVLLMActivateScript(vllmBin string) string {
	if s := activateAdjacentToVLLM(vllmBin); s != "" {
		return s
	}
	try := func(venvRoot string) string {
		p := venvActivateScriptPath(venvRoot)
		if isRegularFile(p) {
			return p
		}
		return ""
	}
	if d := strings.TrimSpace(os.Getenv(EnvVLLMVenv)); d != "" {
		if s := try(d); s != "" {
			return s
		}
	}
	if d := strings.TrimSpace(os.Getenv(EnvVLLMPath)); d != "" {
		if s := try(filepath.Join(filepath.Clean(d), ".venv")); s != "" {
			return s
		}
	}
	if vllmBin != "" {
		if s := try(filepath.Join(filepath.Dir(vllmBin), ".venv")); s != "" {
			return s
		}
	}
	return ""
}

// vllmBinaryInVenvRoot returns $venvRoot/bin/vllm (Unix) or $venvRoot/Scripts/vllm.exe (Windows) if present.
func vllmBinaryInVenvRoot(venvRoot string) string {
	venvRoot = filepath.Clean(venvRoot)
	if runtime.GOOS == "windows" {
		p := filepath.Join(venvRoot, "Scripts", "vllm.exe")
		if isRegularFile(p) {
			return p
		}
		return ""
	}
	p := filepath.Join(venvRoot, "bin", "vllm")
	if isRegularFile(p) {
		return p
	}
	return ""
}

// vllmBinaryInProjectDotVenv returns $project/.venv/bin/vllm when that file exists.
func vllmBinaryInProjectDotVenv(projectRoot string) string {
	return vllmBinaryInVenvRoot(filepath.Join(filepath.Clean(projectRoot), ".venv"))
}

// DiscoverRuntime locates llama-cli and llama-server using LLAMA_CPP_PATH, common install
// directories (including Homebrew on Apple Silicon), then PATH. If neither binary exists,
// it probes http://127.0.0.1:{LLAMA_SERVER_PORT}/health (default port 8080) with a short timeout.
func DiscoverRuntime() RuntimeInfo {
	cli := findLlamaBinary("llama-cli")
	srv := findLlamaBinary("llama-server")
	port := ListenPort()
	info := RuntimeInfo{
		LlamaCLIPath:    cli,
		LlamaServerPath: srv,
		VLLMPath:        findVLLMBinary(),
		ProbePort:       port,
	}
	if cli == "" && srv == "" {
		if probeLlamaServerHealth(port) {
			info.ServerRunning = true
		}
	}
	return info
}

// ListenPort returns the TCP port from LLAMA_SERVER_PORT, or 8080 if unset or invalid.
func ListenPort() int {
	if v := os.Getenv(EnvLlamaServerPort); v != "" {
		if p, err := strconv.Atoi(strings.TrimSpace(v)); err == nil && p > 0 && p <= 65535 {
			return p
		}
	}
	return defaultLlamaServerPort
}

// VLLMPort returns the TCP port from VLLM_SERVER_PORT, or 8000 if unset or invalid.
func VLLMPort() int {
	if v := os.Getenv(EnvVLLMServerPort); v != "" {
		if p, err := strconv.Atoi(strings.TrimSpace(v)); err == nil && p > 0 && p <= 65535 {
			return p
		}
	}
	return defaultVLLMServerPort
}

// ResolveLlamaServerPath returns the detected llama-server binary path, or the first match on PATH.
func ResolveLlamaServerPath(r RuntimeInfo) string {
	if r.LlamaServerPath != "" {
		return r.LlamaServerPath
	}
	if p, err := exec.LookPath("llama-server"); err == nil {
		return p
	}
	return ""
}

// ResolveVLLMPath returns the detected vllm binary path, or the first match on PATH.
func ResolveVLLMPath(r RuntimeInfo) string {
	if r.VLLMPath != "" {
		return r.VLLMPath
	}
	if p, err := exec.LookPath("vllm"); err == nil {
		return p
	}
	return ""
}

func findVLLMBinary() string {
	if dir := os.Getenv(EnvVLLMPath); dir != "" {
		clean := filepath.Clean(dir)
		candidate := filepath.Join(clean, "vllm")
		if st, err := os.Stat(candidate); err == nil && !st.IsDir() && st.Mode().IsRegular() {
			return candidate
		}
		// vllm often lives only at $VLLM_PATH/.venv/bin/vllm until the venv is activated.
		if p := vllmBinaryInProjectDotVenv(clean); p != "" {
			return p
		}
	}
	if d := strings.TrimSpace(os.Getenv(EnvVLLMVenv)); d != "" {
		if p := vllmBinaryInVenvRoot(d); p != "" {
			return p
		}
	}
	var common []string
	common = append(common,
		"/usr/local/bin",
		"/opt/homebrew/bin",
	)
	if home, err := os.UserHomeDir(); err == nil {
		common = append(common, filepath.Join(home, ".local", "bin"))
	}
	for _, dir := range common {
		candidate := filepath.Join(dir, "vllm")
		if st, err := os.Stat(candidate); err == nil && !st.IsDir() && st.Mode().IsRegular() {
			return candidate
		}
	}
	if p, err := exec.LookPath("vllm"); err == nil {
		return p
	}
	return ""
}

func findLlamaBinary(name string) string {
	// 1. LLAMA_CPP_PATH/<name>
	if dir := os.Getenv(EnvLlamaCppPath); dir != "" {
		candidate := filepath.Join(filepath.Clean(dir), name)
		if st, err := os.Stat(candidate); err == nil && !st.IsDir() && st.Mode().IsRegular() {
			return candidate
		}
	}

	// 2. Common install locations (Homebrew Apple Silicon, Linux user local, source build)
	var common []string
	common = append(common,
		"/usr/local/bin",
		"/opt/homebrew/bin",
		"/opt/llama.cpp/build/bin",
	)
	if home, err := os.UserHomeDir(); err == nil {
		common = append(common, filepath.Join(home, ".local", "bin"))
	}
	for _, dir := range common {
		candidate := filepath.Join(dir, name)
		if st, err := os.Stat(candidate); err == nil && !st.IsDir() && st.Mode().IsRegular() {
			return candidate
		}
	}

	// 3. PATH
	if p, err := exec.LookPath(name); err == nil {
		return p
	}
	return ""
}

// probeLlamaServerHealth GETs /health on 127.0.0.1 (avoids localhost IPv6/IPv4 ambiguity).
func probeLlamaServerHealth(port int) bool {
	url := fmt.Sprintf("http://127.0.0.1:%d/health", port)
	client := &http.Client{Timeout: 2 * time.Second}
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return false
	}
	resp, err := client.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}
