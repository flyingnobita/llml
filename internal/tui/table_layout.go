package tui

import (
	"github.com/mattn/go-runewidth"

	btable "charm.land/bubbles/v2/table"
	"github.com/flyingnobita/llml/internal/llamacpp"
)

// tableColumns computes per-column widths from the inner body width (usable
// width inside app horizontal padding) and the current file list. Name expands
// to fit content (capped at maxNameColW); Path takes remaining space after fixed
// columns (Name, Runtime, Size, Last modified).
func tableColumns(totalWidth int, files []llamacpp.ModelFile, homeDir string) []btable.Column {
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
		if w := runewidth.StringWidth(llamacpp.FormatModelFolderDisplay(f.Path, homeDir)); w > longestPath {
			longestPath = w
		}
	}
	if longestName > nameW {
		nameW = longestName
		if nameW > maxNameColW {
			nameW = maxNameColW
		}
	}
	fixed := nameW + runtimeColW + sizeColW + modTimeColW + colPaddingExtra
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
		{Title: "Runtime", Width: runtimeColW},
		{Title: "Path", Width: pathW},
		{Title: "Size", Width: sizeColW},
		{Title: "Last modified", Width: modTimeColW},
	}
}

// tableContentMinWidth approximates the minimum row width so the outer
// viewport knows how wide to make the table. Each cell uses PaddingRight(1) in
// styles.table, so rendered width is sum(column widths) plus one column per cell.
func tableContentMinWidth(cols []btable.Column) int {
	sum := 0
	for _, c := range cols {
		sum += c.Width
	}
	return sum + len(cols)
}

// buildTableRows converts ModelFile entries into display rows using the
// column widths computed by tableColumns. Cells are truncated to fit.
func buildTableRows(files []llamacpp.ModelFile, cols []btable.Column, homeDir string) []btable.Row {
	if len(cols) < 5 {
		return nil
	}
	rows := make([]btable.Row, len(files))
	for i, f := range files {
		rows[i] = btable.Row{
			llamacpp.TruncateRunes(f.Name, cols[0].Width-1),
			llamacpp.TruncateRunes(llamacpp.FormatRuntimeLabel(f.Backend), cols[1].Width-1),
			llamacpp.TruncateRunes(llamacpp.FormatModelFolderDisplay(f.Path, homeDir), cols[2].Width-1),
			llamacpp.FormatSize(f.Size),
			llamacpp.FormatModTime(f.ModTime),
		}
	}
	return rows
}
