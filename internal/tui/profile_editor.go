package tui

import (
	"strings"

	"github.com/flyingnobita/llml/internal/profiles"
)

// profileEditor owns the open panel's profile slice, the active index, and the
// working-form env/args buffers. Callers issue intent methods; the editor
// materializes storage only inside the flush+normalize read accessors.
type profileEditor struct {
	profiles   []ParameterProfile
	index      int
	env        []EnvVar // working form: may have in-progress empty rows
	envCursor  int
	args       []string // working form: display-paired (e.g. "--ctx-size 4096" on one row)
	argsCursor int
}

// flush writes the current working-form buffers into profiles[index] and normalizes.
// Called by lifecycle methods before switching the active profile.
func (e *profileEditor) flush() {
	if e.index < 0 || e.index >= len(e.profiles) {
		return
	}
	e.profiles[e.index].Env = append([]EnvVar(nil), e.env...)
	e.profiles[e.index].Args = flattenArgLines(e.args)
	e.profiles[e.index] = profiles.NormalizeProfile(e.profiles[e.index])
}

// load copies profiles[index] into the working-form buffers (pairing flat argv
// for display) and resets both cursors to 0.
func (e *profileEditor) load() {
	if e.index < 0 || e.index >= len(e.profiles) {
		return
	}
	p := e.profiles[e.index]
	e.env = append([]EnvVar(nil), p.Env...)
	e.args = pairFlagValueForShellDisplay(p.Args)
	e.envCursor = 0
	e.argsCursor = 0
}

// ── no-flush buffer reads ──────────────────────────────────────────────────

func (e *profileEditor) EnvRows() []EnvVar { return e.env }
func (e *profileEditor) ArgRows() []string { return e.args }
func (e *profileEditor) EnvCursor() int    { return e.envCursor }
func (e *profileEditor) ArgsCursor() int   { return e.argsCursor }

// ── flush+normalize reads ──────────────────────────────────────────────────

// ActiveProfile materializes the active profile: it overlays the working-form
// buffers on top of profiles[index] (which holds the current metadata) and
// returns the normalized result. Does not mutate editor state.
func (e *profileEditor) ActiveProfile() ParameterProfile {
	if e.index < 0 || e.index >= len(e.profiles) {
		return profiles.DefaultProfile()
	}
	p := profiles.CopyProfile(e.profiles[e.index])
	p.Env = append([]EnvVar(nil), e.env...)
	p.Args = flattenArgLines(e.args)
	return profiles.NormalizeProfile(p)
}

// Entry materializes all profiles into a storage entry: the active profile is
// built via ActiveProfile(); all others are taken from the profiles slice as-is.
func (e *profileEditor) Entry() modelEntry {
	ps := profiles.CopyProfiles(e.profiles)
	if e.index >= 0 && e.index < len(ps) {
		ps[e.index] = e.ActiveProfile()
	}
	return modelEntry{Profiles: ps, ActiveIndex: e.index}
}

// ── env/args intent methods ────────────────────────────────────────────────

// AddEnvRow appends an empty env row and advances the cursor to the new row.
func (e *profileEditor) AddEnvRow() {
	e.env = append(e.env, EnvVar{})
	e.envCursor = len(e.env) - 1
}

// SetEnvRow replaces the env row at index i.
func (e *profileEditor) SetEnvRow(i int, ev EnvVar) {
	if i >= 0 && i < len(e.env) {
		e.env[i] = ev
	}
}

// DeleteEnvRow removes the env row at envCursor and keeps the cursor valid.
func (e *profileEditor) DeleteEnvRow() {
	n := len(e.env)
	if n == 0 || e.envCursor < 0 || e.envCursor >= n {
		return
	}
	e.env = append(e.env[:e.envCursor], e.env[e.envCursor+1:]...)
	if e.envCursor >= len(e.env) && len(e.env) > 0 {
		e.envCursor = len(e.env) - 1
	}
}

// AddArgRow appends an empty args row and advances the cursor to the new row.
func (e *profileEditor) AddArgRow() {
	e.args = append(e.args, "")
	e.argsCursor = len(e.args) - 1
}

// SetArgRow replaces the args row at index i.
func (e *profileEditor) SetArgRow(i int, s string) {
	if i >= 0 && i < len(e.args) {
		e.args[i] = s
	}
}

// DeleteArgRow removes the args row at argsCursor and keeps the cursor valid.
func (e *profileEditor) DeleteArgRow() {
	n := len(e.args)
	if n == 0 || e.argsCursor < 0 || e.argsCursor >= n {
		return
	}
	e.args = append(e.args[:e.argsCursor], e.args[e.argsCursor+1:]...)
	if e.argsCursor >= len(e.args) && len(e.args) > 0 {
		e.argsCursor = len(e.args) - 1
	}
}

// ── metadata intent methods ────────────────────────────────────────────────
// Metadata (backend/usecase/hardware) lives directly in profiles[index], never
// in the buffer. These methods write directly to profiles[index] and normalize.

// SetBackend updates the backend of the active profile.
func (e *profileEditor) SetBackend(b string) {
	if e.index < 0 || e.index >= len(e.profiles) {
		return
	}
	p := e.profiles[e.index]
	p.Backend = b
	e.profiles[e.index] = profiles.NormalizeProfile(p)
}

// SetHardwareClass updates the hardware class of the active profile.
func (e *profileEditor) SetHardwareClass(c profiles.HardwareClass) {
	if e.index < 0 || e.index >= len(e.profiles) {
		return
	}
	p := e.profiles[e.index]
	p.Hardware.Class = c
	e.profiles[e.index] = profiles.NormalizeProfile(p)
}

// ToggleTag adds tag to the active profile's UseCase.Tags if absent, or removes
// it (case-insensitive) if present.
func (e *profileEditor) ToggleTag(tag string) {
	if e.index < 0 || e.index >= len(e.profiles) {
		return
	}
	p := e.profiles[e.index]
	p.UseCase.Tags = toggleTag(p.UseCase.Tags, tag)
	e.profiles[e.index] = profiles.NormalizeProfile(p)
}

// TogglePrimary adds primStr to the active profile's UseCase.Primary if absent,
// or removes it (case-insensitive) if present.
func (e *profileEditor) TogglePrimary(primStr string) {
	if e.index < 0 || e.index >= len(e.profiles) {
		return
	}
	p := e.profiles[e.index]
	strs := make([]string, len(p.UseCase.Primary))
	for i, v := range p.UseCase.Primary {
		strs[i] = string(v)
	}
	strs = toggleTag(strs, primStr)
	newPrimary := make(profiles.UseCasePrimaries, len(strs))
	for i, s := range strs {
		newPrimary[i] = profiles.UseCasePrimary(s)
	}
	p.UseCase.Primary = newPrimary
	e.profiles[e.index] = profiles.NormalizeProfile(p)
}

// SetHardwareField updates one hardware metadata field of the active profile.
func (e *profileEditor) SetHardwareField(field paramMetadataField, value string) {
	if e.index < 0 || e.index >= len(e.profiles) {
		return
	}
	p := e.profiles[e.index]
	switch field {
	case paramMetadataHardwareGPUCount:
		p.Hardware.GPUCount = profiles.ParseOptionalPositiveInt(value)
	case paramMetadataHardwareMinVRAM:
		p.Hardware.MinVRAMGB = profiles.ParseOptionalPositiveInt(value)
	case paramMetadataHardwareMaxVRAM:
		p.Hardware.MaxVRAMGB = profiles.ParseOptionalPositiveInt(value)
	case paramMetadataHardwareNotes:
		p.Hardware.Notes = strings.TrimSpace(value)
	}
	e.profiles[e.index] = profiles.NormalizeProfile(p)
}

// SetProfileName updates the name of the active profile.
func (e *profileEditor) SetProfileName(name string) {
	if e.index < 0 || e.index >= len(e.profiles) {
		return
	}
	e.profiles[e.index].Name = name
}

// ── lifecycle intent methods ───────────────────────────────────────────────

// AddProfile flushes the current profile, appends a new empty profile with the
// given name, switches to it, and loads its (empty) state into the buffer.
func (e *profileEditor) AddProfile(name string) {
	e.flush()
	e.profiles = append(e.profiles, ParameterProfile{Name: name})
	e.index = len(e.profiles) - 1
	e.load()
}

// DuplicateProfile flushes the current profile, inserts a copy of it immediately
// after the current index with the given name, switches to it, and loads its
// state into the buffer.
func (e *profileEditor) DuplicateProfile(name string) {
	if e.index < 0 || e.index >= len(e.profiles) {
		return
	}
	e.flush()
	clone := profiles.CopyProfile(e.profiles[e.index])
	clone.Name = name
	i := e.index
	e.profiles = append(e.profiles[:i+1], append([]ParameterProfile{clone}, e.profiles[i+1:]...)...)
	e.index = i + 1
	e.load()
}

// DeleteProfile removes the active profile. Returns false and is a no-op when
// there is only one profile. Switches to the nearest remaining profile and loads
// it into the buffer.
func (e *profileEditor) DeleteProfile() bool {
	if len(e.profiles) <= 1 {
		return false
	}
	e.profiles = append(e.profiles[:e.index], e.profiles[e.index+1:]...)
	if e.index >= len(e.profiles) {
		e.index = len(e.profiles) - 1
	}
	e.load()
	return true
}

// MoveActive flushes the current profile, moves the active index by delta, and
// loads the new profile into the buffer. Returns false if the move would go out
// of bounds; the editor is unchanged in that case.
func (e *profileEditor) MoveActive(delta int) bool {
	n := len(e.profiles)
	if n == 0 {
		return false
	}
	next := e.index + delta
	if next < 0 || next >= n {
		return false
	}
	e.flush()
	e.index = next
	e.load()
	return true
}

// ── constructor ───────────────────────────────────────────────────────────

// newProfileEditor builds a profileEditor from a model entry, deep-copying the
// profiles slice and loading the active profile's env/args into the buffer.
func newProfileEditor(ent modelEntry) profileEditor {
	var e profileEditor
	if len(ent.Profiles) == 0 {
		e.profiles = []ParameterProfile{profiles.DefaultProfile()}
	} else {
		e.profiles = profiles.CopyProfiles(ent.Profiles)
	}
	e.index = clampInt(ent.ActiveIndex, 0, len(e.profiles)-1)
	e.load()
	return e
}
