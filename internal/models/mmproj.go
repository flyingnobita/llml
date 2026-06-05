package models

import (
	"os"
	"path/filepath"
	"strings"
)

// isMMProjName reports whether a file name (without path) identifies a
// multimodal projector sidecar. Case-insensitive substring match on "mmproj".
func isMMProjName(name string) bool {
	return strings.Contains(strings.ToLower(name), "mmproj")
}

// ResolveMMProj looks for a sibling mmproj GGUF file in the same directory as
// modelPath and returns the chosen absolute path and any ambiguous candidates.
//
//   - Returns ("", nil) when modelPath is empty or the directory cannot be read.
//   - Returns (chosen, nil) when exactly one sibling mmproj GGUF is found.
//   - Returns (chosen, nil) when multiple sibling mmproj GGUFs exist but exactly
//     one shares the longest common leading-token prefix with the model file name.
//   - Returns ("", candidates) when multiple sibling mmproj GGUFs are found and
//     the prefix heuristic leaves more than one candidate (ambiguous).
func ResolveMMProj(modelPath string) (string, []string) {
	modelPath = strings.TrimSpace(modelPath)
	if modelPath == "" {
		return "", nil
	}
	dir := filepath.Dir(modelPath)
	modelBase := filepath.Base(modelPath)

	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", nil
	}

	var candidates []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if strings.EqualFold(name, modelBase) {
			continue // skip the model file itself
		}
		if !strings.HasSuffix(strings.ToLower(name), ".gguf") {
			continue
		}
		if !isMMProjName(name) {
			continue
		}
		candidates = append(candidates, filepath.Join(dir, name))
	}

	switch len(candidates) {
	case 0:
		return "", nil
	case 1:
		return candidates[0], nil
	default:
		// Try to pick the one with the longest shared leading-token prefix.
		chosen := mmprojPrefixDisambiguate(modelBase, candidates)
		if chosen != "" {
			return chosen, nil
		}
		return "", candidates
	}
}

// mmprojPrefixDisambiguate returns the single candidate whose file name shares
// the longest leading-token prefix with modelBase (case-insensitive, split on
// -, _, .), or "" when multiple candidates share the same maximum or max is 0.
func mmprojPrefixDisambiguate(modelBase string, candidates []string) string {
	modelTokens := mmprojSplitTokens(strings.TrimSuffix(strings.ToLower(modelBase), ".gguf"))

	best := -1
	bestPath := ""
	tie := false

	for _, c := range candidates {
		base := filepath.Base(c)
		cTokens := mmprojSplitTokens(strings.TrimSuffix(strings.ToLower(base), ".gguf"))
		n := mmprojCommonPrefixLen(modelTokens, cTokens)
		if n > best {
			best = n
			bestPath = c
			tie = false
		} else if n == best {
			tie = true
		}
	}

	if best <= 0 || tie {
		return ""
	}
	return bestPath
}

// mmprojSplitTokens splits s on -, _, and . boundaries (adjacent separators collapse).
func mmprojSplitTokens(s string) []string {
	return strings.FieldsFunc(s, func(r rune) bool {
		return r == '-' || r == '_' || r == '.'
	})
}

// mmprojCommonPrefixLen counts how many leading tokens a and b share (case-insensitive).
func mmprojCommonPrefixLen(a, b []string) int {
	n := 0
	for i := range a {
		if i >= len(b) {
			break
		}
		if strings.EqualFold(a[i], b[i]) {
			n++
		} else {
			break
		}
	}
	return n
}
