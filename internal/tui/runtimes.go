package tui

import (
	"strconv"

	"github.com/flyingnobita/llml/internal/fsutil"
	"github.com/flyingnobita/llml/internal/models"
	"github.com/flyingnobita/llml/internal/settings"
)

// String returns the group label shown for the format.
func (f modelFormat) String() string {
	switch f {
	case formatSafetensors:
		return "Safetensors"
	case formatNInfer:
		return "NInfer (.ninfer)"
	case formatSplash:
		return "Splash bundle"
	case formatOllama:
		return "Ollama library"
	default:
		return "GGUF"
	}
}

// runtimeStatus is what detection learned about one Runtime: whether its
// program was found and whether its server answered.
type runtimeStatus struct {
	found   bool
	running bool
}

// runtimeFieldDef describes one editable field of a Runtime: the settings
// value it edits and how the runtime panel labels it.
type runtimeFieldDef struct {
	field runtimeField
	// setting is the settings value the field edits; it names the environment
	// variable that overrides the field and reports where the value came from.
	setting settings.Field
	// label names the input in the detail pane; hint is its placeholder.
	label string
	hint  string
	// inUse returns the program path detection resolved, or "" when none was
	// found. It is set only for path fields: a host or port is in use exactly
	// as resolved.
	inUse func(models.RuntimeInfo) string

	// Exactly one of str and port is set. Path and host fields are strings;
	// an empty host or port input falls back to its default.
	str         func(*settings.Settings) *string
	port        func(*settings.Settings) *int
	isPath      bool
	defaultHost string
	defaultPort int
	// normalize rewrites a host input before it is stored (Ollama accepts URLs).
	normalize func(string) string
}

// env returns the environment variable that overrides the field.
func (d runtimeFieldDef) env() string { return d.setting.EnvVar() }

// value returns what the field's input shows for s.
func (d runtimeFieldDef) value(s settings.Settings) string {
	if d.port != nil {
		return strconv.Itoa(*d.port(&s))
	}
	return *d.str(&s)
}

// apply stores the input raw into s.
func (d runtimeFieldDef) apply(s *settings.Settings, raw string) error {
	switch {
	case d.port != nil:
		p, err := parsePortField(raw, d.defaultPort)
		if err != nil {
			return err
		}
		*d.port(s) = p
	case d.isPath:
		*d.str(s) = fsutil.NormalizePath(raw)
	default:
		if d.normalize != nil {
			raw = d.normalize(raw)
		}
		*d.str(s) = hostField(raw, d.defaultHost)
	}
	return nil
}

// runtimeDef describes one Runtime: its name, the Model Format it runs, its
// fields in display order, and how to read its detection status.
type runtimeDef struct {
	backend models.ModelBackend
	name    string
	format  modelFormat
	fields  []runtimeFieldDef
	status  func(models.RuntimeInfo) runtimeStatus
}

// supported reports whether the Runtime can run on p. The rule itself lives in
// [models.Platform.Supports] because detection needs it too.
func (d runtimeDef) supported(p models.Platform) bool {
	return p.Supports(d.backend)
}

func pathFieldDef(f runtimeField, setting settings.Field, label, hint string, inUse func(models.RuntimeInfo) string, str func(*settings.Settings) *string) runtimeFieldDef {
	return runtimeFieldDef{field: f, setting: setting, label: label, hint: hint, inUse: inUse, str: str, isPath: true}
}

func portFieldDef(f runtimeField, setting settings.Field, port func(*settings.Settings) *int, def int) runtimeFieldDef {
	return runtimeFieldDef{field: f, setting: setting, label: "Port", port: port, defaultPort: def}
}

func hostFieldDef(f runtimeField, setting settings.Field, str func(*settings.Settings) *string, def string) runtimeFieldDef {
	return runtimeFieldDef{field: f, setting: setting, label: "Host", str: str, defaultHost: def}
}

// runtimeTable describes every Runtime, grouped by Model Format in the order
// the runtime panel lists them.
var runtimeTable = []runtimeDef{
	{
		backend: models.BackendLlama, name: "Llama.cpp", format: formatGGUF,
		fields: []runtimeFieldDef{
			pathFieldDef(runtimeFieldLlamaCppPath, settings.FieldLlamaCppPath, "Path", "dir with llama-cli / llama-server",
				models.ResolveLlamaServerPath, func(s *settings.Settings) *string { return &s.LlamaCppPath }),
			portFieldDef(runtimeFieldLlamaPort, settings.FieldLlamaServerPort,
				func(s *settings.Settings) *int { return &s.LlamaServerPort }, settings.DefaultLlamaServerPort),
			hostFieldDef(runtimeFieldLlamaHost, settings.FieldLlamaServerHost,
				func(s *settings.Settings) *string { return &s.LlamaServerHost }, settings.DefaultLlamaServerHost),
		},
		status: func(r models.RuntimeInfo) runtimeStatus {
			return runtimeStatus{found: models.ResolveLlamaServerPath(r) != "", running: r.ServerRunning}
		},
	},
	{
		backend: models.BackendKobold, name: "KoboldCpp", format: formatGGUF,
		fields: []runtimeFieldDef{
			pathFieldDef(runtimeFieldKoboldCppPath, settings.FieldKoboldCppPath, "Path", "koboldcpp binary or its dir",
				models.ResolveKoboldCppPath, func(s *settings.Settings) *string { return &s.KoboldCppPath }),
			portFieldDef(runtimeFieldKoboldCppPort, settings.FieldKoboldCppPort,
				func(s *settings.Settings) *int { return &s.KoboldCppPort }, settings.DefaultKoboldCppPort),
		},
		status: func(r models.RuntimeInfo) runtimeStatus {
			return runtimeStatus{found: models.ResolveKoboldCppPath(r) != "", running: r.KoboldCppRunning}
		},
	},
	{
		backend: models.BackendVLLM, name: "vLLM", format: formatSafetensors,
		fields: []runtimeFieldDef{
			pathFieldDef(runtimeFieldVLLMPath, settings.FieldVLLMPath, "Path", "dir with the vllm binary",
				models.ResolveVLLMPath, func(s *settings.Settings) *string { return &s.VLLMPath }),
			pathFieldDef(runtimeFieldVLLMVenv, settings.FieldVLLMVenv, "Venv", "venv root (optional)",
				vllmVenvInUse, func(s *settings.Settings) *string { return &s.VLLMVenv }),
			portFieldDef(runtimeFieldVLLMPort, settings.FieldVLLMServerPort,
				func(s *settings.Settings) *int { return &s.VLLMServerPort }, settings.DefaultVLLMServerPort),
			hostFieldDef(runtimeFieldVLLMHost, settings.FieldVLLMServerHost,
				func(s *settings.Settings) *string { return &s.VLLMServerHost }, settings.DefaultVLLMServerHost),
		},
		// vLLM has no server probe.
		status: func(r models.RuntimeInfo) runtimeStatus {
			return runtimeStatus{found: models.ResolveVLLMPath(r) != ""}
		},
	},
	{
		backend: models.BackendOMLX, name: "oMLX", format: formatSafetensors,
		fields: []runtimeFieldDef{
			pathFieldDef(runtimeFieldOMLXPath, settings.FieldOMLXPath, "Path", "omlx CLI or ~/.omlx",
				models.ResolveOMLXPath, func(s *settings.Settings) *string { return &s.OMLXPath }),
			portFieldDef(runtimeFieldOMLXPort, settings.FieldOMLXPort,
				func(s *settings.Settings) *int { return &s.OMLXPort }, settings.DefaultOMLXPort),
			hostFieldDef(runtimeFieldOMLXHost, settings.FieldOMLXHost,
				func(s *settings.Settings) *string { return &s.OMLXHost }, settings.DefaultOMLXHost),
		},
		status: func(r models.RuntimeInfo) runtimeStatus {
			return runtimeStatus{found: models.ResolveOMLXPath(r) != "", running: r.OMLXRunning}
		},
	},
	{
		backend: models.BackendNInfer, name: "NInfer", format: formatNInfer,
		fields: []runtimeFieldDef{
			pathFieldDef(runtimeFieldNInferPath, settings.FieldNInferPath, "Path", "checkout or ninfer-serve",
				models.ResolveNInferPath, func(s *settings.Settings) *string { return &s.NInferPath }),
			portFieldDef(runtimeFieldNInferPort, settings.FieldNInferServerPort,
				func(s *settings.Settings) *int { return &s.NInferServerPort }, settings.DefaultNInferServerPort),
			hostFieldDef(runtimeFieldNInferHost, settings.FieldNInferServerHost,
				func(s *settings.Settings) *string { return &s.NInferServerHost }, settings.DefaultNInferHost),
		},
		status: func(r models.RuntimeInfo) runtimeStatus {
			return runtimeStatus{found: models.ResolveNInferPath(r) != "", running: r.NInferRunning}
		},
	},
	{
		backend: models.BackendSplash, name: "Splash", format: formatSplash,
		fields: []runtimeFieldDef{
			pathFieldDef(runtimeFieldSplashPath, settings.FieldSplashPath, "Path", "splash binary or its dir",
				models.ResolveSplashPath, func(s *settings.Settings) *string { return &s.SplashPath }),
			portFieldDef(runtimeFieldSplashPort, settings.FieldSplashPort,
				func(s *settings.Settings) *int { return &s.SplashPort }, settings.DefaultSplashPort),
			hostFieldDef(runtimeFieldSplashHost, settings.FieldSplashHost,
				func(s *settings.Settings) *string { return &s.SplashHost }, settings.DefaultSplashHost),
		},
		status: func(r models.RuntimeInfo) runtimeStatus {
			return runtimeStatus{found: models.ResolveSplashPath(r) != "", running: r.SplashRunning}
		},
	},
	{
		backend: models.BackendOllama, name: "Ollama", format: formatOllama,
		fields: []runtimeFieldDef{
			pathFieldDef(runtimeFieldOllamaPath, settings.FieldOllamaPath, "Path", "ollama binary or its dir",
				models.ResolveOllamaPath, func(s *settings.Settings) *string { return &s.OllamaPath }),
			func() runtimeFieldDef {
				d := hostFieldDef(runtimeFieldOllamaHost, settings.FieldOllamaHost,
					func(s *settings.Settings) *string { return &s.OllamaHost }, settings.DefaultOllamaHost)
				d.normalize = settings.NormalizeOllamaHost
				return d
			}(),
		},
		status: func(r models.RuntimeInfo) runtimeStatus {
			return runtimeStatus{found: models.ResolveOllamaPath(r) != "", running: r.OllamaRunning}
		},
	},
}

// runtimeFor returns the Runtime whose backend is b.
func runtimeFor(b models.ModelBackend) runtimeDef {
	for _, rt := range runtimeTable {
		if rt.backend == b {
			return rt
		}
	}
	return runtimeTable[0]
}

// runtimeBackendsIn returns the profile backend names of the Runtimes that run
// format, in table order.
func runtimeBackendsIn(format modelFormat) []string {
	var out []string
	for _, rt := range runtimeTable {
		if rt.format == format {
			out = append(out, rt.backend.String())
		}
	}
	return out
}
