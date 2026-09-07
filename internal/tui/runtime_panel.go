package tui

import (
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/flyingnobita/llml/internal/fsutil"
	"github.com/flyingnobita/llml/internal/models"
)

// Runtime panel row labels (sorted alphabetically in [RuntimePanelLines]).
const (
	runtimePanelLabelKoboldCppPath   = "koboldcpp path"
	runtimePanelLabelKoboldCppPort   = "koboldcpp port"
	runtimePanelLabelLlamaServerPath = "llama-server path"
	runtimePanelLabelLlamaServerPort = "llama.cpp port"
	runtimePanelLabelLlamaServerHost = "llama.cpp host"
	runtimePanelLabelOllamaHost      = "ollama host"
	runtimePanelLabelOllamaPath      = "ollama path"
	runtimePanelLabelVLLMPath        = "vllm path"
	runtimePanelLabelVLLMPort        = "vllm port"
	runtimePanelLabelVLLMHost        = "vllm host"
	runtimePanelLabelVLLMVenv        = "vllm venv path"
)

// runtimePanelEnvLabelWidth is the width of the left column (labels) in RuntimePanelLines.
const runtimePanelEnvLabelWidth = len(runtimePanelLabelLlamaServerPath) // 17; longest label

// portDisplay renders a resolved TCP port, or "—" when the runtime has not been probed.
func portDisplay(port int) string {
	if port <= 0 {
		return "—"
	}
	return strconv.Itoa(port)
}

// pathDisplay renders a configured path relative to the home directory, or "—" when unset.
func pathDisplay(v string) string {
	if v = strings.TrimSpace(v); v == "" {
		return "—"
	}
	return FormatPathDisplay(v, fsutil.HomeDir())
}

// valueOrDash renders a resolved string value, or "—" when it is empty.
func valueOrDash(v string) string {
	if v = strings.TrimSpace(v); v == "" {
		return "—"
	}
	return v
}

// llamaServerPathPanelDisplay returns the resolved llama-server binary path for the runtime
// panel, or "(server at :port)" when a server responds on the health probe but no binary was
// found, or "—" otherwise.
func llamaServerPathPanelDisplay(r models.RuntimeInfo) string {
	p := models.ResolveLlamaServerPath(r)
	if p != "" {
		return FormatPathDisplay(p, fsutil.HomeDir())
	}
	if r.ServerRunning {
		port := r.ProbePort
		if port <= 0 {
			port = r.LlamaServerPort
		}
		return fmt.Sprintf("(server at :%d)", port)
	}
	return "—"
}

// vllmPathPanelDisplay returns the resolved vllm binary path for the runtime panel, or "—".
func vllmPathPanelDisplay(r models.RuntimeInfo) string {
	p := models.ResolveVLLMPath(r)
	if p == "" {
		return "—"
	}
	return FormatPathDisplay(p, fsutil.HomeDir())
}

// vllmVenvPanelDisplay returns the value shown for vLLM venv in the runtime panel:
// the configured venv root when set, otherwise the root inferred from the same
// rules as vLLM activation (adjacent bin layout, $VLLM_PATH/.venv,
// dirname(vllm)/.venv), or "—" when none applies.
func vllmVenvPanelDisplay(r models.RuntimeInfo) string {
	if strings.TrimSpace(r.VLLMVenv) != "" {
		return pathDisplay(r.VLLMVenv)
	}
	vllmBin := models.ResolveVLLMPath(r)
	act := models.ResolveVLLMActivateScript(vllmBin, r.VLLMVenv, r.VLLMConfiguredPath)
	if root := models.VenvRootFromActivateScript(act); root != "" {
		return FormatPathDisplay(root, fsutil.HomeDir())
	}
	return "—"
}

func koboldCppPathPanelDisplay(r models.RuntimeInfo) string {
	p := models.ResolveKoboldCppPath(r)
	if p == "" {
		return "—"
	}
	return FormatPathDisplay(p, fsutil.HomeDir())
}

func ollamaPathPanelDisplay(r models.RuntimeInfo) string {
	p := models.ResolveOllamaPath(r)
	if p == "" {
		return "—"
	}
	return FormatPathDisplay(p, fsutil.HomeDir())
}

// RuntimePanelLines returns lines for the TUI footer: each row is a label (left) and its current
// value (right), sorted alphabetically by label. Binary paths use [models.ResolveLlamaServerPath]
// and [models.ResolveVLLMPath]; host and port rows come from the resolved settings
// carried on r. The venv row shows the configured venv root when set, otherwise
// the inferred venv root when activation would run. Lines are truncated to maxWidth display width.
func RuntimePanelLines(maxWidth int, r models.RuntimeInfo) []string {
	if maxWidth < MinModalInnerWidth {
		maxWidth = MinModalInnerWidth
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
		{runtimePanelLabelKoboldCppPath, koboldCppPathPanelDisplay(r)},
		{runtimePanelLabelKoboldCppPort, portDisplay(r.KoboldCppPort)},
		{runtimePanelLabelLlamaServerPath, llamaServerPathPanelDisplay(r)},
		{runtimePanelLabelLlamaServerHost, valueOrDash(r.LlamaServerHost)},
		{runtimePanelLabelLlamaServerPort, portDisplay(r.LlamaServerPort)},
		{runtimePanelLabelOllamaHost, valueOrDash(r.OllamaHost)},
		{runtimePanelLabelOllamaPath, ollamaPathPanelDisplay(r)},
		{runtimePanelLabelVLLMPath, vllmPathPanelDisplay(r)},
		{runtimePanelLabelVLLMPort, portDisplay(r.VLLMServerPort)},
		{runtimePanelLabelVLLMHost, valueOrDash(r.VLLMServerHost)},
		{runtimePanelLabelVLLMVenv, vllmVenvPanelDisplay(r)},
	}
	slices.SortFunc(rows, func(a, b struct{ key, value string }) int {
		return strings.Compare(a.key, b.key)
	})
	out := make([]string, len(rows))
	for i := range rows {
		out[i] = line(rows[i].key, rows[i].value)
	}
	return out
}
