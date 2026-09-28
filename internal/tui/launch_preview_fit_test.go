package tui

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"

	"github.com/flyingnobita/llml/internal/config"
	"github.com/flyingnobita/llml/internal/models"
	"github.com/flyingnobita/llml/internal/profiles"
)

// longMissingRow is a Safetensors row at a realistic Hugging Face snapshot path
// that does not exist, with a profile that runs it on mlx-lm. At width 100 its
// model id line wraps to two lines, and with mlx-lm off it carries two warnings.
func longMissingRow(t *testing.T) models.ModelFile {
	t.Helper()
	row := testRow(models.BackendVLLM, filepath.FromSlash(
		"/Users/someone/.cache/huggingface/hub/models--mlx-community--Qwen3.8-27B-4bit/snapshots/3e6447f082e89cc7f0bc6e5441afd38dfce760ff"))
	saveProfiles(t, row.Path, profiles.Entry{Profiles: []profiles.Profile{{Name: "mlx", Backend: "mlx-lm"}}})
	return row
}

// previewTextRows is how many text rows the launch preview shows.
func previewTextRows(m Model) int {
	return m.preview.viewport.Height() - m.preview.viewport.Style.GetVerticalFrameSize()
}

func assertFitsTerminal(t *testing.T, m Model) {
	t.Helper()
	if h := strings.Count(m.View().Content, "\n") + 1; h > m.layout.height {
		t.Errorf("view is %d rows, terminal is %d", h, m.layout.height)
	}
}

// Every launch warning shows without scrolling, however far a long model id
// wraps: the preview grows to fit them and still shows the command's first line.
func TestLaunchPreviewFit_longPathShowsEveryWarning(t *testing.T) {
	useTempConfigDir(t)
	row := longMissingRow(t)
	off := config.RuntimeStates{}.With(models.BackendMLXLM, false)
	m := dimModel(t, newLaunchFakes().services, linuxPlatform, off, row)
	m.layout.width = 100
	m = m.layoutTable()

	view := plainView(m)
	for _, want := range []string{"mlx-lm is off", "model folder not found", "not run it", "mlx_lm.server"} {
		if !strings.Contains(view, want) {
			t.Errorf("want %q visible without scrolling:\n%s", want, view)
		}
	}
	assertFitsTerminal(t, m)
}

// Each warning is its own line: joining them into one block padded the shorter
// one to the longer one's width, and soft wrap turned that padding into an
// empty row that took one of the preview's lines.
func TestLaunchPreviewFit_noBlankRowBetweenWarnings(t *testing.T) {
	useTempConfigDir(t)
	row := longMissingRow(t)
	off := config.RuntimeStates{}.With(models.BackendMLXLM, false)
	m := dimModel(t, newLaunchFakes().services, linuxPlatform, off, row)
	m.layout.width = 100
	m = m.layoutTable()

	// The drawn viewport, not its content: the blank row only appears once
	// soft wrap lays the content out.
	drawn := strings.Split(ansi.Strip(m.preview.viewport.View()), "\n")
	for i, l := range drawn[1 : len(drawn)-1] { // skip the border rows
		text := strings.Trim(l, "│ ░█")
		if strings.Contains(text, "mlx_lm.server") {
			break
		}
		if text == "" {
			t.Errorf("row %d above the command is blank:\n%s", i+1, strings.Join(drawn, "\n"))
		}
	}
}

// The preview keeps its usual height when everything fits, so rows without
// warnings do not move the layout.
func TestLaunchPreviewFit_keepsDefaultHeightWhenItFits(t *testing.T) {
	useTempConfigDir(t)
	row := testRow(models.BackendLlama, filepath.FromSlash("/models/qwen.gguf"))
	m := dimModel(t, newLaunchFakes().services, linuxPlatform, config.RuntimeStates{}, row)
	m = m.layoutTable()
	if got := previewTextRows(m); got != launchPreviewVisibleLines {
		t.Errorf("preview shows %d text rows, want the default %d", got, launchPreviewVisibleLines)
	}
}

// Moving the cursor onto a row that needs a taller preview grows it, even
// though cursor moves take the fast path that skips a full layout.
func TestLaunchPreviewFit_growsWhenCursorMovesOntoRow(t *testing.T) {
	useTempConfigDir(t)
	plain := testRow(models.BackendLlama, filepath.FromSlash("/models/a.gguf"))
	row := longMissingRow(t)
	off := config.RuntimeStates{}.With(models.BackendMLXLM, false)
	m := dimModel(t, newLaunchFakes().services, linuxPlatform, off, plain, row)
	m.layout.width = 100
	m = m.layoutTable()
	if strings.Contains(plainView(m), "mlx-lm is off") {
		t.Fatal("the plain row should be selected first")
	}

	m = press(t, m, keyDown)
	view := plainView(m)
	if !strings.Contains(view, "model folder not found") {
		t.Errorf("after moving onto the row, both warnings should show:\n%s", view)
	}
	assertFitsTerminal(t, m)

	m = press(t, m, keyUp)
	if got := previewTextRows(m); got != launchPreviewVisibleLines {
		t.Errorf("back on the plain row, preview shows %d text rows, want %d", got, launchPreviewVisibleLines)
	}
	assertFitsTerminal(t, m)
}
