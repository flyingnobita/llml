package tui

import "github.com/flyingnobita/llml/internal/models"

// newTestModel returns a Model suitable for unit tests: fixed dimensions, not loading, empty table.
func newTestModel() Model {
	m := New()
	m.layout.width = 120
	m.layout.height = 40
	m.loading = false
	m.table.files = []models.ModelFile{}
	return m
}
