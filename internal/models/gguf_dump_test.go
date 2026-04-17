package models

import (
	"bytes"
	"strings"
	"testing"
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
