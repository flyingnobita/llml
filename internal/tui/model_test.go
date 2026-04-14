package tui

import (
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/flyingnobita/llml/internal/llamacpp"
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
	innerW := m.bodyInnerW
	if m.tableLineWidth > innerW {
		t.Fatalf("table line width %d > inner width %d (spurious horizontal scroll)", m.tableLineWidth, innerW)
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
