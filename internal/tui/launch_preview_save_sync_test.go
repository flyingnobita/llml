package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/flyingnobita/llml/internal/models"
)

// baseModelForPreview returns a minimal GGUF row and runtime so launch preview is populated.
func baseModelForPreview(t *testing.T, modelPath string) Model {
	t.Helper()
	m := New()
	m.layout.width = 120
	m.layout.height = 40
	m.loading = false
	m.runtime = models.RuntimeInfo{LlamaServerPath: "/fake/llama-server"}
	m.table.files = []models.ModelFile{
		{
			Backend: models.BackendLlama,
			Path:    modelPath,
			Name:    filepath.Base(modelPath),
			Size:    1,
			ModTime: time.Unix(0, 0),
		},
	}
	m = m.layoutTable()
	m.table.tbl.SetCursor(0)
	return m
}

func TestCommitRuntimeConfig_refreshesLaunchPreview(t *testing.T) {
	cfg := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", cfg)
	t.Setenv("HOME", cfg)
	t.Cleanup(func() { _ = os.Unsetenv(models.EnvLlamaServerPort) })

	modelPath := filepath.Join(cfg, "probe.gguf")
	m := baseModelForPreview(t, modelPath)

	before := launchPreviewCommandLine(m)
	if strings.Contains(before, "61111") {
		t.Fatalf("unexpected port in initial preview: %s", before)
	}

	m, _ = m.openRuntimeConfig()
	m.rc.inputs[runtimeFieldLlamaPort].SetValue("61111")
	m, _ = m.commitRuntimeConfig()
	if m.rc.open {
		t.Fatal("runtime modal should close after commit")
	}

	after := strings.TrimSpace(m.preview.lastCmd)
	if after == "" {
		t.Fatal("expected launch preview after runtime commit")
	}
	if !strings.Contains(after, "61111") {
		t.Fatalf("preview should include new llama port; got:\n%s", after)
	}
}

func TestPersistParamPanel_refreshesLaunchPreview(t *testing.T) {
	cfg := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", cfg)
	t.Setenv("HOME", cfg)
	t.Setenv("AppData", cfg)

	modelPath := filepath.Join(cfg, "with-params.gguf")
	if err := saveModelEntry(modelPath, modelEntry{
		Profiles:    []ParameterProfile{{Name: "default", Env: nil, Args: nil}},
		ActiveIndex: 0,
	}); err != nil {
		t.Fatal(err)
	}

	m := baseModelForPreview(t, modelPath)
	if strings.Contains(m.preview.lastCmd, "ctx-size") {
		t.Fatalf("unexpected arg in initial preview: %s", m.preview.lastCmd)
	}

	m.params.open = true
	m.params.modelPath = filepath.Clean(modelPath)
	m.params.modelDisplayName = filepath.Base(modelPath)
	m.params.profiles = copyProfiles([]ParameterProfile{{Name: "default", Env: nil, Args: nil}})
	m.params.profileIndex = 0
	m.params.loadCurrentProfileIn()
	m.params.args = []string{"--ctx-size 4096"}

	m, cmd := m.persistParamPanel()
	if cmd != nil {
		t.Fatalf("unexpected cmd: %v", cmd)
	}
	if m.lastRunNote != "" {
		t.Fatalf("persist error: %s", m.lastRunNote)
	}

	after := strings.TrimSpace(m.preview.lastCmd)
	if after == "" {
		t.Fatal("expected launch preview after param persist")
	}
	if !strings.Contains(after, "ctx-size") || !strings.Contains(after, "4096") {
		t.Fatalf("preview should include persisted argv; got:\n%s", after)
	}
}
