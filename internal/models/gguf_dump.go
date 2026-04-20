package models

import (
	"encoding/binary"
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/abrander/gguf"
)

// DumpGGUFOptions configures [DumpGGUF].
type DumpGGUFOptions struct {
	// Tensors, when true, appends a tensor list (name, GGML type, dimensions). Can be huge.
	Tensors bool
}

// DumpGGUF writes GGUF header info, all metadata key-value pairs (sorted by key), and
// optionally tensor names to w. It does not read tensor weights.
func DumpGGUF(w io.Writer, path string, opts DumpGGUFOptions) error {
	r, err := gguf.OpenFile(path)
	if err != nil {
		return err
	}
	return writeGGUFReport(w, r, opts)
}

func writeGGUFReport(w io.Writer, r *gguf.Reader, opts DumpGGUFOptions) error {
	fmt.Fprintf(w, "gguf_version: %d\n", r.Version)
	fmt.Fprintf(w, "byte_order: %s\n", byteOrderLabel(r.ByteOrder))

	keys := make([]string, 0, len(r.Metadata))
	for k := range r.Metadata {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	fmt.Fprintln(w, "metadata:")
	for _, k := range keys {
		fmt.Fprintf(w, "  %s: %s\n", k, formatGGUFMetadataValue(r.Metadata[k]))
	}

	if opts.Tensors {
		fmt.Fprintln(w, "tensors:")
		for _, t := range r.Tensors {
			fmt.Fprintf(w, "  %s: %s %v\n", t.Name, t.Type.String(), t.Dimensions)
		}
	}
	return nil
}

func byteOrderLabel(o binary.ByteOrder) string {
	switch o {
	case binary.LittleEndian:
		return "little-endian"
	case binary.BigEndian:
		return "big-endian"
	default:
		return fmt.Sprintf("%v", o)
	}
}

// formatGGUFMetadataValue renders a metadata value read by abrander/gguf. Large arrays
// (e.g. tokenizer vocab) are summarized instead of printed in full.
func formatGGUFMetadataValue(v any) string {
	if v == nil {
		return "<nil>"
	}
	switch vv := v.(type) {
	case string:
		return formatGGUFStringScalar(vv)
	case bool:
		return fmt.Sprintf("%t", vv)
	case uint8:
		return fmt.Sprintf("%d", vv)
	case int8:
		return fmt.Sprintf("%d", vv)
	case uint16:
		return fmt.Sprintf("%d", vv)
	case int16:
		return fmt.Sprintf("%d", vv)
	case uint32:
		return fmt.Sprintf("%d", vv)
	case int32:
		return fmt.Sprintf("%d", vv)
	case uint64:
		return fmt.Sprintf("%d", vv)
	case int64:
		return fmt.Sprintf("%d", vv)
	case float32:
		return fmt.Sprintf("%g", vv)
	case float64:
		return fmt.Sprintf("%g", vv)
	case []string:
		return formatGGUFStringSlice(vv)
	case []bool:
		return formatGGUFSlice("bool", len(vv), func(i int) string { return fmt.Sprintf("%t", vv[i]) })
	case []uint8:
		return formatGGUFSlice("uint8", len(vv), func(i int) string { return fmt.Sprintf("%d", vv[i]) })
	case []int8:
		return formatGGUFSlice("int8", len(vv), func(i int) string { return fmt.Sprintf("%d", vv[i]) })
	case []uint16:
		return formatGGUFSlice("uint16", len(vv), func(i int) string { return fmt.Sprintf("%d", vv[i]) })
	case []int16:
		return formatGGUFSlice("int16", len(vv), func(i int) string { return fmt.Sprintf("%d", vv[i]) })
	case []uint32:
		return formatGGUFSlice("uint32", len(vv), func(i int) string { return fmt.Sprintf("%d", vv[i]) })
	case []int32:
		return formatGGUFSlice("int32", len(vv), func(i int) string { return fmt.Sprintf("%d", vv[i]) })
	case []uint64:
		return formatGGUFSlice("uint64", len(vv), func(i int) string { return fmt.Sprintf("%d", vv[i]) })
	case []int64:
		return formatGGUFSlice("int64", len(vv), func(i int) string { return fmt.Sprintf("%d", vv[i]) })
	case []float32:
		return formatGGUFSlice("float32", len(vv), func(i int) string { return fmt.Sprintf("%g", vv[i]) })
	case []float64:
		return formatGGUFSlice("float64", len(vv), func(i int) string { return fmt.Sprintf("%g", vv[i]) })
	default:
		if stringer, ok := v.(fmt.Stringer); ok {
			return stringer.String()
		}
		return fmt.Sprintf("%v (%T)", v, v)
	}
}

const maxScalarStringRunes = 500

func formatGGUFStringScalar(s string) string {
	r := []rune(s)
	if len(r) <= maxScalarStringRunes {
		return fmt.Sprintf("%q", s)
	}
	return fmt.Sprintf("%q… (%d runes, truncated)", string(r[:maxScalarStringRunes]), len(r))
}

// renderSliceHead builds the opening "[elem0, elem1, …, elemN" portion (no closing bracket)
// for the first min(n, head) elements using at(i). The caller appends the closing.
func renderSliceHead(n, head int, at func(int) string) string {
	var b strings.Builder
	b.WriteByte('[')
	for i := 0; i < n && i < head; i++ {
		if i > 0 {
			b.WriteString(", ")
		}
		b.WriteString(at(i))
	}
	return b.String()
}

func formatGGUFStringSlice(vv []string) string {
	const head = 6
	if len(vv) == 0 {
		return "[]"
	}
	body := renderSliceHead(len(vv), head, func(i int) string { return fmt.Sprintf("%q", vv[i]) })
	if len(vv) <= head {
		return body + "]"
	}
	return fmt.Sprintf("%s, …] (len=%d strings)", body, len(vv))
}

func formatGGUFSlice(elemName string, n int, at func(i int) string) string {
	const head = 8
	if n == 0 {
		return "[]"
	}
	body := renderSliceHead(n, head, at)
	if n <= head {
		return body + "]"
	}
	return fmt.Sprintf("%s, …] (%s[len=%d])", body, elemName, n)
}
