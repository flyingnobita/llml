package tui

import (
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/flyingnobita/llml/internal/llamacpp"
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

func TestParamPanelViewIncludesMainAppBackdrop(t *testing.T) {
	m := New()
	// Tall terminal so the centered modal does not cover the title row; on 24 lines
	// a ~22-line modal obscures the title and this test would falsely fail.
	m.width = 100
	m.height = 40
	m.loading = false
	m.files = []llamacpp.ModelFile{
		{Backend: llamacpp.BackendLlama, Path: "/x.gguf", Name: "x", Size: 1, ModTime: time.Unix(0, 0)},
	}
	m = m.layoutTable()
	m.paramPanelOpen = true
	m.paramModelDisplayName = "test/model"
	m.paramProfiles = []ParameterProfile{{Name: "default"}}

	bg := m.mainAppPlacedView()
	if !strings.Contains(bg, "LLM Launcher") {
		t.Fatalf("mainAppPlacedView missing title (len=%d)", len(bg))
	}

	v := m.View()
	content := v.Content
	if !strings.Contains(content, "LLM") || !strings.Contains(content, "Launcher") {
		t.Fatalf("overlaid view missing title (backdrop should remain above modal)")
	}
	if !strings.Contains(content, "Parameters") {
		t.Fatal("expected parameters modal in view")
	}
	// Subtitle line remains visible outside the modal on a tall layout.
	if !strings.Contains(content, "filesystem scan") {
		t.Fatal("expected main view subtitle in backdrop outside modal")
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
