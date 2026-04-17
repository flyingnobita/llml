package tui

import (
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/mattn/go-runewidth"
)

// FormatPathDisplay shortens the user home directory prefix to ~/ for TUI display only.
// Pass homeDir from [os.UserHomeDir] (or tests). If homeDir is empty, the path is returned
// unchanged (no tilde shortening). The original path should be kept for programmatic use.
func FormatPathDisplay(absPath string, homeDir string) string {
	if homeDir == "" {
		return absPath
	}
	home := filepath.Clean(homeDir)
	p := filepath.Clean(absPath)
	rel, err := filepath.Rel(home, p)
	if err != nil {
		return absPath
	}
	if strings.HasPrefix(rel, "..") {
		return absPath
	}
	if rel == "." {
		return "~"
	}
	return filepath.Join("~", rel)
}

// hfHubRepoDirPrefix marks Hugging Face hub cache repo directories under .../hub/.
const hfHubRepoDirPrefix = "models--"

// FormatModelFolderDisplay returns a display path for the "model folder": for Hugging Face hub
// layouts it stops at the models--* repo directory (omits snapshots/<revision>/). For a GGUF file
// it uses the parent directory of the file; for a directory path (safetensors / vLLM rows) it
// uses that path directly. homeDir is passed to [FormatPathDisplay].
func FormatModelFolderDisplay(filePath string, homeDir string) string {
	clean := filepath.Clean(filePath)
	var parent string
	if st, err := os.Stat(clean); err == nil && st.IsDir() {
		parent = clean
	} else {
		parent = filepath.Dir(clean)
	}
	dir := parent
	for {
		if strings.HasPrefix(filepath.Base(dir), hfHubRepoDirPrefix) {
			return FormatPathDisplay(dir, homeDir)
		}
		up := filepath.Dir(dir)
		if up == dir {
			break
		}
		dir = up
	}
	return FormatPathDisplay(parent, homeDir)
}

// FormatModTime renders local filesystem modification time (not inference "last run").
func FormatModTime(t time.Time) string {
	if t.IsZero() {
		return "—"
	}
	return t.Local().Format("2006-01-02 15:04")
}

// TruncateRunes limits visible width using runewidth, adding an ellipsis when trimmed.
func TruncateRunes(s string, maxWidth int) string {
	if maxWidth < 2 {
		return ""
	}
	if runewidth.StringWidth(s) <= maxWidth {
		return s
	}
	var b strings.Builder
	w := 0
	for _, r := range s {
		rw := runewidth.RuneWidth(r)
		if w+rw > maxWidth-1 {
			break
		}
		b.WriteRune(r)
		w += rw
	}
	return b.String() + "…"
}
