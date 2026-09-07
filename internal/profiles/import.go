package profiles

import (
	"errors"
	"fmt"
	"os"
	"strings"
)

// argFirstToken extracts the flag part (first token) from a panel-row arg string.
func argFirstToken(panelRow string) string {
	s := strings.TrimSpace(panelRow)
	if s == "" {
		return ""
	}
	i := strings.IndexByte(s, ' ')
	if i < 0 {
		return s
	}
	return s[:i]
}

// StripModelLocationParams removes model-location env vars and args for the given
// backend. Returns kept env, kept args, descriptions of dropped env, and descriptions
// of dropped args.
func StripModelLocationParams(backend string, env []PortableEnvVar, args []string) ([]PortableEnvVar, []string, []string, []string) {
	rules := rulesByBackend[backend]

	var keptEnv []PortableEnvVar
	var droppedEnv []string
	for _, e := range env {
		if rules.isEnv(e.Key) {
			droppedEnv = append(droppedEnv, e.Key+"="+e.Value)
		} else {
			keptEnv = append(keptEnv, e)
		}
	}

	var keptArgs []string
	var droppedArgs []string
	for _, a := range args {
		if rules.isArg(a) {
			droppedArgs = append(droppedArgs, a)
		} else {
			keptArgs = append(keptArgs, a)
		}
	}

	return keptEnv, keptArgs, droppedEnv, droppedArgs
}

// ReadPortable reads and validates a portable profile TOML file.
// It accepts schema_version 3 (current) and 2 (legacy: primary was a single string).
//
//nolint:gosec // G304: path from user input (CLI arg or filepicker) — user-intended file.
func ReadPortable(path string) (*PortableFile, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return parsePortable(b)
}

// PortableToProfile converts a portable profile to the internal representation,
// applying all normalizers.
func PortableToProfile(pp PortableProfile) Profile {
	backend := NormalizeBackendInput(pp.Backend)
	if backend == "" {
		backend = "llama"
	}
	useCase := NormalizeUseCase(UseCaseMetadata{
		Primary: NormalizeUseCasePrimariesInput(pp.UseCase.Primary),
		Tags:    NormalizeTagsCSV(strings.Join(pp.UseCase.Tags, ",")),
	})
	hardware := NormalizeHardware(HardwareMetadata{
		Class:     NormalizeHardwareClassInput(pp.Hardware.Class),
		GPUCount:  pp.Hardware.GPUCount,
		MinVRAMGB: pp.Hardware.MinVRAMGB,
		MaxVRAMGB: pp.Hardware.MaxVRAMGB,
		Notes:     pp.Hardware.Notes,
	})
	env := make([]EnvVar, len(pp.Env))
	for i, e := range pp.Env {
		env[i] = EnvVar(e)
	}
	return NormalizeProfile(Profile{
		Name:     pp.Name,
		Backend:  backend,
		UseCase:  useCase,
		Hardware: hardware,
		Env:      env,
		Args:     FlattenArgLines(pp.Args),
	})
}

// ImportResult summarizes the outcome of ImportProfiles.
type ImportResult struct {
	Added            int
	Replaced         int
	Skipped          int
	FilteredEnvDescs []string
	FilteredArgDescs []string
}

// ImportProfiles merges profiles into the model-params.json entry for targetKey.
// If force is false, profiles whose name already exists are skipped. If force is
// true, existing profiles with the same name are replaced in-place.
func ImportProfiles(targetKey string, profiles []Profile, force bool) (*ImportResult, error) {
	if targetKey == "" {
		return nil, errors.New("target model key is required")
	}
	ent, err := LoadEntry(targetKey)
	if err != nil {
		return nil, fmt.Errorf("loading entry for %s: %w", targetKey, err)
	}

	// If the only existing profile is an empty sentinel default, clear it so
	// imported profiles don't end up alongside a meaningless placeholder.
	if len(ent.Profiles) == 1 && isSentinelDefault(ent.Profiles[0]) {
		ent.Profiles = nil
	}

	result := &ImportResult{}
	existingNames := make(map[string]int)
	for i, p := range ent.Profiles {
		existingNames[p.Name] = i
	}

	for _, p := range profiles {
		if idx, exists := existingNames[p.Name]; exists {
			if force {
				ent.Profiles[idx] = p
				result.Replaced++
			} else {
				result.Skipped++
			}
		} else {
			ent.Profiles = append(ent.Profiles, p)
			existingNames[p.Name] = len(ent.Profiles) - 1
			result.Added++
		}
	}

	if result.Added == 0 && result.Replaced == 0 {
		return result, nil
	}

	if err := SaveEntry(targetKey, ent); err != nil {
		return nil, fmt.Errorf("saving entry for %s: %w", targetKey, err)
	}

	return result, nil
}

func isSentinelDefault(p Profile) bool {
	return p.Name == "default" &&
		len(p.Args) == 0 &&
		len(p.Env) == 0 &&
		p.Backend == "" &&
		len(p.UseCase.Primary) == 0 &&
		len(p.UseCase.Tags) == 0 &&
		p.Hardware.Class == ""
}
