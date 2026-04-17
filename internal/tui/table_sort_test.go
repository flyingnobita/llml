package tui

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/flyingnobita/llml/internal/models"
)

func TestSortModelFiles_nameAscDesc(t *testing.T) {
	files := []models.ModelFile{
		{Name: "zebra", Path: "/z"},
		{Name: "alpha", Path: "/a"},
	}
	sortModelFiles(files, tableSortColFileName, false)
	if files[0].Name != "alpha" || files[1].Name != "zebra" {
		t.Fatalf("name asc: got %#v", files)
	}
	sortModelFiles(files, tableSortColFileName, true)
	if files[0].Name != "zebra" || files[1].Name != "alpha" {
		t.Fatalf("name desc: got %#v", files)
	}
}

func TestSortModelFiles_idAsc(t *testing.T) {
	p1 := filepath.Join("hub", "models--z--z-model", "snapshots", "x")
	p2 := filepath.Join("hub", "models--a--b", "snapshots", "y")
	files := []models.ModelFile{
		{Path: p1, Name: "x"},
		{Path: p2, Name: "y"},
	}
	sortModelFiles(files, tableSortColID, false)
	if files[0].Path != p2 || files[1].Path != p1 {
		t.Fatalf("id asc: got %#v", files)
	}
}

func TestSortModelFiles_pathAsc(t *testing.T) {
	files := []models.ModelFile{
		{Path: "/b", Name: "b"},
		{Path: "/a", Name: "a"},
	}
	sortModelFiles(files, tableSortColPath, false)
	if files[0].Path != "/a" || files[1].Path != "/b" {
		t.Fatalf("path asc: got %#v", files)
	}
}

func TestSortModelFiles_runtime(t *testing.T) {
	files := []models.ModelFile{
		{Backend: models.BackendVLLM, Path: "/v"},
		{Backend: models.BackendLlama, Path: "/l"},
	}
	sortModelFiles(files, tableSortColRuntime, false)
	if files[0].Backend != models.BackendLlama || files[1].Backend != models.BackendVLLM {
		t.Fatalf("runtime asc: got %#v", files)
	}
	sortModelFiles(files, tableSortColRuntime, true)
	if files[0].Backend != models.BackendVLLM || files[1].Backend != models.BackendLlama {
		t.Fatalf("runtime desc: got %#v", files)
	}
}

func TestSortModelFiles_size(t *testing.T) {
	files := []models.ModelFile{
		{Size: 200, Path: "/big"},
		{Size: 10, Path: "/small"},
	}
	sortModelFiles(files, tableSortColSize, false)
	if files[0].Size != 10 || files[1].Size != 200 {
		t.Fatalf("size asc: got %#v", files)
	}
}

func TestSortModelFiles_modTime(t *testing.T) {
	tOld := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	tNew := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	files := []models.ModelFile{
		{ModTime: tNew, Path: "/n"},
		{ModTime: tOld, Path: "/o"},
	}
	sortModelFiles(files, tableSortColModTime, false)
	if !files[0].ModTime.Equal(tOld) || !files[1].ModTime.Equal(tNew) {
		t.Fatalf("mtime asc: got %#v", files)
	}
}

func TestSortModelFiles_stableEqualKeys(t *testing.T) {
	files := []models.ModelFile{
		{Name: "same", Path: "/first"},
		{Name: "same", Path: "/second"},
	}
	sortModelFiles(files, tableSortColFileName, false)
	if files[0].Path != "/first" || files[1].Path != "/second" {
		t.Fatalf("stable tie-break: got %#v", files)
	}
}

func TestSortModelFiles_emptyOrSingle(t *testing.T) {
	var empty []models.ModelFile
	sortModelFiles(empty, tableSortColFileName, false)
	if len(empty) != 0 {
		t.Fatal("empty slice mutated")
	}
	one := []models.ModelFile{{Name: "only", Path: "/o"}}
	sortModelFiles(one, tableSortColFileName, true)
	if len(one) != 1 || one[0].Path != "/o" {
		t.Fatalf("single: got %#v", one)
	}
}

func TestCompareModelFilesCol_invalidColFallsBackToPath(t *testing.T) {
	a := models.ModelFile{Path: "/a", Name: "x"}
	b := models.ModelFile{Path: "/b", Name: "x"}
	if compareModelFilesCol(a, b, 99) >= 0 {
		t.Fatal("expected /a < /b for fallback path compare")
	}
}
