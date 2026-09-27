package models

import (
	"bytes"
	"context"
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"
)

// ninferFixture returns bytes shaped like an NInfer v3 container: header,
// space-padded JSON manifest, then an opaque payload.
func ninferFixture(manifest string) []byte {
	const pad = 64
	body := []byte(manifest)
	body = append(body, bytes.Repeat([]byte(" "), pad)...)
	var buf bytes.Buffer
	buf.WriteString("NINFER\x00\x03")
	_ = binary.Write(&buf, binary.LittleEndian, uint64(len(body)))
	buf.Write(make([]byte, 16))
	buf.Write(body)
	buf.Write([]byte{0xde, 0xad, 0xbe, 0xef})
	return buf.Bytes()
}

const qwenManifest = `{"components":{"text":{"config":{"architectures":["Qwen3_5ForCausalLM"],"model_type":"qwen3_5_text"}},"vision":{"config":{"model_type":"qwen3_5_vision"}}}}`

func TestReadNInferManifest(t *testing.T) {
	t.Parallel()

	m, ok := readNInferManifest(bytes.NewReader(ninferFixture(qwenManifest)))
	if !ok {
		t.Fatal("valid container not recognized")
	}
	text := m.Components["text"].Config
	if text.ModelType != "qwen3_5_text" || len(text.Architectures) != 1 || text.Architectures[0] != "Qwen3_5ForCausalLM" {
		t.Errorf("text config = %+v", text)
	}
}

func TestReadNInferManifestRejectsOtherFiles(t *testing.T) {
	t.Parallel()

	oversized := ninferFixture(qwenManifest)
	binary.LittleEndian.PutUint64(oversized[8:16], ninferMaxManifestBytes+1)

	tests := map[string][]byte{
		"empty":         nil,
		"gguf magic":    append([]byte("GGUF"), make([]byte, 64)...),
		"short header":  []byte("NINFER\x00\x03"),
		"oversized":     oversized,
		"truncated":     ninferFixture(qwenManifest)[:40],
		"not json":      ninferFixture("not json"),
		"zero manifest": append([]byte("NINFER\x00\x03"), make([]byte, 24)...),
	}
	for name, data := range tests {
		if _, ok := readNInferManifest(bytes.NewReader(data)); ok {
			t.Errorf("%s: accepted as NInfer", name)
		}
	}
}

func TestNInferParamsSummary(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	good := filepath.Join(dir, "good.ninfer")
	if err := os.WriteFile(good, ninferFixture(qwenManifest), 0o600); err != nil {
		t.Fatal(err)
	}
	if got, want := ninferParamsSummary(good), "ninfer · qwen3_5_text · Qwen3_5ForCausalLM"; got != want {
		t.Errorf("summary = %q, want %q", got, want)
	}

	noText := filepath.Join(dir, "notext.ninfer")
	if err := os.WriteFile(noText, ninferFixture(`{"components":{}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if got := ninferParamsSummary(noText); got != "ninfer · —" {
		t.Errorf("summary without text component = %q", got)
	}
}

func TestDiscoverFindsNInferArtifacts(t *testing.T) {
	t.Parallel()
	root := t.TempDir()

	p := filepath.Join(root, "qwen", "qwen3_8_27b_nvfp4.ninfer")
	if err := os.MkdirAll(filepath.Dir(p), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, ninferFixture(qwenManifest), 0o600); err != nil {
		t.Fatal(err)
	}

	files, err := Discover(context.Background(), Options{ExtraRoots: []string{root}, SkipDefaultRoots: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 1 {
		t.Fatalf("got %d models, want 1: %+v", len(files), files)
	}
	f := files[0]
	if f.Backend != BackendNInfer || f.Path != p || f.Name != "qwen3_8_27b_nvfp4.ninfer" || f.LaunchTarget() != p {
		t.Errorf("model = %+v", f)
	}
	if f.Parameters != "ninfer · qwen3_5_text · Qwen3_5ForCausalLM" {
		t.Errorf("parameters = %q", f.Parameters)
	}
}

func TestFindNInferBinary(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	t.Setenv("HOME", t.TempDir())

	checkout := t.TempDir()
	apps := filepath.Join(checkout, "build", "apps")
	if err := os.MkdirAll(apps, 0o750); err != nil {
		t.Fatal(err)
	}
	bin := makeFakeExecutable(t, apps, "ninfer-serve")

	for _, configured := range []string{checkout, apps, bin} {
		if got := findNInferBinary(configured); got != bin {
			t.Errorf("findNInferBinary(%q) = %q, want %q", configured, got, bin)
		}
	}
	if got := findNInferBinary(t.TempDir()); got != "" {
		t.Errorf("empty dir resolved to %q", got)
	}

	// A non-executable file named ninfer-serve is a failed build, not a server.
	broken := t.TempDir()
	if err := os.WriteFile(filepath.Join(broken, "ninfer-serve"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if got := findNInferBinary(broken); got != "" {
		t.Errorf("non-executable file resolved to %q", got)
	}
}

func TestProbeHost(t *testing.T) {
	t.Parallel()

	for in, want := range map[string]string{
		"":          defaultProbeHost,
		"0.0.0.0":   defaultProbeHost,
		"::":        defaultProbeHost,
		"10.0.0.2":  "10.0.0.2",
		"localhost": "localhost",
	} {
		if got := probeHost(in); got != want {
			t.Errorf("probeHost(%q) = %q, want %q", in, got, want)
		}
	}
}
