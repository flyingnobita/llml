package tui

import (
	"path/filepath"
	"regexp"
	"strings"
)

// llamaServerAlias returns the API model id alias: leaf name of the GGUF path (matches File Name column).
func llamaServerAlias(modelPath string) string {
	return filepath.Base(filepath.Clean(modelPath))
}

// shellSingleQuoted returns s wrapped in single quotes for POSIX sh (safe for paths with spaces).
func shellSingleQuoted(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'"'"'`) + "'"
}

// shellSafeWord matches strings that are safe to pass unquoted in a POSIX shell command.
var shellSafeWord = regexp.MustCompile(`^[./a-zA-Z0-9_:=@,.+-]+$`)

// shellWord quotes a for POSIX sh when necessary: unquoted when the word is clearly safe,
// single-quoted otherwise. Makes echoed commands readable while remaining safe.
func shellWord(a string) string {
	if a != "" && shellSafeWord.MatchString(a) {
		return a
	}
	return shellSingleQuoted(a)
}

// shellEnvPrefix emits VAR='value' assignments for a shell command prefix (empty if none).
func shellEnvPrefix(env []EnvVar) string {
	var b strings.Builder
	for _, e := range env {
		if e.Key == "" {
			continue
		}
		b.WriteString(e.Key)
		b.WriteByte('=')
		b.WriteString(shellSingleQuoted(e.Value))
		b.WriteByte(' ')
	}
	return b.String()
}

// joinShellArgv joins args into a space-separated shell argv string, quoting as needed.
func joinShellArgv(args []string) string {
	if len(args) == 0 {
		return ""
	}
	var b strings.Builder
	for i, a := range args {
		if i > 0 {
			b.WriteByte(' ')
		}
		b.WriteString(shellWord(a))
	}
	return b.String()
}

// pairFlagValueForShellDisplay merges "--flag value" pairs into one string per display line
// when the value does not look like another flag.
func pairFlagValueForShellDisplay(tokens []string) []string {
	if len(tokens) == 0 {
		return nil
	}
	var out []string
	for i := 0; i < len(tokens); i++ {
		if strings.HasPrefix(tokens[i], "-") && i+1 < len(tokens) && !strings.HasPrefix(tokens[i+1], "-") {
			out = append(out, tokens[i]+" "+tokens[i+1])
			i++
		} else {
			out = append(out, tokens[i])
		}
	}
	return out
}

// shellCommandDisplayMultiline builds a POSIX/bash-style command for display and copy-paste: each
// env assignment on its own line; argv is split into lines with backslash continuation, merging
// each flag with its value on one line when applicable. If activateScript is non-empty, the first
// line is `. '/path/to/activate' && \`. Continuation lines after the first argv line are indented
// with [shellDisplayArgIndent] when withPlusPrefix is false (launch preview, clipboard). If
// withPlusPrefix is true, the first line starts with "+ " (split-pane log style) and every
// continuation line is indented to align under the prefix.
func shellCommandDisplayMultiline(withPlusPrefix bool, activateScript string, env []EnvVar, words []string) string {
	words = pairFlagValueForShellDisplay(words)
	var raw []string
	if activateScript != "" {
		raw = append(raw, ". "+shellSingleQuoted(activateScript)+" && \\")
	}
	for _, e := range env {
		if e.Key == "" {
			continue
		}
		raw = append(raw, e.Key+"="+shellSingleQuoted(e.Value)+" \\")
	}
	wordStartIdx := len(raw)
	for i, w := range words {
		if i < len(words)-1 {
			raw = append(raw, w+" \\")
		} else {
			raw = append(raw, w)
		}
	}
	if len(raw) == 0 {
		return ""
	}
	if !withPlusPrefix {
		for i := wordStartIdx + 1; i < len(raw); i++ {
			raw[i] = shellDisplayArgIndent + raw[i]
		}
		return strings.Join(raw, "\n")
	}
	out := make([]string, len(raw))
	out[0] = "+ " + raw[0]
	for i := 1; i < len(raw); i++ {
		out[i] = "  " + raw[i]
	}
	return strings.Join(out, "\n")
}
