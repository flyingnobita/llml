package models

import (
	"os"
	"path/filepath"
	"testing"
)

// makeFakeExecutable writes an empty executable file at dir/name and returns its path.
func makeFakeExecutable(t *testing.T, dir, name string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte{}, 0o755); err != nil {
		t.Fatal(err)
	}
	return p
}
