package tui

import (
	"strings"

	"charm.land/bubbles/v2/viewport"
)

// viewportVerticalScrollPercent returns [0,1] for vertical scroll position. The
// upstream [viewport.Model.ScrollPercent] compares outer Height to total line count
// and divides by (total−Height), which is wrong for bordered viewports (the maximum
// Y offset uses total−Height+frameSize) and breaks when SoftWrap inflates total.
func viewportVerticalScrollPercent(vp viewport.Model) float64 {
	total := vp.TotalLineCount()
	if total == 0 {
		return 0
	}
	vs := vp.Style.GetVerticalFrameSize()
	maxY := total - vp.Height() + vs
	if maxY <= 0 {
		return 0
	}
	y := float64(vp.YOffset())
	p := y / float64(maxY)
	if p < 0 {
		return 0
	}
	if p > 1 {
		return 1
	}
	return p
}

// verticalScrollBarColumn renders a single-column scroll indicator: filled cells
// from the top grow with scroll position ([viewportVerticalScrollPercent]).
func verticalScrollBarColumn(pct float64, trackH int) string {
	if trackH < 2 {
		return ""
	}
	if pct < 0 {
		pct = 0
	}
	if pct > 1 {
		pct = 1
	}
	filled := int(pct * float64(trackH))
	if filled > trackH {
		filled = trackH
	}
	var b strings.Builder
	for i := 0; i < trackH; i++ {
		if i > 0 {
			b.WriteByte('\n')
		}
		if i < filled {
			b.WriteString("█")
		} else {
			b.WriteString("░")
		}
	}
	return b.String()
}

// horizontalScrollBarLine renders a filled track (█) and remainder (░) for horizontal scroll position.
func horizontalScrollBarLine(pct float64, maxWidth int) string {
	if maxWidth < 14 {
		return ""
	}
	inner := maxWidth - 4
	if inner < 8 {
		return ""
	}
	filled := int(pct * float64(inner))
	if filled > inner {
		filled = inner
	}
	if filled < 0 {
		filled = 0
	}
	return "  " + strings.Repeat("█", filled) + strings.Repeat("░", inner-filled) + "  "
}

// clampRenderedHeightKeepTopBottom trims a rendered multi-line string to maxH lines,
// preserving the top and bottom halves and discarding the middle.
func clampRenderedHeightKeepTopBottom(s string, maxH int) string {
	if maxH <= 0 {
		return s
	}
	lines := strings.Split(s, "\n")
	if len(lines) <= maxH {
		return s
	}
	topKeep := maxH / 2
	if topKeep < 1 {
		topKeep = 1
	}
	bottomKeep := maxH - topKeep
	if bottomKeep < 1 {
		bottomKeep = 1
	}
	out := append([]string{}, lines[:topKeep]...)
	out = append(out, lines[len(lines)-bottomKeep:]...)
	return strings.Join(out, "\n")
}
