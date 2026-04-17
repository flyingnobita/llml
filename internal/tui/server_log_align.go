package tui

import (
	"regexp"
	"strings"

	"github.com/charmbracelet/x/ansi"
)

// vLLM and similar stacks prefix log lines with "(ProcessName pid=N) LEVEL date [file:line]".
// Libraries such as tqdm write progress lines to the same stream without that prefix, so those
// lines appear flush-left while structured lines are indented — this regexp captures the full
// prefix so we can pad bare lines to match.
var vllmStructuredLogPrefix = regexp.MustCompile(
	`^\([^)]+\)\s+(?:DEBUG|INFO|WARNING|ERROR|CRITICAL)\s+\d{1,2}-\d{1,2}\s+\d{1,2}:\d{1,2}:\d{1,2}\s+\[[^\]]+\]\s*`,
)

// normalizeSplitServerLogLine pads stderr lines that omit the structured prefix (e.g. tqdm)
// so they align with vLLM-style lines. alignWidth is the maximum measured prefix width so far;
// it is updated when a structured line is seen. Invocation echo lines starting with "+" are
// left unchanged.
func normalizeSplitServerLogLine(line string, alignWidth *int) string {
	line = strings.TrimRight(line, "\r")
	if i := strings.LastIndex(line, "\r"); i >= 0 {
		line = line[i+1:]
	}
	if line == "" {
		return line
	}
	if trimmed := strings.TrimLeft(line, " "); strings.HasPrefix(trimmed, "+") {
		return line
	}
	if loc := vllmStructuredLogPrefix.FindStringIndex(line); loc != nil {
		w := ansi.StringWidth(line[:loc[1]])
		if w > *alignWidth {
			*alignWidth = w
		}
		return line
	}
	if strings.HasPrefix(strings.TrimLeft(line, " "), "(") {
		return line
	}
	pad := *alignWidth
	if pad == 0 && !looksLikeUnprefixedProgressLine(line) {
		return line
	}
	if pad == 0 {
		pad = defaultServerLogAlignWidth
	}
	if pad > serverLogAlignPadMax {
		pad = serverLogAlignPadMax
	}
	return strings.Repeat(" ", pad) + line
}

func looksLikeUnprefixedProgressLine(line string) bool {
	// tqdm ends with e.g. "?it/s]" or "1.25s/it]" (rate + closing bracket).
	if strings.Contains(line, "it/s]") || strings.Contains(line, "s/it]") {
		return true
	}
	if strings.Contains(line, "%|") && strings.Contains(line, "Completed") {
		return true
	}
	return false
}
