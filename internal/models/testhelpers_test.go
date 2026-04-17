package models

import (
	"os"
	"path/filepath"
	"strings"
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

// hfHubRepoDir returns the path to a models--* cache directory for repoID (org/model).
func hfHubRepoDir(t *testing.T, home, repoID string) string {
	t.Helper()
	enc := "models--" + strings.ReplaceAll(repoID, "/", "--")
	d := filepath.Join(home, ".cache", "huggingface", "hub", enc)
	if err := os.MkdirAll(d, 0o755); err != nil {
		t.Fatal(err)
	}
	return d
}
