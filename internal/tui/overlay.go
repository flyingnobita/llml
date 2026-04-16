package tui

import (
	"charm.land/lipgloss/v2"
	uv "github.com/charmbracelet/ultraviolet"
	"github.com/charmbracelet/x/ansi"
)

// overlayCentered paints backdrop full-screen, then draws modal centered on top.
// It uses a cell blit for the modal (Buffer.Draw) so the backdrop is not cleared
// under the modal rectangle; this preserves the main TUI (table, server log, etc.)
// unlike stacking two lipgloss layers via Compositor, where each StyledString.Draw
// clears its bounds first and can erase the title row or backdrop.
func overlayCentered(backdrop, modal string, termW, termH int) string {
	if termW < 1 || termH < 1 {
		return backdrop
	}

	canvas := lipgloss.NewCanvas(termW, termH)
	uv.NewStyledString(backdrop).Draw(canvas, canvas.Bounds())

	mw := lipgloss.Width(modal)
	mh := lipgloss.Height(modal)
	if mw < 1 {
		mw = 1
	}
	if mh < 1 {
		mh = 1
	}
	if mw > termW {
		mw = termW
	}
	if mh > termH {
		mh = termH
	}

	modalBuf := uv.NewScreenBuffer(mw, mh)
	modalBuf.Method = ansi.GraphemeWidth
	uv.NewStyledString(modal).Draw(modalBuf, modalBuf.Bounds())

	x := (termW - mw) / 2
	y := (termH - mh) / 2
	if x < 0 {
		x = 0
	}
	if y < 0 {
		y = 0
	}

	modalBuf.Buffer.Draw(canvas, uv.Rect(x, y, mw, mh))
	return canvas.Render()
}
