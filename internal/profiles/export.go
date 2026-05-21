package profiles

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/BurntSushi/toml"

	"github.com/flyingnobita/llml/internal/fsutil"
)

// SchemaVersion is the current portable profile format version.
const SchemaVersion = 2

// envExcludePatterns are env var keys excluded from portable export
// (model-location parameters per docs/profile-format.md §8).
var envExcludePatterns = []string{
	"LLAMA_CACHE",
	"LLAMA_ARG_MODEL",
	"HF_HOME",
	"HUGGINGFACE_HUB_CACHE",
	"KOBOLDCPP_MODEL",
	"KOBOLDCPP_LORA",
	"KOBOLDCPP_MMPROJ",
	"KOBOLDCPP_TOKENIZER",
	"KOBOLDCPP_MODELS_DIR",
}

// envExcludePrefixes are env var key prefixes to exclude.
var envExcludePrefixes = []string{
	"LLAMA_ARG_",
}

// PortableProfile is one profile in the portable TOML format.
type PortableProfile struct {
	Name      string           `toml:"name"`
	Backend   string           `toml:"backend"`
	ModelHint string           `toml:"model_hint,omitempty"`
	Args      []string         `toml:"args,omitempty"`
	Env       []PortableEnvVar `toml:"env,omitempty"`
	UseCase   PortableUseCase  `toml:"use_case,omitempty"`
	Hardware  PortableHardware `toml:"hardware,omitempty"`
}

// PortableEnvVar is one env var in the portable TOML format.
type PortableEnvVar struct {
	Key   string `toml:"key"`
	Value string `toml:"value"`
}

// PortableUseCase maps to [profiles.use_case] in portable TOML.
type PortableUseCase struct {
	Primary string   `toml:"primary,omitempty"`
	Tags    []string `toml:"tags,omitempty"`
}

// PortableHardware maps to [profiles.hardware] in portable TOML.
type PortableHardware struct {
	Class     string `toml:"class,omitempty"`
	GPUCount  *int   `toml:"gpu_count,omitempty"`
	MinVRAMGB *int   `toml:"min_vram_gb,omitempty"`
	MaxVRAMGB *int   `toml:"max_vram_gb,omitempty"`
	Notes     string `toml:"notes,omitempty"`
}

// PortableFile is the top-level portable TOML document.
type PortableFile struct {
	SchemaVersion int               `toml:"schema_version"`
	Profiles      []PortableProfile `toml:"profiles"`
}

var numericValueRE = regexp.MustCompile(`^-?\d+(\.\d+)?$`)

// isValue reports whether tok is a flag value (not a flag name) for args recombination.
func isValue(tok string) bool {
	if !strings.HasPrefix(tok, "-") {
		return true
	}
	return numericValueRE.MatchString(tok)
}

// RecombineArgs converts internal split-token args to portable panel-row strings.
func RecombineArgs(args []string) []string {
	var out []string
	i := 0
	for i < len(args) {
		tok := args[i]
		switch {
		case strings.HasPrefix(tok, "-") && !strings.Contains(tok, " "):
			if i+1 < len(args) {
				next := args[i+1]
				if isValue(next) {
					out = append(out, tok+" "+next)
					i += 2
					continue
				}
			}
			out = append(out, tok)
			i++
		case strings.HasPrefix(tok, "-") && strings.Contains(tok, " "):
			out = append(out, tok)
			i++
		default:
			i++
		}
	}
	return out
}

// ModelHint derives a portable model_hint from a model storage key.
func ModelHint(key string) string {
	if after, ok := strings.CutPrefix(key, "ollama://"); ok {
		return after
	}
	// HF hub paths: .../models--{org}--{name}/snapshots/{hash}[/{file}]
	// Extract the human-readable model name from the models-- segment.
	for _, part := range strings.Split(filepath.ToSlash(key), "/") {
		if strings.HasPrefix(part, "models--") {
			if sub := strings.SplitN(part, "--", 3); len(sub) == 3 && sub[2] != "" {
				return sub[2]
			}
		}
	}
	base := filepath.Base(key)
	hint := strings.TrimSuffix(base, filepath.Ext(base))
	if hint == "" || hint == "." {
		return base
	}
	return hint
}

// ShouldExcludeEnv reports whether an env var key is a model-location
// parameter that should be excluded from portable export.
func ShouldExcludeEnv(key string) bool {
	upper := strings.ToUpper(key)
	if slices.Contains(envExcludePatterns, upper) {
		return true
	}
	for _, prefix := range envExcludePrefixes {
		if strings.HasPrefix(upper, prefix) {
			return true
		}
	}
	return false
}

// ProfileToPortable converts one internal profile to its portable representation.
func ProfileToPortable(p Profile, modelKey string) PortableProfile {
	backend := p.Backend
	if backend == "" {
		backend = "llama"
	}
	pp := PortableProfile{
		Name:      p.Name,
		Backend:   backend,
		ModelHint: ModelHint(modelKey),
		Args:      RecombineArgs(p.Args),
	}
	for _, e := range p.Env {
		if !ShouldExcludeEnv(e.Key) {
			pp.Env = append(pp.Env, PortableEnvVar(e))
		}
	}
	if p.UseCase.Primary != "" || len(p.UseCase.Tags) > 0 {
		pp.UseCase = PortableUseCase{
			Primary: string(p.UseCase.Primary),
			Tags:    append([]string(nil), p.UseCase.Tags...),
		}
	}
	if p.Hardware.Class != "" || p.Hardware.GPUCount != nil ||
		p.Hardware.MinVRAMGB != nil || p.Hardware.MaxVRAMGB != nil ||
		p.Hardware.Notes != "" {
		hw := PortableHardware{
			Class: string(p.Hardware.Class),
			Notes: p.Hardware.Notes,
		}
		if p.Hardware.GPUCount != nil {
			v := *p.Hardware.GPUCount
			hw.GPUCount = &v
		}
		if p.Hardware.MinVRAMGB != nil {
			v := *p.Hardware.MinVRAMGB
			hw.MinVRAMGB = &v
		}
		if p.Hardware.MaxVRAMGB != nil {
			v := *p.Hardware.MaxVRAMGB
			hw.MaxVRAMGB = &v
		}
		pp.Hardware = hw
	}
	return pp
}

// EntryToPortable converts all profiles in one model entry, skipping profiles
// that carry no args and no env vars (nothing portable to share).
func EntryToPortable(modelKey string, ent Entry) []PortableProfile {
	var out []PortableProfile
	for _, p := range ent.Profiles {
		pp := ProfileToPortable(p, modelKey)
		if len(pp.Args) == 0 && len(pp.Env) == 0 {
			continue
		}
		out = append(out, pp)
	}
	return out
}

// ModelGroup bundles a model key with its portable profiles, preserving
// the grouping for UIs that show profiles organized by model.
type ModelGroup struct {
	ModelKey string
	Profiles []PortableProfile
}

// AllToPortableGrouped reads model-params.json and returns profiles
// organized by model key. Groups are sorted alphabetically by ModelHint.
func AllToPortableGrouped() ([]ModelGroup, error) {
	f, err := ReadFile()
	if err != nil {
		return nil, err
	}
	var out []ModelGroup
	for key, raw := range f.Models {
		ent, err := ParseEntry(raw, f.Version)
		if err != nil {
			continue
		}
		pps := EntryToPortable(key, ent)
		if len(pps) > 0 {
			out = append(out, ModelGroup{ModelKey: key, Profiles: pps})
		}
	}
	sort.Slice(out, func(i, j int) bool {
		return ModelHint(out[i].ModelKey) < ModelHint(out[j].ModelKey)
	})
	return out, nil
}

// AllToPortable reads model-params.json and converts every profile.
func AllToPortable() ([]PortableProfile, error) {
	f, err := ReadFile()
	if err != nil {
		return nil, err
	}
	var out []PortableProfile
	for key, raw := range f.Models {
		ent, err := ParseEntry(raw, f.Version)
		if err != nil {
			continue
		}
		out = append(out, EntryToPortable(key, ent)...)
	}
	return out, nil
}

// DefaultExportFilename returns the date-stamped default filename.
func DefaultExportFilename() string {
	return "llml-profiles-" + time.Now().Format("20060102") + ".toml"
}

// NextAvailablePath returns a path that does not already exist on disk.
// If dest does not exist, it is returned as-is with existed=false.
// If dest exists, candidates "base-N.ext" are tried (N = 2, 3, … up to 999).
func NextAvailablePath(dest string) (path string, existed bool, err error) {
	if _, err := os.Stat(dest); os.IsNotExist(err) {
		return dest, false, nil
	}
	ext := filepath.Ext(dest)
	base := strings.TrimSuffix(dest, ext)
	for n := 2; n <= 999; n++ {
		cand := fmt.Sprintf("%s-%d%s", base, n, ext)
		if _, err := os.Stat(cand); os.IsNotExist(err) {
			return cand, true, nil
		}
	}
	return "", true, fmt.Errorf("too many existing files matching pattern; clean up or choose a different name")
}

// WritePortable writes profiles to a portable TOML file at dest.
// If force is false and the file exists, an error is returned.
func WritePortable(dest string, profiles []PortableProfile, force bool) error {
	if !force {
		if _, err := os.Stat(dest); err == nil {
			return fmt.Errorf("file exists: %s (use --force to overwrite)", dest)
		}
	}
	f := PortableFile{
		SchemaVersion: SchemaVersion,
		Profiles:      profiles,
	}
	data, err := toml.Marshal(f)
	if err != nil {
		return err
	}
	return fsutil.WriteFileAtomic(dest, data, 0o644)
}
