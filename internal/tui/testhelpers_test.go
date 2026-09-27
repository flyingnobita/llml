package tui

import "github.com/flyingnobita/llml/internal/models"

// newTestModel returns a Model suitable for unit tests: fixed dimensions, not
// loading, empty table, and fake services so nothing reaches the user's files.
func newTestModel() Model {
	m := NewWithServices(testServices())
	m.layout.width = 120
	m.layout.height = 40
	m.loading = false
	m.table.files = []models.ModelFile{}
	return m
}
