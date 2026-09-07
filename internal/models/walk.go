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
// Unlike [filepath.WalkDir], symbolic links to directories are followed so HF hub layouts work.
//
// The walk checks ctx before each directory and returns ctx.Err() when it is
// done, so an abandoned scan does not keep reading the filesystem.
func walkSearchTree(ctx context.Context, root string, maxDepth int, onFile func(fullPath, parentDir string, ent os.DirEntry, depth int) error) error {
	w := treeWalker{ctx: ctx, root: root, maxDepth: maxDepth, onFile: onFile}
	return w.walk(root)
}

// treeWalker carries the walk's fixed parameters so the recursive step does not
// have to thread them through every call.
type treeWalker struct {
	ctx      context.Context
	root     string
	maxDepth int
	onFile   func(fullPath, parentDir string, ent os.DirEntry, depth int) error
}

func (w treeWalker) walk(dir string) error {
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
func (w treeWalker) visit(dir string, ent os.DirEntry) error {
	full := filepath.Join(dir, ent.Name())
	rel, err := filepath.Rel(w.root, full)
	if err != nil {
		return nil
	}
	depth := strings.Count(rel, string(filepath.Separator))

	// Stat rather than ent.IsDir: symlinked directories must be followed, which
	// is how the Hugging Face hub layout is laid out.
	st, err := os.Stat(full)
	if err != nil {
		return nil
	}

	if st.IsDir() {
		if _, skip := skipDirNames[ent.Name()]; skip {
			return nil
		}
		if depth >= w.maxDepth {
			return nil
		}
		return w.walk(full)
	}

	if depth > w.maxDepth {
		return nil
	}
	return w.onFile(full, dir, ent, depth)
}
