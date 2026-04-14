package tui

import (
	"sort"
	"strings"

	"github.com/flyingnobita/llml/internal/llamacpp"
)

// Table sort column indices (must match [tableColumns] order: Name, Runtime, Path, Size, Last modified).
const (
	tableSortColName = iota
	tableSortColRuntime
	tableSortColPath
	tableSortColSize
	tableSortColModTime
	tableSortColCount
)

// defaultSortCol matches discovery order ([llamacpp.Discover] sorts by path ascending).
const defaultSortCol = tableSortColPath

// sortModelFiles reorders files in place with a stable sort by column and direction.
func sortModelFiles(files []llamacpp.ModelFile, col int, desc bool) {
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

func clampSortCol(col int) int {
	if col < 0 || col >= tableSortColCount {
		return defaultSortCol
	}
	return col
}

func compareModelFilesCol(a, b llamacpp.ModelFile, col int) int {
	switch col {
	case tableSortColName:
		return strings.Compare(a.Name, b.Name)
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

// applyTableSort sorts [Model.files] according to [Model.sortCol] and [Model.sortDesc], rebuilds the
// table, and moves the cursor to the row whose path matched selPath when non-empty.
//
// The cursor must be restored BEFORE layoutTable because [launchPreviewLineCount]
// inside layoutTable reads [Model.SelectedModel] to size the table body. A stale
// cursor after reorder points at a different model whose preview command may wrap
// to a different number of lines, shifting the table height by 1 row.
func (m Model) applyTableSort(selPath string) Model {
	sortModelFiles(m.files, m.sortCol, m.sortDesc)
	if selPath != "" {
		for i := range m.files {
			if m.files[i].Path == selPath {
				m.tbl.SetCursor(i)
				break
			}
		}
	}
	m = m.layoutTable()
	return m
}
