package llamacpp

import (
	"path/filepath"
	"strings"
)

// InferModelID returns a model identifier inferred from a local path, suitable for
// display as a canonical-style id. When the path includes a Hugging Face hub cache
// segment (models--ns--repo-name with slashes encoded as "--"), it decodes that
// segment into ns/repo/name for non-GGUF paths. For a .gguf file under that layout,
// the id is provider/filenameStem: provider is the first namespace segment from the
// decoded hub folder (e.g. unsloth from models--unsloth--…), and filenameStem is the
// .gguf file name without extension. Otherwise it uses the .gguf file stem alone or
// the last path component for directories.
func InferModelID(path string) string {
	if path == "" {
		return ""
	}
	clean := filepath.Clean(path)
	pathSlash := filepath.ToSlash(clean)
	base := filepath.Base(clean)
	ext := filepath.Ext(base)
	isGGUF := strings.EqualFold(ext, ".gguf")
	stem := strings.TrimSuffix(base, ext)

	if decoded, ok := decodeHFModelsRepoID(pathSlash); ok {
		if isGGUF {
			provider := hfRepoProvider(decoded)
			return provider + "/" + stem
		}
		return decoded
	}
	if isGGUF {
		return stem
	}
	return base
}

// decodeHFModelsRepoID returns the Hugging Face repo-style id from the first
// path segment matching models--ns--rest.
func decodeHFModelsRepoID(pathSlash string) (string, bool) {
	for _, seg := range strings.Split(pathSlash, "/") {
		if !strings.HasPrefix(seg, "models--") {
			continue
		}
		rest := strings.TrimPrefix(seg, "models--")
		if rest == "" {
			continue
		}
		parts := strings.Split(rest, "--")
		if len(parts) >= 2 {
			return strings.Join(parts, "/"), true
		}
	}
	return "", false
}

// hfRepoProvider returns the first segment of a decoded hub repo id (e.g. unsloth
// from unsloth/gemma-4-31B-it).
func hfRepoProvider(decodedRepoID string) string {
	i := strings.Index(decodedRepoID, "/")
	if i < 0 {
		return decodedRepoID
	}
	return decodedRepoID[:i]
}
