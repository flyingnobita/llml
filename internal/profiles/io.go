package profiles

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/flyingnobita/llml/internal/fsutil"
	"github.com/flyingnobita/llml/internal/userdata"
)

// ConfigPath returns the path to model-params.json.
func ConfigPath() (string, error) {
	return userdata.ModelParamsPath()
}

// ReadFile reads the model-params.json root document.
func ReadFile() (file, error) {
	path, err := ConfigPath()
	if err != nil {
		return file{}, err
	}
	return readFile(path)
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

func applyMigrationDefaults(ent Entry) Entry {
	for i := range ent.Profiles {
		ent.Profiles[i].Backend = normalizeBackend(ent.Profiles[i].Backend)
	}
	return ent
}
