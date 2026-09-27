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
		return "NInfer"
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
// value it edits, the environment variable that overrides it, and how the
// runtime panel labels it.
type runtimeFieldDef struct {
	field runtimeField
	env   string
	// label names the input in the runtime panel; summaryLabel names the same
	// value in the Active Configuration list, whose value comes from summary.
	label        string
	summaryLabel string
	summary      func(models.RuntimeInfo) string

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

func pathFieldDef(f runtimeField, env, label, summaryLabel string, summary func(models.RuntimeInfo) string, str func(*settings.Settings) *string) runtimeFieldDef {
	return runtimeFieldDef{field: f, env: env, label: label, summaryLabel: summaryLabel, summary: summary, str: str, isPath: true}
}

func portFieldDef(f runtimeField, env, summaryLabel string, summary func(models.RuntimeInfo) int, port func(*settings.Settings) *int, def int) runtimeFieldDef {
	return runtimeFieldDef{
		field: f, env: env, label: "Port", summaryLabel: summaryLabel,
		summary: func(r models.RuntimeInfo) string { return portDisplay(summary(r)) },
		port:    port, defaultPort: def,
	}
}

func hostFieldDef(f runtimeField, env, summaryLabel string, summary func(models.RuntimeInfo) string, str func(*settings.Settings) *string, def string) runtimeFieldDef {
	return runtimeFieldDef{
		field: f, env: env, label: "Host", summaryLabel: summaryLabel,
		summary: func(r models.RuntimeInfo) string { return valueOrDash(summary(r)) },
		str:     str, defaultHost: def,
	}
}

// runtimeTable describes every Runtime, grouped by Model Format in the order
// the runtime panel lists them.
var runtimeTable = []runtimeDef{
	{
		backend: models.BackendLlama, name: "Llama.cpp", format: formatGGUF,
		fields: []runtimeFieldDef{
			pathFieldDef(runtimeFieldLlamaCppPath, settings.EnvLlamaCppPath, "Path (llama-cli / llama-server)",
				runtimePanelLabelLlamaServerPath, llamaServerPathPanelDisplay,
				func(s *settings.Settings) *string { return &s.LlamaCppPath }),
			portFieldDef(runtimeFieldLlamaPort, settings.EnvLlamaServerPort, runtimePanelLabelLlamaServerPort,
				func(r models.RuntimeInfo) int { return r.LlamaServerPort },
				func(s *settings.Settings) *int { return &s.LlamaServerPort }, settings.DefaultLlamaServerPort),
			hostFieldDef(runtimeFieldLlamaHost, settings.EnvLlamaServerHost, runtimePanelLabelLlamaServerHost,
				func(r models.RuntimeInfo) string { return r.LlamaServerHost },
				func(s *settings.Settings) *string { return &s.LlamaServerHost }, settings.DefaultLlamaServerHost),
		},
		status: func(r models.RuntimeInfo) runtimeStatus {
			return runtimeStatus{found: models.ResolveLlamaServerPath(r) != "", running: r.ServerRunning}
		},
	},
	{
		backend: models.BackendKobold, name: "KoboldCpp", format: formatGGUF,
		fields: []runtimeFieldDef{
			pathFieldDef(runtimeFieldKoboldCppPath, settings.EnvKoboldCppPath, "Path (koboldcpp binary)",
				runtimePanelLabelKoboldCppPath, koboldCppPathPanelDisplay,
				func(s *settings.Settings) *string { return &s.KoboldCppPath }),
			portFieldDef(runtimeFieldKoboldCppPort, settings.EnvKoboldCppPort, runtimePanelLabelKoboldCppPort,
				func(r models.RuntimeInfo) int { return r.KoboldCppPort },
				func(s *settings.Settings) *int { return &s.KoboldCppPort }, settings.DefaultKoboldCppPort),
		},
		status: func(r models.RuntimeInfo) runtimeStatus {
			return runtimeStatus{found: models.ResolveKoboldCppPath(r) != "", running: r.KoboldCppRunning}
		},
	},
	{
		backend: models.BackendVLLM, name: "vLLM", format: formatSafetensors,
		fields: []runtimeFieldDef{
			pathFieldDef(runtimeFieldVLLMPath, settings.EnvVLLMPath, "Path (vllm binary)",
				runtimePanelLabelVLLMPath, vllmPathPanelDisplay,
				func(s *settings.Settings) *string { return &s.VLLMPath }),
			pathFieldDef(runtimeFieldVLLMVenv, settings.EnvVLLMVenv, "Venv Root (Optional)",
				runtimePanelLabelVLLMVenv, vllmVenvPanelDisplay,
				func(s *settings.Settings) *string { return &s.VLLMVenv }),
			portFieldDef(runtimeFieldVLLMPort, settings.EnvVLLMServerPort, runtimePanelLabelVLLMPort,
				func(r models.RuntimeInfo) int { return r.VLLMServerPort },
				func(s *settings.Settings) *int { return &s.VLLMServerPort }, settings.DefaultVLLMServerPort),
			hostFieldDef(runtimeFieldVLLMHost, settings.EnvVLLMServerHost, runtimePanelLabelVLLMHost,
				func(r models.RuntimeInfo) string { return r.VLLMServerHost },
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
			pathFieldDef(runtimeFieldOMLXPath, settings.EnvOMLXPath, "Path (omlx CLI or ~/.omlx)",
				runtimePanelLabelOMLXPath, func(r models.RuntimeInfo) string {
					return binaryPathPanelDisplay(models.ResolveOMLXPath(r), r.OMLXRunning, r.OMLXPort)
				},
				func(s *settings.Settings) *string { return &s.OMLXPath }),
			portFieldDef(runtimeFieldOMLXPort, settings.EnvOMLXPort, runtimePanelLabelOMLXPort,
				func(r models.RuntimeInfo) int { return r.OMLXPort },
				func(s *settings.Settings) *int { return &s.OMLXPort }, settings.DefaultOMLXPort),
			hostFieldDef(runtimeFieldOMLXHost, settings.EnvOMLXHost, runtimePanelLabelOMLXHost,
				func(r models.RuntimeInfo) string { return r.OMLXHost },
				func(s *settings.Settings) *string { return &s.OMLXHost }, settings.DefaultOMLXHost),
		},
		status: func(r models.RuntimeInfo) runtimeStatus {
			return runtimeStatus{found: models.ResolveOMLXPath(r) != "", running: r.OMLXRunning}
		},
	},
	{
		backend: models.BackendNInfer, name: "NInfer", format: formatNInfer,
		fields: []runtimeFieldDef{
			pathFieldDef(runtimeFieldNInferPath, settings.EnvNInferPath, "Path (checkout or ninfer-serve)",
				runtimePanelLabelNInferPath, func(r models.RuntimeInfo) string {
					return binaryPathPanelDisplay(models.ResolveNInferPath(r), r.NInferRunning, r.NInferPort)
				},
				func(s *settings.Settings) *string { return &s.NInferPath }),
			portFieldDef(runtimeFieldNInferPort, settings.EnvNInferServerPort, runtimePanelLabelNInferPort,
				func(r models.RuntimeInfo) int { return r.NInferPort },
				func(s *settings.Settings) *int { return &s.NInferServerPort }, settings.DefaultNInferServerPort),
			hostFieldDef(runtimeFieldNInferHost, settings.EnvNInferServerHost, runtimePanelLabelNInferHost,
				func(r models.RuntimeInfo) string { return r.NInferServerHost },
				func(s *settings.Settings) *string { return &s.NInferServerHost }, settings.DefaultNInferHost),
		},
		status: func(r models.RuntimeInfo) runtimeStatus {
			return runtimeStatus{found: models.ResolveNInferPath(r) != "", running: r.NInferRunning}
		},
	},
	{
		backend: models.BackendSplash, name: "Splash", format: formatSplash,
		fields: []runtimeFieldDef{
			pathFieldDef(runtimeFieldSplashPath, settings.EnvSplashPath, "Path (splash binary)",
				runtimePanelLabelSplashPath, func(r models.RuntimeInfo) string {
					return binaryPathPanelDisplay(models.ResolveSplashPath(r), r.SplashRunning, r.SplashPort)
				},
				func(s *settings.Settings) *string { return &s.SplashPath }),
			portFieldDef(runtimeFieldSplashPort, settings.EnvSplashPort, runtimePanelLabelSplashPort,
				func(r models.RuntimeInfo) int { return r.SplashPort },
				func(s *settings.Settings) *int { return &s.SplashPort }, settings.DefaultSplashPort),
			hostFieldDef(runtimeFieldSplashHost, settings.EnvSplashHost, runtimePanelLabelSplashHost,
				func(r models.RuntimeInfo) string { return r.SplashHost },
				func(s *settings.Settings) *string { return &s.SplashHost }, settings.DefaultSplashHost),
		},
		status: func(r models.RuntimeInfo) runtimeStatus {
			return runtimeStatus{found: models.ResolveSplashPath(r) != "", running: r.SplashRunning}
		},
	},
	{
		backend: models.BackendOllama, name: "Ollama", format: formatOllama,
		fields: []runtimeFieldDef{
			pathFieldDef(runtimeFieldOllamaPath, settings.EnvOllamaPath, "Path (ollama binary)",
				runtimePanelLabelOllamaPath, ollamaPathPanelDisplay,
				func(s *settings.Settings) *string { return &s.OllamaPath }),
			func() runtimeFieldDef {
				d := hostFieldDef(runtimeFieldOllamaHost, settings.EnvOllamaHost, runtimePanelLabelOllamaHost,
					func(r models.RuntimeInfo) string { return r.OllamaHost },
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

// runtimeForField returns the Runtime that owns field f.
func runtimeForField(f runtimeField) runtimeDef {
	for _, rt := range runtimeTable {
		for _, d := range rt.fields {
			if d.field == f {
				return rt
			}
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
