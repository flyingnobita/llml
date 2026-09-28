package tui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
)

// shortHelpModel opens the shortcuts popup in an 80x24 terminal, where the
// list does not fit and must scroll.
func shortHelpModel(t *testing.T) Model {
	t.Helper()
	m := newTestModel()
	m.layout.width = 80
	m.layout.height = 24
	m.buildID = "dev (16728e6, 2026-09-28, modified)"
	m = m.layoutTable()
	m, _ = pressHelpKey(t, m, tea.KeyPressMsg{Code: '?', Text: "?"})
	if !m.helpOpen {
		t.Fatal("? did not open the shortcuts popup")
	}
	if m.helpMaxOffset() == 0 {
		t.Fatal("expected the shortcut list to overflow at 80x24")
	}
	return m
}

func pressHelpKey(t *testing.T, m Model, msg tea.KeyPressMsg) (Model, tea.Cmd) {
	t.Helper()
	next, cmd := m.Update(msg)
	return next.(Model), cmd
}

func TestHelpPanel_fitsShortTerminalWithIdentity(t *testing.T) {
	t.Parallel()
	m := shortHelpModel(t)
	block := ansi.Strip(m.helpPanelModalBlock())
	if h := strings.Count(block, "\n") + 1; h > m.layout.height {
		t.Errorf("popup is %d rows, terminal is %d", h, m.layout.height)
	}
	for _, want := range []string{"Keyboard Shortcuts", m.buildID, FooterHintHelpScroll, "esc"} {
		if !strings.Contains(block, want) {
			t.Errorf("popup at 80x24 is missing %q", want)
		}
	}
}

func TestHelpPanel_scrollKeysClamp(t *testing.T) {
	t.Parallel()
	m := shortHelpModel(t)
	maxOff := m.helpMaxOffset()
	page := m.helpBodyHeight()

	steps := []struct {
		name string
		msg  tea.KeyPressMsg
		want int
	}{
		{"up at the top stays put", tea.KeyPressMsg{Code: tea.KeyUp}, 0},
		{"j moves down one", tea.KeyPressMsg{Code: 'j', Text: "j"}, 1},
		{"down moves down one", tea.KeyPressMsg{Code: tea.KeyDown}, 2},
		{"k moves up one", tea.KeyPressMsg{Code: 'k', Text: "k"}, 1},
		{"pgdown moves a page, clamped", tea.KeyPressMsg{Code: tea.KeyPgDown}, min(1+page, maxOff)},
		{"end goes to the bottom", tea.KeyPressMsg{Code: tea.KeyEnd}, maxOff},
		{"down at the bottom stays put", tea.KeyPressMsg{Code: tea.KeyDown}, maxOff},
		{"pgup moves a page, clamped", tea.KeyPressMsg{Code: tea.KeyPgUp}, max(maxOff-page, 0)},
		{"home goes to the top", tea.KeyPressMsg{Code: tea.KeyHome}, 0},
	}
	for _, s := range steps {
		m, _ = pressHelpKey(t, m, s.msg)
		if m.helpOffset != s.want {
			t.Fatalf("%s: offset %d, want %d", s.name, m.helpOffset, s.want)
		}
	}
}

func TestHelpPanel_bottomShowsLastShortcut(t *testing.T) {
	t.Parallel()
	m := shortHelpModel(t)
	m, _ = pressHelpKey(t, m, tea.KeyPressMsg{Code: tea.KeyEnd})
	if block := ansi.Strip(m.helpPanelModalBlock()); !strings.Contains(block, "Quit (warn if still running)") {
		t.Error("scrolled to the end, the last shortcut is not shown")
	}
}

func TestHelpPanel_reopensAtTop(t *testing.T) {
	t.Parallel()
	m := shortHelpModel(t)
	m, _ = pressHelpKey(t, m, tea.KeyPressMsg{Code: tea.KeyEnd})
	m, _ = pressHelpKey(t, m, tea.KeyPressMsg{Code: tea.KeyEscape})
	if m.helpOpen {
		t.Fatal("esc did not close the popup")
	}
	m, _ = pressHelpKey(t, m, tea.KeyPressMsg{Code: '?', Text: "?"})
	if m.helpOffset != 0 {
		t.Errorf("reopened popup at offset %d, want 0", m.helpOffset)
	}
}

func TestHelpPanel_questionMarkCloses(t *testing.T) {
	t.Parallel()
	m := shortHelpModel(t)
	m, _ = pressHelpKey(t, m, tea.KeyPressMsg{Code: '?', Text: "?"})
	if m.helpOpen {
		t.Error("? did not close the popup")
	}
}

func TestHelpPanel_swallowsOtherKeys(t *testing.T) {
	t.Parallel()
	m := shortHelpModel(t)
	for _, msg := range []tea.KeyPressMsg{
		{Code: 'q', Text: "q"},
		{Code: 'S', Text: "S"},
		{Code: tea.KeyEnter},
	} {
		next, cmd := pressHelpKey(t, m, msg)
		if !next.helpOpen {
			t.Errorf("%q closed the popup", msg.String())
		}
		if cmd != nil {
			t.Errorf("%q returned a command; the popup should swallow it", msg.String())
		}
	}
}

func TestHelpPanel_noScrollHintWhenItFits(t *testing.T) {
	t.Parallel()
	m := newTestModel()
	m.layout.height = 60
	m = m.layoutTable().openHelp()
	if m.helpMaxOffset() != 0 {
		t.Fatal("expected every shortcut to fit at 60 rows")
	}
	if strings.Contains(ansi.Strip(m.helpPanelModalBlock()), FooterHintHelpScroll) {
		t.Error("scroll hint shown although nothing scrolls")
	}
}
