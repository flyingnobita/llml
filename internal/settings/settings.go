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

	// ExtraModelPaths are additional filesystem roots to scan for models.
	ExtraModelPaths []string
	// HFHubCache and HFHome locate the Hugging Face hub cache. HFHubCache wins
	// when both are set; see [Settings.HuggingFaceHubCache].
	HFHubCache string
	HFHome     string
}

// Layer is a partial [Settings]: a nil field means "this source says nothing
// about that value", which is what lets a lower-precedence layer supply it.
type Layer struct {
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

	ExtraModelPaths []string
	HFHubCache      *string
	HFHome          *string
}

// Defaults returns the built-in layer. It sets every field that has a
// meaningful default, so [Resolve] always produces usable hosts and ports.
func Defaults() Layer {
	return Layer{
		LlamaServerHost: ptr(DefaultLlamaServerHost),
		LlamaServerPort: ptr(DefaultLlamaServerPort),
		VLLMServerHost:  ptr(DefaultVLLMServerHost),
		VLLMServerPort:  ptr(DefaultVLLMServerPort),
		OllamaHost:      ptr(DefaultOllamaHost),
		KoboldCppPort:   ptr(DefaultKoboldCppPort),
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
		takeString(&s.LlamaCppPath, l.LlamaCppPath)
		takeString(&s.LlamaServerHost, l.LlamaServerHost)
		takeInt(&s.LlamaServerPort, l.LlamaServerPort)
		takeString(&s.VLLMPath, l.VLLMPath)
		takeString(&s.VLLMVenv, l.VLLMVenv)
		takeString(&s.VLLMServerHost, l.VLLMServerHost)
		takeInt(&s.VLLMServerPort, l.VLLMServerPort)
		takeString(&s.OllamaPath, l.OllamaPath)
		takeString(&s.OllamaHost, l.OllamaHost)
		takeString(&s.KoboldCppPath, l.KoboldCppPath)
		takeInt(&s.KoboldCppPort, l.KoboldCppPort)
		takeString(&s.HFHubCache, l.HFHubCache)
		takeString(&s.HFHome, l.HFHome)
		roots.Add(l.ExtraModelPaths...)
	}
	s.ExtraModelPaths = roots.Slice()
	return s
}

// takeString assigns v to dst only if v is set and dst has not been claimed by
// a higher-precedence layer.
func takeString(dst *string, v *string) {
	if v != nil && *dst == "" {
		*dst = *v
	}
}

func takeInt(dst *int, v *int) {
	if v != nil && *dst == 0 {
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
