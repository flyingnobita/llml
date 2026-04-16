package llamacpp

import (
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
)

func formatBinLabel(abs string) string {
	if abs == "" {
		return "—"
	}
	return "✓"
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

// Runtime panel row labels (sorted alphabetically in [RuntimePanelLines]).
const (
	runtimePanelLabelLlamaServerPath = "llama-server path"
	runtimePanelLabelLlamaServerPort = "llama-server port"
	runtimePanelLabelVLLMPath        = "vllm path"
	runtimePanelLabelVLLMPort        = "vllm port"
	runtimePanelLabelVLLMVenv        = "vllm venv path"
)

// runtimePanelEnvLabelWidth is the width of the left column (labels) in RuntimePanelLines.
const runtimePanelEnvLabelWidth = len(runtimePanelLabelLlamaServerPath) // 17; longest label

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
	home, _ := os.UserHomeDir()
	return FormatPathDisplay(v, home)
}

// llamaServerPathPanelDisplay returns the resolved llama-server binary path for the runtime
// panel, or "(server at :port)" when a server responds on the health probe but no binary was
// found, or "—" otherwise.
func llamaServerPathPanelDisplay(r RuntimeInfo) string {
	p := ResolveLlamaServerPath(r)
	if p != "" {
		home, _ := os.UserHomeDir()
		return FormatPathDisplay(p, home)
	}
	if r.ServerRunning {
		port := r.ProbePort
		if port <= 0 {
			port = ListenPort()
		}
		return fmt.Sprintf("(server at :%d)", port)
	}
	return "—"
}

// vllmPathPanelDisplay returns the resolved vllm binary path for the runtime panel, or "—".
func vllmPathPanelDisplay(r RuntimeInfo) string {
	p := ResolveVLLMPath(r)
	if p == "" {
		return "—"
	}
	home, _ := os.UserHomeDir()
	return FormatPathDisplay(p, home)
}

// vllmVenvPanelDisplay returns the value shown for vLLM venv in the runtime panel: the env var
// when set, otherwise the venv root inferred from the same rules as vLLM activation (adjacent
// bin layout, $VLLM_PATH/.venv, dirname(vllm)/.venv), or "—" when none applies.
func vllmVenvPanelDisplay(r RuntimeInfo) string {
	if strings.TrimSpace(os.Getenv(EnvVLLMVenv)) != "" {
		return pathEnvDisplay(EnvVLLMVenv)
	}
	vllmBin := ResolveVLLMPath(r)
	act := ResolveVLLMActivateScript(vllmBin)
	if root := venvRootFromActivateScript(act); root != "" {
		home, _ := os.UserHomeDir()
		return FormatPathDisplay(root, home)
	}
	return "—"
}

// RuntimePanelLines returns lines for the TUI footer: each row is a label (left) and its current
// value (right), sorted alphabetically by label. Binary paths use [ResolveLlamaServerPath] and
// [ResolveVLLMPath]; port rows use the env when set, otherwise the effective default ([ListenPort] /
// [VLLMPort]). The venv row shows VLLM_VENV when set, otherwise the inferred venv root when
// activation would run. Lines are truncated to maxWidth display width.
func RuntimePanelLines(maxWidth int, r RuntimeInfo) []string {
	if maxWidth < 24 {
		maxWidth = 24
	}
	valW := maxWidth - runtimePanelEnvLabelWidth - 1
	if valW < 8 {
		valW = 8
	}
	line := func(label, value string) string {
		v := TruncateRunes(value, valW)
		s := fmt.Sprintf("%-*s %s", runtimePanelEnvLabelWidth, label, v)
		return TruncateRunes(s, maxWidth)
	}
	rows := []struct {
		key   string
		value string
	}{
		{runtimePanelLabelLlamaServerPath, llamaServerPathPanelDisplay(r)},
		{runtimePanelLabelLlamaServerPort, portEnvDisplay(EnvLlamaServerPort, ListenPort())},
		{runtimePanelLabelVLLMPath, vllmPathPanelDisplay(r)},
		{runtimePanelLabelVLLMPort, portEnvDisplay(EnvVLLMServerPort, VLLMPort())},
		{runtimePanelLabelVLLMVenv, vllmVenvPanelDisplay(r)},
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].key < rows[j].key })
	out := make([]string, len(rows))
	for i := range rows {
		out[i] = line(rows[i].key, rows[i].value)
	}
	return out
}
