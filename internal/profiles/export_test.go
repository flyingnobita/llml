package profiles

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/BurntSushi/toml"
)

// testLlmlDir returns the llml config directory for a temp $HOME.
// Uses os.UserConfigDir so the path is correct on macOS and Linux.
func testLlmlDir(t *testing.T, homeDir string) string {
	t.Helper()
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(homeDir, ".config"))
	cfgDir, err := os.UserConfigDir()
	if err != nil {
		t.Fatal(err)
	}
	llmlDir := filepath.Join(cfgDir, "llml")
	if err := os.MkdirAll(llmlDir, 0o755); err != nil {
		t.Fatal(err)
	}
	return llmlDir
}

func TestRecombineArgs(t *testing.T) {
	tests := []struct {
		name string
		in   []string
		want []string
	}{
		{
			name: "flag value pairs",
			in:   []string{"--n-gpu-layers", "80", "--ctx-size", "4096"},
			want: []string{"--n-gpu-layers 80", "--ctx-size 4096"},
		},
		{
			name: "standalone flags",
			in:   []string{"--flash-attn", "--no-mmap"},
			want: []string{"--flash-attn", "--no-mmap"},
		},
		{
			name: "mixed",
			in:   []string{"--n-gpu-layers", "80", "--flash-attn", "--ctx-size", "4096"},
			want: []string{"--n-gpu-layers 80", "--flash-attn", "--ctx-size 4096"},
		},
		{
			name: "negative numeric value",
			in:   []string{"--temperature", "-0.5", "--repetition-penalty", "-1.0"},
			want: []string{"--temperature -0.5", "--repetition-penalty -1.0"},
		},
		{
			name: "already panel rows pass through",
			in:   []string{"--n-gpu-layers 80", "--flash-attn"},
			want: []string{"--n-gpu-layers 80", "--flash-attn"},
		},
		{
			name: "mixed already panel and tokens",
			in:   []string{"--n-gpu-layers 80", "--flash-attn", "--ctx-size", "4096"},
			want: []string{"--n-gpu-layers 80", "--flash-attn", "--ctx-size 4096"},
		},
		{
			name: "empty",
			in:   nil,
			want: nil,
		},
		{
			name: "single flag",
			in:   []string{"--help"},
			want: []string{"--help"},
		},
		{
			name: "single flag with value",
			in:   []string{"--port", "8080"},
			want: []string{"--port 8080"},
		},
		{
			name: "negative number without flag",
			in:   []string{"-0.5"},
			want: []string{"-0.5"},
		},
		{
			name: "koboldcpp args",
			in:   []string{"--gpulayers", "80", "--contextsize", "4096", "--flashattention"},
			want: []string{"--gpulayers 80", "--contextsize 4096", "--flashattention"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := RecombineArgs(tt.in)
			if len(got) != len(tt.want) {
				t.Fatalf("RecombineArgs(%q) = %q (len=%d), want %q (len=%d)", tt.in, got, len(got), tt.want, len(tt.want))
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("RecombineArgs(%q)[%d] = %q, want %q", tt.in, i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestModelHint(t *testing.T) {
	tests := []struct {
		key  string
		want string
	}{
		{"/models/Llama-3-8B-Q4.gguf", "Llama-3-8B-Q4"},
		{"/home/user/models/mistral-7b-v0.1.Q4_K_M.gguf", "mistral-7b-v0.1.Q4_K_M"},
		{"/models/Mistral-7B-Instruct-v01", "Mistral-7B-Instruct-v01"},
		{"ollama://llama3.2", "llama3.2"},
		{"ollama://mistral", "mistral"},
		{"ollama://qwen2.5:72b", "qwen2.5:72b"},
		{".gguf", ".gguf"},
		{"/home/user/.hidden.gguf", ".hidden"},
		// HF hub snapshot directories
		{"/home/user/.cache/huggingface/hub/models--google--gemma-4-E4B-it/snapshots/83df0a889143b1dbfc61b591bbc639540fd9ce4c", "gemma-4-E4B-it"},
		{"/home/user/.cache/huggingface/hub/models--opendatalab--MinerU2.5-2509-1.2B/snapshots/879e58bdd9566632b27a8a81f0e2961873311f67", "MinerU2.5-2509-1.2B"},
		{"/home/user/.cache/huggingface/hub/models--unsloth--Qwen3.6-35B-A3B-GGUF/snapshots/abc123/Qwen3.6-35B-A3B-UD-Q4_K_XL.gguf", "Qwen3.6-35B-A3B-GGUF"},
	}

	for _, tt := range tests {
		t.Run(tt.key, func(t *testing.T) {
			got := ModelHint(tt.key)
			if got != tt.want {
				t.Errorf("ModelHint(%q) = %q, want %q", tt.key, got, tt.want)
			}
		})
	}
}

func TestShouldExcludeEnv(t *testing.T) {
	excluded := []string{
		"LLAMA_CACHE", "llama_cache",
		"LLAMA_ARG_MODEL", "llama_arg_model",
		"LLAMA_ARG_FOO", "llama_arg_bar",
		"HF_HOME", "hf_home",
		"HUGGINGFACE_HUB_CACHE", "huggingface_hub_cache",
		"KOBOLDCPP_MODEL", "koboldcpp_model",
		"KOBOLDCPP_LORA", "koboldcpp_lora",
		"KOBOLDCPP_MMPROJ", "koboldcpp_mmproj",
		"KOBOLDCPP_TOKENIZER", "koboldcpp_tokenizer",
		"KOBOLDCPP_MODELS_DIR", "koboldcpp_models_dir",
	}
	kept := []string{
		"OLLAMA_NUM_GPU",
		"OLLAMA_NUM_CTX",
		"LLAMA_SERVER_PORT",
		"MY_CUSTOM_VAR",
		"KOBOLDCPP_PORT",
	}

	for _, k := range excluded {
		if !ShouldExcludeEnv(k) {
			t.Errorf("ShouldExcludeEnv(%q) = false, want true", k)
		}
	}
	for _, k := range kept {
		if ShouldExcludeEnv(k) {
			t.Errorf("ShouldExcludeEnv(%q) = true, want false", k)
		}
	}
}

func TestProfileToPortable(t *testing.T) {
	p := Profile{
		Name:    "gpu-full",
		Backend: "llama",
		UseCase: UseCaseMetadata{
			Primary: UseCaseChat,
			Tags:    []string{"interactive", "balanced"},
		},
		Hardware: HardwareMetadata{
			Class:     HardwareClassGPU,
			GPUCount:  intPtr(1),
			MinVRAMGB: intPtr(24),
			MaxVRAMGB: intPtr(24),
			Notes:     "Tested on M1 Max",
		},
		Env: []EnvVar{
			{Key: "OLLAMA_NUM_GPU", Value: "999"},
			{Key: "LLAMA_CACHE", Value: "/tmp/cache"},
			{Key: "HF_HOME", Value: "/tmp/hf"},
		},
		Args: []string{"--n-gpu-layers", "80", "--flash-attn", "--ctx-size", "4096"},
	}

	pp := ProfileToPortable(p, "/models/Llama-3-8B-Q4.gguf")

	if pp.Name != "gpu-full" {
		t.Errorf("Name = %q, want %q", pp.Name, "gpu-full")
	}
	if pp.Backend != "llama" {
		t.Errorf("Backend = %q, want %q", pp.Backend, "llama")
	}
	if pp.ModelHint != "Llama-3-8B-Q4" {
		t.Errorf("ModelHint = %q, want %q", pp.ModelHint, "Llama-3-8B-Q4")
	}

	if len(pp.Args) != 3 {
		t.Fatalf("len(Args) = %d, want 3", len(pp.Args))
	}
	if pp.Args[0] != "--n-gpu-layers 80" {
		t.Errorf("Args[0] = %q", pp.Args[0])
	}
	if pp.Args[1] != "--flash-attn" {
		t.Errorf("Args[1] = %q", pp.Args[1])
	}
	if pp.Args[2] != "--ctx-size 4096" {
		t.Errorf("Args[2] = %q", pp.Args[2])
	}

	if len(pp.Env) != 1 {
		t.Fatalf("len(Env) = %d, want 1 (excluded LLAMA_CACHE and HF_HOME)", len(pp.Env))
	}
	if pp.Env[0].Key != "OLLAMA_NUM_GPU" || pp.Env[0].Value != "999" {
		t.Errorf("Env[0] = {%q, %q}", pp.Env[0].Key, pp.Env[0].Value)
	}

	if pp.UseCase.Primary != "chat" {
		t.Errorf("UseCase.Primary = %q", pp.UseCase.Primary)
	}
	if len(pp.UseCase.Tags) != 2 {
		t.Errorf("len(UseCase.Tags) = %d", len(pp.UseCase.Tags))
	}

	if pp.Hardware.Class != "gpu" {
		t.Errorf("Hardware.Class = %q", pp.Hardware.Class)
	}
	if pp.Hardware.GPUCount == nil || *pp.Hardware.GPUCount != 1 {
		t.Errorf("Hardware.GPUCount = %v", pp.Hardware.GPUCount)
	}
	if pp.Hardware.Notes != "Tested on M1 Max" {
		t.Errorf("Hardware.Notes = %q", pp.Hardware.Notes)
	}
}

func TestProfileToPortableEmptyBackendDefaultsToLlama(t *testing.T) {
	p := Profile{Name: "default"}
	pp := ProfileToPortable(p, "/models/test.gguf")
	if pp.Backend != "llama" {
		t.Errorf("Backend = %q, want %q", pp.Backend, "llama")
	}
}

func TestProfileToPortableOllamaModelHint(t *testing.T) {
	p := Profile{Name: "default"}
	pp := ProfileToPortable(p, "ollama://llama3.2")
	if pp.ModelHint != "llama3.2" {
		t.Errorf("ModelHint = %q, want %q", pp.ModelHint, "llama3.2")
	}
}

func TestEntryToPortable(t *testing.T) {
	ent := Entry{
		Profiles: []Profile{
			{Name: "gpu-full", Backend: "llama", Args: []string{"--n-gpu-layers", "80"}},
			{Name: "cpu-only", Backend: "llama", Args: []string{"--n-gpu-layers", "0"}},
		},
		ActiveIndex: 0,
	}
	pps := EntryToPortable("/models/Llama-3-8B-Q4.gguf", ent)
	if len(pps) != 2 {
		t.Fatalf("len = %d, want 2", len(pps))
	}
	if pps[0].Name != "gpu-full" {
		t.Errorf("pps[0].Name = %q", pps[0].Name)
	}
	if pps[1].Name != "cpu-only" {
		t.Errorf("pps[1].Name = %q", pps[1].Name)
	}
}

func TestDefaultExportFilename(t *testing.T) {
	name := DefaultExportFilename()
	if !strings.HasPrefix(name, "llml-profiles-") {
		t.Errorf("filename %q missing prefix", name)
	}
	if !strings.HasSuffix(name, ".toml") {
		t.Errorf("filename %q missing .toml suffix", name)
	}
	if len(name) != len("llml-profiles-YYYYMMDD.toml") {
		t.Errorf("filename %q has unexpected length %d", name, len(name))
	}
}

func TestNextAvailablePath(t *testing.T) {
	dir := t.TempDir()

	// No collision.
	dest := filepath.Join(dir, "test.toml")
	path, existed, err := NextAvailablePath(dest)
	if err != nil {
		t.Fatal(err)
	}
	if existed {
		t.Error("expected existed=false")
	}
	if path != dest {
		t.Errorf("path = %q, want %q", path, dest)
	}

	// Create the file, then check collision.
	if err := os.WriteFile(dest, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	path, existed, err = NextAvailablePath(dest)
	if err != nil {
		t.Fatal(err)
	}
	if !existed {
		t.Error("expected existed=true")
	}
	if path != filepath.Join(dir, "test-2.toml") {
		t.Errorf("path = %q, want test-2.toml", path)
	}

	// Create test-2.toml, next should be test-3.toml.
	if err := os.WriteFile(path, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	path, _, err = NextAvailablePath(dest)
	if err != nil {
		t.Fatal(err)
	}
	if path != filepath.Join(dir, "test-3.toml") {
		t.Errorf("path = %q, want test-3.toml", path)
	}
}

func TestNextAvailablePathNoExtension(t *testing.T) {
	dir := t.TempDir()

	dest := filepath.Join(dir, "myfile")
	if err := os.WriteFile(dest, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	path, existed, err := NextAvailablePath(dest)
	if err != nil {
		t.Fatal(err)
	}
	if !existed {
		t.Error("expected existed=true")
	}
	if path != filepath.Join(dir, "myfile-2") {
		t.Errorf("path = %q, want myfile-2", path)
	}
}

func TestWritePortable(t *testing.T) {
	dir := t.TempDir()
	dest := filepath.Join(dir, "export.toml")

	profiles := []PortableProfile{
		{
			Name:      "test-profile",
			Backend:   "llama",
			ModelHint: "Test-Model",
			Args:      []string{"--n-gpu-layers 80", "--ctx-size 4096"},
			Env: []PortableEnvVar{
				{Key: "MY_VAR", Value: "123"},
			},
		},
	}

	if err := WritePortable(dest, profiles, false); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(dest)
	if err != nil {
		t.Fatal(err)
	}
	content := string(data)

	// Verify schema_version and key fields.
	if !strings.Contains(content, "schema_version = 2") {
		t.Error("missing schema_version = 2")
	}
	if !strings.Contains(content, `name = "test-profile"`) {
		t.Error("missing profile name")
	}
	if !strings.Contains(content, `backend = "llama"`) {
		t.Error("missing backend")
	}
	if !strings.Contains(content, `model_hint = "Test-Model"`) {
		t.Error("missing model_hint")
	}

	// Collision without force.
	if err := WritePortable(dest, profiles, false); err == nil {
		t.Error("expected error on collision without force")
	}

	// Collision with force.
	if err := WritePortable(dest, profiles, true); err != nil {
		t.Errorf("expected success with force: %v", err)
	}
}

func TestWritePortableMultipleProfiles(t *testing.T) {
	dir := t.TempDir()
	dest := filepath.Join(dir, "multi.toml")

	profiles := []PortableProfile{
		{Name: "profile-a", Backend: "llama", ModelHint: "Model-A"},
		{Name: "profile-b", Backend: "vllm", ModelHint: "Model-B"},
	}

	if err := WritePortable(dest, profiles, false); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(dest)
	if err != nil {
		t.Fatal(err)
	}
	content := string(data)

	if strings.Count(content, "[[profiles]]") != 2 {
		t.Errorf("expected 2 [[profiles]] entries, got content:\n%s", content)
	}
}

func TestWritePortableEmptyProfiles(t *testing.T) {
	dir := t.TempDir()
	dest := filepath.Join(dir, "empty.toml")

	if err := WritePortable(dest, nil, false); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(dest)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "schema_version = 2") {
		t.Error("missing schema_version even with empty profiles")
	}
}

func TestAllToPortable(t *testing.T) {
	// AllToPortable reads the real model-params.json if it exists,
	// or returns an empty slice if it doesn't. We test that it doesn't error.
	profiles, err := AllToPortable()
	if err != nil {
		t.Fatal(err)
	}
	// May be empty if no model-params.json exists, which is fine.
	_ = profiles
}

func TestPortableFileRoundTrip(t *testing.T) {
	profiles := []PortableProfile{
		{
			Name:      "balanced-q4",
			Backend:   "llama",
			ModelHint: "Llama-3-8B-GGUF",
			Args:      []string{"--n-gpu-layers 80", "--ctx-size 4096", "--threads 8"},
			UseCase: PortableUseCase{
				Primary: "chat",
				Tags:    []string{"interactive", "balanced"},
			},
			Hardware: PortableHardware{
				Class:     "gpu",
				GPUCount:  intPtr(1),
				MinVRAMGB: intPtr(24),
				MaxVRAMGB: intPtr(24),
				Notes:     "Tested on M1 Max 32GB unified memory.",
			},
		},
	}

	f := PortableFile{
		SchemaVersion: 2,
		Profiles:      profiles,
	}

	data, err := toml.Marshal(f)
	if err != nil {
		t.Fatal(err)
	}

	// Verify it can be unmarshaled back.
	var f2 PortableFile
	if err := toml.Unmarshal(data, &f2); err != nil {
		t.Fatalf("unmarshal failed: %v\ncontent:\n%s", err, string(data))
	}
	if f2.SchemaVersion != 2 {
		t.Errorf("SchemaVersion = %d, want 2", f2.SchemaVersion)
	}
	if len(f2.Profiles) != 1 {
		t.Fatalf("len(Profiles) = %d, want 1", len(f2.Profiles))
	}
	p := f2.Profiles[0]
	if p.Name != "balanced-q4" {
		t.Errorf("Name = %q", p.Name)
	}
	if p.Backend != "llama" {
		t.Errorf("Backend = %q", p.Backend)
	}
	if len(p.Args) != 3 {
		t.Errorf("len(Args) = %d, want 3", len(p.Args))
	}
	if p.UseCase.Primary != "chat" {
		t.Errorf("UseCase.Primary = %q", p.UseCase.Primary)
	}
	if len(p.UseCase.Tags) != 2 {
		t.Errorf("len(UseCase.Tags) = %d", len(p.UseCase.Tags))
	}
	if p.Hardware.Class != "gpu" {
		t.Errorf("Hardware.Class = %q", p.Hardware.Class)
	}
	if p.Hardware.GPUCount == nil || *p.Hardware.GPUCount != 1 {
		t.Errorf("Hardware.GPUCount = %v", p.Hardware.GPUCount)
	}
	if p.Hardware.Notes != "Tested on M1 Max 32GB unified memory." {
		t.Errorf("Hardware.Notes = %q", p.Hardware.Notes)
	}

	content := string(data)
	if !strings.Contains(content, "schema_version = 2") {
		t.Error("missing schema_version")
	}
	if !strings.Contains(content, "[[profiles]]") {
		t.Error("missing [[profiles]]")
	}
	// Env should be omitted when empty.
	if strings.Contains(content, "[[profiles.env]]") {
		t.Error("unexpected [[profiles.env]] when env is empty")
	}
}

func TestPortableFileRoundTripWithEnv(t *testing.T) {
	profiles := []PortableProfile{
		{
			Name:    "gpu-full",
			Backend: "ollama",
			Env: []PortableEnvVar{
				{Key: "OLLAMA_NUM_GPU", Value: "999"},
				{Key: "OLLAMA_NUM_CTX", Value: "8192"},
			},
		},
	}

	f := PortableFile{SchemaVersion: 2, Profiles: profiles}
	data, err := toml.Marshal(f)
	if err != nil {
		t.Fatal(err)
	}

	var f2 PortableFile
	if err := toml.Unmarshal(data, &f2); err != nil {
		t.Fatalf("unmarshal failed: %v\ncontent:\n%s", err, string(data))
	}
	if len(f2.Profiles) != 1 {
		t.Fatalf("len(Profiles) = %d", len(f2.Profiles))
	}
	if len(f2.Profiles[0].Env) != 2 {
		t.Fatalf("len(Env) = %d, want 2", len(f2.Profiles[0].Env))
	}
}

func TestAllToPortableGrouped(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	llmlDir := testLlmlDir(t, dir)
	paramsPath := filepath.Join(llmlDir, "model-params.json")

	// Write a model-params.json with two models, each with multiple profiles.
	data := []byte(`{
  "version": 2,
  "models": {
    "/models/Mistral-7B.gguf": {
      "profiles": [
        {"name": "default", "backend": "llama", "env": [], "args": ["--ctx-size", "4096"]},
        {"name": "koboldcpp", "backend": "koboldcpp", "env": [], "args": ["--usecublas"]}
      ],
      "activeIndex": 0
    },
    "/models/Aria-2.gguf": {
      "profiles": [
        {"name": "default", "backend": "llama", "env": [], "args": ["--n-gpu-layers", "80"]}
      ],
      "activeIndex": 0
    }
  }
}`)
	if err := os.WriteFile(paramsPath, data, 0o644); err != nil {
		t.Fatal(err)
	}

	groups, err := AllToPortableGrouped()
	if err != nil {
		t.Fatal(err)
	}

	// Should be sorted alphabetically by ModelHint: Aria-2 before Mistral-7B.
	if len(groups) != 2 {
		t.Fatalf("len(groups) = %d, want 2", len(groups))
	}
	if groups[0].ModelKey != "/models/Aria-2.gguf" {
		t.Errorf("group 0 key = %q, want /models/Aria-2.gguf", groups[0].ModelKey)
	}
	if groups[1].ModelKey != "/models/Mistral-7B.gguf" {
		t.Errorf("group 1 key = %q, want /models/Mistral-7B.gguf", groups[1].ModelKey)
	}
	if len(groups[1].Profiles) != 2 {
		t.Fatalf("len(groups[1].Profiles) = %d, want 2", len(groups[1].Profiles))
	}
}

func TestAllToPortableGrouped_GroupKeysPreserved(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	llmlDir := testLlmlDir(t, dir)
	paramsPath := filepath.Join(llmlDir, "model-params.json")

	data := []byte(`{
  "version": 2,
  "models": {
    "ollama://llama3.2": {
      "profiles": [
        {"name": "default", "backend": "ollama", "env": [{"key": "OLLAMA_NUM_GPU", "value": "1"}], "args": []}
      ],
      "activeIndex": 0
    }
  }
}`)
	if err := os.WriteFile(paramsPath, data, 0o644); err != nil {
		t.Fatal(err)
	}

	groups, err := AllToPortableGrouped()
	if err != nil {
		t.Fatal(err)
	}
	if len(groups) != 1 {
		t.Fatalf("len(groups) = %d, want 1", len(groups))
	}
	if groups[0].ModelKey != "ollama://llama3.2" {
		t.Errorf("ModelKey = %q, want ollama://llama3.2", groups[0].ModelKey)
	}
}

func TestAllToPortableGrouped_ProfilesConverted(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	llmlDir := testLlmlDir(t, dir)
	paramsPath := filepath.Join(llmlDir, "model-params.json")

	// Verify that profiles within a group are properly converted to portable form.
	// Use version 3 which supports the backend field on profiles.
	data := []byte(`{
  "version": 3,
  "models": {
    "/models/Test.gguf": {
      "profiles": [
        {"name": "default", "backend": "vllm", "env": [{"key": "FOO", "value": "bar"}], "args": ["--n-gpu-layers", "80"]}
      ],
      "activeIndex": 0
    }
  }
}`)
	if err := os.WriteFile(paramsPath, data, 0o644); err != nil {
		t.Fatal(err)
	}

	groups, err := AllToPortableGrouped()
	if err != nil {
		t.Fatal(err)
	}
	if len(groups) != 1 {
		t.Fatalf("len(groups) = %d, want 1", len(groups))
	}
	if len(groups[0].Profiles) != 1 {
		t.Fatalf("len(profiles) = %d, want 1", len(groups[0].Profiles))
	}
	pp := groups[0].Profiles[0]
	if pp.Name != "default" {
		t.Errorf("Name = %q, want default", pp.Name)
	}
	if pp.Backend != "vllm" {
		t.Errorf("Backend = %q, want vllm", pp.Backend)
	}
	if len(pp.Args) != 1 || pp.Args[0] != "--n-gpu-layers 80" {
		t.Errorf("Args = %v, want [--n-gpu-layers 80]", pp.Args)
	}
	if len(pp.Env) != 1 || pp.Env[0].Key != "FOO" {
		t.Errorf("Env = %v", pp.Env)
	}
}

func TestAllToPortable_SkipParseEntryError(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	llmlDir := testLlmlDir(t, dir)
	paramsPath := filepath.Join(llmlDir, "model-params.json")

	// Version 2 model with one valid entry and one that will fail ParseEntry (null).
	data := []byte(`{
	  "version": 2,
	  "models": {
	    "/models/Valid.gguf": {
	      "profiles": [{"name": "default", "env": [], "args": ["--ctx-size", "4096"]}],
	      "activeIndex": 0
	    },
	    "/models/Invalid.gguf": [1, 2, 3]
	  }
	}`)
	if err := os.WriteFile(paramsPath, data, 0o644); err != nil {
		t.Fatal(err)
	}

	profiles, err := AllToPortable()
	if err != nil {
		t.Fatal(err)
	}
	// Should skip the invalid entry and only return the valid one.
	if len(profiles) != 1 {
		t.Fatalf("len(profiles) = %d, want 1 (invalid entry skipped)", len(profiles))
	}
	if profiles[0].ModelHint != "Valid" {
		t.Errorf("ModelHint = %q, want Valid", profiles[0].ModelHint)
	}
}

func TestAllToPortableGrouped_SkipParseEntryError(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	llmlDir := testLlmlDir(t, dir)
	paramsPath := filepath.Join(llmlDir, "model-params.json")

	data := []byte(`{
	  "version": 2,
	  "models": {
	    "/models/Valid.gguf": {
	      "profiles": [{"name": "default", "env": [], "args": ["--ctx-size", "4096"]}],
	      "activeIndex": 0
	    },
	    "/models/Invalid.gguf": [1, 2, 3]
	  }
	}`)
	if err := os.WriteFile(paramsPath, data, 0o644); err != nil {
		t.Fatal(err)
	}

	groups, err := AllToPortableGrouped()
	if err != nil {
		t.Fatal(err)
	}
	if len(groups) != 1 {
		t.Fatalf("len(groups) = %d, want 1 (invalid entry skipped)", len(groups))
	}
}

func TestNextAvailablePath_FirstExists(t *testing.T) {
	dir := t.TempDir()
	dest := filepath.Join(dir, "export.toml")

	// Create first file.
	if err := os.WriteFile(dest, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	path, existed, err := NextAvailablePath(dest)
	if err != nil {
		t.Fatal(err)
	}
	if !existed {
		t.Error("expected existed=true")
	}
	if path != filepath.Join(dir, "export-2.toml") {
		t.Errorf("path = %q, want export-2.toml", path)
	}
}

func intPtr(v int) *int { return &v }
