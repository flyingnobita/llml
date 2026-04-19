package tui

import (
	"strings"
	"testing"
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
