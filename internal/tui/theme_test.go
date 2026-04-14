package tui

import (
	"strings"
	"testing"
)

func TestResolveThemeWithDetector_explicitDark(t *testing.T) {
	t.Setenv(EnvLLMLTheme, "dark")
	got := resolveThemeWithDetector(func() bool { return false })
	want := DarkTheme()
	if got != want {
		t.Fatalf("dark env: got %+v want %+v", got, want)
	}
}

func TestResolveThemeWithDetector_explicitLight(t *testing.T) {
	t.Setenv(EnvLLMLTheme, "light")
	got := resolveThemeWithDetector(func() bool { return true })
	want := LightTheme()
	if got != want {
		t.Fatalf("light env: got %+v want %+v", got, want)
	}
}

func TestResolveThemeWithDetector_caseInsensitive(t *testing.T) {
	t.Setenv(EnvLLMLTheme, "DaRk")
	got := resolveThemeWithDetector(func() bool { return false })
	if got != DarkTheme() {
		t.Fatalf("expected DarkTheme for DaRk, got %+v", got)
	}
}

func TestResolveThemeWithDetector_autoEmptyUsesDetector(t *testing.T) {
	t.Setenv(EnvLLMLTheme, "")
	gotDark := resolveThemeWithDetector(func() bool { return true })
	if gotDark != DarkTheme() {
		t.Fatalf("auto + dark terminal: got %+v", gotDark)
	}
	t.Setenv(EnvLLMLTheme, "auto")
	gotLight := resolveThemeWithDetector(func() bool { return false })
	if gotLight != LightTheme() {
		t.Fatalf("auto + light terminal: got %+v", gotLight)
	}
}

func TestResolveThemeWithDetector_unknownUsesDetector(t *testing.T) {
	t.Setenv(EnvLLMLTheme, "not-a-theme")
	got := resolveThemeWithDetector(func() bool { return true })
	if got != DarkTheme() {
		t.Fatalf("unknown env + dark detector: got %+v", got)
	}
}

func TestThemeToastText(t *testing.T) {
	if s := themeToastText(themePickDark, DarkTheme()); s != "Theme: dark" {
		t.Fatalf("dark: %q", s)
	}
	if s := themeToastText(themePickLight, LightTheme()); s != "Theme: light" {
		t.Fatalf("light: %q", s)
	}
	if s := themeToastText(themePickAuto, DarkTheme()); s != "Theme: auto (dark)" {
		t.Fatalf("auto dark: %q", s)
	}
	if s := themeToastText(themePickAuto, LightTheme()); s != "Theme: auto (light)" {
		t.Fatalf("auto light: %q", s)
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
	if m.themePick != themePickDark {
		t.Fatalf("expected themePickDark, got %d", m.themePick)
	}
	if m.theme != DarkTheme() {
		t.Fatalf("expected DarkTheme on model, got %+v", m.theme)
	}
	if got := m.styles.title.Render("x"); got == "" {
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
	m.width = 120
	m.height = 40
	m.bodyInnerW = m.width - appPaddingH*2
	m.themeToast = "Theme: light"
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
	if m.themePick != themePickDark {
		t.Fatalf("start pick %d", m.themePick)
	}
	m, _ = m.cycleTheme()
	if m.themeToast == "" {
		t.Fatal("expected theme toast after cycle")
	}
	if m.themePick != themePickLight {
		t.Fatalf("after 1: want light got %d", m.themePick)
	}
	if m.theme != LightTheme() {
		t.Fatal("expected LightTheme palette")
	}
	m, _ = m.cycleTheme()
	if m.themePick != themePickAuto {
		t.Fatalf("after 2: want auto got %d", m.themePick)
	}
	m, _ = m.cycleTheme()
	if m.themePick != themePickDark {
		t.Fatalf("after 3: want dark got %d", m.themePick)
	}
}
