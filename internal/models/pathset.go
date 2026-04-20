package models

import (
	"path/filepath"
	"strings"
)

// PathSet is an ordered, deduplicated set of normalized filesystem paths.
// The zero value is not valid; use [NewPathSet].
type PathSet struct {
	seen map[string]struct{}
	out  []string
}

// NewPathSet returns an empty PathSet.
func NewPathSet() *PathSet {
	return &PathSet{seen: make(map[string]struct{})}
}

// Add normalizes p (expand tilde, clean, trim space) and appends it if not already present.
// Empty or "." paths are silently skipped.
func (ps *PathSet) Add(p string) {
	p = filepath.Clean(ExpandTildePath(strings.TrimSpace(p)))
	if p == "" || p == "." {
		return
	}
	if _, ok := ps.seen[p]; ok {
		return
	}
	ps.seen[p] = struct{}{}
	ps.out = append(ps.out, p)
}

// Slice returns the accumulated paths in insertion order.
func (ps *PathSet) Slice() []string { return ps.out }
