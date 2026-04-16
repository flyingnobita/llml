package llamacpp

import (
	"os"
	"path/filepath"
	"strings"
)

// walkSearchTree walks root recursively with the same depth and skip-dir rules as GGUF
// discovery. For each file (non-directory) within maxDepth, onFile is called with the
// full path, parent directory, directory entry, and depth (path segments below root).
// Unlike [filepath.WalkDir], symbolic links to directories are followed so HF hub layouts work.
func walkSearchTree(root string, maxDepth int, onFile func(fullPath, parentDir string, ent os.DirEntry, depth int) error) error {
	var walk func(string) error
	walk = func(dir string) error {
		entries, err := os.ReadDir(dir)
		if err != nil {
			return nil
		}
		for _, ent := range entries {
			name := ent.Name()
			full := filepath.Join(dir, name)
			rel, err := filepath.Rel(root, full)
			if err != nil {
				continue
			}
			depth := strings.Count(rel, string(filepath.Separator))

			st, err := os.Stat(full)
			if err != nil {
				continue
			}
			if st.IsDir() {
				if _, skip := skipDirNames[name]; skip {
					continue
				}
				if depth >= maxDepth {
					continue
				}
				if err := walk(full); err != nil {
					return err
				}
				continue
			}

			if depth > maxDepth {
				continue
			}
			if err := onFile(full, dir, ent, depth); err != nil {
				return err
			}
		}
		return nil
	}
	return walk(root)
}
