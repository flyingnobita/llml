package tui

import (
	"github.com/mattn/go-runewidth"

	"github.com/flyingnobita/llm-launch/internal/llamacpp"
	btable "github.com/flyingnobita/llm-launch/internal/tui/btable"
)

// tableColumns computes per-column widths from the terminal width and the
// current file list. Name expands to fit content (capped at maxNameColW);
// Path takes remaining space after fixed columns.
func tableColumns(totalWidth int, files []llamacpp.ModelFile) []btable.Column {
	if totalWidth < minTerminalWidth {
		totalWidth = minTerminalWidth
	}
	nameW := defaultNameColW
	longestName := 0
	longestPath := 0
	for _, f := range files {
		if w := runewidth.StringWidth(f.Name); w > longestName {
			longestName = w
		}
		if w := runewidth.StringWidth(llamacpp.FormatModelFolderDisplay(f.Path)); w > longestPath {
			longestPath = w
		}
	}
	if longestName > nameW {
		nameW = longestName
		if nameW > maxNameColW {
			nameW = maxNameColW
		}
	}
	fixed := nameW + sizeColW + modTimeColW + paramColW + colPaddingExtra
	pathW := totalWidth - fixed
	if pathW < minPathColW {
		pathW = minPathColW
	}
	if longestPath+2 > pathW {
		pathW = longestPath + 2
	}
	if pathW > maxPathColW {
		pathW = maxPathColW
	}

	return []btable.Column{
		{Title: "Name", Width: nameW},
		{Title: "Path", Width: pathW},
		{Title: "Size", Width: sizeColW},
		{Title: "Last modified", Width: modTimeColW},
		{Title: "Parameters", Width: paramColW},
	}
}

// tableContentMinWidth approximates the minimum row width so the outer
// viewport knows how wide to make the table (bubbles/table pads each cell).
func tableContentMinWidth(cols []btable.Column) int {
	sum := 0
	for _, c := range cols {
		sum += c.Width
	}
	return sum + 4*len(cols)
}

// buildTableRows converts ModelFile entries into display rows using the
// column widths computed by tableColumns. Cells are truncated to fit.
func buildTableRows(files []llamacpp.ModelFile, cols []btable.Column) []btable.Row {
	if len(cols) < 5 {
		return nil
	}
	rows := make([]btable.Row, len(files))
	for i, f := range files {
		rows[i] = btable.Row{
			llamacpp.TruncateRunes(f.Name, cols[0].Width-1),
			llamacpp.TruncateRunes(llamacpp.FormatModelFolderDisplay(f.Path), cols[1].Width-1),
			llamacpp.FormatSize(f.Size),
			llamacpp.FormatModTime(f.ModTime),
			llamacpp.TruncateRunes(f.Parameters, cols[4].Width-1),
		}
	}
	return rows
}
