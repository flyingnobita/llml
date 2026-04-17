package tui

import (
	"strings"
	"testing"

	"github.com/flyingnobita/llml/internal/models"
	"github.com/mattn/go-runewidth"
	"time"
)

func TestFormatSortColumnTitle_inactiveColumn(t *testing.T) {
	got := formatSortColumnTitle("File Name", tableSortColFileName, tableSortColPath, 20, false)
	if got != "File Name" {
		t.Fatalf("inactive: %q", got)
	}
}

func TestFormatSortColumnTitle_activeAscDesc(t *testing.T) {
	gotAsc := formatSortColumnTitle("File Name", tableSortColFileName, tableSortColFileName, 20, false)
	if !strings.Contains(gotAsc, sortIndicatorAsc) || strings.Contains(gotAsc, sortIndicatorDesc) {
		t.Fatalf("asc: %q", gotAsc)
	}
	gotDesc := formatSortColumnTitle("File Name", tableSortColFileName, tableSortColFileName, 20, true)
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

// TestLayoutTableConvergesInOnePass verifies that layoutTable is idempotent — calling it
// twice produces identical dimensions. This confirms the log h-bar determination uses exact
// frame sizes (no guess-and-redo second pass).
func TestLayoutTableConvergesInOnePass(t *testing.T) {
	file := models.ModelFile{Backend: models.BackendLlama, Path: "/m.gguf", Name: "m", Size: 1, ModTime: time.Unix(0, 0)}
	cases := []struct {
		name          string
		width, height int
		logLine       string
	}{
		{"no server", 120, 40, ""},
		{"short log line", 120, 40, "short line"},
		{"wide log line triggers hbar", 120, 40, strings.Repeat("x", 150)},
		{"narrow terminal wide log", 80, 24, strings.Repeat("x", 100)},
		{"wide terminal", 200, 50, strings.Repeat("x", 80)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m := New()
			m.layout.width = tc.width
			m.layout.height = tc.height
			m.table.files = []models.ModelFile{file}
			if tc.logLine != "" {
				m.server.running = true
				m = m.appendServerLogLine(tc.logLine)
			}
			m1 := m.layoutTable()
			m2 := m1.layoutTable()
			if m1.layout.tableBodyH != m2.layout.tableBodyH {
				t.Errorf("tableBodyH not idempotent: first=%d second=%d", m1.layout.tableBodyH, m2.layout.tableBodyH)
			}
			if m1.server.viewportH != m2.server.viewportH {
				t.Errorf("viewportH not idempotent: first=%d second=%d", m1.server.viewportH, m2.server.viewportH)
			}
		})
	}
}
