package tui

import (
	"slices"
	"strings"

	"github.com/flyingnobita/llml/internal/models"
)

type tableSortCol int

// Table sort column indices (must match [tableColumns] order: Model ID, Runtime, Size, Path, File Name, Last modified).
const (
	tableSortColID tableSortCol = iota
	tableSortColRuntime
	tableSortColSize
	tableSortColPath
	tableSortColFileName
	tableSortColModTime
	tableSortColCount
)

// defaultSortCol is the initial table sort shown at startup.
const defaultSortCol tableSortCol = tableSortColRuntime

// sortModelFiles reorders files in place with a stable sort by column and direction.
func sortModelFiles(files []models.ModelFile, col tableSortCol, desc bool) {
	if len(files) < 2 {
		return
	}
	col = clampSortCol(col)
	slices.SortStableFunc(files, func(a, b models.ModelFile) int {
		c := compareModelFilesCol(a, b, col)
		if desc {
			return -c
		}
		return c
	})
}

func clampSortCol(col tableSortCol) tableSortCol {
	if col < 0 || col >= tableSortColCount {
		return defaultSortCol
	}
	return col
}

func compareModelFilesCol(a, b models.ModelFile, col tableSortCol) int {
	switch col {
	case tableSortColFileName:
		return strings.Compare(a.Name, b.Name)
	case tableSortColID:
		return strings.Compare(modelIDForRow(a), modelIDForRow(b))
	case tableSortColRuntime:
		return int(a.Backend) - int(b.Backend)
	case tableSortColPath:
		return strings.Compare(a.DisplayLocation(), b.DisplayLocation())
	case tableSortColSize:
		if a.Size < b.Size {
			return -1
		}
		if a.Size > b.Size {
			return 1
		}
		return 0
	case tableSortColModTime:
		if a.ModTime.Before(b.ModTime) {
			return -1
		}
		if a.ModTime.After(b.ModTime) {
			return 1
		}
		return 0
	default:
		return strings.Compare(a.DisplayLocation(), b.DisplayLocation())
	}
}

// applyTableSort sorts [Model.table.files] according to [Model.table.sortCol] and [Model.table.sortDesc], rebuilds the
// table, and moves the cursor to the row whose path matched selPath when non-empty.
//
// The cursor must be restored BEFORE layoutTable because [Model.launchPreviewPaneLayoutHeight]
// inside layoutTable reads [Model.SelectedModel] to size the table body. A stale
// cursor after reorder points at a different model whose preview command may wrap
// to a different number of lines, shifting the table height by 1 row.
func (m Model) applyTableSort(selPath string) Model {
	sortModelFiles(m.table.files, m.table.sortCol, m.table.sortDesc)
	if selPath != "" {
		for i := range m.table.files {
			if m.table.files[i].Identity() == selPath {
				m.table.tbl.SetCursor(i)
				break
			}
		}
	}
	m = m.layoutTable()
	return m
}
