package tui

import (
	"sort"
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

// defaultSortCol matches discovery order ([models.Discover] sorts by path ascending).
const defaultSortCol tableSortCol = tableSortColPath

// sortModelFiles reorders files in place with a stable sort by column and direction.
func sortModelFiles(files []models.ModelFile, col tableSortCol, desc bool) {
	if len(files) < 2 {
		return
	}
	col = clampSortCol(col)
	sort.SliceStable(files, func(i, j int) bool {
		c := compareModelFilesCol(files[i], files[j], col)
		if c != 0 {
			if desc {
				return c > 0
			}
			return c < 0
		}
		return false
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
		return strings.Compare(models.InferModelID(a.Path), models.InferModelID(b.Path))
	case tableSortColRuntime:
		return int(a.Backend) - int(b.Backend)
	case tableSortColPath:
		return strings.Compare(a.Path, b.Path)
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
		return strings.Compare(a.Path, b.Path)
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
			if m.table.files[i].Path == selPath {
				m.table.tbl.SetCursor(i)
				break
			}
		}
	}
	m = m.layoutTable()
	return m
}
