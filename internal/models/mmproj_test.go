package models

import (
	"os"
	"path/filepath"
	"testing"
)

// writeGGUF creates an empty file with the given name in dir and returns the path.
func writeGGUF(t *testing.T, dir, name string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte{}, 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestResolveMMProj_emptyPath(t *testing.T) {
	chosen, cands := ResolveMMProj("")
	if chosen != "" || len(cands) != 0 {
		t.Fatalf("empty input: got chosen=%q cands=%v", chosen, cands)
	}
}

func TestResolveMMProj_noSibling(t *testing.T) {
	dir := t.TempDir()
	model := writeGGUF(t, dir, "gemma-4-Q4.gguf")
	chosen, cands := ResolveMMProj(model)
	if chosen != "" || len(cands) != 0 {
		t.Fatalf("no sibling: got chosen=%q cands=%v", chosen, cands)
	}
}

func TestResolveMMProj_singleSibling(t *testing.T) {
	dir := t.TempDir()
	model := writeGGUF(t, dir, "gemma-4-26B-Q4_K_XL.gguf")
	mmproj := writeGGUF(t, dir, "mmproj-BF16.gguf")
	chosen, cands := ResolveMMProj(model)
	if chosen != mmproj {
		t.Fatalf("single sibling: got chosen=%q want %q", chosen, mmproj)
	}
	if len(cands) != 0 {
		t.Fatalf("single sibling: unexpected cands %v", cands)
	}
}

func TestResolveMMProj_modelFileNotReturnedAsItsOwnMMProj(t *testing.T) {
	// A model file that happens to contain "mmproj" in its name should not be
	// treated as its own projector.
	dir := t.TempDir()
	model := writeGGUF(t, dir, "some-mmproj-large.gguf")
	chosen, cands := ResolveMMProj(model)
	if chosen != "" || len(cands) != 0 {
		t.Fatalf("self-match: got chosen=%q cands=%v", chosen, cands)
	}
}

func TestResolveMMProj_multipleSiblingsNoSharedPrefix(t *testing.T) {
	dir := t.TempDir()
	model := writeGGUF(t, dir, "gemma-4-26B-Q4.gguf")
	writeGGUF(t, dir, "mmproj-BF16.gguf")
	writeGGUF(t, dir, "mmproj-F16.gguf")
	chosen, cands := ResolveMMProj(model)
	if chosen != "" {
		t.Fatalf("ambiguous: expected empty chosen, got %q", chosen)
	}
	if len(cands) != 2 {
		t.Fatalf("ambiguous: expected 2 candidates, got %v", cands)
	}
}

func TestResolveMMProj_multipleSiblingsOneSharesPrefix(t *testing.T) {
	dir := t.TempDir()
	// Model: "modelA-Q4.gguf"; one mmproj shares the "modelA" prefix.
	model := writeGGUF(t, dir, "modelA-Q4.gguf")
	mmprojA := writeGGUF(t, dir, "modelA-mmproj-BF16.gguf")
	writeGGUF(t, dir, "modelB-mmproj-BF16.gguf")
	chosen, cands := ResolveMMProj(model)
	if chosen != mmprojA {
		t.Fatalf("prefix-match: got chosen=%q want %q", chosen, mmprojA)
	}
	if len(cands) != 0 {
		t.Fatalf("prefix-match: unexpected cands %v", cands)
	}
}

func TestResolveMMProj_nonGgufIgnored(t *testing.T) {
	dir := t.TempDir()
	model := writeGGUF(t, dir, "model.gguf")
	// A .bin file with mmproj in its name should not be returned.
	p := filepath.Join(dir, "mmproj.bin")
	if err := os.WriteFile(p, []byte{}, 0o600); err != nil {
		t.Fatal(err)
	}
	chosen, cands := ResolveMMProj(model)
	if chosen != "" || len(cands) != 0 {
		t.Fatalf("non-gguf: got chosen=%q cands=%v", chosen, cands)
	}
}

func TestResolveMMProj_realGemma4Path(t *testing.T) {
	const snap = "/Users/flyingnobita/.cache/huggingface/hub/models--unsloth--gemma-4-26B-A4B-it-GGUF/snapshots/3365c68df1a83799b846d05324ebfadbb8cc70b3"
	modelPath := filepath.Join(snap, "gemma-4-26B-A4B-it-UD-Q4_K_XL.gguf")
	if _, err := os.Stat(modelPath); err != nil {
		t.Skipf("real Gemma 4 GGUF not present (%v)", err)
	}
	wantMMProj := filepath.Join(snap, "mmproj-BF16.gguf")
	chosen, cands := ResolveMMProj(modelPath)
	if chosen != wantMMProj {
		t.Fatalf("got chosen=%q want %q (cands=%v)", chosen, wantMMProj, cands)
	}
	if len(cands) != 0 {
		t.Fatalf("expected no ambiguous candidates, got %v", cands)
	}
}

func TestIsMMProjName(t *testing.T) {
	cases := []struct {
		name string
		want bool
	}{
		{"mmproj-BF16.gguf", true},
		{"mmproj-F16.gguf", true},
		{"MMPROJ-BF16.gguf", true},
		{"gemma-4-mmproj.gguf", true},
		{"gemma-4-Q4.gguf", false},
		{"llama-3-8B.gguf", false},
	}
	for _, c := range cases {
		if got := isMMProjName(c.name); got != c.want {
			t.Errorf("isMMProjName(%q) = %v, want %v", c.name, got, c.want)
		}
	}
}
