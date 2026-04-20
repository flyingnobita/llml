package models

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/abrander/gguf"
)

// withGGUFMetadata opens path as a GGUF file and calls fn with its metadata.
// The file is closed before returning.
func withGGUFMetadata(path string, fn func(gguf.Metadata) error) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	r, err := gguf.Open(f)
	if err != nil {
		return err
	}
	return fn(r.Metadata)
}

// GGUFGeneralName returns trimmed general.name from the GGUF file's KV metadata.
// It returns an error if the file cannot be read, is not valid GGUF, the key is absent,
// or the value is empty after trimming.
func GGUFGeneralName(path string) (string, error) {
	var name string
	err := withGGUFMetadata(path, func(meta gguf.Metadata) error {
		s, err := meta.String("general.name")
		if err != nil {
			return fmt.Errorf("general.name: %w", err)
		}
		s = strings.TrimSpace(s)
		if s == "" {
			return fmt.Errorf("general.name empty or missing")
		}
		name = s
		return nil
	})
	return name, err
}

var contextLengthKeys = []string{
	"llama.context_length",
	"qwen2.context_length",
	"qwen.context_length",
	"gemma.context_length",
	"phi3.context_length",
	"deepseek.context_length",
	"general.context_length",
}

// ggufParamsSummary reads GGUF metadata for a compact Parameters cell.
func ggufParamsSummary(path string) string {
	var result string
	_ = withGGUFMetadata(path, func(meta gguf.Metadata) error {
		var parts []string
		if arch, err := meta.String("general.architecture"); err == nil && arch != "" {
			parts = append(parts, arch)
		}
		for _, key := range contextLengthKeys {
			if ctx, err := meta.Int(key); err == nil && ctx > 0 {
				parts = append(parts, "ctx "+strconv.Itoa(ctx))
				break
			}
		}
		if len(parts) == 0 {
			if name, err := meta.String("general.name"); err == nil && name != "" {
				result = truncateMeta(name, 48)
				return nil
			}
			result = "—"
			return nil
		}
		result = strings.Join(parts, " · ")
		return nil
	})
	if result == "" {
		result = "—"
	}
	return result
}

// truncateMeta limits long general.name strings.
func truncateMeta(s string, max int) string {
	runes := []rune(s)
	if len(runes) <= max {
		return s
	}
	return string(runes[:max-1]) + "…"
}
