package tui

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
)

func TestResolveThemeWithDetector(t *testing.T) {
	cases := []struct {
		name      string
		env       string
		darkBG    bool
		wantTheme Theme
	}{
		{"explicit_dark", "dark", false, DarkTheme()},
		{"explicit_light", "light", true, LightTheme()},
		{"case_insensitive_DaRk", "DaRk", false, DarkTheme()},
		{"auto_empty_dark_terminal", "", true, DarkTheme()},
		{"auto_keyword_light_terminal", "auto", false, LightTheme()},
		{"unknown_falls_back_to_detector", "not-a-theme", true, DarkTheme()},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv(EnvLLMLTheme, tc.env)
			det := tc.darkBG
			got := resolveThemeWithDetector(func() bool { return det })
			if got != tc.wantTheme {
				t.Fatalf("got %+v want %+v", got, tc.wantTheme)
			}
		})
	}
}

func TestThemeToastText(t *testing.T) {
	cases := []struct {
		pick  int
		theme Theme
		want  string
	}{
		{themePickDark, DarkTheme(), "Theme: dark"},
		{themePickLight, LightTheme(), "Theme: light"},
		{themePickAuto, DarkTheme(), "Theme: auto (dark)"},
		{themePickAuto, LightTheme(), "Theme: auto (light)"},
	}
	for _, tc := range cases {
		if s := themeToastText(tc.pick, tc.theme); s != tc.want {
			t.Fatalf("pick %d: got %q want %q", tc.pick, s, tc.want)
		}
	}
}

func TestDarkAndLightThemesDistinct(t *testing.T) {
	d, l := DarkTheme(), LightTheme()
	if d.Body == l.Body {
		t.Fatal("expected Body colors to differ between dark and light themes")
	}
	if d.TableSelected == l.TableSelected {
		t.Fatal("expected TableSelected colors to differ")
	}
	if d.TableSelectedBg == l.TableSelectedBg {
		t.Fatal("expected TableSelectedBg colors to differ between themes")
	}
	if d.SplitPaneBorderFocused == l.SplitPaneBorderFocused {
		t.Fatal("expected SplitPaneBorderFocused colors to differ between themes")
	}
	if d.ParamProfileInactive == d.ParamProfileName {
		t.Fatal("expected ParamProfileInactive to differ from ParamProfileName (dark)")
	}
	if l.ParamProfileInactive == l.ParamProfileName {
		t.Fatal("expected ParamProfileInactive to differ from ParamProfileName (light)")
	}
}

func TestSplitPaneChromeStylesDifferFocusedVsDim(t *testing.T) {
	st := newStyles(DarkTheme())
	if st.splitPaneChromeFocused.Render("x") == st.splitPaneChromeDim.Render("x") {
		t.Fatal("focused and dim split-pane chrome should render differently")
	}
}

func TestParamSectionBoxBorderChangesWhenDetailFocused(t *testing.T) {
	st := newStyles(DarkTheme())
	plain := st.paramSectionBox.Width(12).Render("x")
	focused := st.paramSectionBoxFocused.Width(12).Render("x")
	if plain == focused {
		t.Fatal("param section focused style should change border rendering vs unfocused")
	}
}

func TestParamPanelBoxUsesTighterVerticalPadding(t *testing.T) {
	st := newStyles(DarkTheme())
	port := st.portConfigBox.Width(20).Render("x")
	panel := st.paramPanelBox.Width(20).Render("x")
	if lipgloss.Height(panel) >= lipgloss.Height(port) {
		t.Fatalf("expected param panel box to be tighter than generic modal box: panel=%d port=%d", lipgloss.Height(panel), lipgloss.Height(port))
	}
}

func TestThemesHaveTableSelectedBackground(t *testing.T) {
	for _, th := range []struct {
		name  string
		theme Theme
	}{
		{"dark", DarkTheme()},
		{"light", LightTheme()},
	} {
		if th.theme.TableSelectedBg == nil {
			t.Fatalf("%s theme: TableSelectedBg must not be nil", th.name)
		}
		if th.theme.TableSelected == nil {
			t.Fatalf("%s theme: TableSelected must not be nil", th.name)
		}
	}
}

func TestNewModelHasThemedStyles(t *testing.T) {
	t.Setenv(EnvLLMLTheme, "dark")
	m := New()
	if m.ui.themePick != themePickDark {
		t.Fatalf("expected themePickDark, got %d", m.ui.themePick)
	}
	if m.ui.theme != DarkTheme() {
		t.Fatalf("expected DarkTheme on model, got %+v", m.ui.theme)
	}
	if got := m.ui.styles.title.Render("x"); got == "" {
		t.Fatal("expected non-empty themed title render")
	}
}

func TestInitialThemePick(t *testing.T) {
	t.Setenv(EnvLLMLTheme, "light")
	if p := initialThemePick(); p != themePickLight {
		t.Fatalf("light: got %d", p)
	}
	t.Setenv(EnvLLMLTheme, "")
	if p := initialThemePick(); p != themePickAuto {
		t.Fatalf("empty: want auto got %d", p)
	}
	t.Setenv(EnvLLMLTheme, "bogus")
	if p := initialThemePick(); p != themePickAuto {
		t.Fatalf("bogus: want auto got %d", p)
	}
}

func TestAppTitleBlockIncludesInlineToast(t *testing.T) {
	t.Setenv(EnvLLMLTheme, "dark")
	m := New()
	m.layout.width = 120
	m.layout.height = 40
	m.layout.bodyInnerW = m.layout.width - appPaddingH*2
	m.ui.themeToast = "Theme: light"
	block := m.appTitleBlock(m.innerWidth())
	if block == "" {
		t.Fatal("empty title block")
	}
	if !strings.Contains(block, "LLM Launcher") || !strings.Contains(block, "Theme: light") {
		t.Fatalf("expected title and toast in one block, got %q", block)
	}
}

func TestCycleThemeRotatesPick(t *testing.T) {
	t.Setenv(EnvLLMLTheme, "dark")
	m := New()
	if m.ui.themePick != themePickDark {
		t.Fatalf("start pick %d", m.ui.themePick)
	}
	m, _ = m.cycleTheme()
	if m.ui.themeToast == "" {
		t.Fatal("expected theme toast after cycle")
	}
	if m.ui.themePick != themePickLight {
		t.Fatalf("after 1: want light got %d", m.ui.themePick)
	}
	if m.ui.theme != LightTheme() {
		t.Fatal("expected LightTheme palette")
	}
	m, _ = m.cycleTheme()
	if m.ui.themePick != themePickAuto {
		t.Fatalf("after 2: want auto got %d", m.ui.themePick)
	}
	m, _ = m.cycleTheme()
	if m.ui.themePick != themePickDark {
		t.Fatalf("after 3: want dark got %d", m.ui.themePick)
	}
}

// Styles built once in newStyles must follow a theme change. The help panel
// styles moved out of the render path into newStyles, so this guards that they
// are rebuilt rather than captured once.
func TestCycleTheme_restylesHelpPanel(t *testing.T) {
	m := newTestModel()

	m.ui.themePick = themePickDark
	m.ui.theme = themeFromPick(m.ui.themePick, true)
	m.ui.styles = newStyles(m.ui.theme)
	darkKey := m.ui.styles.helpKey.GetForeground()
	darkSection := m.ui.styles.helpSectionTitle.GetForeground()

	m2, _ := m.cycleTheme() // dark -> light

	if m2.ui.styles.helpKey.GetForeground() == darkKey {
		t.Error("help key style did not follow the theme change")
	}
	if m2.ui.styles.helpSectionTitle.GetForeground() == darkSection {
		t.Error("help section title style did not follow the theme change")
	}
}

// The Notes textarea holds its own copy of its styles, so cycleTheme pushes the
// current ones into it. Those styles are deliberately empty today (they exist to
// clear distracting bubbles defaults), so this asserts the wiring, not a colour.
func TestCycleTheme_refreshesNotesTextareaStyles(t *testing.T) {
	m := newTestModel()
	m2, _ := m.cycleTheme()

	got := m2.params.notesInput.Styles()
	want := m2.ui.styles.notesTextarea
	if got.Focused.CursorLine.GetBackground() != want.Focused.CursorLine.GetBackground() {
		t.Error("notes textarea styles were not refreshed on theme change")
	}
}
