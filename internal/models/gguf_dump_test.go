package models

import (
	"bytes"
	"encoding/binary"
	"strings"
	"testing"

	"github.com/abrander/gguf"
)

func TestDumpGGUF_nonexistent(t *testing.T) {
	var buf bytes.Buffer
	err := DumpGGUF(&buf, "/nonexistent/nope.gguf", DumpGGUFOptions{})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestFormatGGUFMetadataValue_scalars(t *testing.T) {
	tests := []struct {
		v    interface{}
		want string
	}{
		{"hi", `"hi"`},
		{true, "true"},
		{uint32(7), "7"},
		{float32(1.5), "1.5"},
	}
	for _, tc := range tests {
		got := formatGGUFMetadataValue(tc.v)
		if got != tc.want {
			t.Errorf("%#v: got %q want %q", tc.v, got, tc.want)
		}
	}
}

func TestFormatGGUFMetadataValue_stringSliceLong(t *testing.T) {
	s := make([]string, 20)
	for i := range s {
		s[i] = "x"
	}
	got := formatGGUFMetadataValue(s)
	if !strings.Contains(got, "len=20") || !strings.Contains(got, "…") {
		t.Fatalf("expected summary for long slice: %q", got)
	}
}

// writeGGUFReport is what gguf-dump actually prints. Driving it directly avoids
// needing a real GGUF file, which cannot be built without the writer half of
// the library.
func TestWriteGGUFReport(t *testing.T) {
	t.Parallel()

	r := &gguf.Reader{
		Version:   3,
		ByteOrder: binary.LittleEndian,
		Metadata: map[string]any{
			"general.name":          "Qwen3 8B",
			"general.architecture":  "qwen3",
			"qwen3.context_length":  uint32(32768),
			"tokenizer.ggml.tokens": []string{"a", "b", "c"},
		},
	}

	var buf bytes.Buffer
	if err := writeGGUFReport(&buf, r, DumpGGUFOptions{}); err != nil {
		t.Fatal(err)
	}
	out := buf.String()

	for _, want := range []string{
		"gguf_version: 3",
		"byte_order: little-endian",
		"metadata:",
		`general.architecture: "qwen3"`,
		`general.name: "Qwen3 8B"`,
		"qwen3.context_length: 32768",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q:\n%s", want, out)
		}
	}

	// Metadata is sorted so successive dumps of one file are comparable.
	archAt := strings.Index(out, "general.architecture")
	nameAt := strings.Index(out, "general.name")
	if archAt < 0 || nameAt < 0 || archAt > nameAt {
		t.Errorf("metadata should be sorted by key:\n%s", out)
	}

	// Tensors are omitted unless asked for; a real vocab makes them enormous.
	if strings.Contains(out, "tensors:") {
		t.Errorf("tensors should be opt-in:\n%s", out)
	}
}

func TestWriteGGUFReport_tensorsOptIn(t *testing.T) {
	t.Parallel()

	r := &gguf.Reader{
		Version:   3,
		ByteOrder: binary.BigEndian,
		Metadata:  map[string]any{},
		Tensors: []gguf.TensorInfo{
			{Name: "token_embd.weight", Dimensions: []uint64{4096, 32000}},
		},
	}

	var buf bytes.Buffer
	if err := writeGGUFReport(&buf, r, DumpGGUFOptions{Tensors: true}); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(out, "byte_order: big-endian") {
		t.Errorf("byte order not reported:\n%s", out)
	}
	if !strings.Contains(out, "token_embd.weight") {
		t.Errorf("tensor not listed:\n%s", out)
	}
}
