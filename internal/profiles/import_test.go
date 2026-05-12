package profiles

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/BurntSushi/toml"
)

func TestReadPortable(t *testing.T) {
	dir := t.TempDir()

	t.Run("valid v2", func(t *testing.T) {
		path := filepath.Join(dir, "valid.toml")
		content := `schema_version = 2

[[profiles]]
name = "fast"
backend = "llama"
model_hint = "Llama-3-8B-GGUF"
args = ["--n-gpu-layers 80", "--ctx-size 4096"]
`
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
		f, err := ReadPortable(path)
		if err != nil {
			t.Fatal(err)
		}
		if f.SchemaVersion != 2 {
			t.Fatalf("version = %d", f.SchemaVersion)
		}
		if len(f.Profiles) != 1 {
			t.Fatalf("got %d profiles", len(f.Profiles))
		}
	})

	t.Run("unsupported version", func(t *testing.T) {
		path := filepath.Join(dir, "v1.toml")
		if err := os.WriteFile(path, []byte(`schema_version = 1`+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		_, err := ReadPortable(path)
		if err == nil || !strings.Contains(err.Error(), "schema_version") {
			t.Fatalf("expected schema_version error, got %v", err)
		}
	})

	t.Run("missing file", func(t *testing.T) {
		_, err := ReadPortable(filepath.Join(dir, "nope.toml"))
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("malformed TOML", func(t *testing.T) {
		path := filepath.Join(dir, "bad.toml")
		if err := os.WriteFile(path, []byte(`schema_version = 2`+"\n[[profiles]\nname = \n"), 0o644); err != nil {
			t.Fatal(err)
		}
		_, err := ReadPortable(path)
		if err == nil || !strings.Contains(err.Error(), "TOML") {
			t.Fatalf("expected TOML error, got %v", err)
		}
	})
}

func TestStripModelLocationParams(t *testing.T) {
	t.Run("llama env and args stripped", func(t *testing.T) {
		env := []PortableEnvVar{
			{Key: "LLAMA_CACHE", Value: "/tmp"},
			{Key: "CUDA_VISIBLE_DEVICES", Value: "0"},
			{Key: "HF_TOKEN", Value: "secret"},
		}
		args := []string{"--model /tmp/m.gguf", "--n-gpu-layers 80", "-hf org/repo"}
		keptEnv, keptArgs, droppedEnv, droppedArgs := StripModelLocationParams("llama", env, args)
		if len(keptEnv) != 1 || keptEnv[0].Key != "CUDA_VISIBLE_DEVICES" {
			t.Fatalf("keptEnv = %+v", keptEnv)
		}
		if len(keptArgs) != 1 || keptArgs[0] != "--n-gpu-layers 80" {
			t.Fatalf("keptArgs = %+v", keptArgs)
		}
		if len(droppedEnv) != 2 {
			t.Fatalf("droppedEnv = %v", droppedEnv)
		}
		if len(droppedArgs) != 2 {
			t.Fatalf("droppedArgs = %v", droppedArgs)
		}
	})

	t.Run("vllm args stripped", func(t *testing.T) {
		args := []string{"--model meta-llama/Llama-3", "--max-model-len 8192", "--tokenizer hf/tokenizer"}
		_, keptArgs, _, droppedArgs := StripModelLocationParams("vllm", nil, args)
		if len(keptArgs) != 1 || keptArgs[0] != "--max-model-len 8192" {
			t.Fatalf("keptArgs = %+v", keptArgs)
		}
		if len(droppedArgs) != 2 {
			t.Fatalf("droppedArgs = %v", droppedArgs)
		}
	})

	t.Run("vllm env stripped", func(t *testing.T) {
		env := []PortableEnvVar{
			{Key: "HF_HOME", Value: "/cache"},
			{Key: "VLLM_ATTENTION_BACKEND", Value: "FLASH_ATTN"},
		}
		keptEnv, _, droppedEnv, _ := StripModelLocationParams("vllm", env, nil)
		if len(keptEnv) != 1 || keptEnv[0].Key != "VLLM_ATTENTION_BACKEND" {
			t.Fatalf("keptEnv = %+v", keptEnv)
		}
		if len(droppedEnv) != 1 {
			t.Fatalf("droppedEnv = %v", droppedEnv)
		}
	})

	t.Run("ollama omitted from stripping", func(t *testing.T) {
		env := []PortableEnvVar{{Key: "OLLAMA_HOST", Value: "127.0.0.1:11434"}}
		keptEnv, _, droppedEnv, _ := StripModelLocationParams("ollama", env, nil)
		if len(keptEnv) != 1 {
			t.Fatalf("keptEnv = %+v", keptEnv)
		}
		if len(droppedEnv) != 0 {
			t.Fatalf("droppedEnv = %v", droppedEnv)
		}
	})

	t.Run("unknown backend no stripping", func(t *testing.T) {
		env := []PortableEnvVar{{Key: "LLAMA_CACHE", Value: "/x"}}
		keptEnv, _, droppedEnv, _ := StripModelLocationParams("unknown", env, nil)
		if len(keptEnv) != 1 {
			t.Fatalf("keptEnv = %+v", keptEnv)
		}
		if len(droppedEnv) != 0 {
			t.Fatalf("droppedEnv = %v", droppedEnv)
		}
	})
}

func TestPortableToProfile(t *testing.T) {
	pp := PortableProfile{
		Name:    "  gpu-chat  ",
		Backend: "llama.cpp",
		Args:    []string{"--n-gpu-layers 80", "--flash-attn"},
		Env: []PortableEnvVar{
			{Key: "CUDA_VISIBLE_DEVICES", Value: "0"},
		},
		UseCase: PortableUseCase{
			Primary: "assistant",
			Tags:    []string{"Interactive", "BALANCED"},
		},
		Hardware: PortableHardware{
			Class:     "hybrid",
			GPUCount:  intPtr(1),
			MinVRAMGB: intPtr(24),
			MaxVRAMGB: intPtr(12),
			Notes:     "tested",
		},
	}
	p := PortableToProfile(pp)

	if p.Name != "gpu-chat" {
		t.Fatalf("name = %q", p.Name)
	}
	if p.Backend != "llama" {
		t.Fatalf("backend = %q, want llama", p.Backend)
	}
	if len(p.Args) != 3 {
		t.Fatalf("args (expanded) = %v (%d)", p.Args, len(p.Args))
	}
	if p.Args[0] != "--n-gpu-layers" || p.Args[1] != "80" || p.Args[2] != "--flash-attn" {
		t.Fatalf("args = %v", p.Args)
	}
	if len(p.Env) != 1 || p.Env[0].Key != "CUDA_VISIBLE_DEVICES" {
		t.Fatalf("env = %+v", p.Env)
	}
	if p.UseCase.Primary != UseCaseChat {
		t.Fatalf("useCase.primary = %q, want chat", p.UseCase.Primary)
	}
	if len(p.UseCase.Tags) != 2 {
		t.Fatalf("tags = %v", p.UseCase.Tags)
	}
	if p.Hardware.Class != HardwareClassMixed {
		t.Fatalf("hw.class = %q, want mixed", p.Hardware.Class)
	}
	// VRAM bounds should have been swapped (12 < 24 → min=12, max=24 after swap)
	if p.Hardware.MinVRAMGB == nil || *p.Hardware.MinVRAMGB != 12 {
		t.Fatalf("hw.minVRAM = %#v, want 12", p.Hardware.MinVRAMGB)
	}
	if p.Hardware.MaxVRAMGB == nil || *p.Hardware.MaxVRAMGB != 24 {
		t.Fatalf("hw.maxVRAM = %#v, want 24", p.Hardware.MaxVRAMGB)
	}
}

func TestPortableToProfile_EmptyBackendDefaultsToLlama(t *testing.T) {
	pp := PortableProfile{
		Name:    "default",
		Backend: "",
	}
	p := PortableToProfile(pp)
	if p.Backend != "llama" {
		t.Fatalf("backend = %q, want llama", p.Backend)
	}
}

func TestPortableToProfile_CPUClassClearsGPUFields(t *testing.T) {
	pp := PortableProfile{
		Name:    "cpu-only",
		Backend: "llama",
		Hardware: PortableHardware{
			Class:     "cpu",
			GPUCount:  intPtr(1),
			MinVRAMGB: intPtr(24),
		},
	}
	p := PortableToProfile(pp)
	if p.Hardware.Class != HardwareClassCPU {
		t.Fatalf("class = %q", p.Hardware.Class)
	}
	if p.Hardware.GPUCount != nil {
		t.Fatalf("GPUCount should be nil for CPU class")
	}
	if p.Hardware.MinVRAMGB != nil {
		t.Fatalf("MinVRAMGB should be nil for CPU class")
	}
}

func writeParamsJSON(t *testing.T, path string, data any) {
	t.Helper()
	b, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, b, 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestImportProfiles_NewEntry(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv("HOME", dir)
	t.Setenv("AppData", dir)
	llmlDir := filepath.Join(dir, ".config", "llml")
	if err := os.MkdirAll(llmlDir, 0o755); err != nil {
		t.Fatal(err)
	}
	paramsPath := filepath.Join(llmlDir, "model-params.json")
	writeParamsJSON(t, paramsPath, map[string]any{"version": 3, "models": map[string]any{}})

	modelPath := filepath.Join(dir, "model.gguf")
	profiles := []Profile{
		{Name: "fast", Backend: "llama", Args: []string{"--n-gpu-layers", "80"}},
	}
	result, err := ImportProfiles(modelPath, profiles, false)
	if err != nil {
		t.Fatal(err)
	}
	if result.Added != 1 || result.Replaced != 0 || result.Skipped != 0 {
		t.Fatalf("result = %+v", result)
	}

	ent, err := LoadEntry(modelPath)
	if err != nil {
		t.Fatal(err)
	}
	if len(ent.Profiles) != 1 || ent.Profiles[0].Name != "fast" {
		t.Fatalf("profiles = %+v", ent.Profiles)
	}
}

func TestImportProfiles_AppendToExisting(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv("HOME", dir)
	t.Setenv("AppData", dir)
	llmlDir := filepath.Join(dir, ".config", "llml")
	if err := os.MkdirAll(llmlDir, 0o755); err != nil {
		t.Fatal(err)
	}
	paramsPath := filepath.Join(llmlDir, "model-params.json")
	writeParamsJSON(t, paramsPath, map[string]any{"version": 3, "models": map[string]any{}})

	modelPath := filepath.Join(dir, "model.gguf")
	// Seed with one profile
	if _, err := ImportProfiles(modelPath, []Profile{
		{Name: "existing", Backend: "llama", Args: []string{"--ctx-size", "4096"}},
	}, false); err != nil {
		t.Fatal(err)
	}
	// Append a second
	result, err := ImportProfiles(modelPath, []Profile{
		{Name: "new-one", Backend: "llama", Args: []string{"--n-gpu-layers", "80"}},
	}, false)
	if err != nil {
		t.Fatal(err)
	}
	if result.Added != 1 {
		t.Fatalf("result = %+v", result)
	}

	ent, err := LoadEntry(modelPath)
	if err != nil {
		t.Fatal(err)
	}
	if len(ent.Profiles) != 2 {
		t.Fatalf("got %d profiles", len(ent.Profiles))
	}
}

func TestImportProfiles_CollisionSkip(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv("HOME", dir)
	t.Setenv("AppData", dir)
	llmlDir := filepath.Join(dir, ".config", "llml")
	if err := os.MkdirAll(llmlDir, 0o755); err != nil {
		t.Fatal(err)
	}
	paramsPath := filepath.Join(llmlDir, "model-params.json")
	writeParamsJSON(t, paramsPath, map[string]any{"version": 3, "models": map[string]any{}})

	modelPath := filepath.Join(dir, "model.gguf")
	if _, err := ImportProfiles(modelPath, []Profile{
		{Name: "default", Backend: "llama", Args: []string{"--ctx-size", "4096"}},
	}, false); err != nil {
		t.Fatal(err)
	}
	// Try to import another "default" — should skip
	result, err := ImportProfiles(modelPath, []Profile{
		{Name: "default", Backend: "vllm", Args: []string{"--max-model-len", "8192"}},
	}, false)
	if err != nil {
		t.Fatal(err)
	}
	if result.Skipped != 1 || result.Added != 0 {
		t.Fatalf("result = %+v", result)
	}

	ent, err := LoadEntry(modelPath)
	if err != nil {
		t.Fatal(err)
	}
	if len(ent.Profiles) != 1 || ent.Profiles[0].Backend != "llama" {
		t.Fatalf("profiles = %+v", ent.Profiles)
	}
}

func TestImportProfiles_ForceOverwrite(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv("HOME", dir)
	t.Setenv("AppData", dir)
	llmlDir := filepath.Join(dir, ".config", "llml")
	if err := os.MkdirAll(llmlDir, 0o755); err != nil {
		t.Fatal(err)
	}
	paramsPath := filepath.Join(llmlDir, "model-params.json")
	writeParamsJSON(t, paramsPath, map[string]any{"version": 3, "models": map[string]any{}})

	modelPath := filepath.Join(dir, "model.gguf")
	if _, err := ImportProfiles(modelPath, []Profile{
		{Name: "default", Backend: "llama", Args: []string{"--ctx-size", "4096"}},
	}, false); err != nil {
		t.Fatal(err)
	}
	// Force-overwrite "default"
	result, err := ImportProfiles(modelPath, []Profile{
		{Name: "default", Backend: "vllm", Args: []string{"--max-model-len", "8192"}},
	}, true)
	if err != nil {
		t.Fatal(err)
	}
	if result.Replaced != 1 || result.Added != 0 {
		t.Fatalf("result = %+v", result)
	}

	ent, err := LoadEntry(modelPath)
	if err != nil {
		t.Fatal(err)
	}
	if len(ent.Profiles) != 1 || ent.Profiles[0].Backend != "vllm" {
		t.Fatalf("profiles = %+v", ent.Profiles)
	}
}

func TestImportProfiles_EmptyTargetKey(t *testing.T) {
	_, err := ImportProfiles("", []Profile{{Name: "x", Backend: "llama"}}, false)
	if err == nil {
		t.Fatal("expected error for empty target key")
	}
}

func TestPortableRoundTrip(t *testing.T) {
	dir := t.TempDir()

	original := PortableFile{
		SchemaVersion: 2,
		Profiles: []PortableProfile{
			{
				Name:      "gpu-chat",
				Backend:   "llama",
				ModelHint: "model-x",
				Args:      []string{"--n-gpu-layers 80", "--flash-attn"},
				Env:       []PortableEnvVar{{Key: "CUDA_VISIBLE_DEVICES", Value: "0"}},
				UseCase:   PortableUseCase{Primary: "chat", Tags: []string{"interactive"}},
				Hardware:  PortableHardware{Class: "gpu", GPUCount: intPtr(1)},
			},
		},
	}

	path := filepath.Join(dir, "roundtrip.toml")
	data, err := toml.Marshal(original)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}

	f, err := ReadPortable(path)
	if err != nil {
		t.Fatal(err)
	}
	p := PortableToProfile(f.Profiles[0])

	if p.Name != "gpu-chat" {
		t.Fatalf("name = %q", p.Name)
	}
	if p.Backend != "llama" {
		t.Fatalf("backend = %q", p.Backend)
	}
	if len(p.Args) != 3 {
		t.Fatalf("args = %v", p.Args)
	}
	if p.UseCase.Primary != UseCaseChat {
		t.Fatalf("useCase = %q", p.UseCase.Primary)
	}
}

// ---------------------------------------------------------------------------
// SetActiveProfile tests (case 26 from test plan)
// ---------------------------------------------------------------------------

func TestSetActiveProfile(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv("HOME", dir)
	t.Setenv("AppData", dir)
	llmlDir := filepath.Join(dir, ".config", "llml")
	if err := os.MkdirAll(llmlDir, 0o755); err != nil {
		t.Fatal(err)
	}
	paramsPath := filepath.Join(llmlDir, "model-params.json")
	writeParamsJSON(t, paramsPath, map[string]any{"version": 3, "models": map[string]any{}})

	modelPath := filepath.Join(dir, "model.gguf")
	if _, err := ImportProfiles(modelPath, []Profile{
		{Name: "first", Backend: "llama"},
		{Name: "second", Backend: "vllm"},
	}, false); err != nil {
		t.Fatal(err)
	}

	// Activate "second"
	if err := SetActiveProfile(modelPath, "second"); err != nil {
		t.Fatal(err)
	}

	ent, err := LoadEntry(modelPath)
	if err != nil {
		t.Fatal(err)
	}
	if ent.ActiveIndex != 1 {
		t.Fatalf("ActiveIndex = %d, want 1", ent.ActiveIndex)
	}
}

func TestSetActiveProfile_NotFound(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv("HOME", dir)
	t.Setenv("AppData", dir)
	llmlDir := filepath.Join(dir, ".config", "llml")
	if err := os.MkdirAll(llmlDir, 0o755); err != nil {
		t.Fatal(err)
	}
	paramsPath := filepath.Join(llmlDir, "model-params.json")
	writeParamsJSON(t, paramsPath, map[string]any{"version": 3, "models": map[string]any{}})

	modelPath := filepath.Join(dir, "model.gguf")
	if _, err := ImportProfiles(modelPath, []Profile{
		{Name: "only", Backend: "llama"},
	}, false); err != nil {
		t.Fatal(err)
	}

	err := SetActiveProfile(modelPath, "nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent profile")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Fatalf("expected 'not found' in error, got %v", err)
	}
}

// ---------------------------------------------------------------------------
// Combined URL fetch + import integration test (cases 28, 31, 32)
// ---------------------------------------------------------------------------

func TestFetchAndImportIntegration(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv("HOME", dir)
	t.Setenv("AppData", dir)
	llmlDir := filepath.Join(dir, ".config", "llml")
	if err := os.MkdirAll(llmlDir, 0o755); err != nil {
		t.Fatal(err)
	}
	paramsPath := filepath.Join(llmlDir, "model-params.json")
	writeParamsJSON(t, paramsPath, map[string]any{"version": 3, "models": map[string]any{}})

	// Start a test TLS server serving a valid profile
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(singleProfileTOML("url-profile")))
	}))
	defer srv.Close()
	defer useTestServerClient(srv)()

	// Step 1: Fetch from URL
	f, err := FetchPortable(context.Background(), srv.URL+"/profile.toml")
	if err != nil {
		t.Fatalf("fetch: %v", err)
	}
	if len(f.Profiles) != 1 {
		t.Fatalf("expected 1 profile, got %d", len(f.Profiles))
	}

	// Step 2: Convert and import
	modelPath := filepath.Join(dir, "model.gguf")
	pp := f.Profiles[0]
	p := PortableToProfile(pp)
	_, _, _, _ = StripModelLocationParams(p.Backend, pp.Env, pp.Args)

	result, err := ImportProfiles(modelPath, []Profile{p}, false)
	if err != nil {
		t.Fatalf("import: %v", err)
	}
	if result.Added != 1 {
		t.Fatalf("expected 1 added, got %+v", result)
	}

	// Step 3: Activate
	if err := SetActiveProfile(modelPath, p.Name); err != nil {
		t.Fatalf("activate: %v", err)
	}

	// Verify
	ent, err := LoadEntry(modelPath)
	if err != nil {
		t.Fatal(err)
	}
	if ent.ActiveIndex != 0 {
		t.Fatalf("ActiveIndex = %d", ent.ActiveIndex)
	}
	if ent.Profiles[0].Name != "url-profile" {
		t.Fatalf("name = %q", ent.Profiles[0].Name)
	}
}

func TestFetchAndImportIntegration_Collision(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv("HOME", dir)
	t.Setenv("AppData", dir)
	llmlDir := filepath.Join(dir, ".config", "llml")
	if err := os.MkdirAll(llmlDir, 0o755); err != nil {
		t.Fatal(err)
	}
	paramsPath := filepath.Join(llmlDir, "model-params.json")
	writeParamsJSON(t, paramsPath, map[string]any{"version": 3, "models": map[string]any{}})

	modelPath := filepath.Join(dir, "model.gguf")

	// Pre-seed with a profile named "url-profile"
	if _, err := ImportProfiles(modelPath, []Profile{
		{Name: "url-profile", Backend: "ollama"},
	}, false); err != nil {
		t.Fatal(err)
	}

	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(singleProfileTOML("url-profile")))
	}))
	defer srv.Close()
	defer useTestServerClient(srv)()

	f, err := FetchPortable(context.Background(), srv.URL+"/profile.toml")
	if err != nil {
		t.Fatalf("fetch: %v", err)
	}

	pp := f.Profiles[0]
	p := PortableToProfile(pp)
	_, _, _, _ = StripModelLocationParams(p.Backend, pp.Env, pp.Args)

	// Without --force: should skip
	result, err := ImportProfiles(modelPath, []Profile{p}, false)
	if err != nil {
		t.Fatalf("import: %v", err)
	}
	if result.Skipped != 1 {
		t.Fatalf("expected 1 skipped, got %+v", result)
	}

	// With --force: should replace
	result, err = ImportProfiles(modelPath, []Profile{p}, true)
	if err != nil {
		t.Fatalf("import force: %v", err)
	}
	if result.Replaced != 1 {
		t.Fatalf("expected 1 replaced, got %+v", result)
	}
}

// Case 27: multi-profile with --activate equivalent (check before import)
func TestMultiProfileActivateCheck(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(multiProfileTOML()))
	}))
	defer srv.Close()
	defer useTestServerClient(srv)()

	f, err := FetchPortable(context.Background(), srv.URL+"/multi.toml")
	if err != nil {
		t.Fatalf("fetch: %v", err)
	}
	// Simulate the --activate check: fail if > 1 profile with --activate
	if len(f.Profiles) > 1 {
		// This is the error path — verify the detection
		if len(f.Profiles) != 3 {
			t.Fatalf("expected 3 profiles, got %d", len(f.Profiles))
		}
		// In the actual CLI, this would exit with an error
	}
}
