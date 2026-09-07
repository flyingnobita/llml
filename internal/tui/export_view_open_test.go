package tui

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/flyingnobita/llml/internal/profiles"
)

// Export panel tests: opening the panel, scrolling, and running the export.
// Split out of export_view_test.go, which had grown past 1900 lines.

func TestOpenExportView_PopulatesItems(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	llmlDir := testLlmlDir(t, dir)
	paramsPath := filepath.Join(llmlDir, "model-params.json")

	data := []byte(`{
	  "version": 3,
	  "models": {
	    "/models/Alpha.gguf": {
	      "profiles": [
	        {"name": "default", "backend": "llama", "env": [], "args": ["--ctx-size", "4096"]},
	        {"name": "kobold", "backend": "koboldcpp", "env": [], "args": ["--usecublas"]}
	      ],
	      "activeIndex": 0
	    },
	    "/models/Beta.gguf": {
	      "profiles": [
	        {"name": "default", "backend": "vllm", "env": [], "args": ["--n-gpu-layers", "80"]}
	      ],
	      "activeIndex": 0
	    }
	  }
	}`)
	if err := os.WriteFile(paramsPath, data, 0o644); err != nil {
		t.Fatal(err)
	}

	m := New()
	m = m.openExportView()

	if !m.export.open {
		t.Fatal("export should be open")
	}
	if m.export.focus != exportFocusList {
		t.Errorf("focus = %d, want exportFocusList (%d)", m.export.focus, exportFocusList)
	}

	// Should have: Alpha header + 2 profiles + Beta header + 1 profile = 5 items
	if len(m.export.items) != 5 {
		t.Fatalf("len(items) = %d, want 5", len(m.export.items))
	}

	// First item should be Alpha header.
	if m.export.items[0].kind != exportItemHeader {
		t.Error("items[0] should be header")
	}
	if m.export.items[0].modelDisplay != "Alpha" {
		t.Errorf("items[0].modelDisplay = %q, want Alpha", m.export.items[0].modelDisplay)
	}

	// Second item: first profile of Alpha (llama comes before koboldcpp alphabetically).
	if m.export.items[1].kind != exportItemProfile {
		t.Error("items[1] should be profile")
	}
	if m.export.items[1].backend != "koboldcpp" {
		t.Errorf("items[1].backend = %q, want koboldcpp (sorted)", m.export.items[1].backend)
	}
	if m.export.items[1].profileName != "kobold" {
		t.Errorf("items[1].profileName = %q, want kobold", m.export.items[1].profileName)
	}

	// Third: second profile of Alpha (llama).
	if m.export.items[2].kind != exportItemProfile {
		t.Error("items[2] should be profile")
	}
	if m.export.items[2].backend != "llama" {
		t.Errorf("items[2].backend = %q, want llama", m.export.items[2].backend)
	}

	// Fourth item: Beta header.
	if m.export.items[3].kind != exportItemHeader {
		t.Error("items[3] should be header")
	}
	if m.export.items[3].modelDisplay != "Beta" {
		t.Errorf("items[3].modelDisplay = %q, want Beta", m.export.items[3].modelDisplay)
	}

	// Fifth item: Beta's profile.
	if m.export.items[4].kind != exportItemProfile {
		t.Error("items[4] should be profile")
	}
	if m.export.items[4].backend != "vllm" {
		t.Errorf("items[4].backend = %q, want vllm", m.export.items[4].backend)
	}

	// Cursor should start on first profile (skip header).
	if m.export.cursor != 1 {
		t.Errorf("cursor = %d, want 1 (first profile after header)", m.export.cursor)
	}
}

func TestOpenExportView_NoFileOpensEmpty(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(dir, ".config"))

	m := New()
	m = m.openExportView()

	// With no model-params.json the modal still opens so the user gets feedback.
	if !m.export.open {
		t.Error("export should open even when no profiles exist")
	}
	if len(m.export.items) != 0 {
		t.Errorf("expected 0 items, got %d", len(m.export.items))
	}
}

// --- ScrollbarGlyph tests ---

func TestScrollbarGlyph_NoScrollWhenFits(t *testing.T) {
	got := scrollbarGlyph(0, 10, 5, 0)
	if got != "" {
		t.Errorf("when totalItems <= maxVis, should be empty, got %q", got)
	}
}

func TestScrollbarGlyph_TopArrow(t *testing.T) {
	got := scrollbarGlyph(0, 10, 20, 2)
	if got != "▴" {
		t.Errorf("top arrow expected at row 0 with offset > 0, got %q", got)
	}
}

func TestScrollbarGlyph_BottomArrow(t *testing.T) {
	// scrollOffset=11, maxVis=10, totalItems=25: items 11-20 visible, items 21-24 still below.
	got := scrollbarGlyph(9, 10, 25, 11)
	if got != "▾" {
		t.Errorf("bottom arrow expected at last visible row when more items below, got %q", got)
	}
}

func TestScrollbarGlyph_Thumb(t *testing.T) {
	// When totalItems > maxVis, and row is within thumb range.
	got := scrollbarGlyph(0, 10, 200, 0)
	// At offset 0, thumb starts at 0, first row should be thumb.
	if got != "█" {
		t.Errorf("thumb expected at row 0, offset 0 with 200 items, got %q", got)
	}
}

func TestScrollbarGlyph_Track(t *testing.T) {
	got := scrollbarGlyph(5, 10, 200, 0)
	// At offset 0 with totalItems=200, thumb is small. Row 5 could be track.
	// thumbSize = max(1, 10*10/200) = max(1, 0) = 1
	// thumbStart = 0*10/200 = 0
	// So thumb is only at row 0, row 5 is track.
	if got != "┃" {
		t.Errorf("track expected at row 5, offset 0 with 200 items, got %q", got)
	}
}

func TestScrollbarGlyph_Padding(t *testing.T) {
	got := scrollbarGlyph(10, 10, 20, 10)
	// Row 10 with maxVis=10, totalItems=20, offset=10
	// actualIdx = 10+10 = 20, which >= totalItems (20), so padding.
	if got != " " {
		t.Errorf("padding expected for out-of-range row, got %q", got)
	}
}

// --- AdjustExportScroll tests ---

func TestAdjustExportScroll_CursorAboveScrollOffset(t *testing.T) {
	m := New()
	m.export.open = true
	m.export.cursor = 0
	m.export.scrollOffset = 5

	m = m.adjustExportScroll()
	if m.export.scrollOffset != 0 {
		t.Errorf("scrollOffset = %d, want 0 (cursor above offset)", m.export.scrollOffset)
	}
}

func TestAdjustExportScroll_CursorBelowScrollWindow(t *testing.T) {
	m := New()
	m.layout.height = 40
	m.export.open = true
	m.export.cursor = 20
	m.export.scrollOffset = 0

	m = m.adjustExportScroll()
	// maxVis = max(40-13, 3) = 27
	// cursor (20) >= scrollOffset(0) + maxVis(27)? No. So offset stays 0.
	// Let me make cursor far enough.
	m.export.cursor = 30
	m.export.scrollOffset = 0
	m = m.adjustExportScroll()
	// maxVis = 27, cursor(30) >= 0+27 = 27, so offset = 30-27+1 = 4
	if m.export.scrollOffset != 4 {
		t.Errorf("scrollOffset = %d, want 4", m.export.scrollOffset)
	}
}

// --- doExportAttempt tests ---

func TestDoExportAttempt_SuccessNewFile(t *testing.T) {
	dir := t.TempDir()
	dest := filepath.Join(dir, "export.toml")

	m := New()
	m.export.open = true
	m.export.outputPath = dest
	m.export.items = []exportProfileItem{
		{kind: exportItemProfile, modelDisplay: "A", backend: "llama", profileName: "p1", checked: true,
			pp: profiles.PortableProfile{Name: "p1", Backend: "llama", ModelHint: "A"}},
	}
	m.export.cursor = 0

	tm, _ := m.doExportAttempt()
	m2 := asModel(tm)

	if m2.export.open {
		t.Error("export should close after successful write")
	}
	if _, err := os.Stat(dest); os.IsNotExist(err) {
		t.Error("file should exist after export")
	}
}

func TestDoExportAttempt_CollisionOpensSubModal(t *testing.T) {
	dir := t.TempDir()
	dest := filepath.Join(dir, "export.toml")

	// Create the file first so collision triggers.
	if err := os.WriteFile(dest, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	m := New()
	m.export.open = true
	m.export.outputPath = dest
	m.export.items = []exportProfileItem{
		{kind: exportItemProfile, modelDisplay: "A", backend: "llama", profileName: "p1", checked: true,
			pp: profiles.PortableProfile{Name: "p1", Backend: "llama", ModelHint: "A"}},
	}
	m.export.cursor = 0

	tm, _ := m.doExportAttempt()
	m2 := asModel(tm)

	if !m2.collision.open {
		t.Fatal("collision sub-modal should open when file exists")
	}
	if m2.collision.dest != dest {
		t.Errorf("collision.dest = %q, want %q", m2.collision.dest, dest)
	}
	if m2.collision.suffixPath == "" {
		t.Error("collision.suffixPath should not be empty")
	}
}

// --- Export key handler tests ---
