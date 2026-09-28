package tui

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/flyingnobita/llml/internal/config"
	"github.com/flyingnobita/llml/internal/models"
	"github.com/flyingnobita/llml/internal/settings"
)

// After r re-runs detection, the footer reports exactly the Runtimes rows
// need and detection did not find: the note comes back while llama-server is
// still missing, and goes once it is found.
func TestMissingRuntimeFooter_followsReloadRuntime(t *testing.T) {
	dir := useTempConfigDir(t)
	row := testRow(models.BackendLlama, filepath.Join(dir, "gemma.gguf"))
	m := dimModel(t, newLaunchFakes().services, linuxPlatform, config.RuntimeStates{}, row)

	missing := panelRuntime(linuxPlatform)
	missing.LlamaServerPath, missing.ServerRunning = "", false
	for _, tc := range []struct {
		name    string
		runtime models.RuntimeInfo
		want    bool
	}{
		{"still missing", missing, true},
		{"found", panelRuntime(linuxPlatform), false},
	} {
		m = press(t, m, keyText("r"))
		next, _ := m.Update(runtimeReadyMsg{settings: defaultSettings(), runtime: tc.runtime, states: runtimeStatesRead{states: m.runtimeStates}})
		m = next.(Model)
		if got := strings.Contains(m.lastRunNote, MissingLlamaServerFooterNote); got != tc.want {
			t.Errorf("%s: footer names llama-server = %t, want %t (note %q)", tc.name, got, tc.want, m.lastRunNote)
		}
	}
}

// Saving the c panel re-runs detection and keeps the footer for a Runtime a
// row still needs and detection did not find.
func TestMissingRuntimeFooter_followsRuntimePanelSave(t *testing.T) {
	dir := useTempConfigDir(t)
	row := testRow(models.BackendLlama, filepath.Join(dir, "gemma.gguf"))
	svc := newLaunchFakes().services
	svc.discoverRuntime = func(context.Context, settings.Settings, models.BackendSet) models.RuntimeInfo {
		rt := panelRuntime(linuxPlatform)
		rt.LlamaServerPath, rt.ServerRunning = "", false
		return rt
	}
	m := dimModel(t, svc, linuxPlatform, config.RuntimeStates{}, row)

	m = press(t, m, keyText("c"), keyEnter)
	if m.rc.open {
		t.Fatalf("enter should save and close the runtime panel:\n%s", plainView(m))
	}
	if !strings.Contains(m.lastRunNote, MissingLlamaServerFooterNote) {
		t.Errorf("the footer should still name the missing llama-server, note %q", m.lastRunNote)
	}
}
