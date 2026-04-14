package btable

import "testing"

func TestSetRows_emptyThenDataSelectsFirstRow(t *testing.T) {
	m := New(WithColumns([]Column{{Title: "c", Width: 10}}))
	m.SetRows(nil)
	if m.Cursor() != -1 {
		t.Fatalf("empty table: expected cursor -1, got %d", m.Cursor())
	}
	m.SetRows([]Row{[]string{"a"}, []string{"b"}})
	if m.Cursor() != 0 {
		t.Fatalf("after rows load: expected cursor 0 (first row), got %d", m.Cursor())
	}
}
