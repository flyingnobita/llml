package tui

import (
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/flyingnobita/llml/internal/models"
	"github.com/flyingnobita/llml/internal/profiles"
)

func setTestParamEditor(m *Model, ent modelEntry) {
	m.params.editor = newProfileEditor(ent)
}

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
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv("HOME", dir)
	t.Setenv("AppData", dir)
	m := New()
	m.table.files = []models.ModelFile{
		{Path: "/m/a.gguf", Backend: models.BackendLlama},
	}
	m.params.modelPath = "/m/a.gguf"
	m.params.open = true
	m.params.focus = paramFocusProfiles
	setTestParamEditor(&m, modelEntry{Profiles: []ParameterProfile{
		{
			Name:     "cuda",
			Backend:  "koboldcpp",
			UseCase:  profiles.UseCaseMetadata{Primary: profiles.UseCasePrimaries{profiles.UseCaseGeneral}, Tags: []string{"interactive"}},
			Hardware: profiles.HardwareMetadata{Class: profiles.HardwareClassGPU},
			Env:      []EnvVar{{Key: "FOO", Value: "bar"}},
			Args:     []string{"--x"},
		},
		{Name: "cpu"},
	}, ActiveIndex: 0})

	m, _ = m.updateParamPanelKey(tea.KeyPressMsg{Code: 'c', Text: "c"})
	if len(m.params.editor.profiles) != 3 {
		t.Fatalf("want 3 profiles, got %d", len(m.params.editor.profiles))
	}
	if m.params.editor.index != 1 {
		t.Fatalf("want cursor on new clone at index 1, got %d", m.params.editor.index)
	}
	clone := m.params.editor.profiles[1]
	if clone.Name != "cuda copy" {
		t.Fatalf("clone name = %q", clone.Name)
	}
	if len(clone.Env) != 1 || clone.Env[0].Key != "FOO" || clone.Env[0].Value != "bar" {
		t.Fatalf("clone env: %+v", clone.Env)
	}
	if len(clone.Args) != 1 || clone.Args[0] != "--x" {
		t.Fatalf("clone args: %+v", clone.Args)
	}
	if clone.Backend != "koboldcpp" || !slices.Contains(clone.UseCase.Primary, profiles.UseCaseGeneral) || clone.Hardware.Class != profiles.HardwareClassGPU {
		t.Fatalf("clone metadata: %+v", clone)
	}
	if m.params.editor.profiles[0].Name != "cuda" {
		t.Fatal("original profile name changed")
	}
	if len(m.params.editor.profiles[0].Env) != 1 {
		t.Fatal("original profile env should still be one row (synced from editor state)")
	}
}

func TestParamPanelDeleteConfirm(t *testing.T) {
	m := New()
	m.layout.width = 80
	m.layout.height = 24
	m.params.open = true
	m.params.focus = paramFocusProfiles
	setTestParamEditor(&m, modelEntry{Profiles: []ParameterProfile{{Name: "a"}, {Name: "b"}}, ActiveIndex: 0})

	m, _ = m.updateParamPanelKey(tea.KeyPressMsg{Code: 'd', Text: "d"})
	if m.params.confirmDelete != paramConfirmProfile {
		t.Fatal("expected confirm dialog after d with 2+ profiles")
	}
	if len(m.params.editor.profiles) != 2 {
		t.Fatal("delete must not run before confirmation")
	}

	m, _ = m.updateParamPanelKey(tea.KeyPressMsg{Code: 'n', Text: "n"})
	if m.params.confirmDelete != paramConfirmNone {
		t.Fatal("n should dismiss confirm dialog")
	}

	setTestParamEditor(&m, modelEntry{Profiles: []ParameterProfile{{Name: "only"}}, ActiveIndex: 0})
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
	setTestParamEditor(&m, modelEntry{Profiles: []ParameterProfile{{Name: "p", Env: []EnvVar{{Key: "K", Value: "V"}}, Args: nil}}, ActiveIndex: 0})

	m, _ = m.updateParamPanelKey(tea.KeyPressMsg{Code: 'd', Text: "d"})
	if m.params.confirmDelete != paramConfirmEnvRow {
		t.Fatalf("expected env row confirm, got %d", m.params.confirmDelete)
	}
	if len(m.params.editor.env) != 1 {
		t.Fatal("row not deleted yet")
	}
	m, _ = m.updateParamPanelKey(tea.KeyPressMsg{Code: 'n', Text: "n"})
	if m.params.confirmDelete != paramConfirmNone {
		t.Fatal("n should dismiss confirm")
	}
	if len(m.params.editor.env) != 1 {
		t.Fatal("row still present after cancel")
	}
}

func TestParamPanelEscapeClosesPanelByKeyCode(t *testing.T) {
	m := New()
	m.layout.width = 80
	m.layout.height = 24
	m.params.open = true
	m.params.focus = paramFocusProfiles
	setTestParamEditor(&m, modelEntry{Profiles: []ParameterProfile{{Name: "a"}}, ActiveIndex: 0})

	m, _ = m.updateParamPanelKey(tea.KeyPressMsg{Code: tea.KeyEscape})
	if m.params.open {
		t.Fatal("expected escape key code to close parameter panel")
	}
}

func TestParamPanelEscapeClosesEvenWithMissingRuntimeNote(t *testing.T) {
	m := New()
	m.layout.width = 80
	m.layout.height = 24
	m.table.files = []models.ModelFile{
		{Path: "/m/a.gguf", Backend: models.BackendLlama, Name: "a"},
	}
	m.table.effectiveBackends = map[string]models.ModelBackend{
		"/m/a.gguf": models.BackendKobold,
	}
	m.params.open = true
	m.params.focus = paramFocusProfiles
	m.params.modelPath = "/m/a.gguf"
	setTestParamEditor(&m, modelEntry{Profiles: []ParameterProfile{{Name: "a", Backend: "koboldcpp"}}, ActiveIndex: 0})

	m, _ = m.updateParamPanelKey(tea.KeyPressMsg{Code: tea.KeyEscape})
	if m.params.open {
		t.Fatal("expected escape to close parameter panel even when a missing-runtime note is set")
	}
	if !strings.Contains(m.lastRunNote, MissingKoboldCppFooterNote) {
		t.Fatalf("expected missing koboldcpp note after close, got %q", m.lastRunNote)
	}
}

func TestCommitParamLineEdit_blankEnvLikeCancel(t *testing.T) {
	m := New()
	m.params.focus = paramFocusEnv
	m.params.editor.env = []EnvVar{{Key: "K", Value: "V"}}
	m.params.editor.envCursor = 0
	m.params.editKind = paramEditEnvLine
	m.params.editInput.SetValue("   ")

	m = m.commitParamLineEdit()
	if m.paramEnvLen() != 1 || m.params.editor.env[0].Key != "K" || m.params.editor.env[0].Value != "V" {
		t.Fatalf("blank commit should keep existing env, got %#v", m.params.editor.env)
	}
	if m.params.editKind != paramEditNone {
		t.Fatal("expected edit closed")
	}
}

func TestCommitParamLineEdit_blankEnvRemovesNewEmptyRow(t *testing.T) {
	m := New()
	m.params.focus = paramFocusEnv
	m.params.editor.env = []EnvVar{{}}
	m.params.editor.envCursor = 0
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
	m.params.editor.args = []string{"--foo"}
	m.params.editor.argsCursor = 0
	m.params.editKind = paramEditArgLine
	m.params.editInput.SetValue("\t ")

	m = m.commitParamLineEdit()
	if m.paramArgsLen() != 1 || m.params.editor.args[0] != "--foo" {
		t.Fatalf("blank commit should keep existing arg, got %#v", m.params.editor.args)
	}
}

func TestCommitParamLineEdit_blankArgRemovesNewEmptyRow(t *testing.T) {
	m := New()
	m.params.focus = paramFocusArgs
	m.params.editor.args = []string{""}
	m.params.editor.argsCursor = 0
	m.params.editKind = paramEditArgLine
	m.params.editInput.SetValue("  ")

	m = m.commitParamLineEdit()
	if m.paramArgsLen() != 0 {
		t.Fatalf("blank commit on new empty arg row should remove row, got %#v", m.params.editor.args)
	}
}

func TestParamPanelEditTabDoesNotSwitchSections(t *testing.T) {
	m := New()
	m.params.open = true
	m.params.focus = paramFocusEnv
	m.params.editKind = paramEditEnvLine
	m.params.editor.env = []EnvVar{{Key: "K", Value: "V"}}
	m.params.editor.envCursor = 0
	m.params.editInput.SetValue("K=V")

	m, _ = m.updateParamPanelKey(tea.KeyPressMsg{Code: tea.KeyTab, Text: "tab"})
	if m.params.focus != paramFocusEnv {
		t.Fatalf("tab changed focus during edit: got %v", m.params.focus)
	}
	if m.params.editKind != paramEditEnvLine {
		t.Fatalf("tab ended edit: got %v", m.params.editKind)
	}

	m, _ = m.updateParamPanelKey(tea.KeyPressMsg{Text: "shift+tab"})
	if m.params.focus != paramFocusEnv {
		t.Fatalf("shift+tab changed focus during edit: got %v", m.params.focus)
	}
	if m.params.editKind != paramEditEnvLine {
		t.Fatalf("shift+tab ended edit: got %v", m.params.editKind)
	}
}

func TestParamPanelIdleTabSkipsArgsAsSeparateSection(t *testing.T) {
	m := New()
	m.params.open = true
	m.params.focus = paramFocusEnv
	m.params.editor.env = []EnvVar{{Key: "K", Value: "V"}}
	m.params.editor.args = []string{"--ctx-size 4096"}

	m, _ = m.updateParamPanelKey(tea.KeyPressMsg{Code: tea.KeyTab, Text: "tab"})
	if m.params.focus != paramFocusProfiles {
		t.Fatalf("tab from env focus should wrap to profiles, got %v", m.params.focus)
	}

	m.params.focus = paramFocusArgs
	m, _ = m.updateParamPanelKey(tea.KeyPressMsg{Text: "shift+tab"})
	if m.params.focus != paramFocusMetadata {
		t.Fatalf("shift+tab from args focus should go to metadata, got %v", m.params.focus)
	}
}

func TestParamPanelCursorCrossesBetweenEnvAndArgs(t *testing.T) {
	m := New()
	m.params.open = true
	m.params.focus = paramFocusEnv
	m.params.editor.env = []EnvVar{{Key: "K", Value: "V"}}
	m.params.editor.args = []string{"--ctx-size 4096", "--threads 8"}
	m.params.editor.envCursor = 0

	m, _ = m.moveParamCursor(1)
	if m.params.focus != paramFocusArgs || m.params.editor.argsCursor != 0 {
		t.Fatalf("down from last env row should enter args, got focus=%v argsCursor=%d", m.params.focus, m.params.editor.argsCursor)
	}

	m.params.editor.argsCursor = 0
	m, _ = m.moveParamCursor(-1)
	if m.params.focus != paramFocusEnv || m.params.editor.envCursor != 0 {
		t.Fatalf("up from first arg row should return to env, got focus=%v envCursor=%d", m.params.focus, m.params.editor.envCursor)
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
	setTestParamEditor(&m, modelEntry{Profiles: []ParameterProfile{{
		Name:    "default",
		Backend: "vllm",
		UseCase: profiles.UseCaseMetadata{Primary: profiles.UseCasePrimaries{profiles.UseCaseGeneral}, Tags: []string{"interactive"}},
		Hardware: profiles.HardwareMetadata{
			Class: profiles.HardwareClassGPU,
		},
	}}, ActiveIndex: 0})

	bg := m.mainAppPlacedView()
	if !strings.Contains(bg, "LLM Launcher") {
		t.Fatalf("mainAppPlacedView missing title (len=%d)", len(bg))
	}

	v := m.View()
	content := ansi.Strip(v.Content)
	if !strings.Contains(content, "LLM") || !strings.Contains(content, "Launcher") {
		t.Fatalf("overlaid view missing title (backdrop should remain above modal)")
	}
	if !strings.Contains(content, "Parameter Profiles") {
		t.Fatal("expected parameter profiles modal in view")
	}
	if !strings.Contains(content, "Profile Metadata") {
		t.Fatal("expected metadata section in parameters modal")
	}
	// Hardware Class is GPU — radio row must show the selected chip.
	flatContent := strings.ReplaceAll(content, "\n", " ")
	if !strings.Contains(flatContent, "(•) gpu") {
		t.Fatal("expected hardware class gpu radio chip in parameters modal")
	}
	if !strings.Contains(content, "(active)") {
		t.Fatal("expected (active) prefix on active profile in parameters modal")
	}
	// Main footer remains visible in the backdrop on a tall layout (not covered by the modal).
	if !strings.Contains(content, FooterHintRunSplit) {
		t.Fatal("expected main footer in backdrop outside modal")
	}
}

func TestParamPanelMetadataTracksActiveProfile(t *testing.T) {
	m := New()
	m.layout.width = 120
	m.layout.height = 40
	m.params.open = true
	m.params.modelDisplayName = "test/model"
	setTestParamEditor(&m, modelEntry{Profiles: []ParameterProfile{
		{
			Name:    "general",
			Backend: "vllm",
			UseCase: profiles.UseCaseMetadata{Primary: profiles.UseCasePrimaries{profiles.UseCaseGeneral}, Tags: []string{"coding"}},
			Hardware: profiles.HardwareMetadata{
				Class: profiles.HardwareClassGPU,
			},
		},
		{
			Name:    "cpu",
			Backend: "",
			UseCase: profiles.UseCaseMetadata{},
			Hardware: profiles.HardwareMetadata{
				Class: profiles.HardwareClassCPU,
				Notes: "quiet box",
			},
		},
	}, ActiveIndex: 0})
	view1 := m.paramPanelModalBlock()
	flat1 := strings.ReplaceAll(view1, "\n", " ")
	// Backend radio shows "(•) vllm"; Primary and Tags checkboxes show checked chips.
	if !strings.Contains(flat1, "(•) vllm") || !strings.Contains(flat1, "[✓] general") || !strings.Contains(flat1, "[✓] coding") {
		t.Fatalf("expected active metadata for first profile:\n%s", view1)
	}
	m = m.moveProfile(1)
	view2 := m.paramPanelModalBlock()
	flat2 := strings.ReplaceAll(view2, "\n", " ")
	// Hardware Class radio shows "(•) cpu"; Notes field (renamed) shows value.
	if !strings.Contains(flat2, "(•) cpu") {
		t.Fatalf("expected hardware class cpu radio in second profile:\n%s", view2)
	}
	if !strings.Contains(view2, "Notes") || !strings.Contains(view2, "quiet box") {
		t.Fatalf("expected notes field for second profile:\n%s", view2)
	}
	// Backend radio shows "(•) (none)" for empty backend; Primary shows "[ ] general".
	if !strings.Contains(flat2, "(•) (none)") || !strings.Contains(flat2, "Use Case Primary") || !strings.Contains(flat2, "[ ] general") {
		t.Fatalf("expected unspecified placeholders:\n%s", view2)
	}
}

func TestParamPanelProfilesSectionUsesFocusedChrome(t *testing.T) {
	m := New()
	m.layout.width = 100
	m.layout.height = 40
	m.params.open = true
	setTestParamEditor(&m, modelEntry{Profiles: []ParameterProfile{{Name: "default"}}, ActiveIndex: 0})
	m.params.focus = paramFocusProfiles
	focused := m.paramPanelModalBlock()

	m.params.focus = paramFocusMetadata
	unfocused := m.paramPanelModalBlock()
	if focused == unfocused {
		t.Fatal("expected profile section rendering to differ when its section is focused")
	}
}

func TestParamPanelFooterHintsMarkTabAsIdleOnly(t *testing.T) {
	if !strings.Contains(FooterParamFooterProfiles, "tab: section") {
		t.Fatalf("profiles footer hints = %q", FooterParamFooterProfiles)
	}
	if !strings.Contains(FooterParamFooterDetailRows, "tab: section") {
		t.Fatalf("detail footer hints = %q", FooterParamFooterDetailRows)
	}
}

func TestHelpPanelDocumentsIdleOnlySectionSwitching(t *testing.T) {
	m := New()
	m.layout.width = 100
	m.layout.height = 40
	view := m.helpPanelModalBlock()
	if !strings.Contains(view, "Section") {
		t.Fatalf("help panel missing section hint:\n%s", view)
	}
}

func TestParamPanelMetadataTagsRowRendersCheckboxes(t *testing.T) {
	m := New()
	m.layout.width = 120
	m.layout.height = 40
	m.params.open = true
	m.params.focus = paramFocusMetadata
	m.params.metadataCursor = int(paramMetadataUseCaseTags)
	m.params.modelDisplayName = "test/model"
	setTestParamEditor(&m, modelEntry{Profiles: []ParameterProfile{{
		Name:    "default",
		UseCase: profiles.UseCaseMetadata{Tags: []string{"thinking", "coding"}},
	}}, ActiveIndex: 0})

	view := m.paramPanelModalBlock()
	// Collapse newlines so word-wrapped chips are still detectable as substrings.
	flat := strings.ReplaceAll(view, "\n", " ")
	if !strings.Contains(flat, "Tags") {
		t.Fatalf("expected Tags label in view:\n%s", view)
	}
	if !strings.Contains(flat, "[✓] thinking") {
		t.Fatalf("expected checked thinking tag:\n%s", view)
	}
	if !strings.Contains(flat, "[✓] coding") {
		t.Fatalf("expected checked coding tag:\n%s", view)
	}
	if !strings.Contains(flat, "[ ] image") {
		t.Fatalf("expected unchecked image tag:\n%s", view)
	}
}

func TestParamPanelMetadataEditsPersistAndSwitchProfiles(t *testing.T) {
	cfg := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", cfg)
	t.Setenv("HOME", cfg)
	t.Setenv("AppData", cfg)

	modelPath := filepath.Join(cfg, "meta-edit.gguf")
	if err := saveModelEntry(modelPath, modelEntry{
		Profiles: []ParameterProfile{
			{Name: "general"},
			{Name: "cpu", Backend: "ollama", Hardware: profiles.HardwareMetadata{Class: profiles.HardwareClassCPU}},
		},
		ActiveIndex: 0,
	}); err != nil {
		t.Fatal(err)
	}

	m := New()
	m.table.files = []models.ModelFile{
		{Path: filepath.Clean(modelPath), Backend: models.BackendLlama},
	}
	m.params.open = true
	m.params.modelPath = filepath.Clean(modelPath)
	m.params.modelDisplayName = "meta-edit.gguf"
	ent, err := loadModelEntry(modelPath)
	if err != nil {
		t.Fatal(err)
	}
	setTestParamEditor(&m, ent)
	m.params.focus = paramFocusMetadata

	m.params.metadataCursor = int(paramMetadataBackend)
	// backendCursor starts at 0 ("(none)"); move right to "llama" then select with space.
	m, _ = m.updateParamPanelKey(tea.KeyPressMsg{Code: tea.KeyRight, Text: ""})
	m, _ = m.updateParamPanelKey(tea.KeyPressMsg{Code: ' ', Text: "space"})
	if got := m.params.editor.profiles[0].Backend; got != "llama" {
		t.Fatalf("backend = %q", got)
	}

	// Toggle "general" (primaryCursor=0 → CanonicalPrimaries[0]) via space.
	m.params.metadataCursor = int(paramMetadataUseCasePrimary)
	m.params.primaryCursor = 0 // "general" is CanonicalPrimaries[0]
	m, _ = m.updateParamPanelKey(tea.KeyPressMsg{Code: ' ', Text: "space"})
	if got := m.params.editor.profiles[0].UseCase.Primary; !slices.Contains(got, profiles.UseCaseGeneral) {
		t.Fatalf("use case primary = %v", got)
	}

	// Toggle the "image" tag via the Tags checkbox row (CanonicalTags[0] == "image").
	m.params.metadataCursor = int(paramMetadataUseCaseTags)
	m.params.tagCursor = 0 // "image" is profiles.CanonicalTags[0]
	m, _ = m.updateParamPanelKey(tea.KeyPressMsg{Code: ' ', Text: "space"})
	if got := m.params.editor.profiles[0].UseCase.Tags; !hasTag(got, "image") {
		t.Fatalf("tags = %#v, expected image tag after space toggle", got)
	}

	m = m.moveProfile(1)
	if got := m.params.editor.profiles[m.params.editor.index].Backend; got != "ollama" {
		t.Fatalf("second profile backend = %q", got)
	}
	if got := m.params.editor.profiles[m.params.editor.index].Hardware.Class; got != profiles.HardwareClassCPU {
		t.Fatalf("second profile hardware class = %q", got)
	}

	got, err := loadModelEntry(modelPath)
	if err != nil {
		t.Fatal(err)
	}
	if got.Profiles[0].Backend != "llama" {
		t.Fatalf("saved backend = %q", got.Profiles[0].Backend)
	}
	if !slices.Contains(got.Profiles[0].UseCase.Primary, profiles.UseCaseGeneral) {
		t.Fatalf("saved use case = %v", got.Profiles[0].UseCase.Primary)
	}
	if !hasTag(got.Profiles[0].UseCase.Tags, "image") {
		t.Fatalf("saved tags = %#v, expected image tag", got.Profiles[0].UseCase.Tags)
	}
}

func TestParamPanelHardwareMetadataPersistsAndClears(t *testing.T) {
	cfg := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", cfg)
	t.Setenv("HOME", cfg)
	t.Setenv("AppData", cfg)

	modelPath := filepath.Join(cfg, "hardware-edit.gguf")
	if err := saveModelEntry(modelPath, modelEntry{
		Profiles:    []ParameterProfile{{Name: "gpu"}},
		ActiveIndex: 0,
	}); err != nil {
		t.Fatal(err)
	}

	m := New()
	m.params.open = true
	m.params.modelPath = filepath.Clean(modelPath)
	m.params.modelDisplayName = "hardware-edit.gguf"
	ent, err := loadModelEntry(modelPath)
	if err != nil {
		t.Fatal(err)
	}
	setTestParamEditor(&m, ent)
	m.params.focus = paramFocusMetadata

	m.params.metadataCursor = int(paramMetadataHardwareClass)
	// hardwareClassCursor starts at 0 (unspecified); right×2 to gpu, then select with space.
	m, _ = m.updateParamPanelKey(tea.KeyPressMsg{Code: tea.KeyRight, Text: ""}) // 0→1 (cpu)
	m, _ = m.updateParamPanelKey(tea.KeyPressMsg{Code: tea.KeyRight, Text: ""}) // 1→2 (gpu)
	m, _ = m.updateParamPanelKey(tea.KeyPressMsg{Code: ' ', Text: "space"})     // select gpu
	if got := m.params.editor.profiles[0].Hardware.Class; got != profiles.HardwareClassGPU {
		t.Fatalf("hardware class = %q", got)
	}

	setField := func(field paramMetadataField, value string) {
		t.Helper()
		m.params.metadataCursor = int(field)
		m.params.editKind = paramEditMetadataValue
		if field == paramMetadataHardwareNotes {
			m.params.notesInput.SetValue(value)
		} else {
			m.params.editInput.SetValue(value)
		}
		m = m.commitParamLineEdit()
	}
	setField(paramMetadataHardwareGPUCount, "2")
	setField(paramMetadataHardwareMinVRAM, "48")
	setField(paramMetadataHardwareMaxVRAM, "24")
	setField(paramMetadataHardwareNotes, "  tested on 4090  ")
	m, _ = m.persistParamPanel()

	got, err := loadModelEntry(modelPath)
	if err != nil {
		t.Fatal(err)
	}
	if got.Profiles[0].Hardware.GPUCount == nil || *got.Profiles[0].Hardware.GPUCount != 2 {
		t.Fatalf("saved gpu count = %#v", got.Profiles[0].Hardware.GPUCount)
	}
	if got.Profiles[0].Hardware.MinVRAMGB == nil || *got.Profiles[0].Hardware.MinVRAMGB != 24 {
		t.Fatalf("saved min vram = %#v", got.Profiles[0].Hardware.MinVRAMGB)
	}
	if got.Profiles[0].Hardware.MaxVRAMGB == nil || *got.Profiles[0].Hardware.MaxVRAMGB != 48 {
		t.Fatalf("saved max vram = %#v", got.Profiles[0].Hardware.MaxVRAMGB)
	}
	if got.Profiles[0].Hardware.Notes != "tested on 4090" {
		t.Fatalf("saved notes = %q", got.Profiles[0].Hardware.Notes)
	}

	setField(paramMetadataHardwareGPUCount, "")
	setField(paramMetadataHardwareMinVRAM, "")
	setField(paramMetadataHardwareMaxVRAM, "")
	setField(paramMetadataHardwareNotes, "")
	m, _ = m.persistParamPanel()

	got, err = loadModelEntry(modelPath)
	if err != nil {
		t.Fatal(err)
	}
	if got.Profiles[0].Hardware.GPUCount != nil || got.Profiles[0].Hardware.MinVRAMGB != nil || got.Profiles[0].Hardware.MaxVRAMGB != nil {
		t.Fatalf("expected cleared numeric hardware fields, got %+v", got.Profiles[0].Hardware)
	}
	if got.Profiles[0].Hardware.Notes != "" {
		t.Fatalf("expected cleared notes, got %q", got.Profiles[0].Hardware.Notes)
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
