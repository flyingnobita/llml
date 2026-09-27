package models

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/flyingnobita/llml/internal/settings"
)

const splashManifestJSON = `{"model":"Qwen3.8-27B","format":{"name":"splash-packed-q4"},"artifacts":[{"path":"target/a.bin","size":100},{"path":"draft/b.bin","size":23}]}`

// writeSplashBundle lays out a Splash bundle under root the way the Hugging
// Face cache stores incoai/Qwen3.8-27B-Splash, and returns the snapshot dir.
func writeSplashBundle(t *testing.T, root, manifest string) string {
	t.Helper()
	dir := filepath.Join(root, "models--incoai--Qwen3.8-27B-Splash", "snapshots", "abc123")
	for _, sub := range []string{"target", "draft", "tokenizer"} {
		if err := os.MkdirAll(filepath.Join(dir, sub), 0o750); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(dir, "manifest.json"), []byte(manifest), 0o600); err != nil {
		t.Fatal(err)
	}
	return dir
}

// writeHFModelDir writes a minimal safetensors model directory with the given
// config.json architecture.
func writeHFModelDir(t *testing.T, dir, arch string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o750); err != nil {
		t.Fatal(err)
	}
	cfg := `{"model_type":"qwen3_5","architectures":["` + arch + `"]}`
	if err := os.WriteFile(filepath.Join(dir, "config.json"), []byte(cfg), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "model.safetensors"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestDiscoverFindsSplashBundles(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	dir := writeSplashBundle(t, root, splashManifestJSON)

	files, err := Discover(context.Background(), Options{ExtraRoots: []string{root}, SkipDefaultRoots: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 1 {
		t.Fatalf("got %d models, want 1: %+v", len(files), files)
	}
	f := files[0]
	if f.Backend != BackendSplash || f.Path != dir || f.Size != 123 {
		t.Errorf("model = %+v", f)
	}
	if f.Parameters != "splash · Qwen3.8-27B · splash-packed-q4" {
		t.Errorf("parameters = %q", f.Parameters)
	}
	if got := SplashModelRef(f.Path); got != "incoai/Qwen3.8-27B-Splash" {
		t.Errorf("SplashModelRef = %q", got)
	}
}

// manifest.json is a common name; only one with Splash's format and a target/
// directory beside it is a bundle.
func TestSplashSourceIgnoresOtherManifests(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSplashBundle(t, root, `{"format":{"name":"something-else"}}`)

	plain := filepath.Join(root, "webapp")
	if err := os.MkdirAll(plain, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(plain, "manifest.json"), []byte(splashManifestJSON), 0o600); err != nil {
		t.Fatal(err)
	}

	files, err := Discover(context.Background(), Options{ExtraRoots: []string{root}, SkipDefaultRoots: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 0 {
		t.Fatalf("got %+v, want no models", files)
	}
}

func TestSplashModelRefOutsideHubCache(t *testing.T) {
	t.Parallel()
	if got := SplashModelRef("/models/qwen-splash/"); got != "/models/qwen-splash" {
		t.Errorf("SplashModelRef = %q, want the cleaned path", got)
	}
}

func TestDiscoverClaimsOMLXModelsAndDropsDrafts(t *testing.T) {
	t.Parallel()
	omlxDir := t.TempDir()
	other := t.TempDir()

	target := filepath.Join(omlxDir, "scottlowry", "Qwen3.8-27B-oQ4e-mtp")
	writeHFModelDir(t, target, "Qwen3_5ForConditionalGeneration")
	writeHFModelDir(t, filepath.Join(omlxDir, "z-lab", "Qwen3.8-27B-DFlash2"), "DFlash2DraftModel")
	elsewhere := filepath.Join(other, "plain-model")
	writeHFModelDir(t, elsewhere, "Qwen3ForCausalLM")

	files, err := Discover(context.Background(), Options{
		Settings:         settings.Settings{OMLXModelDirs: []string{omlxDir}},
		ExtraRoots:       []string{omlxDir, other},
		SkipDefaultRoots: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	byPath := map[string]ModelFile{}
	for _, f := range files {
		byPath[f.Path] = f
	}
	if len(files) != 2 {
		t.Fatalf("got %d models, want the target and the plain model (draft dropped): %+v", len(files), files)
	}
	if f := byPath[target]; f.Backend != BackendOMLX || f.Parameters != "omlx · qwen3_5 · Qwen3_5ForConditionalGeneration" {
		t.Errorf("oMLX row = %+v", f)
	}
	if f := byPath[elsewhere]; f.Backend != BackendVLLM {
		t.Errorf("model outside oMLX dirs should stay vLLM, got %+v", f)
	}
}

func TestOMLXModelDirFor(t *testing.T) {
	t.Parallel()
	dirs := []string{"/Users/u/.omlx/models", "/Volumes/ssd/mlx"}
	tests := map[string]string{
		"/Users/u/.omlx/models/org/model": "/Users/u/.omlx/models",
		"/Volumes/ssd/mlx/model":          "/Volumes/ssd/mlx",
		"/Users/u/.omlx/models":           "",
		"/Users/u/.omlx/models-old/model": "",
		"/elsewhere/model":                "",
	}
	for path, want := range tests {
		if got := OMLXModelDirFor(path, dirs); got != want {
			t.Errorf("OMLXModelDirFor(%q) = %q, want %q", path, got, want)
		}
	}
}

func TestFindOMLXBinary(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("PATH", t.TempDir())

	binDir := filepath.Join(home, ".omlx", "bin")
	if err := os.MkdirAll(binDir, 0o750); err != nil {
		t.Fatal(err)
	}
	shim := makeFakeExecutable(t, binDir, "omlx")

	// The app's shim is found with no configuration, and from each form
	// OMLX_PATH may take. It wins over the common directories, where Homebrew
	// may hold a symlink to it.
	for _, configured := range []string{"", filepath.Join(home, ".omlx"), binDir, shim} {
		if got := findOMLXBinary(configured); got != shim {
			t.Errorf("findOMLXBinary(%q) = %q, want %q", configured, got, shim)
		}
	}
}

func TestFindSplashBinary(t *testing.T) {
	t.Setenv("PATH", t.TempDir())

	dir := t.TempDir()
	bin := makeFakeExecutable(t, dir, "splash")
	for _, configured := range []string{dir, bin} {
		if got := findSplashBinary(configured); got != bin {
			t.Errorf("findSplashBinary(%q) = %q, want %q", configured, got, bin)
		}
	}
}

func TestPlatformSupports(t *testing.T) {
	t.Parallel()
	mac := Platform{GOOS: "darwin", GOARCH: "arm64"}
	intelMac := Platform{GOOS: "darwin", GOARCH: "amd64"}
	linux := Platform{GOOS: "linux", GOARCH: "amd64"}

	tests := []struct {
		p    Platform
		b    ModelBackend
		want bool
	}{
		{mac, BackendOMLX, true},
		{mac, BackendSplash, true},
		{mac, BackendNInfer, false},
		{intelMac, BackendOMLX, false},
		{linux, BackendSplash, false},
		{linux, BackendNInfer, true},
		{linux, BackendVLLM, true},
		{Platform{}, BackendOMLX, true},
		{Platform{}, BackendNInfer, true},
	}
	for _, tt := range tests {
		if got := tt.p.Supports(tt.b); got != tt.want {
			t.Errorf("%+v.Supports(%s) = %v, want %v", tt.p, tt.b, got, tt.want)
		}
	}
}
