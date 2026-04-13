package llamacpp

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/mattn/go-runewidth"
)

// FormatSize renders a byte size with binary (IEC) units.
func FormatSize(b int64) string {
	const unit = 1024
	if b < 0 {
		return "—"
	}
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div := float64(b)
	units := []string{"KiB", "MiB", "GiB", "TiB", "PiB"}
	idx := -1
	for div >= unit && idx < len(units)-1 {
		div /= unit
		idx++
	}
	return fmt.Sprintf("%.1f %s", div, units[idx])
}

// FormatPathDisplay shortens the user home directory prefix to ~/ for TUI display only.
// The original path should be kept for programmatic use (open, copy, compare).
func FormatPathDisplay(absPath string) string {
	home, err := os.UserHomeDir()
	if err != nil {
		return absPath
	}
	home = filepath.Clean(home)
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

// FormatVLLMModelName returns a short label for a safetensors checkpoint directory.
// For Hugging Face hub layouts it decodes the nearest `models--*` folder: the cache encodes
// repo ids by replacing "/" with "--", so we invert that for a readable `org/model` name.
// Otherwise it falls back to the directory basename (e.g. a non-Hub layout).
func FormatVLLMModelName(absDir string) string {
	clean := filepath.Clean(absDir)
	for d := clean; ; d = filepath.Dir(d) {
		base := filepath.Base(d)
		if strings.HasPrefix(base, hfHubRepoDirPrefix) {
			rest := strings.TrimPrefix(base, hfHubRepoDirPrefix)
			if rest == "" {
				break
			}
			return strings.ReplaceAll(rest, "--", "/")
		}
		up := filepath.Dir(d)
		if up == d {
			break
		}
	}
	return filepath.Base(clean)
}

// FormatModelFolderDisplay returns a display path for the "model folder": for Hugging Face hub
// layouts it stops at the models--* repo directory (omits snapshots/<revision>/). For a GGUF file
// it uses the parent directory of the file; for a directory path (safetensors / vLLM rows) it
// uses that path directly.
func FormatModelFolderDisplay(filePath string) string {
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
			return FormatPathDisplay(dir)
		}
		up := filepath.Dir(dir)
		if up == dir {
			break
		}
		dir = up
	}
	return FormatPathDisplay(parent)
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
