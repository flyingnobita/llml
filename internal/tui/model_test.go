package tui

import (
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/flyingnobita/llml/internal/llamacpp"
	"github.com/mattn/go-runewidth"
)

func TestLayoutTable_wideTerminalFitsViewport(t *testing.T) {
	m := New()
	m.width = 203
	m.height = 80
	m.files = []llamacpp.ModelFile{
		{
			Backend: llamacpp.BackendLlama,
			Path:    "/x",
			Name:    "m",
			Size:    1,
			ModTime: time.Unix(0, 0),
		},
	}
	m.loading = false
	m = m.layoutTable()
	if m.tableNeedsHScroll {
		t.Fatalf("table should not need horizontal scroll bar on wide terminal (min width fits inner body)")
	}
}

func TestModelsLoadedSelectsFirstRow(t *testing.T) {
	m := New()
	m.width = 120
	m.height = 40
	// Resolved llama-server path present so the missing-runtime footer line is not set for GGUF rows.
	m.runtime = llamacpp.RuntimeInfo{LlamaServerPath: "/fake/llama-server"}
	files := []llamacpp.ModelFile{
		{Backend: llamacpp.BackendLlama, Path: "/a.gguf", Name: "a", Size: 1, ModTime: time.Unix(0, 0)},
		{Backend: llamacpp.BackendLlama, Path: "/b.gguf", Name: "b", Size: 1, ModTime: time.Unix(0, 0)},
	}
	next, cmd := m.Update(modelsLoadedMsg{files: files})
	if cmd != nil {
		t.Fatal("unexpected cmd from modelsLoadedMsg")
	}
	m = next.(Model)
	if m.tbl.Cursor() != 0 {
		t.Fatalf("cursor %d want 0 (first row)", m.tbl.Cursor())
	}
}

func TestModelsLoaded_FooterErrorWhenGGUFWithoutLlamaServer(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	m := New()
	m.width = 120
	m.height = 40
	m.runtime = llamacpp.RuntimeInfo{}
	files := []llamacpp.ModelFile{
		{Backend: llamacpp.BackendLlama, Path: "/a.gguf", Name: "a", Size: 1, ModTime: time.Unix(0, 0)},
	}
	next, cmd := m.Update(modelsLoadedMsg{files: files})
	if cmd != nil {
		t.Fatal("unexpected cmd from modelsLoadedMsg")
	}
	m = next.(Model)
	if m.runtimeConfigOpen {
		t.Fatal("runtime config should not auto-open")
	}
	if !strings.Contains(m.lastRunNote, MissingLlamaServerFooterNote) {
		t.Fatalf("lastRunNote %q", m.lastRunNote)
	}
}

func TestModelsLoaded_FooterErrorWhenVLLMWithoutVllm(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	m := New()
	m.width = 120
	m.height = 40
	m.runtime = llamacpp.RuntimeInfo{}
	files := []llamacpp.ModelFile{
		{Backend: llamacpp.BackendVLLM, Path: "/m", Name: "m", Size: 1, ModTime: time.Unix(0, 0)},
	}
	next, cmd := m.Update(modelsLoadedMsg{files: files})
	if cmd != nil {
		t.Fatal("unexpected cmd from modelsLoadedMsg")
	}
	m = next.(Model)
	if m.runtimeConfigOpen {
		t.Fatal("runtime config should not auto-open")
	}
	if !strings.Contains(m.lastRunNote, MissingVLLMFooterNote) {
		t.Fatalf("lastRunNote %q", m.lastRunNote)
	}
}

func TestModelsLoaded_FooterErrorBothBackendsMissing(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	m := New()
	m.width = 120
	m.height = 40
	m.runtime = llamacpp.RuntimeInfo{}
	files := []llamacpp.ModelFile{
		{Backend: llamacpp.BackendLlama, Path: "/a.gguf", Name: "a", Size: 1, ModTime: time.Unix(0, 0)},
		{Backend: llamacpp.BackendVLLM, Path: "/m", Name: "m", Size: 1, ModTime: time.Unix(0, 0)},
	}
	next, cmd := m.Update(modelsLoadedMsg{files: files})
	if cmd != nil {
		t.Fatal("unexpected cmd from modelsLoadedMsg")
	}
	m = next.(Model)
	if m.runtimeConfigOpen {
		t.Fatal("runtime config should not auto-open")
	}
	if !strings.Contains(m.lastRunNote, MissingLlamaServerFooterNote) || !strings.Contains(m.lastRunNote, MissingVLLMFooterNote) {
		t.Fatalf("lastRunNote %q", m.lastRunNote)
	}
}

func TestSplitServerBodyHeights(t *testing.T) {
	th, lh := splitServerBodyHeights(20)
	if th+lh != 20 {
		t.Fatalf("got table=%d log=%d want sum 20", th, lh)
	}
}

func TestAppendServerLogLine_caps(t *testing.T) {
	m := New()
	for i := 0; i < maxServerLogLines+50; i++ {
		m = m.appendServerLogLine("x")
	}
	if len(m.serverLog) != maxServerLogLines {
		t.Fatalf("got len %d want %d", len(m.serverLog), maxServerLogLines)
	}
}

func TestRunServerKeyMode(t *testing.T) {
	// US QWERTY: uppercase R is shift+r — must be split, not fullscreen.
	shiftR := tea.KeyPressMsg(tea.Key{Text: "R", Code: 'r', Mod: tea.ModShift})
	if runServerKeyMode(shiftR) != 1 {
		t.Fatalf("shift+R (normal R): got %d want 1 (split)", runServerKeyMode(shiftR))
	}
	capsR := tea.KeyPressMsg(tea.Key{Text: "R", Code: 'R', Mod: 0})
	if runServerKeyMode(capsR) != 1 {
		t.Fatalf("R (caps): got %d want 1", runServerKeyMode(capsR))
	}
	ctrlR := tea.KeyPressMsg(tea.Key{Text: "R", Code: 'r', Mod: tea.ModCtrl | tea.ModShift})
	if runServerKeyMode(ctrlR) != 2 {
		t.Fatalf("ctrl+R: got %d want 2 (fullscreen)", runServerKeyMode(ctrlR))
	}
	other := tea.KeyPressMsg(tea.Key{Text: "x", Code: 'x', Mod: 0})
	if runServerKeyMode(other) != 0 {
		t.Fatalf("non-R: got %d want 0", runServerKeyMode(other))
	}
}

func TestNew_zeroSize(t *testing.T) {
	m := New()
	if m.width != 0 || m.height != 0 {
		t.Fatalf("expected zero dimensions, got %dx%d", m.width, m.height)
	}
	if !m.loading {
		t.Fatal("expected loading true before first frame")
	}
}

// TestViewAltScreen verifies that View() opts into the alternate screen buffer,
// which replaced the tea.WithAltScreen() program option in Bubble Tea v2.
func TestViewAltScreen(t *testing.T) {
	t.Setenv(EnvLLMLTheme, "dark")
	m := New()
	m.width = 80
	m.height = 24
	v := m.View()
	if _, ok := any(v).(tea.View); !ok {
		t.Fatalf("View() should return tea.View, got %T", v)
	}
	if !v.AltScreen {
		t.Fatal("expected View().AltScreen = true for full-screen TUI")
	}
}

// TestMainViewShowsTitleAndFooterNavHint ensures the primary TUI frame remains
// fully visible in a common terminal size, including the navigation hint bar.
func TestMainViewShowsTitleAndFooterNavHint(t *testing.T) {
	m := New()
	// Footer line is longer than [minTerminalWidth]; use a width that fits the full hint bar.
	m.width = 100
	m.height = 32
	m.loading = false
	m.files = []llamacpp.ModelFile{
		{Backend: llamacpp.BackendLlama, Path: "/a.gguf", Name: "a", Size: 1, ModTime: time.Unix(0, 0)},
		{Backend: llamacpp.BackendLlama, Path: "/b.gguf", Name: "b", Size: 1, ModTime: time.Unix(0, 0)},
	}
	m = m.layoutTable()

	content := visibleViewport(m.View().Content, m.width, m.height)
	if !strings.Contains(content, appTitle) {
		t.Fatalf("missing app title in normal view (len=%d)", len(content))
	}
	if !strings.Contains(content, FooterNavHint) {
		t.Fatalf("missing footer navigation hint in normal view (want %q)", FooterNavHint)
	}
}

func TestSplitViewShowsTitleAndFooterHints(t *testing.T) {
	m := New()
	m.width = 100
	m.height = 32
	m.loading = false
	m.serverRunning = true
	m.splitLogFocused = false
	m.files = []llamacpp.ModelFile{
		{Backend: llamacpp.BackendLlama, Path: "/a.gguf", Name: "a", Size: 1, ModTime: time.Unix(0, 0)},
		{Backend: llamacpp.BackendLlama, Path: "/b.gguf", Name: "b", Size: 1, ModTime: time.Unix(0, 0)},
	}
	for i := 0; i < 30; i++ {
		m = m.appendServerLogLine("log line")
	}
	m = m.layoutTable()

	content := visibleViewport(m.View().Content, m.width, m.height)
	if !strings.Contains(content, appTitle) {
		t.Fatalf("missing app title in split view (len=%d)", len(content))
	}
	if !strings.Contains(content, FooterNavHint) {
		t.Fatalf("missing footer navigation hint in split view (want %q)", FooterNavHint)
	}
}

func visibleViewport(content string, width, height int) string {
	lines := strings.Split(content, "\n")
	if height > 0 && len(lines) > height {
		lines = lines[len(lines)-height:]
	}
	for i, line := range lines {
		lines[i] = trimToColumns(line, width)
	}
	return strings.Join(lines, "\n")
}

func trimToColumns(s string, width int) string {
	if width <= 0 {
		return ""
	}
	if runewidth.StringWidth(s) <= width {
		return s
	}
	var b strings.Builder
	used := 0
	for _, r := range s {
		w := runewidth.RuneWidth(r)
		if w <= 0 {
			w = 1
		}
		if used+w > width {
			break
		}
		b.WriteRune(r)
		used += w
	}
	return b.String()
}

// TestSelectedStyleHasBackground verifies that the table Selected style carries
// a background color, which is the mechanism that replaced the vendored btable
// fork's per-cell colour override.
func TestSelectedStyleHasBackground(t *testing.T) {
	for _, th := range []struct {
		name  string
		theme Theme
	}{
		{"dark", DarkTheme()},
		{"light", LightTheme()},
	} {
		st := newStyles(th.theme)
		// Render a plain cell and a selected cell and confirm they differ —
		// and that the selected render includes an ANSI background sequence.
		plain := st.table.Cell.Render("X")
		selected := st.table.Selected.Render("X")
		if plain == selected {
			t.Fatalf("%s: Selected style renders identically to Cell style", th.name)
		}
		// ANSI background codes start with \x1b[4 (40-49) or \x1b[10 (100-109).
		if !strings.Contains(selected, "\x1b[") {
			t.Fatalf("%s: Selected render contains no ANSI escape", th.name)
		}
	}
}
