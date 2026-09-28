package tui

import (
	"slices"
	"strings"

	"github.com/charmbracelet/x/ansi"
	"github.com/mattn/go-runewidth"

	btable "charm.land/bubbles/v2/table"

	"github.com/flyingnobita/llml/internal/models"
)

// Unicode sort indicators (ascending / descending).
const (
	sortIndicatorAsc  = "▲"
	sortIndicatorDesc = "▼"
)

// formatSortColumnTitle returns the header label for one column, appending a sort
// triangle when colIdx is the active sort column. The result fits within maxW cells.
func formatSortColumnTitle(base string, colIdx, sortCol tableSortCol, maxW int, sortDesc bool) string {
	if maxW < 1 {
		return ""
	}
	if colIdx != sortCol {
		return TruncateRunes(base, maxW)
	}
	suffix := " " + sortIndicatorAsc
	if sortDesc {
		suffix = " " + sortIndicatorDesc
	}
	sw := runewidth.StringWidth(suffix)
	if sw >= maxW {
		return TruncateRunes(suffix, maxW)
	}
	baseMax := maxW - sw
	if baseMax < 2 {
		return TruncateRunes(suffix, maxW)
	}
	truncated := TruncateRunes(base, baseMax)
	return truncated + suffix
}

// tableColumns computes per-column widths from the inner body width (usable
// width inside app horizontal padding) and the current file list. File Name expands
// to fit content (capped at maxFileNameColW); ID expands (capped at maxIDColW); Path
// takes remaining space after fixed columns (Model ID, Runtime, Size, Path, File Name,
// Last modified). sortCol and sortDesc control the ▲/▼ indicator on the active
// column title. runtimeW is the Runtime column's width (see
// [Model.runtimeColumnWidth]).
func tableColumns(totalWidth int, files []models.ModelFile, homeDir string, sortCol tableSortCol, sortDesc bool, runtimeW int) []btable.Column {
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
		if w := runewidth.StringWidth(modelIDForRow(f)); w > longestID {
			longestID = w
		}
		if w := runewidth.StringWidth(formatLocationForRow(f, homeDir)); w > longestPath {
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
	fixed := nameW + idW + runtimeW + sizeColW + modTimeColW + colPaddingExtra
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
		{Title: formatSortColumnTitle("Model ID", tableSortColID, sortCol, idW, sortDesc), Width: idW},
		{Title: formatSortColumnTitle("Runtime", tableSortColRuntime, sortCol, runtimeW, sortDesc), Width: runtimeW},
		{Title: formatSortColumnTitle("Size", tableSortColSize, sortCol, sizeColW, sortDesc), Width: sizeColW},
		{Title: formatSortColumnTitle("Path", tableSortColPath, sortCol, pathW, sortDesc), Width: pathW},
		{Title: formatSortColumnTitle("File Name", tableSortColFileName, sortCol, nameW, sortDesc), Width: nameW},
		{Title: formatSortColumnTitle("Last Modified", tableSortColModTime, sortCol, modTimeColW, sortDesc), Width: modTimeColW},
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

// runtimeColumnWidth is the Runtime column's width: wide enough for the (off)
// marker when any row is dimmed, and unchanged otherwise.
func (m Model) runtimeColumnWidth() int {
	if m.anyRowDimmed() {
		return runtimeColOffW
	}
	return runtimeColW
}

// tableColumnsAt computes the table's columns for inner body width innerW.
func (m Model) tableColumnsAt(innerW int) []btable.Column {
	return tableColumns(innerW, m.table.files, m.layout.homeDir, m.table.sortCol, m.table.sortDesc, m.runtimeColumnWidth())
}

// tableRows converts the model rows into display rows using the column widths
// computed by tableColumns. Cells are truncated to fit. A row on a Disabled
// Runtime says (off) after its Runtime and is drawn in the muted style.
func (m Model) tableRows(cols []btable.Column) []btable.Row {
	if len(cols) < 6 {
		return nil
	}
	rows := make([]btable.Row, len(m.table.files))
	for i, f := range m.table.files {
		be := m.rowRuntime(f)
		dimmed := !m.runtimeEnabled(be)
		label := models.FormatRuntimeLabel(be)
		if dimmed {
			label += runtimeOffSuffix
		}
		row := btable.Row{
			TruncateRunes(modelIDForRow(f), cols[0].Width-1),
			TruncateRunes(label, cols[1].Width-1),
			models.FormatSize(f.Size),
			TruncateRunes(formatLocationForRow(f, m.layout.homeDir), cols[3].Width-1),
			TruncateRunes(f.Name, cols[4].Width-1),
			FormatModTime(f.ModTime),
		}
		if dimmed {
			for j, cell := range row {
				row[j] = m.ui.styles.bodyDim.Render(cell)
			}
		}
		rows[i] = row
	}
	return rows
}

// tableView renders the model table. A dimmed row under the cursor is drawn
// without its muted styling: each muted cell ends in a style reset, which
// would cut the selection background short. The row still says (off).
func (m Model) tableView() string {
	c := m.table.tbl.Cursor()
	rows := m.table.tbl.Rows()
	if c < 0 || c >= len(rows) || !rowHasStyling(rows[c]) {
		return m.table.tbl.View()
	}
	rows = slices.Clone(rows)
	plain := make(btable.Row, len(rows[c]))
	for i, cell := range rows[c] {
		plain[i] = ansi.Strip(cell)
	}
	rows[c] = plain
	// tbl is a copy, so the stored rows keep their styling for when the
	// cursor moves on.
	tbl := m.table.tbl
	tbl.SetRows(rows)
	return tbl.View()
}

func rowHasStyling(r btable.Row) bool {
	for _, cell := range r {
		if strings.Contains(cell, "\x1b") {
			return true
		}
	}
	return false
}
