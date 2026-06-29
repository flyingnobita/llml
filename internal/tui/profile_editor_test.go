package tui

import (
	"testing"

	"github.com/flyingnobita/llml/internal/profiles"
)

// ── IRON RULE regression: ToggleTag must not discard in-progress env edits ──

func TestProfileEditor_ToggleTagPreservesEnvBuffer(t *testing.T) {
	e := newProfileEditor(modelEntry{
		Profiles: []ParameterProfile{{Name: "default"}},
	})
	// Simulate in-progress env edits in the buffer.
	e.env = []EnvVar{{Key: "FOO", Value: "bar"}, {}} // second row: in-progress empty
	e.ToggleTag("thinking")

	// Buffer must be unchanged after a metadata mutation.
	if len(e.env) != 2 {
		t.Fatalf("env buffer len = %d, want 2", len(e.env))
	}
	if e.env[0].Key != "FOO" || e.env[0].Value != "bar" {
		t.Fatalf("env[0] = %+v, want FOO=bar", e.env[0])
	}

	// Materialized Entry must include the tag AND the non-empty env row.
	ent := e.Entry()
	if len(ent.Profiles) != 1 {
		t.Fatalf("entry profiles len = %d", len(ent.Profiles))
	}
	p := ent.Profiles[0]
	if !hasTag(p.UseCase.Tags, "thinking") {
		t.Fatalf("tag missing from Entry: %v", p.UseCase.Tags)
	}
	if len(p.Env) != 1 || p.Env[0].Key != "FOO" {
		t.Fatalf("Entry env = %v, want [{FOO bar}]", p.Env)
	}
}

// ── Working-vs-storage contract: empty row in buffer, absent from Entry ────

func TestProfileEditor_AddEnvRow_EmptyRowInBufferAbsentFromEntry(t *testing.T) {
	e := newProfileEditor(modelEntry{
		Profiles: []ParameterProfile{{Name: "default", Env: []EnvVar{{Key: "A", Value: "1"}}}},
	})
	e.AddEnvRow() // appends empty row to buffer

	rows := e.EnvRows()
	if len(rows) != 2 {
		t.Fatalf("EnvRows len = %d, want 2", len(rows))
	}
	if rows[1].Key != "" || rows[1].Value != "" {
		t.Fatalf("new row is not empty: %+v", rows[1])
	}

	// Entry must NOT contain the empty row (normalize strips it).
	ent := e.Entry()
	if len(ent.Profiles[0].Env) != 1 {
		t.Fatalf("Entry.Env len = %d (want 1 — empty row stripped)", len(ent.Profiles[0].Env))
	}
	if ent.Profiles[0].Env[0].Key != "A" {
		t.Fatalf("Entry.Env[0] = %+v, want {A 1}", ent.Profiles[0].Env[0])
	}
}

func TestProfileEditor_AddArgRow_EmptyRowInBufferAbsentFromEntry(t *testing.T) {
	e := newProfileEditor(modelEntry{
		Profiles: []ParameterProfile{{Name: "default", Args: []string{"--x"}}},
	})
	e.AddArgRow()

	rows := e.ArgRows()
	if len(rows) != 2 {
		t.Fatalf("ArgRows len = %d, want 2", len(rows))
	}
	if rows[1] != "" {
		t.Fatalf("new arg row is not empty: %q", rows[1])
	}

	ent := e.Entry()
	if len(ent.Profiles[0].Args) != 1 || ent.Profiles[0].Args[0] != "--x" {
		t.Fatalf("Entry.Args = %v, want [--x]", ent.Profiles[0].Args)
	}
}

// ── Lifecycle: flush outgoing edits before switching ─────────────────────

func TestProfileEditor_MoveActive_FlushesOutgoingEdits(t *testing.T) {
	e := newProfileEditor(modelEntry{
		Profiles: []ParameterProfile{
			{Name: "a"},
			{Name: "b"},
		},
	})
	// Edit the buffer for profile 0 without persisting.
	e.env = []EnvVar{{Key: "K", Value: "V"}}

	moved := e.MoveActive(1)
	if !moved {
		t.Fatal("MoveActive(1) returned false")
	}
	if e.index != 1 {
		t.Fatalf("index = %d, want 1", e.index)
	}
	// Buffer is now loaded from profile 1 (empty).
	if len(e.env) != 0 {
		t.Fatalf("buffer not loaded from profile 1: env=%v", e.env)
	}
	// Profile 0 must have the edits flushed into its stored form.
	if len(e.profiles[0].Env) != 1 || e.profiles[0].Env[0].Key != "K" {
		t.Fatalf("profile 0 Env after flush = %v, want [{K V}]", e.profiles[0].Env)
	}
}

func TestProfileEditor_AddProfile_FlushesOutgoingEdits(t *testing.T) {
	e := newProfileEditor(modelEntry{
		Profiles: []ParameterProfile{{Name: "a"}},
	})
	e.env = []EnvVar{{Key: "X", Value: "1"}}

	e.AddProfile("b")

	if e.index != 1 {
		t.Fatalf("index = %d, want 1", e.index)
	}
	// Profile 0 flushed.
	if len(e.profiles[0].Env) != 1 || e.profiles[0].Env[0].Key != "X" {
		t.Fatalf("profile 0 Env = %v, want [{X 1}]", e.profiles[0].Env)
	}
	// New profile buffer is empty.
	if len(e.env) != 0 {
		t.Fatalf("new profile buffer not empty: %v", e.env)
	}
}

// ── DeleteProfile: guards len==1 ──────────────────────────────────────────

func TestProfileEditor_DeleteProfile_GuardsLen1(t *testing.T) {
	e := newProfileEditor(modelEntry{
		Profiles: []ParameterProfile{{Name: "only"}},
	})
	if ok := e.DeleteProfile(); ok {
		t.Fatal("DeleteProfile() on single profile returned true, want false")
	}
	if len(e.profiles) != 1 {
		t.Fatalf("profiles len = %d after no-op delete", len(e.profiles))
	}
}

func TestProfileEditor_DeleteProfile_SwitchesToNextProfile(t *testing.T) {
	e := newProfileEditor(modelEntry{
		Profiles: []ParameterProfile{
			{Name: "a", Env: []EnvVar{{Key: "A", Value: "1"}}},
			{Name: "b"},
		},
	})
	ok := e.DeleteProfile()
	if !ok {
		t.Fatal("DeleteProfile() returned false unexpectedly")
	}
	if len(e.profiles) != 1 {
		t.Fatalf("profiles len = %d, want 1", len(e.profiles))
	}
	if e.profiles[0].Name != "b" {
		t.Fatalf("remaining profile name = %q, want b", e.profiles[0].Name)
	}
	if e.index != 0 {
		t.Fatalf("index = %d, want 0", e.index)
	}
}

// ── Args round-trip: paired display ↔ flat argv via Entry ─────────────────

func TestProfileEditor_ArgsRoundTrip_PairedDisplay_FlatStorage(t *testing.T) {
	flat := []string{"--max-model-len", "8192", "--max-num-seqs", "4", "--enable-auto-tool-choice"}
	e := newProfileEditor(modelEntry{
		Profiles: []ParameterProfile{{Name: "default", Args: flat}},
	})

	// load() should pair them for display.
	rows := e.ArgRows()
	wantPaired := []string{"--max-model-len 8192", "--max-num-seqs 4", "--enable-auto-tool-choice"}
	if len(rows) != len(wantPaired) {
		t.Fatalf("ArgRows len = %d, want %d: %v", len(rows), len(wantPaired), rows)
	}
	for i, want := range wantPaired {
		if rows[i] != want {
			t.Fatalf("ArgRows[%d] = %q, want %q", i, rows[i], want)
		}
	}

	// Entry() must restore flat argv.
	ent := e.Entry()
	got := ent.Profiles[0].Args
	if len(got) != len(flat) {
		t.Fatalf("Entry Args len = %d, want %d: %v", len(got), len(flat), got)
	}
	for i := range flat {
		if got[i] != flat[i] {
			t.Fatalf("Entry Args[%d] = %q, want %q", i, got[i], flat[i])
		}
	}
}

// ── ActiveProfile overlays buffer over stale profiles[index] env/args ─────

func TestProfileEditor_ActiveProfile_OverlaysBuffer(t *testing.T) {
	e := newProfileEditor(modelEntry{
		Profiles: []ParameterProfile{{
			Name:    "p",
			Backend: "llama",
			Env:     []EnvVar{{Key: "OLD", Value: "x"}},
		}},
	})
	// Overwrite buffer with new env rows (simulating in-progress edits).
	e.env = []EnvVar{{Key: "NEW", Value: "y"}}

	p := e.ActiveProfile()
	// Metadata from profiles[index] — Backend should be present.
	if p.Backend != "llama" {
		t.Fatalf("Backend = %q, want llama", p.Backend)
	}
	// Env from buffer, not from stale profiles[index].Env.
	if len(p.Env) != 1 || p.Env[0].Key != "NEW" {
		t.Fatalf("Env = %v, want [{NEW y}]", p.Env)
	}
}

// ── Entry: normalizes active profile (strips empty env/args) ──────────────

func TestProfileEditor_Entry_NormalizesActiveProfile(t *testing.T) {
	e := newProfileEditor(modelEntry{
		Profiles: []ParameterProfile{{Name: "default"}},
	})
	e.env = []EnvVar{{Key: "K", Value: "V"}, {}} // trailing empty
	e.args = []string{"--ctx-size 4096", ""}     // trailing empty

	ent := e.Entry()
	p := ent.Profiles[0]
	if len(p.Env) != 1 {
		t.Fatalf("Env len = %d, want 1 (empty row stripped)", len(p.Env))
	}
	// "--ctx-size 4096" gets expanded to two argv tokens.
	if len(p.Args) != 2 || p.Args[0] != "--ctx-size" || p.Args[1] != "4096" {
		t.Fatalf("Args = %v, want [--ctx-size 4096]", p.Args)
	}
}

// ── DuplicateProfile includes buffer contents in the clone ────────────────

func TestProfileEditor_DuplicateProfile_ClonesBufferContents(t *testing.T) {
	e := newProfileEditor(modelEntry{
		Profiles: []ParameterProfile{{Name: "original", Env: []EnvVar{{Key: "K", Value: "V"}}}},
	})
	// Simulate in-progress env edits.
	e.env = append(e.env, EnvVar{Key: "NEW", Value: "2"})

	e.DuplicateProfile("clone")

	if len(e.profiles) != 2 {
		t.Fatalf("profiles len = %d, want 2", len(e.profiles))
	}
	if e.index != 1 {
		t.Fatalf("index = %d, want 1", e.index)
	}
	// The clone (profiles[1]) must reflect the buffer state that was flushed.
	if len(e.profiles[1].Env) != 2 {
		t.Fatalf("clone Env len = %d, want 2: %v", len(e.profiles[1].Env), e.profiles[1].Env)
	}
}

// ── Cursor management ─────────────────────────────────────────────────────

func TestProfileEditor_DeleteEnvRow_ClampsCursor(t *testing.T) {
	e := newProfileEditor(modelEntry{Profiles: []ParameterProfile{{Name: "p"}}})
	e.env = []EnvVar{{Key: "A"}, {Key: "B"}, {Key: "C"}}
	e.envCursor = 2

	e.DeleteEnvRow() // delete index 2, cursor should clamp to 1
	if e.envCursor != 1 {
		t.Fatalf("envCursor = %d, want 1", e.envCursor)
	}
	if len(e.env) != 2 {
		t.Fatalf("env len = %d, want 2", len(e.env))
	}
}

func TestProfileEditor_DeleteArgRow_ClampsCursor(t *testing.T) {
	e := newProfileEditor(modelEntry{Profiles: []ParameterProfile{{Name: "p"}}})
	e.args = []string{"--a", "--b"}
	e.argsCursor = 1

	e.DeleteArgRow()
	if e.argsCursor != 0 {
		t.Fatalf("argsCursor = %d, want 0", e.argsCursor)
	}
	if len(e.args) != 1 {
		t.Fatalf("args len = %d, want 1", len(e.args))
	}
}

// ── Metadata: NormalizeProfile after mutation ─────────────────────────────

func TestProfileEditor_SetHardwareField_SwapsMinMaxVRAM(t *testing.T) {
	e := newProfileEditor(modelEntry{Profiles: []ParameterProfile{{Name: "p"}}})
	e.SetHardwareField(paramMetadataHardwareMinVRAM, "48")
	e.SetHardwareField(paramMetadataHardwareMaxVRAM, "24")

	// NormalizeProfile swaps min/max when min > max.
	p := e.profiles[0]
	if p.Hardware.MinVRAMGB == nil || *p.Hardware.MinVRAMGB != 24 {
		t.Fatalf("MinVRAMGB = %v, want 24", p.Hardware.MinVRAMGB)
	}
	if p.Hardware.MaxVRAMGB == nil || *p.Hardware.MaxVRAMGB != 48 {
		t.Fatalf("MaxVRAMGB = %v, want 48", p.Hardware.MaxVRAMGB)
	}
}

func TestProfileEditor_SetHardwareClass_CPU_ClearsGPUFields(t *testing.T) {
	two := 2
	e := newProfileEditor(modelEntry{
		Profiles: []ParameterProfile{{
			Name:     "p",
			Hardware: profiles.HardwareMetadata{Class: profiles.HardwareClassGPU, GPUCount: &two},
		}},
	})
	e.SetHardwareClass(profiles.HardwareClassCPU)
	// NormalizeProfile clears GPU-specific fields when class is CPU.
	if e.profiles[0].Hardware.GPUCount != nil {
		t.Fatalf("GPUCount should be nil for CPU class, got %v", e.profiles[0].Hardware.GPUCount)
	}
}

// ── newProfileEditor: empty entry gets a default profile ─────────────────

func TestNewProfileEditor_EmptyEntry_GetsDefaultProfile(t *testing.T) {
	e := newProfileEditor(modelEntry{})
	if len(e.profiles) != 1 || e.profiles[0].Name != "default" {
		t.Fatalf("profiles = %v, want [{default}]", e.profiles)
	}
	if e.index != 0 {
		t.Fatalf("index = %d, want 0", e.index)
	}
}
