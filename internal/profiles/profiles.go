package profiles

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/flyingnobita/llml/internal/fsutil"
	"github.com/flyingnobita/llml/internal/userdata"
)

// FileVersion is the current on-disk format for model-params.json.
const FileVersion = 3

// EnvVar is one environment variable applied when launching the server for a model.
type EnvVar struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

// ModelParams holds extra environment variables and argv tokens for one parameter profile.
type ModelParams struct {
	Env  []EnvVar `json:"env"`
	Args []string `json:"args"`
}

// UseCasePrimary is the constrained primary purpose for a profile.
type UseCasePrimary string

const (
	UseCaseUnspecified UseCasePrimary = ""
	UseCaseChat        UseCasePrimary = "chat"
	UseCaseCompletion  UseCasePrimary = "completion"
	UseCaseToolCalling UseCasePrimary = "tool-calling"
	UseCaseEmbedding   UseCasePrimary = "embedding"
	UseCaseEval        UseCasePrimary = "eval"
	UseCaseBatch       UseCasePrimary = "batch"
)

var validUseCasePrimary = []UseCasePrimary{
	UseCaseChat,
	UseCaseCompletion,
	UseCaseToolCalling,
	UseCaseEmbedding,
	UseCaseEval,
	UseCaseBatch,
}

// UseCaseMetadata describes what a profile is for.
type UseCaseMetadata struct {
	Primary UseCasePrimary `json:"primary,omitempty"`
	Tags    []string       `json:"tags,omitempty"`
}

// HardwareClass is the coarse machine class a profile expects.
type HardwareClass string

const (
	HardwareClassUnspecified HardwareClass = ""
	HardwareClassCPU         HardwareClass = "cpu"
	HardwareClassGPU         HardwareClass = "gpu"
	HardwareClassMixed       HardwareClass = "mixed"
)

var validHardwareClass = []HardwareClass{
	HardwareClassCPU,
	HardwareClassGPU,
	HardwareClassMixed,
}

// HardwareMetadata describes the compatibility envelope for a profile.
type HardwareMetadata struct {
	Class     HardwareClass `json:"class,omitempty"`
	GPUCount  *int          `json:"gpuCount,omitempty"`
	MinVRAMGB *int          `json:"minVramGb,omitempty"`
	MaxVRAMGB *int          `json:"maxVramGb,omitempty"`
	Notes     string        `json:"notes,omitempty"`
}

// Profile is one named parameter profile plus structured metadata.
type Profile struct {
	Name     string           `json:"name"`
	Backend  string           `json:"backend,omitempty"`
	UseCase  UseCaseMetadata  `json:"useCase,omitempty"`
	Hardware HardwareMetadata `json:"hardware,omitempty"`
	Env      []EnvVar         `json:"env"`
	Args     []string         `json:"args"`
}

// Entry is stored per model path: several parameter profiles and which one to use when pressing R.
type Entry struct {
	Profiles    []Profile `json:"profiles"`
	ActiveIndex int       `json:"activeIndex"`
}

type modelParamsV1 struct {
	Env  []EnvVar `json:"env"`
	Args []string `json:"args"`
}

type entryV2 struct {
	Profiles    []profileV2 `json:"profiles"`
	ActiveIndex int         `json:"activeIndex"`
}

type profileV2 struct {
	Name string   `json:"name"`
	Env  []EnvVar `json:"env"`
	Args []string `json:"args"`
}

type file struct {
	Version int                        `json:"version"`
	Models  map[string]json.RawMessage `json:"models"`
}

// ConfigPath returns the path to model-params.json.
func ConfigPath() (string, error) {
	return userdata.ModelParamsPath()
}

// DefaultProfile returns the normalized fallback profile for empty or missing entries.
func DefaultProfile() Profile {
	return Profile{Name: "default", Env: nil, Args: nil}
}

// CopyProfile deep-copies one profile.
func CopyProfile(in Profile) Profile {
	out := in
	out.Env = append([]EnvVar(nil), in.Env...)
	out.Args = append([]string(nil), in.Args...)
	out.UseCase.Tags = append([]string(nil), in.UseCase.Tags...)
	if in.Hardware.GPUCount != nil {
		v := *in.Hardware.GPUCount
		out.Hardware.GPUCount = &v
	}
	if in.Hardware.MinVRAMGB != nil {
		v := *in.Hardware.MinVRAMGB
		out.Hardware.MinVRAMGB = &v
	}
	if in.Hardware.MaxVRAMGB != nil {
		v := *in.Hardware.MaxVRAMGB
		out.Hardware.MaxVRAMGB = &v
	}
	return out
}

// CopyProfiles deep-copies a profile slice.
func CopyProfiles(in []Profile) []Profile {
	out := make([]Profile, len(in))
	for i := range in {
		out[i] = CopyProfile(in[i])
	}
	return out
}

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

// ReadFile reads the model-params.json root document.
func ReadFile() (file, error) {
	path, err := ConfigPath()
	if err != nil {
		return file{}, err
	}
	return readFile(path)
}

// LoadEntry returns stored profiles for modelPath, or one empty default profile if none.
func LoadEntry(modelPath string) (Entry, error) {
	cfgPath, err := ConfigPath()
	if err != nil {
		return Entry{}, err
	}
	key := ModelParamsKey(modelPath)
	f, err := readFile(cfgPath)
	if err != nil {
		return Entry{}, err
	}
	raw, ok := f.Models[key]
	if !ok {
		return Entry{Profiles: []Profile{DefaultProfile()}, ActiveIndex: 0}, nil
	}
	return ParseEntry(raw, f.Version)
}

// SaveEntry writes the entry for modelPath and preserves other models in the file.
func SaveEntry(modelPath string, ent Entry) error {
	cfgPath, err := ConfigPath()
	if err != nil {
		return err
	}
	key := ModelParamsKey(modelPath)
	if err := os.MkdirAll(filepath.Dir(cfgPath), 0o755); err != nil {
		return err
	}
	f, err := readFile(cfgPath)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if f.Models == nil {
		f.Models = make(map[string]json.RawMessage)
	}
	f.Version = FileVersion
	ent = NormalizeEntry(ent)
	if len(ent.Profiles) == 0 {
		delete(f.Models, key)
	} else {
		raw, err := json.Marshal(ent)
		if err != nil {
			return err
		}
		f.Models[key] = raw
	}
	out, err := json.MarshalIndent(&f, "", "  ")
	if err != nil {
		return err
	}
	_ = userdata.BackupFileIfExists(cfgPath)
	return fsutil.WriteFileAtomic(cfgPath, out, 0o644)
}

// LoadParamsForRun returns the active profile's env/args for modelPath.
func LoadParamsForRun(modelPath string) (ModelParams, error) {
	ent, err := LoadEntry(modelPath)
	if err != nil {
		return ModelParams{}, err
	}
	if len(ent.Profiles) == 0 {
		return ModelParams{}, nil
	}
	idx := clampInt(ent.ActiveIndex, 0, len(ent.Profiles)-1)
	p := ent.Profiles[idx]
	return NormalizeModelParams(ModelParams{Env: p.Env, Args: p.Args}), nil
}

// ParseEntry decodes one model entry according to the file version.
func ParseEntry(raw json.RawMessage, version int) (Entry, error) {
	switch version {
	case 0, 1:
		var v1 modelParamsV1
		if err := json.Unmarshal(raw, &v1); err != nil {
			return Entry{}, err
		}
		return Entry{
			Profiles: []Profile{
				NormalizeProfile(Profile{Name: "default", Env: v1.Env, Args: v1.Args}),
			},
			ActiveIndex: 0,
		}, nil
	case 2:
		var v2 entryV2
		if err := json.Unmarshal(raw, &v2); err != nil {
			return Entry{}, err
		}
		ent := Entry{ActiveIndex: v2.ActiveIndex}
		for _, p := range v2.Profiles {
			ent.Profiles = append(ent.Profiles, Profile{
				Name: p.Name,
				Env:  p.Env,
				Args: p.Args,
			})
		}
		return applyMigrationDefaults(NormalizeEntry(ent)), nil
	case 3:
		var v3 Entry
		if err := json.Unmarshal(raw, &v3); err != nil {
			return Entry{}, err
		}
		return NormalizeEntry(v3), nil
	default:
		return Entry{}, fmt.Errorf("unsupported model params version %d", version)
	}
}

// ModelParamsKey canonicalizes the per-model storage key.
func ModelParamsKey(modelPath string) string {
	key := strings.TrimSpace(modelPath)
	if key == "" {
		return ""
	}
	if strings.HasPrefix(key, "ollama://") {
		return key
	}
	if strings.Contains(key, "://") {
		return key
	}
	return filepath.Clean(key)
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

func applyMigrationDefaults(ent Entry) Entry {
	for i := range ent.Profiles {
		ent.Profiles[i].Backend = normalizeBackend(ent.Profiles[i].Backend)
	}
	return ent
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

func readFile(path string) (file, error) {
	var f file
	b, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			f.Models = make(map[string]json.RawMessage)
			return f, nil
		}
		return f, err
	}
	if err := json.Unmarshal(b, &f); err != nil {
		return f, err
	}
	if f.Models == nil {
		f.Models = make(map[string]json.RawMessage)
	}
	return f, nil
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

// ValidUseCasePrimary reports whether v is a supported non-empty primary use case.
func ValidUseCasePrimary(v UseCasePrimary) bool {
	return slices.Contains(validUseCasePrimary, v)
}

// ValidHardwareClass reports whether v is a supported non-empty hardware class.
func ValidHardwareClass(v HardwareClass) bool {
	return slices.Contains(validHardwareClass, v)
}
