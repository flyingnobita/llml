package models

import (
	"strings"
	"testing"
)

func TestGgufParamsSummary_nonexistentPath(t *testing.T) {
	if g := ggufParamsSummary("/nonexistent/does-not-exist/model.gguf"); g != "—" {
		t.Fatalf("got %q want —", g)
	}
}

func TestGGUFGeneralName_nonexistent(t *testing.T) {
	_, err := GGUFGeneralName("/nonexistent/does-not-exist/model.gguf")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestTruncateMeta_shortUnchanged(t *testing.T) {
	s := "short"
	if got := truncateMeta(s, 48); got != s {
		t.Fatalf("got %q", got)
	}
}

func TestTruncateMeta_longEllipsis(t *testing.T) {
	s := strings.Repeat("a", 60)
	got := truncateMeta(s, 48)
	r := []rune(got)
	if len(r) != 48 {
		t.Fatalf("len %d want 48", len(r))
	}
	if !strings.HasSuffix(got, "…") {
		t.Fatalf("expected ellipsis suffix: %q", got)
	}
}
