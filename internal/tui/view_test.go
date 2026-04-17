package tui

import (
	"strings"
	"testing"

	"charm.land/bubbles/v2/viewport"
)

func TestViewportVerticalScrollPercent_borderedViewportMatchesMaxYOffset(t *testing.T) {
	st := newStyles(DarkTheme())
	vp := viewport.New(viewport.WithWidth(40), viewport.WithHeight(6))
	vp.Style = st.launchPreviewViewport
	vp.SoftWrap = false
	lines := make([]string, 10)
	for i := range lines {
		lines[i] = "x"
	}
	vp.SetContent(strings.Join(lines, "\n"))
	vs := vp.Style.GetVerticalFrameSize()
	maxY := vp.TotalLineCount() - vp.Height() + vs
	if maxY <= 0 {
		t.Fatalf("expected scrollable content, maxY=%d", maxY)
	}
	vp.SetYOffset(maxY / 2)
	got := viewportVerticalScrollPercent(vp)
	if got < 0.49 || got > 0.51 {
		t.Fatalf("mid scroll want ~0.5, got %v (maxY=%d yOffset=%d)", got, maxY, vp.YOffset())
	}
	vp.SetYOffset(maxY)
	if got := viewportVerticalScrollPercent(vp); got != 1 {
		t.Fatalf("at bottom want 1, got %v", got)
	}
}
