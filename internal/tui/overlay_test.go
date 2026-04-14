package tui

import (
	"strings"
	"testing"
)

func TestOverlayCentered_preservesBackdropAndModal(t *testing.T) {
	lines := make([]string, 24)
	for i := range lines {
		lines[i] = strings.Repeat(".", 80)
	}
	lines[1] = "LLM" + strings.Repeat(".", 77)
	bg := strings.Join(lines, "\n")
	modal := "MODAL"
	out := overlayCentered(bg, modal, 80, 24)
	if !strings.Contains(out, "LLM") {
		t.Fatal("expected backdrop text after overlay")
	}
	if !strings.Contains(out, "MODAL") {
		t.Fatal("expected modal text in output")
	}
}
