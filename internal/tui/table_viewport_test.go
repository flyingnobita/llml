package tui

import (
	"testing"
	"time"

	"charm.land/lipgloss/v2"
	"github.com/flyingnobita/llml/internal/models"
)

func TestNew_tableHscrollMouseWheelDisabled(t *testing.T) {
	m := New()
	if m.table.hscroll.MouseWheelEnabled {
		t.Fatal("table hscroll must disable mouse wheel so vertical wheel does not desync outer wrapper from table cursor")
	}
}

func TestLayoutTable_resetsHscrollYOffset(t *testing.T) {
	m := newTestModel()
	file := models.ModelFile{Backend: models.BackendLlama, Path: "/a.gguf", Name: "a", Size: 1, ModTime: time.Unix(0, 0)}
	m.table.files = []models.ModelFile{file}
	m = m.layoutTable()
	m.table.hscroll.SetYOffset(42)
	m = m.layoutTable()
	if m.table.hscroll.YOffset() != 0 {
		t.Fatalf("layoutTable must reset hscroll YOffset, got %d", m.table.hscroll.YOffset())
	}
}

func TestModelTablePaneView_joinsScrollColumnWhenVerticallyScrollable(t *testing.T) {
	m := newTestModel()
	m.layout.width = 120
	m.layout.height = 50
	var files []models.ModelFile
	for i := range 30 {
		files = append(files, models.ModelFile{
			Backend: models.BackendLlama,
			Path:    "/m.gguf",
			Name:    string(rune('a' + i%26)),
			Size:    1,
			ModTime: time.Unix(0, 0),
		})
	}
	m.table.files = files
	m = m.layoutTable()
	if !m.tablePaneShowsVerticalIndicator() {
		t.Fatal("expected vertical indicator for scrollable table content")
	}
	vp := m.table.hscroll.View()
	if lipgloss.Height(vp) < 2 {
		t.Fatalf("need viewport height >= 2 for scroll track, got %d", lipgloss.Height(vp))
	}
	pane := m.modelTablePaneView()
	if pane == vp {
		t.Fatal("expected modelTablePaneView to join a scroll column when vertically scrollable")
	}
	wv, wp := lipgloss.Width(vp), lipgloss.Width(pane)
	if wp <= wv {
		t.Fatalf("joined pane should be wider than viewport alone (vp=%d pane=%d)", wv, wp)
	}
}

func TestModelTablePaneView_plainViewportWhenNoVerticalOverflow(t *testing.T) {
	m := newTestModel()
	m.table.files = nil
	m = m.layoutTable()
	if m.tablePaneShowsVerticalIndicator() {
		t.Fatal("empty file list should not show vertical scroll indicator")
	}
	if got := m.modelTablePaneView(); got != m.table.hscroll.View() {
		t.Fatal("without vertical overflow, modelTablePaneView must match hscroll view")
	}
}
