package tui

import (
	"strings"
	"testing"

	"github.com/mattn/go-runewidth"
)

func TestFormatSortColumnTitle_inactiveColumn(t *testing.T) {
	got := formatSortColumnTitle("Name", tableSortColName, tableSortColPath, 20, false)
	if got != "Name" {
		t.Fatalf("inactive: %q", got)
	}
}

func TestFormatSortColumnTitle_activeAscDesc(t *testing.T) {
	gotAsc := formatSortColumnTitle("Name", tableSortColName, tableSortColName, 20, false)
	if !strings.Contains(gotAsc, sortIndicatorAsc) || strings.Contains(gotAsc, sortIndicatorDesc) {
		t.Fatalf("asc: %q", gotAsc)
	}
	gotDesc := formatSortColumnTitle("Name", tableSortColName, tableSortColName, 20, true)
	if !strings.Contains(gotDesc, sortIndicatorDesc) {
		t.Fatalf("desc: %q", gotDesc)
	}
}

func TestFormatSortColumnTitle_fitsWidth(t *testing.T) {
	const w = 12
	s := formatSortColumnTitle("Last modified", tableSortColModTime, tableSortColModTime, w, false)
	if runewidth.StringWidth(s) > w {
		t.Fatalf("width %d > %d: %q", runewidth.StringWidth(s), w, s)
	}
}
