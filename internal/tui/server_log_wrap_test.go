package tui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

func wrapMsg() tea.KeyPressMsg {
	return tea.KeyPressMsg(tea.Key{Code: 'w', Text: "w"})
}

func TestUpdateServerSplitKeys_ToggleWrapWhileRunning(t *testing.T) {
	m := newTestModel()
	m.server.running = true
	m.server.exited = false
	m.layout.width = 28
	m.layout.height = 20
	m = m.appendServerLogLine(strings.Repeat("x", 80))
	m = m.layoutTable()

	if !m.serverLogNeedsHorizontalScroll() {
		t.Fatal("expected horizontal scroll before wrap is enabled")
	}

	got, cmd := m.updateServerSplitKeys(wrapMsg())
	if cmd != nil {
		t.Fatalf("unexpected cmd: %v", cmd)
	}
	if !got.server.wrap {
		t.Fatal("expected wrap=true after pressing w")
	}
	if got.server.viewport.SoftWrap {
		t.Fatal("expected viewport SoftWrap=false; wrapped content is rendered explicitly")
	}
	if got.serverLogNeedsHorizontalScroll() {
		t.Fatal("expected horizontal scroll to be disabled when wrap is enabled")
	}
	if !strings.Contains(got.server.viewport.GetContent(), "\n"+serverLogWrapPrefix) {
		t.Fatalf("expected wrapped server output to include continuation prefix, got %q", got.server.viewport.GetContent())
	}
	if got.lastRunNote != "Server output wrap enabled" {
		t.Fatalf("unexpected status note: %q", got.lastRunNote)
	}
}

func TestUpdateServerSplitKeys_ToggleWrapAfterExit(t *testing.T) {
	m := newTestModel()
	m.server.running = true
	m.server.exited = true

	got, cmd := m.updateServerSplitKeys(wrapMsg())
	if cmd != nil {
		t.Fatalf("unexpected cmd: %v", cmd)
	}
	if !got.server.wrap {
		t.Fatal("expected wrap toggle to work after server exit")
	}
	if got.server.viewport.SoftWrap {
		t.Fatal("expected explicit wrapped rendering after server exit")
	}
}

func TestDismissSplitServer_ResetsWrapState(t *testing.T) {
	m := newTestModel()
	m.server.running = true
	m.server.exited = true
	m.server.wrap = true

	got := m.dismissSplitServer()
	if got.server.wrap {
		t.Fatal("expected wrap=false after dismissing split server")
	}
	if got.server.viewport.SoftWrap {
		t.Fatal("expected viewport SoftWrap=false after dismissing split server")
	}
}

func TestFooterHelpLine_IncludesWrapHintDuringSplitServer(t *testing.T) {
	m := newTestModel()
	m.server.running = true

	got := footerHelpLine(m)
	if !strings.Contains(got, FooterHintToggleWrap) {
		t.Fatalf("expected split footer to include wrap hint, got %q", got)
	}
}

func TestHelpSections_IncludeSplitWrapShortcut(t *testing.T) {
	sections := helpSections()
	for _, section := range sections {
		if section.title != "Split Server Pane" {
			continue
		}
		for _, entry := range section.entries {
			if entry.key == "w" && entry.desc == "Toggle word wrap" {
				return
			}
		}
		t.Fatal("expected split server help section to include the wrap shortcut")
	}
	t.Fatal("expected split server help section")
}

func TestRenderWrappedServerLogLine_PrefixesContinuationLines(t *testing.T) {
	got := renderWrappedServerLogLine("alpha beta gamma delta", 10)
	if !strings.Contains(got, "\n"+serverLogWrapPrefix) {
		t.Fatalf("expected continuation prefix in wrapped output, got %q", got)
	}
	lines := strings.Split(got, "\n")
	if len(lines) < 2 {
		t.Fatalf("expected wrapped output with multiple lines, got %q", got)
	}
	if strings.HasPrefix(lines[0], serverLogWrapPrefix) {
		t.Fatalf("expected first line to stay flush-left, got %q", lines[0])
	}
	for _, line := range lines[1:] {
		if !strings.HasPrefix(line, serverLogWrapPrefix) {
			t.Fatalf("expected continuation line prefix, got %q", line)
		}
	}
}
