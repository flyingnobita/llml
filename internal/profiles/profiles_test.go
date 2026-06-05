package profiles

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func TestSaveLoadEntryRoundTrip_v3Metadata(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv("HOME", dir)
	t.Setenv("AppData", dir)

	gpuCount := 1
	minVRAM := 24
	maxVRAM := 24
	ent := Entry{
		Profiles: []Profile{
			{
				Name:    "single-gpu-general",
				Backend: "vllm",
				UseCase: UseCaseMetadata{Primary: UseCasePrimaries{UseCaseGeneral}, Tags: []string{"interactive", "balanced"}},
				Hardware: HardwareMetadata{
					Class:     HardwareClassGPU,
					GPUCount:  &gpuCount,
					MinVRAMGB: &minVRAM,
					MaxVRAMGB: &maxVRAM,
					Notes:     "tested on 4090",
				},
				Env:  []EnvVar{{Key: "CUDA_VISIBLE_DEVICES", Value: "0"}},
				Args: []string{"--max-model-len", "8192"},
			},
		},
		ActiveIndex: 0,
	}
	modelPath := filepath.Join(dir, "m.gguf")
	if err := SaveEntry(modelPath, ent); err != nil {
		t.Fatal(err)
	}

	got, err := LoadEntry(modelPath)
	if err != nil {
		t.Fatal(err)
	}
	if got.Profiles[0].Backend != "vllm" {
		t.Fatalf("backend = %q", got.Profiles[0].Backend)
	}
	if !slices.Contains(got.Profiles[0].UseCase.Primary, UseCaseGeneral) {
		t.Fatalf("use case = %v", got.Profiles[0].UseCase.Primary)
	}
	if got.Profiles[0].Hardware.Class != HardwareClassGPU {
		t.Fatalf("hardware class = %q", got.Profiles[0].Hardware.Class)
	}
	if got.Profiles[0].Hardware.GPUCount == nil || *got.Profiles[0].Hardware.GPUCount != 1 {
		t.Fatalf("gpu count = %#v", got.Profiles[0].Hardware.GPUCount)
	}
}

func TestParseEntryV1MigratesToDefaultProfile(t *testing.T) {
	raw := json.RawMessage(`{"env":[{"key":"K","value":"V"}],"args":["--x"]}`)
	got, err := ParseEntry(raw, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Profiles) != 1 || got.Profiles[0].Name != "default" {
		t.Fatalf("%+v", got.Profiles)
	}
	if got.Profiles[0].Backend != "" {
		t.Fatalf("unexpected backend %q", got.Profiles[0].Backend)
	}
}

func TestParseEntryV2MigratesConservatively(t *testing.T) {
	raw := json.RawMessage(`{"profiles":[{"name":"cuda","env":[{"key":"CUDA_VISIBLE_DEVICES","value":"0"}],"args":["--ctx-size","8192"]}],"activeIndex":0}`)
	got, err := ParseEntry(raw, 2)
	if err != nil {
		t.Fatal(err)
	}
	if got.Profiles[0].Backend != "" {
		t.Fatalf("backend should remain unspecified, got %q", got.Profiles[0].Backend)
	}
	if len(got.Profiles[0].UseCase.Primary) != 0 {
		t.Fatalf("use case should be empty, got %v", got.Profiles[0].UseCase.Primary)
	}
	if got.Profiles[0].Hardware.Class != HardwareClassUnspecified {
		t.Fatalf("hardware class = %q", got.Profiles[0].Hardware.Class)
	}
}

func TestParseEntryV3EmptyProfilesGetsDefault(t *testing.T) {
	raw := json.RawMessage(`{"profiles":[],"activeIndex":0}`)
	got, err := ParseEntry(raw, 3)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Profiles) != 1 || got.Profiles[0].Name != "default" {
		t.Fatalf("%+v", got.Profiles)
	}
}

func TestParseEntryUnknownVersionFails(t *testing.T) {
	if _, err := ParseEntry(json.RawMessage(`{}`), 99); err == nil {
		t.Fatal("expected error for unknown version")
	}
}

func TestNormalizeUseCaseAndHardware(t *testing.T) {
	gpuCount := -2
	minVRAM := 48
	maxVRAM := 24
	p := NormalizeProfile(Profile{
		Name:    "x",
		Backend: "VLLM",
		UseCase: UseCaseMetadata{Primary: UseCasePrimaries{"assistant"}, Tags: []string{" Interactive ", "interactive", "LOW_LATENCY"}},
		Hardware: HardwareMetadata{
			Class:     "GPU",
			GPUCount:  &gpuCount,
			MinVRAMGB: &minVRAM,
			MaxVRAMGB: &maxVRAM,
			Notes:     "  tested on 4090  ",
		},
	})
	if p.Backend != "vllm" {
		t.Fatalf("backend = %q", p.Backend)
	}
	if !slices.Contains(p.UseCase.Primary, UseCaseGeneral) {
		t.Fatalf("use case = %v", p.UseCase.Primary)
	}
	if len(p.UseCase.Tags) != 2 || p.UseCase.Tags[0] != "interactive" || p.UseCase.Tags[1] != "low-latency" {
		t.Fatalf("tags = %#v", p.UseCase.Tags)
	}
	if p.Hardware.Class != HardwareClassGPU {
		t.Fatalf("hardware class = %q", p.Hardware.Class)
	}
	if p.Hardware.GPUCount != nil {
		t.Fatalf("gpu count should normalize away, got %#v", p.Hardware.GPUCount)
	}
	if p.Hardware.MinVRAMGB == nil || *p.Hardware.MinVRAMGB != 24 {
		t.Fatalf("min vram = %#v", p.Hardware.MinVRAMGB)
	}
	if p.Hardware.MaxVRAMGB == nil || *p.Hardware.MaxVRAMGB != 48 {
		t.Fatalf("max vram = %#v", p.Hardware.MaxVRAMGB)
	}
}

func TestNormalizeMetadataInputsFromTUIStrings(t *testing.T) {
	gpuCount := ParseOptionalPositiveInt("2")
	minVRAM := ParseOptionalPositiveInt("48")
	maxVRAM := ParseOptionalPositiveInt("24")
	p := NormalizeProfile(Profile{
		Name:    "x",
		Backend: NormalizeBackendInput(" llama.cpp "),
		UseCase: UseCaseMetadata{
			Primary: NormalizeUseCasePrimariesInput([]string{"tool_calling"}),
			Tags:    NormalizeTagsCSV(" Interactive, low_latency, interactive "),
		},
		Hardware: HardwareMetadata{
			Class:     NormalizeHardwareClassInput("GPU"),
			GPUCount:  gpuCount,
			MinVRAMGB: minVRAM,
			MaxVRAMGB: maxVRAM,
			Notes:     "  tested on 4090  ",
		},
	})
	if p.Backend != "llama" {
		t.Fatalf("backend = %q", p.Backend)
	}
	if len(p.UseCase.Primary) != 0 {
		t.Fatalf("use case = %v", p.UseCase.Primary)
	}
	if len(p.UseCase.Tags) != 2 || p.UseCase.Tags[0] != "interactive" || p.UseCase.Tags[1] != "low-latency" {
		t.Fatalf("tags = %#v", p.UseCase.Tags)
	}
	if p.Hardware.MinVRAMGB == nil || *p.Hardware.MinVRAMGB != 24 {
		t.Fatalf("min vram = %#v", p.Hardware.MinVRAMGB)
	}
	if p.Hardware.MaxVRAMGB == nil || *p.Hardware.MaxVRAMGB != 48 {
		t.Fatalf("max vram = %#v", p.Hardware.MaxVRAMGB)
	}
}

func TestNormalizeBackendKoboldCpp(t *testing.T) {
	got := normalizeBackend("koboldcpp")
	if got != "koboldcpp" {
		t.Fatalf("got %q want koboldcpp", got)
	}
	got2 := normalizeBackend("KOBOLDCPP")
	if got2 != "koboldcpp" {
		t.Fatalf("case-insensitive: got %q", got2)
	}
	got3 := normalizeBackend("")
	if got3 != "" {
		t.Fatalf("empty: got %q", got3)
	}
	got4 := normalizeBackend("unknown")
	if got4 != "" {
		t.Fatalf("unknown: got %q", got4)
	}
}

func TestParseOptionalPositiveIntAndUnknownMetadataFallback(t *testing.T) {
	if got := ParseOptionalPositiveInt(""); got != nil {
		t.Fatalf("blank parse = %#v", got)
	}
	if got := ParseOptionalPositiveInt("nope"); got != nil {
		t.Fatalf("invalid parse = %#v", got)
	}
	p := NormalizeProfile(Profile{
		Name:    "x",
		Backend: NormalizeBackendInput("mystery"),
		UseCase: UseCaseMetadata{
			Primary: NormalizeUseCasePrimariesInput([]string{"mystery"}),
			Tags:    NormalizeTagsCSV(" "),
		},
		Hardware: HardwareMetadata{
			Class:    NormalizeHardwareClassInput("mystery"),
			GPUCount: ParseOptionalPositiveInt("-3"),
		},
	})
	if p.Backend != "" {
		t.Fatalf("backend = %q", p.Backend)
	}
	if len(p.UseCase.Primary) != 0 {
		t.Fatalf("use case should be empty, got %v", p.UseCase.Primary)
	}
	if p.Hardware.Class != HardwareClassUnspecified {
		t.Fatalf("hardware class = %q", p.Hardware.Class)
	}
	if p.Hardware.GPUCount != nil {
		t.Fatalf("gpu count = %#v", p.Hardware.GPUCount)
	}
}

func TestCopyProfileDeepCopiesNestedMetadata(t *testing.T) {
	gpuCount := 1
	minVRAM := 24
	in := Profile{
		Name:    "x",
		Backend: "vllm",
		UseCase: UseCaseMetadata{Primary: UseCasePrimaries{UseCaseGeneral}, Tags: []string{"interactive"}},
		Hardware: HardwareMetadata{
			Class:     HardwareClassGPU,
			GPUCount:  &gpuCount,
			MinVRAMGB: &minVRAM,
		},
		Env:  []EnvVar{{Key: "CUDA_VISIBLE_DEVICES", Value: "0"}},
		Args: []string{"--max-model-len", "8192"},
	}
	out := CopyProfile(in)
	out.UseCase.Tags[0] = "changed"
	*out.Hardware.GPUCount = 8
	out.Env[0].Value = "1"
	if in.UseCase.Tags[0] != "interactive" {
		t.Fatalf("input tags mutated: %#v", in.UseCase.Tags)
	}
	if *in.Hardware.GPUCount != 1 {
		t.Fatalf("input gpu count mutated: %d", *in.Hardware.GPUCount)
	}
	if in.Env[0].Value != "0" {
		t.Fatalf("input env mutated: %#v", in.Env)
	}
}

func TestLoadParamsForRunIgnoresMetadata(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv("HOME", dir)
	t.Setenv("AppData", dir)

	if err := SaveEntry(filepath.Join(dir, "m.gguf"), Entry{
		Profiles: []Profile{
			{
				Name:     "x",
				Backend:  "vllm",
				UseCase:  UseCaseMetadata{Primary: UseCasePrimaries{UseCaseGeneral}},
				Hardware: HardwareMetadata{Class: HardwareClassGPU},
				Env:      []EnvVar{{Key: "CUDA_VISIBLE_DEVICES", Value: "0"}},
				Args:     []string{"--max-model-len", "8192"},
			},
		},
		ActiveIndex: 0,
	}); err != nil {
		t.Fatal(err)
	}
	got, err := LoadParamsForRun(filepath.Join(dir, "m.gguf"))
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Env) != 1 || len(got.Args) != 2 {
		t.Fatalf("got %+v", got)
	}
}

func TestSaveEntryWritesV3File(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv("HOME", dir)
	t.Setenv("AppData", dir)
	modelPath := filepath.Join(dir, "m.gguf")
	if err := SaveEntry(modelPath, Entry{Profiles: []Profile{{Name: "x"}}, ActiveIndex: 0}); err != nil {
		t.Fatal(err)
	}
	path, err := ConfigPath()
	if err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var f file
	if err := json.Unmarshal(b, &f); err != nil {
		t.Fatal(err)
	}
	if f.Version != FileVersion {
		t.Fatalf("version = %d", f.Version)
	}
}
