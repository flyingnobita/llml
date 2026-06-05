package tui

import (
	"encoding/json"
	"strings"

	"github.com/flyingnobita/llml/internal/profiles"
)

const modelParamsFileVersion = profiles.FileVersion

type EnvVar = profiles.EnvVar
type ModelParams = profiles.ModelParams
type ParameterProfile = profiles.Profile
type modelEntry = profiles.Entry

func clampInt(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func modelParamsConfigPath() (string, error) { return profiles.ConfigPath() }

func parseModelEntry(raw json.RawMessage) (modelEntry, error) {
	return profiles.ParseEntry(raw, modelParamsFileVersion)
}

func loadModelEntry(modelPath string) (modelEntry, error) {
	return profiles.LoadEntry(modelPath)
}

func saveModelEntry(modelPath string, ent modelEntry) error {
	return profiles.SaveEntry(modelPath, ent)
}

func modelParamsKey(modelPath string) string { return profiles.ModelParamsKey(modelPath) }

// activeProfileNameForPreview returns the active profile name for the selected model: the
// in-memory name when the params panel is open for that model, otherwise from disk.
func activeProfileNameForPreview(m Model) string {
	sel, _ := m.SelectedModel()
	if sel == "" {
		return ""
	}
	if m.params.open {
		if modelParamsKey(m.params.modelPath) == modelParamsKey(sel) {
			if m.params.profileIndex >= 0 && m.params.profileIndex < len(m.params.profiles) {
				return m.params.profiles[m.params.profileIndex].Name
			}
		}
	}
	ent, err := loadModelEntry(modelParamsKey(sel))
	if err != nil || len(ent.Profiles) == 0 {
		return ""
	}
	idx := clampInt(ent.ActiveIndex, 0, len(ent.Profiles)-1)
	return ent.Profiles[idx].Name
}

// modelParamsForLaunchPreview returns env/argv for the launch preview line: the active profile
// from disk, or the in-memory parameters panel state when it is open for the selected model path.
func modelParamsForLaunchPreview(m Model) (ModelParams, bool) {
	sel, _ := m.SelectedModel()
	if sel == "" {
		return ModelParams{}, false
	}
	if m.params.open {
		if modelParamsKey(m.params.modelPath) == modelParamsKey(sel) {
			var uc profiles.UseCaseMetadata
			if m.params.profileIndex >= 0 && m.params.profileIndex < len(m.params.profiles) {
				uc = m.params.profiles[m.params.profileIndex].UseCase
			}
			return normalizeModelParams(ModelParams{
				Env:     append([]EnvVar(nil), m.params.env...),
				Args:    flattenArgLines(m.params.args),
				UseCase: uc,
			}), true
		}
	}
	p, err := loadModelParamsForRun(sel)
	if err != nil {
		return ModelParams{}, false
	}
	return p, true
}

// loadModelParamsForRun returns the active parameter profile's env/args for modelPath (for R / server launch).
func loadModelParamsForRun(modelPath string) (ModelParams, error) {
	return profiles.LoadParamsForRun(modelPath)
}

// mergeEnv overlays extra on base: keys present in extra replace any existing assignment.
func mergeEnv(base []string, extra []EnvVar) []string {
	drop := make(map[string]struct{})
	for _, e := range extra {
		if e.Key != "" {
			drop[e.Key] = struct{}{}
		}
	}
	var out []string
	for _, line := range base {
		k := line
		if i := strings.IndexByte(line, '='); i >= 0 {
			k = line[:i]
		}
		if _, ok := drop[k]; ok {
			continue
		}
		out = append(out, line)
	}
	for _, e := range extra {
		if e.Key != "" {
			out = append(out, e.Key+"="+e.Value)
		}
	}
	return out
}

// flattenArgLines expands panel rows (each may be one token or "--flag value") to argv tokens.
func flattenArgLines(lines []string) []string { return profiles.FlattenArgLines(lines) }

// normalizeModelParams trims keys and args for storage.
func normalizeModelParams(p ModelParams) ModelParams { return profiles.NormalizeModelParams(p) }

// expandArgLine maps one row from the parameter panel to argv tokens.
func expandArgLine(line string) []string { return profiles.ExpandArgLine(line) }
