package profiles

import (
	"fmt"
	"os"
	"strings"

	"github.com/BurntSushi/toml"
)

// stripEnvByBackend maps backend → uppercase env key → true for model-location env
// vars that must be stripped on import (llml supplies the model path at launch).
var stripEnvByBackend = map[string]map[string]bool{
	"llama": {
		"LLAMA_CACHE":             true,
		"LLAMA_ARG_MODEL":         true,
		"LLAMA_ARG_MODEL_URL":     true,
		"LLAMA_ARG_MODEL_DRAFT":   true,
		"LLAMA_ARG_HF_REPO":       true,
		"LLAMA_ARG_HF_FILE":       true,
		"LLAMA_ARG_HFD_REPO":      true,
		"LLAMA_ARG_HF_REPO_V":     true,
		"LLAMA_ARG_HF_FILE_V":     true,
		"LLAMA_ARG_DOCKER_REPO":   true,
		"LLAMA_ARG_MMPROJ":        true,
		"LLAMA_ARG_MMPROJ_URL":    true,
		"LLAMA_ARG_MODELS_DIR":    true,
		"LLAMA_ARG_MODELS_PRESET": true,
		"HF_TOKEN":                true,
	},
	"koboldcpp": {
		"LLAMA_CACHE":             true,
		"LLAMA_ARG_MODEL":         true,
		"LLAMA_ARG_MODEL_URL":     true,
		"LLAMA_ARG_MODEL_DRAFT":   true,
		"LLAMA_ARG_HF_REPO":       true,
		"LLAMA_ARG_HF_FILE":       true,
		"LLAMA_ARG_HFD_REPO":      true,
		"LLAMA_ARG_HF_REPO_V":     true,
		"LLAMA_ARG_HF_FILE_V":     true,
		"LLAMA_ARG_DOCKER_REPO":   true,
		"LLAMA_ARG_MMPROJ":        true,
		"LLAMA_ARG_MMPROJ_URL":    true,
		"LLAMA_ARG_MODELS_DIR":    true,
		"LLAMA_ARG_MODELS_PRESET": true,
		"HF_TOKEN":                true,
	},
	"vllm": {
		"HF_HOME":                  true,
		"HF_TOKEN":                 true,
		"HF_HUB_TOKEN":             true,
		"HUGGINGFACE_HUB_CACHE":    true,
		"HUGGING_FACE_HUB_TOKEN":   true,
		"TRANSFORMERS_CACHE":       true,
		"VLLM_CACHE_ROOT":          true,
		"VLLM_ASSETS_CACHE":        true,
		"VLLM_MODEL_REDIRECT_PATH": true,
		"VLLM_XLA_CACHE_PATH":      true,
		"VLLM_USE_MODELSCOPE":      true,
		"MODELSCOPE_CACHE":         true,
	},
}

// stripArgByBackend maps backend → arg first token → true for model-location args
// that must be stripped on import.
var stripArgByBackend = map[string]map[string]bool{
	"llama": {
		"-m": true, "--model": true,
		"-mu": true, "--model-url": true,
		"-md": true, "--model-draft": true,
		"-mv": true, "--model-vocoder": true,
		"-hf": true, "-hfr": true, "--hf-repo": true,
		"-hff": true, "--hf-file": true,
		"-hfd": true, "-hfrd": true, "--hf-repo-draft": true,
		"-hfv": true, "-hfrv": true, "--hf-repo-v": true,
		"-hffv": true, "--hf-file-v": true,
		"-hft": true, "--hf-token": true,
		"-dr": true, "--docker-repo": true,
		"-mm": true, "--mmproj": true,
		"-mmu": true, "--mmproj-url": true,
		"--lora": true, "--lora-scaled": true, "--lora-init-without-apply": true,
		"--control-vector": true, "--control-vector-scaled": true,
		"--models-dir": true, "--models-preset": true,
		"-lcs": true, "--lookup-cache-static": true,
		"-lcd": true, "--lookup-cache-dynamic": true,
	},
	"koboldcpp": {
		"-m": true, "--model": true,
		"-mu": true, "--model-url": true,
		"-md": true, "--model-draft": true,
		"-mv": true, "--model-vocoder": true,
		"-hf": true, "-hfr": true, "--hf-repo": true,
		"-hff": true, "--hf-file": true,
		"-hfd": true, "-hfrd": true, "--hf-repo-draft": true,
		"-hfv": true, "-hfrv": true, "--hf-repo-v": true,
		"-hffv": true, "--hf-file-v": true,
		"-hft": true, "--hf-token": true,
		"-dr": true, "--docker-repo": true,
		"-mm": true, "--mmproj": true,
		"-mmu": true, "--mmproj-url": true,
		"--lora": true, "--lora-scaled": true, "--lora-init-without-apply": true,
		"--control-vector": true, "--control-vector-scaled": true,
		"--models-dir": true, "--models-preset": true,
		"-lcs": true, "--lookup-cache-static": true,
		"-lcd": true, "--lookup-cache-dynamic": true,
	},
	"vllm": {
		"--model": true, "--tokenizer": true,
		"--revision": true, "--code-revision": true, "--tokenizer-revision": true,
		"--hf-config-path": true, "--hf-token": true, "--hf-overrides": true,
		"--download-dir": true, "--load-format": true,
		"--model-loader-extra-config": true, "--config": true,
		"--qlora-adapter-name-or-path": true,
		"--lora-modules":               true, "--prompt-adapters": true,
		"--speculative-config": true, "--speculative-model": true,
		"--tokenizer-pool-extra-config": true,
	},
}

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
	envTable := stripEnvByBackend[backend]
	argTable := stripArgByBackend[backend]

	var keptEnv []PortableEnvVar
	var droppedEnv []string
	for _, e := range env {
		if envTable != nil && envTable[strings.ToUpper(e.Key)] {
			droppedEnv = append(droppedEnv, e.Key+"="+e.Value)
		} else {
			keptEnv = append(keptEnv, e)
		}
	}

	var keptArgs []string
	var droppedArgs []string
	for _, a := range args {
		if argTable != nil && argTable[argFirstToken(a)] {
			droppedArgs = append(droppedArgs, a)
		} else {
			keptArgs = append(keptArgs, a)
		}
	}

	return keptEnv, keptArgs, droppedEnv, droppedArgs
}

// ReadPortable reads and validates a portable profile TOML file.
func ReadPortable(path string) (*PortableFile, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var f PortableFile
	if err := toml.Unmarshal(b, &f); err != nil {
		return nil, fmt.Errorf("invalid TOML: %w", err)
	}
	if f.SchemaVersion != SchemaVersion {
		return nil, fmt.Errorf("unsupported schema_version %d (expected %d)", f.SchemaVersion, SchemaVersion)
	}
	return &f, nil
}

// PortableToProfile converts one portable profile to an internal Profile,
// applying all normalizers.
func PortableToProfile(pp PortableProfile) Profile {
	backend := NormalizeBackendInput(pp.Backend)
	if backend == "" {
		backend = "llama"
	}
	useCase := NormalizeUseCase(UseCaseMetadata{
		Primary: NormalizeUseCasePrimaryInput(pp.UseCase.Primary),
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
		env[i] = EnvVar{Key: e.Key, Value: e.Value}
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
		return nil, fmt.Errorf("target model key is required")
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
		p.UseCase.Primary == "" &&
		len(p.UseCase.Tags) == 0 &&
		p.Hardware.Class == ""
}
