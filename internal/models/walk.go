package models

import (
	"context"
	"os"
	"path/filepath"
	"strings"
)

// walkSearchTree walks root recursively with the same depth and skip-dir rules as GGUF
// discovery. For each file (non-directory) within maxDepth, onFile is called with the
// full path, parent directory, directory entry, and depth (path segments below root).
//
// Unlike [filepath.WalkDir], symbolic links to directories are followed, because the
// Hugging Face hub layout stores snapshots as links into blobs. Following links means
// the tree can contain cycles, so each directory is visited at most once, keyed by its
// canonical path; the depth cap alone would let a self-referencing link rescan the same
// subtree up to maxDepth times.
//
// The walk checks ctx before each directory and returns ctx.Err() when it is
// done, so an abandoned scan does not keep reading the filesystem.
func walkSearchTree(ctx context.Context, root string, maxDepth int, onFile func(fullPath, parentDir string, ent os.DirEntry, depth int) error) error {
	w := &treeWalker{
		ctx:      ctx,
		root:     root,
		maxDepth: maxDepth,
		onFile:   onFile,
		visited:  make(map[string]struct{}),
	}
	w.markVisited(root)
	return w.walk(root)
}

// treeWalker carries the walk's fixed parameters and the visited set so the
// recursive step does not have to thread them through every call.
type treeWalker struct {
	ctx      context.Context
	root     string
	maxDepth int
	onFile   func(fullPath, parentDir string, ent os.DirEntry, depth int) error

	// visited holds the canonical path of every directory already walked.
	visited map[string]struct{}
}

// canonical resolves dir through any symlinks so two paths that reach the same
// directory compare equal. It falls back to the cleaned path when the directory
// cannot be resolved, which is the conservative choice: at worst the cycle is
// still bounded by maxDepth.
func canonical(dir string) string {
	if resolved, err := filepath.EvalSymlinks(dir); err == nil {
		return resolved
	}
	return filepath.Clean(dir)
}

// markVisited records dir and reports whether it was already seen.
func (w *treeWalker) markVisited(dir string) bool {
	key := canonical(dir)
	if _, seen := w.visited[key]; seen {
		return true
	}
	w.visited[key] = struct{}{}
	return false
}

func (w *treeWalker) walk(dir string) error {
	if err := w.ctx.Err(); err != nil {
		return err
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		// An unreadable directory is skipped, not fatal: a scan crosses plenty
		// of directories the user cannot read.
		return nil
	}
	for _, ent := range entries {
		if err := w.visit(dir, ent); err != nil {
			return err
		}
	}
	return nil
}

// visit handles one directory entry: recurse into directories, report files.
func (w *treeWalker) visit(dir string, ent os.DirEntry) error {
	full := filepath.Join(dir, ent.Name())
	rel, err := filepath.Rel(w.root, full)
	if err != nil {
		return nil
	}
	depth := strings.Count(rel, string(filepath.Separator))

	isDir, ok := w.entryIsDir(full, ent)
	if !ok {
		return nil
	}

	if isDir {
		if _, skip := skipDirNames[ent.Name()]; skip {
			return nil
		}
		if depth >= w.maxDepth {
			return nil
		}
		if w.markVisited(full) {
			return nil // already walked, reached here through a link
		}
		return w.walk(full)
	}

	if depth > w.maxDepth {
		return nil
	}
	return w.onFile(full, dir, ent, depth)
}

// entryIsDir reports whether the entry is a directory, following symlinks. The
// second result is false when the entry could not be classified and should be
// skipped. ReadDir already knows the type for everything but a symlink, so only
// a link costs a stat.
func (w *treeWalker) entryIsDir(full string, ent os.DirEntry) (bool, bool) {
	if ent.Type()&os.ModeSymlink == 0 {
		return ent.IsDir(), true
	}
	st, err := os.Stat(full)
	if err != nil {
		// A broken link: nothing to walk or report.
		return false, false
	}
	return st.IsDir(), true
}
