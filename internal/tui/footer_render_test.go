package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"

	"github.com/flyingnobita/llml/internal/models"
)

func TestRenderFooterHints_SemanticallyStylesKeysAndSeparators(t *testing.T) {
	m := newTestModel()
	line := FooterHintTabSections + FooterHintSep + FooterNavHint + FooterHintSep + FooterHintHelp

	got := m.renderFooterHints(line)
	plainText := ansi.Strip(got)
	if !strings.Contains(plainText, FooterHintTabSections) || !strings.Contains(plainText, FooterNavHint) || !strings.Contains(plainText, FooterHintHelp) {
		t.Fatalf("rendered footer should preserve hint text, got %q", got)
	}

	plain := m.ui.styles.footer.Render(line)
	if got == plain {
		t.Fatal("expected semantic footer rendering to differ from plain single-style footer rendering")
	}
}

func TestDarkThemeFooterColorsAreDistinct(t *testing.T) {
	th := DarkTheme()
	if th.Footer == th.FooterKey {
		t.Fatal("expected dark theme footer description and key colors to differ")
	}
	if th.Footer == th.FooterSep {
		t.Fatal("expected dark theme footer description and separator colors to differ")
	}
}

func TestImportFooter_UsesSemanticHintRendering(t *testing.T) {
	m := newTestModel()
	m.import_.focus = importFocusPicker

	got := m.importFooter()
	plain := m.ui.styles.footer.Render("tab: path input · enter: select · " + FooterNavHint + " · esc: back")
	if got == plain {
		t.Fatal("expected import footer to use semantic hint rendering, not flat footer color")
	}
}

func TestMainAppPlacedView_HasBlankRowAboveFooter(t *testing.T) {
	m := newTestModel()
	m.layout.width = 100
	m.layout.height = 32
	m.table.files = []models.ModelFile{
		{Backend: models.BackendLlama, Path: "/a.gguf", Name: "a"},
		{Backend: models.BackendLlama, Path: "/b.gguf", Name: "b"},
	}
	m = m.layoutTable()

	content := ansi.Strip(m.mainAppPlacedView())
	lines := strings.Split(content, "\n")
	footerIdx := -1
	for i, line := range lines {
		if strings.Contains(line, FooterNavHint) {
			footerIdx = i
			break
		}
	}
	if footerIdx < 1 {
		t.Fatalf("expected footer line in main view, got %q", content)
	}
	if strings.TrimSpace(lines[footerIdx-1]) != "" {
		t.Fatalf("expected a blank spacer row above the footer, got %q", lines[footerIdx-1])
	}
}
