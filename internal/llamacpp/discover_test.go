package llamacpp

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMergeSearchRoots_dedupes(t *testing.T) {
	t.Setenv(envExtraPaths, "/foo/bar,/foo/bar,/baz")
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatal(err)
	}
	got := MergeSearchRoots([]string{filepath.Join(home, "models")}, false)
	seen := make(map[string]int)
	for _, p := range got {
		seen[p]++
		if seen[p] > 1 {
			t.Fatalf("duplicate path %q", p)
		}
	}
}

func TestDiscover_findsSafetensorsModelDir(t *testing.T) {
	tmp := t.TempDir()
	modelDir := filepath.Join(tmp, "snapshots", "abc123")
	if err := os.MkdirAll(modelDir, 0o755); err != nil {
		t.Fatal(err)
	}
	cfg := `{"model_type":"llama","architectures":["LlamaForCausalLM"]}`
	if err := os.WriteFile(filepath.Join(modelDir, "config.json"), []byte(cfg), 0o644); err != nil {
		t.Fatal(err)
	}
	w := filepath.Join(modelDir, "model.safetensors")
	if err := os.WriteFile(w, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := Discover(Options{ExtraRoots: []string{tmp}, MaxDepth: 8, SkipDefaultRoots: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("want 1 model, got %d", len(got))
	}
	if got[0].Backend != BackendVLLM {
		t.Fatalf("Backend: got %v want BackendVLLM", got[0].Backend)
	}
	if got[0].Path != filepath.Clean(modelDir) {
		t.Fatalf("Path: got %q want %q", got[0].Path, filepath.Clean(modelDir))
	}
	if !strings.Contains(got[0].Parameters, "vllm") {
		t.Fatalf("Parameters: got %q", got[0].Parameters)
	}
	if got[0].Name != "abc123" {
		t.Fatalf("Name (non-Hub): got %q want abc123", got[0].Name)
	}
}

func TestDiscover_vllmNameUsesHFRepoID(t *testing.T) {
	tmp := t.TempDir()
	modelDir := filepath.Join(tmp, "hub", "models--opendatalab--MinerU2.5-2509-1.2B", "snapshots", "879e58bdd9566632b27a88a81f0e2961873311f")
	if err := os.MkdirAll(modelDir, 0o755); err != nil {
		t.Fatal(err)
	}
	cfg := `{"model_type":"llama","architectures":["LlamaForCausalLM"]}`
	if err := os.WriteFile(filepath.Join(modelDir, "config.json"), []byte(cfg), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(modelDir, "m.safetensors"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := Discover(Options{ExtraRoots: []string{tmp}, MaxDepth: 12, SkipDefaultRoots: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("want 1 model, got %d", len(got))
	}
	want := "opendatalab/MinerU2.5-2509-1.2B"
	if got[0].Name != want {
		t.Fatalf("Name: got %q want %q", got[0].Name, want)
	}
}

func TestDiscover_findsGGUF(t *testing.T) {
	tmp := t.TempDir()
	sub := filepath.Join(tmp, "nested", "here")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(sub, "tiny.gguf")
	if err := os.WriteFile(p, []byte{}, 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := Discover(Options{ExtraRoots: []string{tmp}, MaxDepth: 8, SkipDefaultRoots: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("want 1 model, got %d", len(got))
	}
	if got[0].Name != "tiny.gguf" {
		t.Fatalf("Name: got %q", got[0].Name)
	}
	if got[0].Path != filepath.Clean(p) {
		t.Fatalf("Path: got %q want %q", got[0].Path, filepath.Clean(p))
	}
}

func TestDiscover_symlinkGGUF_reportsTargetSize(t *testing.T) {
	tmp := t.TempDir()
	target := filepath.Join(tmp, "blob.bin")
	payload := bytes.Repeat([]byte("z"), 2048)
	if err := os.WriteFile(target, payload, 0o644); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(tmp, "snapshots", "rev", "model.gguf")
	if err := os.MkdirAll(filepath.Dir(link), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, link); err != nil {
		t.Skip("symlink not supported:", err)
	}

	got, err := Discover(Options{ExtraRoots: []string{tmp}, MaxDepth: 8, SkipDefaultRoots: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("want 1 model, got %d", len(got))
	}
	if got[0].Size != int64(len(payload)) {
		t.Fatalf("Size: got %d want %d (symlink Stat should follow to target)", got[0].Size, len(payload))
	}
}

func TestDiscover_followsSymlinkedRepoDir(t *testing.T) {
	tmp := t.TempDir()
	hub := filepath.Join(tmp, "hub")
	if err := os.MkdirAll(hub, 0o755); err != nil {
		t.Fatal(err)
	}
	// Store weights outside the search root so only the symlinked repo path is visible to the walk.
	storage := filepath.Join(tmp, "storage", "models--google--gemma", "snapshots", "abc")
	if err := os.MkdirAll(storage, 0o755); err != nil {
		t.Fatal(err)
	}
	gguf := filepath.Join(storage, "model.gguf")
	if err := os.WriteFile(gguf, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	linkName := filepath.Join(hub, "models--google--gemma-4-E4B-it")
	if err := os.Symlink(filepath.Join(tmp, "storage", "models--google--gemma"), linkName); err != nil {
		t.Skip("symlink not supported:", err)
	}

	got, err := Discover(Options{ExtraRoots: []string{hub}, MaxDepth: 12, SkipDefaultRoots: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("want 1 model through symlinked repo dir, got %d: %+v", len(got), got)
	}
	if got[0].Name != "model.gguf" {
		t.Fatalf("Name: %q", got[0].Name)
	}
}

func TestFormatSize(t *testing.T) {
	tests := []struct {
		bytes int64
		want  string
	}{
		{500, "500 B"},
		{1023, "1023 B"},
		{1024, "1.0 KiB"},
		{2048, "2.0 KiB"},
		{15 * 1024 * 1024 * 1024, "15.0 GiB"},
	}
	for _, tc := range tests {
		if got := FormatSize(tc.bytes); got != tc.want {
			t.Errorf("FormatSize(%d) = %q, want %q", tc.bytes, got, tc.want)
		}
	}
	if s := FormatSize(-1); s != "—" {
		t.Fatalf("negative: got %q", s)
	}
}

func TestSkipListedModel(t *testing.T) {
	if !skipListedModel(ModelFile{Name: "x.gguf", Parameters: "clip"}) {
		t.Fatal("expected clip filtered")
	}
	if !skipListedModel(ModelFile{Name: "x.gguf", Parameters: "flip"}) {
		t.Fatal("expected flip filtered")
	}
	if !skipListedModel(ModelFile{Name: "mmproj-BF16.gguf", Parameters: "gemma4"}) {
		t.Fatal("expected mmproj basename filtered")
	}
	if skipListedModel(ModelFile{Name: "model-Q4_K_M.gguf", Parameters: "gemma4"}) {
		t.Fatal("expected to keep main weights")
	}
}

func TestTruncateRunes(t *testing.T) {
	s := "hello世界"
	if TruncateRunes(s, 100) != s {
		t.Fatalf("short string changed")
	}
	got := TruncateRunes("abcdefghijklmnopqrstuvwxyz", 8)
	if len(got) < 2 {
		t.Fatalf("expected ellipsis suffix")
	}
}
