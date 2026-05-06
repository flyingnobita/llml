package profiles

import (
	"encoding/json"
	"slices"
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

// ValidUseCasePrimary reports whether v is a supported non-empty primary use case.
func ValidUseCasePrimary(v UseCasePrimary) bool {
	return slices.Contains(validUseCasePrimary, v)
}

// ValidHardwareClass reports whether v is a supported non-empty hardware class.
func ValidHardwareClass(v HardwareClass) bool {
	return slices.Contains(validHardwareClass, v)
}
