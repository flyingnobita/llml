package tui

import "github.com/flyingnobita/llml/internal/llamacpp"

// newTestModel returns a Model suitable for unit tests: fixed dimensions, not loading, empty table.
func newTestModel() Model {
	m := New()
	m.width = 120
	m.height = 40
	m.loading = false
	m.files = []llamacpp.ModelFile{}
	return m
}
