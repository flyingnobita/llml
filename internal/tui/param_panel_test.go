package tui

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/flyingnobita/llml/internal/models"
)

func TestParseEnvLine_expandTilde(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	e := parseEnvLine("FOO=" + "~/bar")
	want := filepath.Join(home, "bar")
	if e.Key != "FOO" || e.Value != want {
		t.Fatalf("got %+v want FOO=%q", e, want)
	}
}

func TestCloneProfileName(t *testing.T) {
	profiles := []ParameterProfile{{Name: "cuda"}, {Name: "cuda copy"}}
	if got := cloneProfileName("cuda", profiles); got != "cuda copy 2" {
		t.Fatalf("cloneProfileName = %q", got)
	}
	if got := cloneProfileName("", []ParameterProfile{{Name: "x"}}); got == "" {
		t.Fatal("empty base should fall back to nextProfileName")
	}
}

func TestParamPanelCloneProfile(t *testing.T) {
	m := New()
	m.params.open = true
	m.params.focus = paramFocusProfiles
	m.params.profiles = []ParameterProfile{
		{Name: "cuda", Env: []EnvVar{{Key: "FOO", Value: "bar"}}, Args: []string{"--x"}},
		{Name: "cpu"},
	}
	m.params.profileIndex = 0
	m.params.loadCurrentProfileIn()

	m, _ = m.updateParamPanelKey(tea.KeyPressMsg{Code: 'c', Text: "c"})
	if len(m.params.profiles) != 3 {
		t.Fatalf("want 3 profiles, got %d", len(m.params.profiles))
	}
	if m.params.profileIndex != 1 {
		t.Fatalf("want cursor on new clone at index 1, got %d", m.params.profileIndex)
	}
	clone := m.params.profiles[1]
	if clone.Name != "cuda copy" {
		t.Fatalf("clone name = %q", clone.Name)
	}
	if len(clone.Env) != 1 || clone.Env[0].Key != "FOO" || clone.Env[0].Value != "bar" {
		t.Fatalf("clone env: %+v", clone.Env)
	}
	if len(clone.Args) != 1 || clone.Args[0] != "--x" {
		t.Fatalf("clone args: %+v", clone.Args)
	}
	if m.params.profiles[0].Name != "cuda" {
		t.Fatal("original profile name changed")
	}
	if len(m.params.profiles[0].Env) != 1 {
		t.Fatal("original profile env should still be one row (synced from editor state)")
	}
}

func TestParamPanelDeleteConfirm(t *testing.T) {
	m := New()
	m.layout.width = 80
	m.layout.height = 24
	m.params.open = true
	m.params.focus = paramFocusProfiles
	m.params.profiles = []ParameterProfile{{Name: "a"}, {Name: "b"}}
	m.params.profileIndex = 0

	m, _ = m.updateParamPanelKey(tea.KeyPressMsg{Code: 'd', Text: "d"})
	if m.params.confirmDelete != paramConfirmProfile {
		t.Fatal("expected confirm dialog after d with 2+ profiles")
	}
	if len(m.params.profiles) != 2 {
		t.Fatal("delete must not run before confirmation")
	}

	m, _ = m.updateParamPanelKey(tea.KeyPressMsg{Code: 'n', Text: "n"})
	if m.params.confirmDelete != paramConfirmNone {
		t.Fatal("n should dismiss confirm dialog")
	}

	m.params.profiles = []ParameterProfile{{Name: "only"}}
	m.params.confirmDelete = paramConfirmNone
	m, _ = m.updateParamPanelKey(tea.KeyPressMsg{Code: 'd', Text: "d"})
	if m.params.confirmDelete != paramConfirmNone {
		t.Fatal("no confirm when only one profile")
	}
}

func TestParamPanelDeleteEnvRowConfirm(t *testing.T) {
	m := New()
	m.layout.width = 80
	m.layout.height = 24
	m.params.open = true
	m.params.focus = paramFocusEnv
	m.params.profiles = []ParameterProfile{{Name: "p", Env: []EnvVar{{Key: "K", Value: "V"}}, Args: nil}}
	m.params.profileIndex = 0
	m.params.env = []EnvVar{{Key: "K", Value: "V"}}
	m.params.envCursor = 0

	m, _ = m.updateParamPanelKey(tea.KeyPressMsg{Code: 'd', Text: "d"})
	if m.params.confirmDelete != paramConfirmEnvRow {
		t.Fatalf("expected env row confirm, got %d", m.params.confirmDelete)
	}
	if len(m.params.env) != 1 {
		t.Fatal("row not deleted yet")
	}
	m, _ = m.updateParamPanelKey(tea.KeyPressMsg{Code: 'n', Text: "n"})
	if m.params.confirmDelete != paramConfirmNone {
		t.Fatal("n should dismiss confirm")
	}
	if len(m.params.env) != 1 {
		t.Fatal("row still present after cancel")
	}
}

func TestCommitParamLineEdit_blankEnvLikeCancel(t *testing.T) {
	m := New()
	m.params.focus = paramFocusEnv
	m.params.env = []EnvVar{{Key: "K", Value: "V"}}
	m.params.envCursor = 0
	m.params.editKind = paramEditEnvLine
	m.params.editInput.SetValue("   ")

	m = m.commitParamLineEdit()
	if m.paramEnvLen() != 1 || m.params.env[0].Key != "K" || m.params.env[0].Value != "V" {
		t.Fatalf("blank commit should keep existing env, got %#v", m.params.env)
	}
	if m.params.editKind != paramEditNone {
		t.Fatal("expected edit closed")
	}
}

func TestCommitParamLineEdit_blankEnvRemovesNewEmptyRow(t *testing.T) {
	m := New()
	m.params.focus = paramFocusEnv
	m.params.env = []EnvVar{{}}
	m.params.envCursor = 0
	m.params.editKind = paramEditEnvLine
	m.params.editInput.SetValue("")

	m = m.commitParamLineEdit()
	if m.paramEnvLen() != 0 {
		t.Fatalf("blank commit on new empty env row should remove row, got len %d", m.paramEnvLen())
	}
}

func TestCommitParamLineEdit_blankArgLikeCancel(t *testing.T) {
	m := New()
	m.params.focus = paramFocusArgs
	m.params.args = []string{"--foo"}
	m.params.argsCursor = 0
	m.params.editKind = paramEditArgLine
	m.params.editInput.SetValue("\t ")

	m = m.commitParamLineEdit()
	if m.paramArgsLen() != 1 || m.params.args[0] != "--foo" {
		t.Fatalf("blank commit should keep existing arg, got %#v", m.params.args)
	}
}

func TestCommitParamLineEdit_blankArgRemovesNewEmptyRow(t *testing.T) {
	m := New()
	m.params.focus = paramFocusArgs
	m.params.args = []string{""}
	m.params.argsCursor = 0
	m.params.editKind = paramEditArgLine
	m.params.editInput.SetValue("  ")

	m = m.commitParamLineEdit()
	if m.paramArgsLen() != 0 {
		t.Fatalf("blank commit on new empty arg row should remove row, got %#v", m.params.args)
	}
}

func TestParamPanelViewIncludesMainAppBackdrop(t *testing.T) {
	m := New()
	// Tall terminal so the centered modal does not cover the title row; on 24 lines
	// a ~22-line modal obscures the title and this test would falsely fail.
	m.layout.width = 100
	m.layout.height = 40
	m.loading = false
	m.table.files = []models.ModelFile{
		{Backend: models.BackendLlama, Path: "/x.gguf", Name: "x", Size: 1, ModTime: time.Unix(0, 0)},
	}
	m = m.layoutTable()
	m.params.open = true
	m.params.modelDisplayName = "test/model"
	m.params.profiles = []ParameterProfile{{Name: "default"}}

	bg := m.mainAppPlacedView()
	if !strings.Contains(bg, "LLM Launcher") {
		t.Fatalf("mainAppPlacedView missing title (len=%d)", len(bg))
	}

	v := m.View()
	content := v.Content
	if !strings.Contains(content, "LLM") || !strings.Contains(content, "Launcher") {
		t.Fatalf("overlaid view missing title (backdrop should remain above modal)")
	}
	if !strings.Contains(content, "Parameter Profiles") {
		t.Fatal("expected parameter profiles modal in view")
	}
	if !strings.Contains(content, "(active)") {
		t.Fatal("expected (active) prefix on active profile in parameters modal")
	}
	// Main footer remains visible in the backdrop on a tall layout (not covered by the modal).
	if !strings.Contains(content, FooterHintRunSplit) {
		t.Fatal("expected main footer in backdrop outside modal")
	}
}

func TestParamPanelContentWidth_wideTerminalUsesCap(t *testing.T) {
	m := New()
	m.layout.width = 200
	m.layout.height = 40
	if got := m.paramPanelContentWidth(); got != paramPanelMaxInnerWidth {
		t.Fatalf("paramPanelContentWidth = %d, want %d", got, paramPanelMaxInnerWidth)
	}
}
