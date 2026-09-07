package fsutil

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
)

var (
	cachedHomeDir     string
	cachedHomeDirOnce sync.Once
)

// HomeDir returns the current user's home directory, cached after the first call.
// Returns "" when the home directory cannot be determined.
func HomeDir() string {
	cachedHomeDirOnce.Do(func() {
		d, err := os.UserHomeDir()
		if err != nil {
			return
		}
		cachedHomeDir = d
	})
	return cachedHomeDir
}

// ExpandTildePath trims s and, if it is "~" or begins with "~/", replaces that prefix with the
// current user's home directory from [os.UserHomeDir]. If the home directory cannot be resolved,
// the trimmed input is returned unchanged. Other paths are returned trimmed only.
func ExpandTildePath(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	if s == "~" || strings.HasPrefix(s, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return s
		}
		if s == "~" {
			return home
		}
		return filepath.Join(home, strings.TrimPrefix(s, "~/"))
	}
	return s
}

// NormalizePath trims, expands a leading tilde, and cleans a filesystem path.
// It returns "" for empty, whitespace-only, or degenerate ("." after cleaning) input,
// so callers can treat "" as "unset".
func NormalizePath(v string) string {
	if v = strings.TrimSpace(v); v == "" {
		return ""
	}
	c := filepath.Clean(ExpandTildePath(v))
	if c == "." {
		return ""
	}
	return c
}

// PathSet is an ordered, deduplicated set of normalized filesystem paths.
// The zero value is ready to use.
type PathSet struct {
	seen map[string]struct{}
	out  []string
}

// NewPathSet returns an empty PathSet.
func NewPathSet() *PathSet { return &PathSet{} }

// Add normalizes each path with [NormalizePath] and appends it if not already
// present. Empty and degenerate paths are silently skipped.
func (ps *PathSet) Add(paths ...string) {
	for _, raw := range paths {
		p := NormalizePath(raw)
		if p == "" {
			continue
		}
		if ps.seen == nil {
			ps.seen = make(map[string]struct{})
		}
		if _, dup := ps.seen[p]; dup {
			continue
		}
		ps.seen[p] = struct{}{}
		ps.out = append(ps.out, p)
	}
}

// Slice returns the accumulated paths in insertion order.
func (ps *PathSet) Slice() []string { return ps.out }
