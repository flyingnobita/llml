package tui

import (
	"github.com/mattn/go-runewidth"

	btable "charm.land/bubbles/v2/table"
	"github.com/flyingnobita/llml/internal/llamacpp"
)

// Unicode sort indicators (ascending / descending).
const (
	sortIndicatorAsc  = "▲"
	sortIndicatorDesc = "▼"
)

// formatSortColumnTitle returns the header label for one column, appending a sort
// triangle when colIdx is the active sort column. The result fits within maxW cells.
func formatSortColumnTitle(base string, colIdx, sortCol, maxW int, sortDesc bool) string {
	if maxW < 1 {
		return ""
	}
	if colIdx != sortCol {
		return llamacpp.TruncateRunes(base, maxW)
	}
	suffix := " " + sortIndicatorAsc
	if sortDesc {
		suffix = " " + sortIndicatorDesc
	}
	sw := runewidth.StringWidth(suffix)
	if sw >= maxW {
		return llamacpp.TruncateRunes(suffix, maxW)
	}
	baseMax := maxW - sw
	if baseMax < 2 {
		return llamacpp.TruncateRunes(suffix, maxW)
	}
	truncated := llamacpp.TruncateRunes(base, baseMax)
	return truncated + suffix
}

// tableColumns computes per-column widths from the inner body width (usable
// width inside app horizontal padding) and the current file list. File Name expands
// to fit content (capped at maxFileNameColW); ID expands (capped at maxIDColW); Path
// takes remaining space after fixed columns (File Name, Model ID, Runtime, Path, Size,
// Last modified). sortCol and sortDesc control the ▲/▼ indicator on the active
// column title.
func tableColumns(totalWidth int, files []llamacpp.ModelFile, homeDir string, sortCol int, sortDesc bool) []btable.Column {
	if totalWidth < minTerminalWidth {
		totalWidth = minTerminalWidth
	}
	nameW := defaultFileNameColW
	idW := defaultIDColW
	longestName := 0
	longestID := 0
	longestPath := 0
	for _, f := range files {
		if w := runewidth.StringWidth(f.Name); w > longestName {
			longestName = w
		}
		if w := runewidth.StringWidth(llamacpp.InferModelID(f.Path)); w > longestID {
			longestID = w
		}
		if w := runewidth.StringWidth(llamacpp.FormatModelFolderDisplay(f.Path, homeDir)); w > longestPath {
			longestPath = w
		}
	}
	if longestName > nameW {
		nameW = longestName
		if nameW > maxFileNameColW {
			nameW = maxFileNameColW
		}
	}
	if longestID > idW {
		idW = longestID
		if idW > maxIDColW {
			idW = maxIDColW
		}
	}
	fixed := nameW + idW + runtimeColW + sizeColW + modTimeColW + colPaddingExtra
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
		{Title: formatSortColumnTitle("File Name", tableSortColFileName, sortCol, nameW, sortDesc), Width: nameW},
		{Title: formatSortColumnTitle("Model ID", tableSortColID, sortCol, idW, sortDesc), Width: idW},
		{Title: formatSortColumnTitle("Runtime", tableSortColRuntime, sortCol, runtimeColW, sortDesc), Width: runtimeColW},
		{Title: formatSortColumnTitle("Path", tableSortColPath, sortCol, pathW, sortDesc), Width: pathW},
		{Title: formatSortColumnTitle("Size", tableSortColSize, sortCol, sizeColW, sortDesc), Width: sizeColW},
		{Title: formatSortColumnTitle("Last modified", tableSortColModTime, sortCol, modTimeColW, sortDesc), Width: modTimeColW},
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
	if len(cols) < 6 {
		return nil
	}
	rows := make([]btable.Row, len(files))
	for i, f := range files {
		rows[i] = btable.Row{
			llamacpp.TruncateRunes(f.Name, cols[0].Width-1),
			llamacpp.TruncateRunes(llamacpp.InferModelID(f.Path), cols[1].Width-1),
			llamacpp.TruncateRunes(llamacpp.FormatRuntimeLabel(f.Backend), cols[2].Width-1),
			llamacpp.TruncateRunes(llamacpp.FormatModelFolderDisplay(f.Path, homeDir), cols[3].Width-1),
			llamacpp.FormatSize(f.Size),
			llamacpp.FormatModTime(f.ModTime),
		}
	}
	return rows
}
