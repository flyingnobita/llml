package tui

import (
	"testing"

	tea "charm.land/bubbletea/v2"
)

func TestParamPanelDeleteConfirm(t *testing.T) {
	m := New()
	m.width = 80
	m.height = 24
	m.paramPanelOpen = true
	m.paramFocus = paramFocusProfiles
	m.paramProfiles = []ParameterProfile{{Name: "a"}, {Name: "b"}}
	m.paramProfileIndex = 0

	m, _ = m.updateParamPanelKey(tea.KeyPressMsg{Code: 'd', Text: "d"})
	if m.paramConfirmDelete != paramConfirmProfile {
		t.Fatal("expected confirm dialog after d with 2+ profiles")
	}
	if len(m.paramProfiles) != 2 {
		t.Fatal("delete must not run before confirmation")
	}

	m, _ = m.updateParamPanelKey(tea.KeyPressMsg{Code: 'n', Text: "n"})
	if m.paramConfirmDelete != paramConfirmNone {
		t.Fatal("n should dismiss confirm dialog")
	}

	m.paramProfiles = []ParameterProfile{{Name: "only"}}
	m.paramConfirmDelete = paramConfirmNone
	m, _ = m.updateParamPanelKey(tea.KeyPressMsg{Code: 'd', Text: "d"})
	if m.paramConfirmDelete != paramConfirmNone {
		t.Fatal("no confirm when only one profile")
	}
}

func TestParamPanelDeleteEnvRowConfirm(t *testing.T) {
	m := New()
	m.width = 80
	m.height = 24
	m.paramPanelOpen = true
	m.paramFocus = paramFocusEnv
	m.paramProfiles = []ParameterProfile{{Name: "p", Env: []EnvVar{{Key: "K", Value: "V"}}, Args: nil}}
	m.paramProfileIndex = 0
	m.paramEnv = []EnvVar{{Key: "K", Value: "V"}}
	m.paramEnvCursor = 0

	m, _ = m.updateParamPanelKey(tea.KeyPressMsg{Code: 'd', Text: "d"})
	if m.paramConfirmDelete != paramConfirmEnvRow {
		t.Fatalf("expected env row confirm, got %d", m.paramConfirmDelete)
	}
	if len(m.paramEnv) != 1 {
		t.Fatal("row not deleted yet")
	}
	m, _ = m.updateParamPanelKey(tea.KeyPressMsg{Code: 'n', Text: "n"})
	if m.paramConfirmDelete != paramConfirmNone {
		t.Fatal("n should dismiss confirm")
	}
	if len(m.paramEnv) != 1 {
		t.Fatal("row still present after cancel")
	}
}

func TestCommitParamLineEdit_blankEnvLikeCancel(t *testing.T) {
	m := New()
	m.paramFocus = paramFocusEnv
	m.paramEnv = []EnvVar{{Key: "K", Value: "V"}}
	m.paramEnvCursor = 0
	m.paramEditKind = paramEditEnvLine
	m.paramEditInput.SetValue("   ")

	m = m.commitParamLineEdit()
	if m.paramEnvLen() != 1 || m.paramEnv[0].Key != "K" || m.paramEnv[0].Value != "V" {
		t.Fatalf("blank commit should keep existing env, got %#v", m.paramEnv)
	}
	if m.paramEditKind != paramEditNone {
		t.Fatal("expected edit closed")
	}
}

func TestCommitParamLineEdit_blankEnvRemovesNewEmptyRow(t *testing.T) {
	m := New()
	m.paramFocus = paramFocusEnv
	m.paramEnv = []EnvVar{{}}
	m.paramEnvCursor = 0
	m.paramEditKind = paramEditEnvLine
	m.paramEditInput.SetValue("")

	m = m.commitParamLineEdit()
	if m.paramEnvLen() != 0 {
		t.Fatalf("blank commit on new empty env row should remove row, got len %d", m.paramEnvLen())
	}
}

func TestCommitParamLineEdit_blankArgLikeCancel(t *testing.T) {
	m := New()
	m.paramFocus = paramFocusArgs
	m.paramArgs = []string{"--foo"}
	m.paramArgsCursor = 0
	m.paramEditKind = paramEditArgLine
	m.paramEditInput.SetValue("\t ")

	m = m.commitParamLineEdit()
	if m.paramArgsLen() != 1 || m.paramArgs[0] != "--foo" {
		t.Fatalf("blank commit should keep existing arg, got %#v", m.paramArgs)
	}
}

func TestCommitParamLineEdit_blankArgRemovesNewEmptyRow(t *testing.T) {
	m := New()
	m.paramFocus = paramFocusArgs
	m.paramArgs = []string{""}
	m.paramArgsCursor = 0
	m.paramEditKind = paramEditArgLine
	m.paramEditInput.SetValue("  ")

	m = m.commitParamLineEdit()
	if m.paramArgsLen() != 0 {
		t.Fatalf("blank commit on new empty arg row should remove row, got %#v", m.paramArgs)
	}
}

func TestParamPanelContentWidth_wideTerminalUsesCap(t *testing.T) {
	m := New()
	m.width = 200
	m.height = 40
	if got := m.paramPanelContentWidth(); got != paramPanelMaxInnerWidth {
		t.Fatalf("paramPanelContentWidth = %d, want %d", got, paramPanelMaxInnerWidth)
	}
}
