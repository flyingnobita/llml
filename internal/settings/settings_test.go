package settings

import (
	"path/filepath"
	"slices"
	"testing"
)

// fakeEnv returns a Getenv backed by m, so tests never mutate the real
// environment and can therefore run in parallel.
func fakeEnv(m map[string]string) Getenv {
	return func(k string) string { return m[k] }
}

func TestResolveEnvBeatsConfigBeatsDefault(t *testing.T) {
	t.Parallel()

	env := FromEnv(fakeEnv(map[string]string{
		EnvLlamaCppPath:    "/from/env",
		EnvLlamaServerPort: "9999",
	}))
	cfg := Layer{
		LlamaCppPath:    ptr("/from/config"),
		LlamaServerPort: ptr(7777),
		VLLMServerPort:  ptr(7000),
	}

	s := Resolve(env, cfg, Defaults())

	if s.LlamaCppPath != "/from/env" {
		t.Errorf("LlamaCppPath: env should win, got %q", s.LlamaCppPath)
	}
	if s.LlamaServerPort != 9999 {
		t.Errorf("LlamaServerPort: env should win, got %d", s.LlamaServerPort)
	}
	if s.VLLMServerPort != 7000 {
		t.Errorf("VLLMServerPort: config should win over default, got %d", s.VLLMServerPort)
	}
	if s.KoboldCppPort != DefaultKoboldCppPort {
		t.Errorf("KoboldCppPort: default should apply, got %d", s.KoboldCppPort)
	}
	if s.LlamaServerHost != DefaultLlamaServerHost {
		t.Errorf("LlamaServerHost: default should apply, got %q", s.LlamaServerHost)
	}
}

func TestResolveDefaultsAlwaysProduceUsableHostsAndPorts(t *testing.T) {
	t.Parallel()

	s := Resolve(FromEnv(fakeEnv(nil)), Layer{}, Defaults())

	if s.LlamaServerPort != DefaultLlamaServerPort ||
		s.VLLMServerPort != DefaultVLLMServerPort ||
		s.KoboldCppPort != DefaultKoboldCppPort {
		t.Errorf("ports not defaulted: %+v", s)
	}
	if s.LlamaServerHost == "" || s.VLLMServerHost == "" || s.OllamaHost == "" {
		t.Errorf("hosts not defaulted: %+v", s)
	}
}

func TestFromEnvIgnoresBlankAndInvalidValues(t *testing.T) {
	t.Parallel()

	// A whitespace-only path and an out-of-range port must not claim the field,
	// so a lower-precedence layer still gets to supply it.
	env := FromEnv(fakeEnv(map[string]string{
		EnvLlamaCppPath:    "   ",
		EnvLlamaServerPort: "70000",
		EnvVLLMServerPort:  "not-a-number",
		EnvKoboldCppPort:   "0",
	}))
	s := Resolve(env, Layer{
		LlamaCppPath:    ptr("/from/config"),
		LlamaServerPort: ptr(1234),
	}, Defaults())

	if s.LlamaCppPath != "/from/config" {
		t.Errorf("blank env path should not win, got %q", s.LlamaCppPath)
	}
	if s.LlamaServerPort != 1234 {
		t.Errorf("out-of-range env port should not win, got %d", s.LlamaServerPort)
	}
	if s.VLLMServerPort != DefaultVLLMServerPort {
		t.Errorf("unparseable env port should fall through, got %d", s.VLLMServerPort)
	}
	if s.KoboldCppPort != DefaultKoboldCppPort {
		t.Errorf("zero env port should fall through, got %d", s.KoboldCppPort)
	}
}

// Not parallel: tilde expansion resolves the real home directory, so this is
// the one test here that has to touch the process environment.
func TestFromEnvNormalizesPaths(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	s := Resolve(FromEnv(fakeEnv(map[string]string{
		EnvLlamaCppPath: "~/llama/build/../bin",
	})), Defaults())

	want := filepath.Join(home, "llama", "bin")
	if s.LlamaCppPath != want {
		t.Errorf("LlamaCppPath = %q, want %q", s.LlamaCppPath, want)
	}
}

func TestExtraModelPathsAccumulateAcrossLayers(t *testing.T) {
	t.Parallel()

	env := FromEnv(fakeEnv(map[string]string{
		EnvModelPaths: "/a, /b ,, /a",
	}))
	cfg := Layer{ExtraModelPaths: []string{"/c", "/b"}}

	s := Resolve(env, cfg, Defaults())

	want := []string{"/a", "/b", "/c"}
	if !slices.Equal(s.ExtraModelPaths, want) {
		t.Errorf("ExtraModelPaths = %v, want %v", s.ExtraModelPaths, want)
	}
}

func TestNormalizeOllamaHost(t *testing.T) {
	t.Parallel()

	cases := map[string]string{
		"http://1.2.3.4:11434/": "1.2.3.4:11434",
		"https://host:1/":       "host:1",
		"  127.0.0.1:11434  ":   "127.0.0.1:11434",
		"":                      "",
		"   ":                   "",
		"http://":               "",
	}
	for in, want := range cases {
		if got := NormalizeOllamaHost(in); got != want {
			t.Errorf("NormalizeOllamaHost(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestOllamaHostFallsBackToDefaultWhenEnvHasNoHost(t *testing.T) {
	t.Parallel()

	s := Resolve(FromEnv(fakeEnv(map[string]string{EnvOllamaHost: "http://"})), Defaults())
	if s.OllamaHost != DefaultOllamaHost {
		t.Errorf("OllamaHost = %q, want %q", s.OllamaHost, DefaultOllamaHost)
	}
	if got, want := s.OllamaAPIBaseURL(), "http://"+DefaultOllamaHost+"/api"; got != want {
		t.Errorf("OllamaAPIBaseURL = %q, want %q", got, want)
	}
}

func TestHuggingFaceHubCachePrecedence(t *testing.T) {
	t.Parallel()

	home := "/home/u"
	if got, want := (Settings{HFHubCache: "/cache"}).HuggingFaceHubCache(home), "/cache"; got != want {
		t.Errorf("HFHubCache should win: got %q, want %q", got, want)
	}
	if got, want := (Settings{HFHome: "/hf"}).HuggingFaceHubCache(home), filepath.Join("/hf", "hub"); got != want {
		t.Errorf("HFHome should be used: got %q, want %q", got, want)
	}
	want := filepath.Join(home, ".cache", "huggingface", "hub")
	if got := (Settings{}).HuggingFaceHubCache(home); got != want {
		t.Errorf("fallback: got %q, want %q", got, want)
	}
}

func TestSearchRootsSkipDefaultsAndDeduplicates(t *testing.T) {
	t.Parallel()

	s := Settings{ExtraModelPaths: []string{"/x", "/y"}}
	got := s.SearchRoots([]string{"/y", "/z"}, true)
	want := []string{"/x", "/y", "/z"}
	if !slices.Equal(got, want) {
		t.Errorf("SearchRoots = %v, want %v", got, want)
	}
}

func TestHuggingFaceHubCacheFromEnv(t *testing.T) {
	t.Parallel()

	home := "/home/u"
	def := filepath.Join(home, ".cache", "huggingface", "hub")
	if got := Resolve(FromEnv(fakeEnv(nil)), Defaults()).HuggingFaceHubCache(home); got != def {
		t.Errorf("default: got %q want %q", got, def)
	}

	s := Resolve(FromEnv(fakeEnv(map[string]string{EnvHFHome: "/hfhome"})), Defaults())
	if got, want := s.HuggingFaceHubCache(home), filepath.Join("/hfhome", "hub"); got != want {
		t.Errorf("HF_HOME: got %q want %q", got, want)
	}

	// HUGGINGFACE_HUB_CACHE wins over HF_HOME.
	s = Resolve(FromEnv(fakeEnv(map[string]string{
		EnvHFHubCache: "/custom/hub",
		EnvHFHome:     "/ignored",
	})), Defaults())
	if got, want := s.HuggingFaceHubCache(home), "/custom/hub"; got != want {
		t.Errorf("HUGGINGFACE_HUB_CACHE: got %q want %q", got, want)
	}
}
