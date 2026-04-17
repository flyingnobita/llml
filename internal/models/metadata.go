package models

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/abrander/gguf"
)

// GGUFGeneralName returns trimmed general.name from the GGUF file's KV metadata.
// It returns an error if the file cannot be read, is not valid GGUF, the key is absent,
// or the value is empty after trimming.
func GGUFGeneralName(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	r, err := gguf.Open(f)
	if err != nil {
		return "", err
	}

	s, err := r.Metadata.String("general.name")
	if err != nil {
		return "", fmt.Errorf("general.name: %w", err)
	}
	s = strings.TrimSpace(s)
	if s == "" {
		return "", fmt.Errorf("general.name empty or missing")
	}
	return s, nil
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
	f, err := os.Open(path)
	if err != nil {
		return "—"
	}
	defer f.Close()

	r, err := gguf.Open(f)
	if err != nil {
		return "—"
	}

	var parts []string

	if arch, err := r.Metadata.String("general.architecture"); err == nil && arch != "" {
		parts = append(parts, arch)
	}

	for _, key := range contextLengthKeys {
		if ctx, err := r.Metadata.Int(key); err == nil && ctx > 0 {
			parts = append(parts, "ctx "+strconv.Itoa(ctx))
			break
		}
	}

	if len(parts) == 0 {
		if name, err := r.Metadata.String("general.name"); err == nil && name != "" {
			return truncateMeta(name, 48)
		}
		return "—"
	}
	return strings.Join(parts, " · ")
}

// truncateMeta limits long general.name strings.
func truncateMeta(s string, max int) string {
	runes := []rune(s)
	if len(runes) <= max {
		return s
	}
	return string(runes[:max-1]) + "…"
}
