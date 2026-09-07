package settings

import (
	"os"
	"strings"

	"github.com/flyingnobita/llml/internal/fsutil"
)

// Getenv reads one environment variable. [FromEnv] takes one so tests can
// supply a map instead of mutating the real process environment.
type Getenv func(string) string

// OSGetenv reads the real process environment. It is the only place in the
// codebase that should be passed to [FromEnv] outside of tests.
func OSGetenv(key string) string { return os.Getenv(key) }

// FromEnv builds the highest-precedence layer from environment variables.
// Unset, empty, and unparseable values are left nil so a lower layer supplies
// them.
func FromEnv(getenv Getenv) Layer {
	var l Layer
	setPath(&l.LlamaCppPath, getenv(EnvLlamaCppPath))
	setPath(&l.VLLMPath, getenv(EnvVLLMPath))
	setPath(&l.VLLMVenv, getenv(EnvVLLMVenv))
	setPath(&l.OllamaPath, getenv(EnvOllamaPath))
	setPath(&l.KoboldCppPath, getenv(EnvKoboldCppPath))
	setPath(&l.HFHubCache, getenv(EnvHFHubCache))
	setPath(&l.HFHome, getenv(EnvHFHome))

	setHost(&l.LlamaServerHost, getenv(EnvLlamaServerHost))
	setHost(&l.VLLMServerHost, getenv(EnvVLLMServerHost))
	setOllamaHost(&l.OllamaHost, getenv(EnvOllamaHost))

	setPort(&l.LlamaServerPort, getenv(EnvLlamaServerPort))
	setPort(&l.VLLMServerPort, getenv(EnvVLLMServerPort))
	setPort(&l.KoboldCppPort, getenv(EnvKoboldCppPort))

	l.ExtraModelPaths = SplitPathList(getenv(EnvModelPaths))
	return l
}

func setPath(dst **string, raw string) {
	if v := fsutil.NormalizePath(raw); v != "" {
		*dst = &v
	}
}

func setHost(dst **string, raw string) {
	if v := normalizeHost(raw); v != "" {
		*dst = &v
	}
}

func setOllamaHost(dst **string, raw string) {
	if v := NormalizeOllamaHost(raw); v != "" {
		*dst = &v
	}
}

func setPort(dst **int, raw string) {
	if p, ok := parsePort(raw); ok {
		*dst = &p
	}
}

// NormalizeOllamaHost strips a scheme and trailing slash from an OLLAMA_HOST
// value, leaving bare host:port. It returns "" for input that carries no host,
// so the caller falls through to the next layer.
func NormalizeOllamaHost(raw string) string {
	v := strings.TrimSpace(raw)
	v = strings.TrimPrefix(v, "http://")
	v = strings.TrimPrefix(v, "https://")
	v = strings.TrimSuffix(v, "/")
	return strings.TrimSpace(v)
}
