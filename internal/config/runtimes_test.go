package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/flyingnobita/llml/internal/models"
	"github.com/flyingnobita/llml/internal/userdata"
)

func TestRuntimeStatesPath_nextToConfig(t *testing.T) {
	isolatedConfigDir(t)

	cfg, err := ConfigPath()
	if err != nil {
		t.Fatal(err)
	}
	got, err := RuntimeStatesPath()
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Dir(got) != filepath.Dir(cfg) || filepath.Base(got) != "runtimes.toml" {
		t.Errorf("RuntimeStatesPath() = %q, want runtimes.toml next to %q", got, cfg)
	}
}

func TestReadRuntimeStates_missingFileMeansNothingSeen(t *testing.T) {
	isolatedConfigDir(t)

	got, err := ReadRuntimeStates()
	if err != nil {
		t.Fatalf("a missing file is not an error: %v", err)
	}
	for _, b := range []models.ModelBackend{models.BackendLlama, models.BackendOllama, models.BackendSplash} {
		if _, seen := got.Lookup(b); seen {
			t.Errorf("%v seen in a missing file", b)
		}
		if !got.Enabled(b) {
			t.Errorf("%v: a Runtime with no entry must count as on", b)
		}
	}
	if len(got.Disabled()) != 0 {
		t.Errorf("Disabled() = %v, want empty", got.Disabled())
	}
}

func TestRuntimeStates_roundTrip(t *testing.T) {
	isolatedConfigDir(t)

	want := RuntimeStates{}.
		With(models.BackendLlama, true).
		With(models.BackendOllama, false).
		With(models.BackendKobold, false)
	if err := WriteRuntimeStates(want); err != nil {
		t.Fatal(err)
	}
	got, err := ReadRuntimeStates()
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		b        models.ModelBackend
		on, seen bool
	}{
		{models.BackendLlama, true, true},
		{models.BackendOllama, false, true},
		{models.BackendKobold, false, true},
		{models.BackendVLLM, true, false},
	} {
		on, seen := got.Lookup(tc.b)
		if seen != tc.seen || (seen && on != tc.on) {
			t.Errorf("Lookup(%v) = (%t, %t), want (%t, %t)", tc.b, on, seen, tc.on, tc.seen)
		}
	}
	off := got.Disabled()
	if !off.Has(models.BackendOllama) || !off.Has(models.BackendKobold) || off.Has(models.BackendLlama) || len(off) != 2 {
		t.Errorf("Disabled() = %v, want {ollama, koboldcpp}", off)
	}
}

func TestReadRuntimeStates_keepsUnknownRuntimes(t *testing.T) {
	isolatedConfigDir(t)

	path, err := RuntimeStatesPath()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	// A newer llml may record a Runtime this one does not know. Writing the
	// file back must not drop it.
	doc := "schema_version = 1\n\n[runtimes]\n  future = false\n  llama = true\n"
	if err := os.WriteFile(path, []byte(doc), 0o600); err != nil {
		t.Fatal(err)
	}
	st, err := ReadRuntimeStates()
	if err != nil {
		t.Fatal(err)
	}
	if err := WriteRuntimeStates(st.With(models.BackendVLLM, false)); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"future = false", "llama = true", "vllm = false"} {
		if !strings.Contains(string(b), want) {
			t.Errorf("rewritten file lacks %q:\n%s", want, b)
		}
	}
}

func TestWriteRuntimeStates_backsUpBeforeOverwrite(t *testing.T) {
	isolatedConfigDir(t)

	if err := WriteRuntimeStates(RuntimeStates{}.With(models.BackendLlama, true)); err != nil {
		t.Fatal(err)
	}
	path, err := RuntimeStatesPath()
	if err != nil {
		t.Fatal(err)
	}
	backups := filepath.Join(filepath.Dir(path), userdata.BackupDirName)
	if entries, _ := os.ReadDir(backups); len(entries) != 0 {
		t.Fatalf("first write has nothing to back up, found %d backups", len(entries))
	}
	first, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	if err := WriteRuntimeStates(RuntimeStates{}.With(models.BackendLlama, false)); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(backups)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || !strings.HasPrefix(entries[0].Name(), "runtimes.toml.") {
		t.Fatalf("want one runtimes.toml backup, got %v", entries)
	}
	saved, err := os.ReadFile(filepath.Join(backups, entries[0].Name()))
	if err != nil {
		t.Fatal(err)
	}
	if string(saved) != string(first) {
		t.Errorf("backup holds %q, want the previous file %q", saved, first)
	}
}

func TestRuntimeStates_withDoesNotMutateReceiver(t *testing.T) {
	t.Parallel()

	base := RuntimeStates{}.With(models.BackendLlama, true)
	_ = base.With(models.BackendLlama, false)
	if !base.Enabled(models.BackendLlama) {
		t.Error("With changed the receiver; RuntimeStates must be copy-on-write")
	}
}
