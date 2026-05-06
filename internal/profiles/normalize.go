package profiles

import (
	"fmt"
	"strconv"
	"strings"
)

// NormalizeProfile trims, validates, and normalizes one profile.
func NormalizeProfile(p Profile) Profile {
	nm := NormalizeModelParams(ModelParams{Env: p.Env, Args: p.Args})
	name := strings.TrimSpace(p.Name)
	if name == "" {
		name = "default"
	}
	backend := normalizeBackend(p.Backend)
	useCase := NormalizeUseCase(p.UseCase)
	hardware := NormalizeHardware(p.Hardware)
	return Profile{
		Name:     name,
		Backend:  backend,
		UseCase:  useCase,
		Hardware: hardware,
		Env:      nm.Env,
		Args:     nm.Args,
	}
}

// NormalizeEntry trims and canonicalizes one model entry.
func NormalizeEntry(ent Entry) Entry {
	var profiles []Profile
	for i := range ent.Profiles {
		p := NormalizeProfile(ent.Profiles[i])
		if p.Name == "default" && len(profiles) > 0 {
			p.Name = fmt.Sprintf("Parameter Profile %d", len(profiles)+1)
		}
		if p.Name == "" {
			p.Name = fmt.Sprintf("Parameter Profile %d", len(profiles)+1)
		}
		profiles = append(profiles, p)
	}
	if len(profiles) == 0 {
		return Entry{Profiles: []Profile{DefaultProfile()}, ActiveIndex: 0}
	}
	idx := clampInt(ent.ActiveIndex, 0, len(profiles)-1)
	return Entry{Profiles: profiles, ActiveIndex: idx}
}

// NormalizeModelParams trims keys and args for storage.
func NormalizeModelParams(p ModelParams) ModelParams {
	var env []EnvVar
	for _, e := range p.Env {
		k := strings.TrimSpace(e.Key)
		if k == "" {
			continue
		}
		env = append(env, EnvVar{Key: k, Value: e.Value})
	}
	var args []string
	for _, a := range p.Args {
		a = strings.TrimSpace(a)
		if a == "" {
			continue
		}
		args = append(args, ExpandArgLine(a)...)
	}
	return ModelParams{Env: env, Args: args}
}

// ExpandArgLine maps one panel row to argv tokens.
func ExpandArgLine(line string) []string {
	line = strings.TrimSpace(line)
	if line == "" {
		return nil
	}
	if !strings.HasPrefix(line, "-") || !strings.Contains(line, " ") {
		return []string{line}
	}
	i := strings.IndexByte(line, ' ')
	return []string{line[:i], strings.TrimSpace(line[i+1:])}
}

// FlattenArgLines expands panel rows to argv tokens.
func FlattenArgLines(lines []string) []string {
	var out []string
	for _, line := range lines {
		out = append(out, ExpandArgLine(line)...)
	}
	return out
}

// NormalizeUseCase trims and validates use-case metadata.
func NormalizeUseCase(uc UseCaseMetadata) UseCaseMetadata {
	primary := normalizeUseCasePrimary(uc.Primary)
	seen := map[string]struct{}{}
	var tags []string
	for _, tag := range uc.Tags {
		tag = normalizeTag(tag)
		if tag == "" {
			continue
		}
		if _, ok := seen[tag]; ok {
			continue
		}
		seen[tag] = struct{}{}
		tags = append(tags, tag)
	}
	return UseCaseMetadata{Primary: primary, Tags: tags}
}

// NormalizeBackendInput trims and canonicalizes a backend string from user input.
func NormalizeBackendInput(v string) string {
	return normalizeBackend(v)
}

// NormalizeUseCasePrimaryInput trims and canonicalizes a use-case primary from user input.
func NormalizeUseCasePrimaryInput(v string) UseCasePrimary {
	return normalizeUseCasePrimary(UseCasePrimary(v))
}

// NormalizeHardwareClassInput trims and canonicalizes a hardware class from user input.
func NormalizeHardwareClassInput(v string) HardwareClass {
	return normalizeHardwareClass(HardwareClass(v))
}

// NormalizeTagsCSV splits a comma-separated string and canonicalizes tags.
func NormalizeTagsCSV(v string) []string {
	parts := strings.FieldsFunc(v, func(r rune) bool {
		return r == ',' || r == '\n'
	})
	return NormalizeUseCase(UseCaseMetadata{Tags: parts}).Tags
}

// ParseOptionalPositiveInt returns a positive integer pointer, or nil for blank/invalid input.
func ParseOptionalPositiveInt(v string) *int {
	v = strings.TrimSpace(v)
	if v == "" {
		return nil
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return nil
	}
	return normalizePositiveInt(&n)
}

// NormalizeHardware trims and validates hardware metadata.
func NormalizeHardware(hw HardwareMetadata) HardwareMetadata {
	class := normalizeHardwareClass(hw.Class)
	gpuCount := normalizePositiveInt(hw.GPUCount)
	minVRAM := normalizePositiveInt(hw.MinVRAMGB)
	maxVRAM := normalizePositiveInt(hw.MaxVRAMGB)
	if minVRAM != nil && maxVRAM != nil && *minVRAM > *maxVRAM {
		minVRAM, maxVRAM = maxVRAM, minVRAM
	}
	notes := strings.TrimSpace(hw.Notes)
	if class == HardwareClassCPU {
		gpuCount = nil
		minVRAM = nil
		maxVRAM = nil
	}
	return HardwareMetadata{
		Class:     class,
		GPUCount:  gpuCount,
		MinVRAMGB: minVRAM,
		MaxVRAMGB: maxVRAM,
		Notes:     notes,
	}
}

// ProfileNameTaken reports whether name is already used, excluding skip index.
func ProfileNameTaken(profiles []Profile, name string, skip int) bool {
	n := strings.TrimSpace(name)
	for i, p := range profiles {
		if i == skip {
			continue
		}
		if strings.TrimSpace(p.Name) == n {
			return true
		}
	}
	return false
}

// NextProfileName returns the next generated unique profile name.
func NextProfileName(profiles []Profile) string {
	for n := 1; n < 1000; n++ {
		cand := "Parameter Profile"
		if n > 1 {
			cand = fmt.Sprintf("Parameter Profile %d", n)
		}
		if !ProfileNameTaken(profiles, cand, -1) {
			return cand
		}
	}
	return "Parameter Profile"
}

// CloneProfileName picks a unique profile name derived from base.
func CloneProfileName(base string, profiles []Profile) string {
	b := strings.TrimSpace(base)
	if b == "" {
		return NextProfileName(profiles)
	}
	cand := b + " copy"
	if !ProfileNameTaken(profiles, cand, -1) {
		return cand
	}
	for n := 2; n < 1000; n++ {
		cand = fmt.Sprintf("%s copy %d", b, n)
		if !ProfileNameTaken(profiles, cand, -1) {
			return cand
		}
	}
	return NextProfileName(profiles)
}

func normalizeBackend(v string) string {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "", "unknown":
		return ""
	case "llama", "llama.cpp":
		return "llama"
	case "vllm":
		return "vllm"
	case "ollama":
		return "ollama"
	case "koboldcpp":
		return "koboldcpp"
	default:
		return ""
	}
}

func normalizeUseCasePrimary(v UseCasePrimary) UseCasePrimary {
	s := strings.ToLower(strings.TrimSpace(string(v)))
	switch s {
	case "", "unknown", "unspecified":
		return UseCaseUnspecified
	case "chat", "assistant":
		return UseCaseChat
	case "completion", "generate", "generation":
		return UseCaseCompletion
	case "tool-calling", "tool_calling", "tools":
		return UseCaseToolCalling
	case "embedding", "embeddings":
		return UseCaseEmbedding
	case "eval", "evaluation":
		return UseCaseEval
	case "batch", "offline":
		return UseCaseBatch
	default:
		return UseCaseUnspecified
	}
}

func normalizeHardwareClass(v HardwareClass) HardwareClass {
	s := strings.ToLower(strings.TrimSpace(string(v)))
	switch s {
	case "", "unknown", "unspecified":
		return HardwareClassUnspecified
	case "cpu", "cpu-only":
		return HardwareClassCPU
	case "gpu":
		return HardwareClassGPU
	case "mixed", "hybrid":
		return HardwareClassMixed
	default:
		return HardwareClassUnspecified
	}
}

func normalizeTag(v string) string {
	v = strings.ToLower(strings.TrimSpace(v))
	v = strings.ReplaceAll(v, "_", "-")
	fields := strings.Fields(v)
	v = strings.Join(fields, "-")
	return v
}

func normalizePositiveInt(v *int) *int {
	if v == nil || *v <= 0 {
		return nil
	}
	out := *v
	return &out
}

func clampInt(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
