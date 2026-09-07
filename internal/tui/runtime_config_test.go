package tui

import (
	"path/filepath"
	"testing"

	"github.com/flyingnobita/llml/internal/settings"
)

func TestParsePortField(t *testing.T) {
	t.Parallel()

	// An empty field means "use the default", which is how clearing a port
	// restores built-in behavior.
	if p, err := parsePortField("", 8000); err != nil || p != 8000 {
		t.Fatalf("empty: got (%d, %v), want (8000, nil)", p, err)
	}
	if p, err := parsePortField("  9090 ", 8000); err != nil || p != 9090 {
		t.Fatalf("value: got (%d, %v), want (9090, nil)", p, err)
	}
	for _, bad := range []string{"0", "65536", "abc"} {
		if _, err := parsePortField(bad, 8000); err == nil {
			t.Errorf("parsePortField(%q) should fail", bad)
		}
	}
}

func TestHostField(t *testing.T) {
	t.Parallel()

	if g := hostField("  0.0.0.0 ", "127.0.0.1"); g != "0.0.0.0" {
		t.Errorf("got %q", g)
	}
	if g := hostField("   ", "127.0.0.1"); g != "127.0.0.1" {
		t.Errorf("empty should fall back to the default, got %q", g)
	}
}

func TestValidatePortInput(t *testing.T) {
	t.Parallel()

	if err := validatePortInput("8080"); err != nil {
		t.Fatal(err)
	}
	if validatePortInput("12a") == nil {
		t.Fatal("expected error")
	}
	if validatePortInput("123456") == nil {
		t.Fatal("expected error")
	}
}

func TestValidatePortCommit(t *testing.T) {
	t.Parallel()

	if err := validatePortCommit(""); err != nil {
		t.Fatal(err)
	}
	if err := validatePortCommit("8080"); err != nil {
		t.Fatal(err)
	}
	if validatePortCommit("0") == nil {
		t.Fatal("expected error")
	}
}

// The panel must prefill from the resolved settings, and a freshly opened panel
// must therefore not read as dirty.
func TestRuntimeFieldValues_prefillIsNotDirty(t *testing.T) {
	t.Parallel()

	m := New()
	m.settings = settings.Resolve(settings.Layer{
		LlamaCppPath:    ptrTo("/opt/llama"),
		LlamaServerPort: ptrTo(61111),
		OllamaHost:      ptrTo("box:11434"),
	}, settings.Defaults())

	m, _ = m.openRuntimeConfig()

	if m.rc.inputs[runtimeFieldLlamaCppPath].Value() != "/opt/llama" {
		t.Errorf("llama path = %q", m.rc.inputs[runtimeFieldLlamaCppPath].Value())
	}
	if m.rc.inputs[runtimeFieldLlamaPort].Value() != "61111" {
		t.Errorf("llama port = %q", m.rc.inputs[runtimeFieldLlamaPort].Value())
	}
	if m.rc.inputs[runtimeFieldOllamaHost].Value() != "box:11434" {
		t.Errorf("ollama host = %q", m.rc.inputs[runtimeFieldOllamaHost].Value())
	}
	if m.runtimeConfigDirty() {
		t.Error("a freshly prefilled panel should not be dirty")
	}

	m.rc.inputs[runtimeFieldVLLMPort].SetValue("7777")
	if !m.runtimeConfigDirty() {
		t.Error("an edited field should mark the panel dirty")
	}
}

// Not parallel: tilde expansion resolves the real home directory.
func TestSettingsFromRuntimeInputs(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	m := New()
	// Values the panel cannot edit must survive the round trip.
	m.settings.ExtraModelPaths = []string{"/roots/a"}
	m.settings.HFHubCache = "/hf/cache"
	m, _ = m.openRuntimeConfig()

	m.rc.inputs[runtimeFieldLlamaCppPath].SetValue("~/llama/bin")
	m.rc.inputs[runtimeFieldLlamaPort].SetValue("61111")
	m.rc.inputs[runtimeFieldVLLMPort].SetValue("") // empty means default
	m.rc.inputs[runtimeFieldOllamaHost].SetValue("http://box:11434/")
	m.rc.inputs[runtimeFieldVLLMHost].SetValue("  ") // blank means default

	got, err := m.settingsFromRuntimeInputs()
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(home, "llama", "bin"); got.LlamaCppPath != want {
		t.Errorf("LlamaCppPath = %q, want %q", got.LlamaCppPath, want)
	}
	if got.LlamaServerPort != 61111 {
		t.Errorf("LlamaServerPort = %d", got.LlamaServerPort)
	}
	if got.VLLMServerPort != settings.DefaultVLLMServerPort {
		t.Errorf("empty port should use the default, got %d", got.VLLMServerPort)
	}
	if got.OllamaHost != "box:11434" {
		t.Errorf("OllamaHost = %q, want box:11434 (scheme stripped)", got.OllamaHost)
	}
	if got.VLLMServerHost != settings.DefaultVLLMServerHost {
		t.Errorf("blank host should use the default, got %q", got.VLLMServerHost)
	}
	if len(got.ExtraModelPaths) != 1 || got.ExtraModelPaths[0] != "/roots/a" {
		t.Errorf("ExtraModelPaths should be carried over, got %v", got.ExtraModelPaths)
	}
	if got.HFHubCache != "/hf/cache" {
		t.Errorf("HFHubCache should be carried over, got %q", got.HFHubCache)
	}
}

func TestSettingsFromRuntimeInputs_rejectsBadPort(t *testing.T) {
	t.Parallel()

	m := New()
	m, _ = m.openRuntimeConfig()
	m.rc.inputs[runtimeFieldKoboldCppPort].SetValue("70000")

	if _, err := m.settingsFromRuntimeInputs(); err == nil {
		t.Fatal("expected an error for an out-of-range port")
	}
}

func ptrTo[T any](v T) *T { return &v }
