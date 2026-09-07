package tui

import (
	"github.com/flyingnobita/llml/internal/profiles"
)

// activeProfileNameForPreview returns the active profile name for the selected model: the
// in-memory name when the params panel is open for that model, otherwise from disk.
func activeProfileNameForPreview(m Model) string {
	sel, _ := m.SelectedModel()
	if sel == "" {
		return ""
	}
	if m.params.open {
		if profiles.ModelParamsKey(m.params.modelPath) == profiles.ModelParamsKey(sel) {
			return m.params.editor.ActiveProfile().Name
		}
	}
	ent, err := profiles.LoadEntry(profiles.ModelParamsKey(sel))
	if err != nil || len(ent.Profiles) == 0 {
		return ""
	}
	idx := clampIndex(ent.ActiveIndex, len(ent.Profiles)-1)
	return ent.Profiles[idx].Name
}

// modelParamsForLaunchPreview returns env/argv for the launch preview line: the active profile
// from disk, or the in-memory parameters panel state when it is open for the selected model path.
func modelParamsForLaunchPreview(m Model) (profiles.ModelParams, bool) {
	sel, _ := m.SelectedModel()
	if sel == "" {
		return profiles.ModelParams{}, false
	}
	if m.params.open {
		if profiles.ModelParamsKey(m.params.modelPath) == profiles.ModelParamsKey(sel) {
			ap := m.params.editor.ActiveProfile()
			return profiles.ModelParams{Env: ap.Env, Args: ap.Args, UseCase: ap.UseCase}, true
		}
	}
	p, err := profiles.LoadParamsForRun(sel)
	if err != nil {
		return profiles.ModelParams{}, false
	}
	return p, true
}
