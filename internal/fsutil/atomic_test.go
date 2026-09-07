package fsutil

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestWriteFileAtomic(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "out.json")
	data := []byte(`{"ok":true}`)

	if err := WriteFileAtomic(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != string(data) {
		t.Fatalf("got %q", b)
	}
	if runtime.GOOS != "windows" {
		st, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		if st.Mode()&0o777 != 0o644 {
			t.Fatalf("mode: %v", st.Mode())
		}
	}
}

// A write into a directory that does not exist must fail cleanly, not panic,
// and must leave nothing behind.
func TestWriteFileAtomic_missingDirectory(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "nope", "out.json")
	if err := WriteFileAtomic(path, []byte("x"), 0o644); err == nil {
		t.Fatal("expected an error writing into a missing directory")
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("nothing should have been created, got %v", err)
	}
}

// A failed write must not leave a temp file next to the target, since those
// would accumulate in the user's config directory.
func TestWriteFileAtomic_leavesNoTempOnSuccess(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "out.json")
	if err := WriteFileAtomic(path, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Name() != "out.json" {
		names := make([]string, len(entries))
		for i, e := range entries {
			names[i] = e.Name()
		}
		t.Errorf("expected only the target file, got %v", names)
	}
}

// Overwriting must replace the contents rather than append or merge, and must
// keep the requested permissions.
func TestWriteFileAtomic_overwriteReplacesContentAndPerm(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "out.json")
	if err := WriteFileAtomic(path, []byte("a longer first version"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := WriteFileAtomic(path, []byte("short"), 0o644); err != nil {
		t.Fatal(err)
	}

	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != "short" {
		t.Errorf("content = %q, want %q", b, "short")
	}
	if runtime.GOOS != "windows" {
		st, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		if got := st.Mode().Perm(); got != 0o644 {
			t.Errorf("perm = %v, want 0644", got)
		}
	}
}

func TestHomeDir(t *testing.T) {
	// Not parallel: HomeDir consults the environment, and caches the result.
	got := HomeDir()
	want, err := os.UserHomeDir()
	if err != nil {
		t.Skipf("no home directory available: %v", err)
	}
	if got != want {
		t.Errorf("HomeDir() = %q, want %q", got, want)
	}
	// The value is cached, so a second call must agree.
	if second := HomeDir(); second != got {
		t.Errorf("HomeDir() is not stable: %q then %q", got, second)
	}
}

func TestNormalizePath(t *testing.T) {
	t.Parallel()

	cases := map[string]string{
		"  /opt/llama/bin  ": "/opt/llama/bin",
		"/opt/llama/bin/..":  "/opt/llama",
		"":                   "",
		"   ":                "",
		".":                  "",
		"./":                 "",
	}
	for in, want := range cases {
		if got := NormalizePath(in); got != want {
			t.Errorf("NormalizePath(%q) = %q, want %q", in, got, want)
		}
	}
}
