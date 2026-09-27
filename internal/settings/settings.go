// Package settings resolves llml's runtime configuration once, from layered
// sources, into a single value that is passed explicitly to the code that needs
// it.
//
// Precedence is environment variables, then the values persisted in
// config.toml, then built-in defaults. That order is expressed by the argument
// order of [Resolve]: the first layer that sets a field wins. No code in this
// package or its callers writes to the process environment; [FromEnv] is the
// only reader of it.
//
// This package deliberately imports nothing from config, models, or tui so that
// every one of those packages can depend on it.
package settings

import (
	"strconv"
	"strings"

	"github.com/flyingnobita/llml/internal/fsutil"
)

// Environment variables recognized by [FromEnv]. These are also the key names
// used for the corresponding `default_*` fields in config.toml's [runtime]
// table; see internal/config.
const (
	// EnvLlamaCppPath is a directory containing llama-cli and llama-server.
	EnvLlamaCppPath = "LLAMA_CPP_PATH"
	// EnvLlamaServerPort is the TCP port for llama-server and its /health probe.
	EnvLlamaServerPort = "LLAMA_SERVER_PORT"
	// EnvLlamaServerHost is the listen host for llama-server.
	EnvLlamaServerHost = "LLAMA_SERVER_HOST"
	// EnvVLLMServerPort is the TCP port for vllm serve.
	EnvVLLMServerPort = "VLLM_SERVER_PORT"
	// EnvVLLMServerHost is the listen host for vllm serve.
	EnvVLLMServerHost = "VLLM_SERVER_HOST"
	// EnvVLLMPath is a directory containing a vllm executable (checked before PATH).
	EnvVLLMPath = "VLLM_PATH"
	// EnvVLLMVenv is a Python venv root (the directory containing bin/activate on Unix).
	EnvVLLMVenv = "VLLM_VENV"
	// EnvOllamaPath is a directory or absolute binary path for the ollama executable.
	EnvOllamaPath = "OLLAMA_PATH"
	// EnvOllamaHost is the Ollama API host, as host:port.
	EnvOllamaHost = "OLLAMA_HOST"
	// EnvKoboldCppPath is a directory or absolute binary path for the koboldcpp executable.
	EnvKoboldCppPath = "KOBOLDCPP_PATH"
	// EnvKoboldCppPort is the TCP port for KoboldCpp.
	EnvKoboldCppPort = "KOBOLDCPP_PORT"
	// EnvNInferPath is an NInfer checkout root, a directory containing
	// ninfer-serve, or the absolute ninfer-serve path.
	EnvNInferPath = "NINFER_PATH"
	// EnvNInferServerPort is the TCP port for ninfer-serve and its /health probe.
	EnvNInferServerPort = "NINFER_SERVER_PORT"
	// EnvNInferServerHost is the listen host for ninfer-serve.
	EnvNInferServerHost = "NINFER_SERVER_HOST"
	// EnvOMLXPath is a directory containing the omlx CLI, the oMLX base
	// directory (~/.omlx, whose bin/ holds the app's CLI shim), or the omlx path.
	EnvOMLXPath = "OMLX_PATH"
	// EnvOMLXHost is the listen host for omlx serve.
	EnvOMLXHost = "OMLX_HOST"
	// EnvOMLXPort is the TCP port for omlx serve and its /health probe.
	EnvOMLXPort = "OMLX_PORT"
	// EnvOMLXModelDirs lists oMLX model directories, comma-separated. When
	// unset, the dirs configured in the oMLX app are used.
	EnvOMLXModelDirs = "OMLX_MODEL_DIRS"
	// EnvSplashPath is a directory containing the splash executable, or its path.
	EnvSplashPath = "SPLASH_PATH"
	// EnvSplashHost is the listen host for splash serve. Splash's own launcher
	// scripts use the same name.
	EnvSplashHost = "SPLASH_HOST"
	// EnvSplashPort is the TCP port for splash serve; Splash itself reads it too.
	EnvSplashPort = "SPLASH_PORT"
	// EnvModelPaths lists extra model search roots, comma-separated.
	EnvModelPaths = "LLML_MODEL_PATHS"
	// EnvHFHubCache overrides the Hugging Face hub cache directory.
	EnvHFHubCache = "HUGGINGFACE_HUB_CACHE"
	// EnvHFHome overrides HF_HOME; the hub cache then defaults to $HF_HOME/hub.
	EnvHFHome = "HF_HOME"
)

// Built-in defaults, used when neither the environment nor config.toml sets a value.
const (
	DefaultLlamaServerHost = "127.0.0.1"
	DefaultLlamaServerPort = 8080
	DefaultVLLMServerHost  = "127.0.0.1"
	// DefaultVLLMServerPort matches vLLM's own default listen port.
	DefaultVLLMServerPort = 8000
	DefaultKoboldCppPort  = 5001
	DefaultOllamaHost     = "127.0.0.1:11434"
	DefaultNInferHost     = "127.0.0.1"
	// DefaultNInferServerPort differs from ninfer-serve's own default (8080) so
	// it does not collide with llama-server's default port.
	DefaultNInferServerPort = 18080
	// DefaultOMLXHost and DefaultOMLXPort match oMLX's own defaults.
	DefaultOMLXHost = "127.0.0.1"
	DefaultOMLXPort = 8000
	// DefaultSplashHost and DefaultSplashPort match Splash's own defaults.
	DefaultSplashHost = "127.0.0.1"
	DefaultSplashPort = 8000
)

// Settings holds every runtime value in fully resolved form. Path fields are
// normalized (trimmed, tilde-expanded, cleaned) and are "" when unset. Host and
// port fields always hold a usable value, never a zero one.
type Settings struct {
	LlamaCppPath    string
	LlamaServerHost string
	LlamaServerPort int

	VLLMPath       string
	VLLMVenv       string
	VLLMServerHost string
	VLLMServerPort int

	OllamaPath string
	OllamaHost string

	KoboldCppPath string
	KoboldCppPort int

	NInferPath       string
	NInferServerHost string
	NInferServerPort int

	OMLXPath string
	OMLXHost string
	OMLXPort int
	// OMLXModelDirs are oMLX's model directories. Models found under them are
	// oMLX rows; see [Settings.OMLXModelRoots] for the fallback when empty.
	OMLXModelDirs []string

	SplashPath string
	SplashHost string
	SplashPort int

	// ExtraModelPaths are additional filesystem roots to scan for models.
	ExtraModelPaths []string
	// HFHubCache and HFHome locate the Hugging Face hub cache. HFHubCache wins
	// when both are set; see [Settings.HuggingFaceHubCache].
	HFHubCache string
	HFHome     string

	// sources records which layer supplied each runtime field; see [Settings.Source].
	sources [fieldCount]Source
}

// Layer is a partial [Settings]: a nil field means "this source says nothing
// about that value", which is what lets a lower-precedence layer supply it.
type Layer struct {
	// Origin is the kind of source this layer reads, reported by
	// [Settings.Source] for each runtime field the layer supplies.
	Origin Origin

	LlamaCppPath    *string
	LlamaServerHost *string
	LlamaServerPort *int

	VLLMPath       *string
	VLLMVenv       *string
	VLLMServerHost *string
	VLLMServerPort *int

	OllamaPath *string
	OllamaHost *string

	KoboldCppPath *string
	KoboldCppPort *int

	NInferPath       *string
	NInferServerHost *string
	NInferServerPort *int

	OMLXPath *string
	OMLXHost *string
	OMLXPort *int
	// OMLXModelDirs is taken whole from the first layer that sets it; unlike
	// ExtraModelPaths it does not accumulate, since it names oMLX's own
	// configuration rather than extra places to look.
	OMLXModelDirs []string

	SplashPath *string
	SplashHost *string
	SplashPort *int

	ExtraModelPaths []string
	HFHubCache      *string
	HFHome          *string
}

// Defaults returns the built-in layer. It sets every field that has a
// meaningful default, so [Resolve] always produces usable hosts and ports.
func Defaults() Layer {
	return Layer{
		Origin: OriginDefault,

		LlamaServerHost: ptr(DefaultLlamaServerHost),
		LlamaServerPort: ptr(DefaultLlamaServerPort),
		VLLMServerHost:  ptr(DefaultVLLMServerHost),
		VLLMServerPort:  ptr(DefaultVLLMServerPort),
		OllamaHost:      ptr(DefaultOllamaHost),
		KoboldCppPort:   ptr(DefaultKoboldCppPort),

		NInferServerHost: ptr(DefaultNInferHost),
		NInferServerPort: ptr(DefaultNInferServerPort),

		OMLXHost:   ptr(DefaultOMLXHost),
		OMLXPort:   ptr(DefaultOMLXPort),
		SplashHost: ptr(DefaultSplashHost),
		SplashPort: ptr(DefaultSplashPort),
	}
}

// Resolve folds layers into a single [Settings]. The first layer that sets a
// field determines its value, so callers pass layers in descending precedence:
// FromEnv, then the config.toml layer, then Defaults.
//
// ExtraModelPaths is the one exception: it accumulates across all layers,
// deduplicated, because extra search roots from the environment and from
// config.toml are additive rather than competing.
func Resolve(layers ...Layer) Settings {
	var s Settings
	var roots fsutil.PathSet
	for _, l := range layers {
		take(&s, l.Origin, FieldLlamaCppPath, &s.LlamaCppPath, l.LlamaCppPath)
		take(&s, l.Origin, FieldLlamaServerHost, &s.LlamaServerHost, l.LlamaServerHost)
		take(&s, l.Origin, FieldLlamaServerPort, &s.LlamaServerPort, l.LlamaServerPort)
		take(&s, l.Origin, FieldVLLMPath, &s.VLLMPath, l.VLLMPath)
		take(&s, l.Origin, FieldVLLMVenv, &s.VLLMVenv, l.VLLMVenv)
		take(&s, l.Origin, FieldVLLMServerHost, &s.VLLMServerHost, l.VLLMServerHost)
		take(&s, l.Origin, FieldVLLMServerPort, &s.VLLMServerPort, l.VLLMServerPort)
		take(&s, l.Origin, FieldOllamaPath, &s.OllamaPath, l.OllamaPath)
		take(&s, l.Origin, FieldOllamaHost, &s.OllamaHost, l.OllamaHost)
		take(&s, l.Origin, FieldKoboldCppPath, &s.KoboldCppPath, l.KoboldCppPath)
		take(&s, l.Origin, FieldKoboldCppPort, &s.KoboldCppPort, l.KoboldCppPort)
		take(&s, l.Origin, FieldNInferPath, &s.NInferPath, l.NInferPath)
		take(&s, l.Origin, FieldNInferServerHost, &s.NInferServerHost, l.NInferServerHost)
		take(&s, l.Origin, FieldNInferServerPort, &s.NInferServerPort, l.NInferServerPort)
		take(&s, l.Origin, FieldOMLXPath, &s.OMLXPath, l.OMLXPath)
		take(&s, l.Origin, FieldOMLXHost, &s.OMLXHost, l.OMLXHost)
		take(&s, l.Origin, FieldOMLXPort, &s.OMLXPort, l.OMLXPort)
		if len(s.OMLXModelDirs) == 0 && len(l.OMLXModelDirs) > 0 {
			var dirs fsutil.PathSet
			dirs.Add(l.OMLXModelDirs...)
			s.OMLXModelDirs = dirs.Slice()
		}
		take(&s, l.Origin, FieldSplashPath, &s.SplashPath, l.SplashPath)
		take(&s, l.Origin, FieldSplashHost, &s.SplashHost, l.SplashHost)
		take(&s, l.Origin, FieldSplashPort, &s.SplashPort, l.SplashPort)
		takeString(&s.HFHubCache, l.HFHubCache)
		takeString(&s.HFHome, l.HFHome)
		roots.Add(l.ExtraModelPaths...)
	}
	s.ExtraModelPaths = roots.Slice()
	return s
}

// take assigns v to dst, and records origin as the source of f, only if v is
// set and dst has not been claimed by a higher-precedence layer. A zero v claims
// nothing, so it cannot record a source for a value it did not supply.
func take[T comparable](s *Settings, origin Origin, f Field, dst, v *T) {
	var zero T
	if v == nil || *v == zero || *dst != zero {
		return
	}
	*dst = *v
	s.sources[f] = origin.source(f)
}

// takeString assigns v to dst only if v is set and dst has not been claimed by
// a higher-precedence layer.
func takeString(dst *string, v *string) {
	if v != nil && *dst == "" {
		*dst = *v
	}
}

func ptr[T any](v T) *T { return &v }

// normalizeHost trims raw and returns "" when it holds nothing usable, so an
// empty or whitespace-only value falls through to the next layer.
func normalizeHost(raw string) string { return strings.TrimSpace(raw) }

// parsePort returns the port encoded in raw, and false when raw is empty or is
// not a valid TCP port. An invalid value is treated as unset so that a typo in
// the environment falls back to config.toml or the default rather than
// producing an unusable port.
func parsePort(raw string) (int, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0, false
	}
	p, err := strconv.Atoi(raw)
	if err != nil || p <= 0 || p > 65535 {
		return 0, false
	}
	return p, true
}

// SplitPathList splits a comma-separated list of paths, dropping empties.
func SplitPathList(v string) []string {
	var out []string
	for part := range strings.SplitSeq(v, ",") {
		if part = strings.TrimSpace(part); part != "" {
			out = append(out, part)
		}
	}
	return out
}
