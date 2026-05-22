package tui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

func quitMsg() tea.KeyPressMsg {
	return tea.KeyPressMsg(tea.Key{Code: 'q', Text: "q"})
}

func stopServerMsg() tea.KeyPressMsg {
	return tea.KeyPressMsg(tea.Key{Code: 's', Text: "s"})
}

func TestUpdateServerSplitKeys_QuitOpensConfirmWhileRunning(t *testing.T) {
	m := newTestModel()
	m.server.running = true
	m.server.exited = false

	got, cmd := m.updateServerSplitKeys(quitMsg())
	if cmd != nil {
		t.Fatalf("unexpected cmd: %v", cmd)
	}
	if !got.quit.open {
		t.Fatal("expected quit confirmation to open")
	}
	if !got.server.running {
		t.Fatal("expected server to keep running while quit is unconfirmed")
	}
}

func TestUpdateServerSplitKeys_CtrlCOpensConfirmWhileRunning(t *testing.T) {
	m := newTestModel()
	m.server.running = true
	m.server.exited = false

	got, cmd := m.updateServerSplitKeys(tea.KeyPressMsg(tea.Key{Code: 'c', Text: "c", Mod: tea.ModCtrl}))
	if cmd != nil {
		t.Fatalf("unexpected cmd: %v", cmd)
	}
	if !got.quit.open {
		t.Fatal("expected quit confirmation to open")
	}
}

func TestUpdateQuitConfirmKey_YQuits(t *testing.T) {
	m := newTestModel()
	m.quit.open = true

	got, cmd := m.updateQuitConfirmKey(tea.KeyPressMsg(tea.Key{Code: 'y', Text: "y"}))
	if got.quit.open {
		t.Fatal("expected quit confirmation to close after confirming")
	}
	if cmd == nil {
		t.Fatal("expected quit cmd")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Fatalf("expected tea.QuitMsg, got %T", cmd())
	}
}

func TestUpdateQuitConfirmKey_EnterDoesNothing(t *testing.T) {
	m := newTestModel()
	m.quit.open = true

	got, cmd := m.updateQuitConfirmKey(tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter, Text: "enter"}))
	if cmd != nil {
		t.Fatalf("unexpected cmd: %v", cmd)
	}
	if !got.quit.open {
		t.Fatal("expected quit confirmation to stay open on enter")
	}
}

func TestUpdateQuitConfirmKey_CancelStays(t *testing.T) {
	m := newTestModel()
	m.quit.open = true

	got, cmd := m.updateQuitConfirmKey(tea.KeyPressMsg(tea.Key{Code: 'n', Text: "n"}))
	if cmd != nil {
		t.Fatalf("unexpected cmd: %v", cmd)
	}
	if got.quit.open {
		t.Fatal("expected quit confirmation to close after cancel")
	}
}

func TestQuitConfirmModalBlock_WarnsServerKeepsRunning(t *testing.T) {
	m := newTestModel()

	got := m.quitConfirmModalBlock()
	if !strings.Contains(got, "still running") {
		t.Fatalf("expected running warning, got %q", got)
	}
	if !strings.Contains(got, "keep running") {
		t.Fatalf("expected keep-running warning, got %q", got)
	}
}

func TestFooterHelpLine_IncludesStopAndQuitHintsDuringSplitServer(t *testing.T) {
	m := newTestModel()
	m.server.running = true

	got := footerHelpLine(m)
	if !strings.Contains(got, FooterHintStopServer) {
		t.Fatalf("expected split footer to include stop hint, got %q", got)
	}
	if !strings.Contains(got, FooterHintQuit) {
		t.Fatalf("expected split footer to include quit hint, got %q", got)
	}
}

func TestHelpSections_IncludeSplitStopAndQuitShortcuts(t *testing.T) {
	sections := helpSections()
	for _, section := range sections {
		if section.title != "Split Server Pane" {
			continue
		}
		var foundStop bool
		var foundQuit bool
		for _, entry := range section.entries {
			if entry.key == FooterKeyStopServer && entry.desc == "Stop server" {
				foundStop = true
			}
			if entry.key == FooterKeyQuit && entry.desc == "Quit (warn if still running)" {
				foundQuit = true
			}
		}
		if !foundStop {
			t.Fatal("expected split server help section to include the stop shortcut")
		}
		if !foundQuit {
			t.Fatal("expected split server help section to include the quit shortcut")
		}
		return
	}
	t.Fatal("expected split server help section")
}

func TestUpdateServerSplitKeys_StopServerKeyDoesNotOpenQuitConfirm(t *testing.T) {
	m := newTestModel()
	m.server.running = true
	m.server.exited = false

	got, _ := m.updateServerSplitKeys(stopServerMsg())
	if got.quit.open {
		t.Fatal("expected stop-server key not to open quit confirmation")
	}
}
