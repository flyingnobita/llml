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
	if s.NInferServerPort != DefaultNInferServerPort || s.NInferServerHost != DefaultNInferHost {
		t.Errorf("ninfer not defaulted: %+v", s)
	}
}

// Every built-in default port matches the one the Runtime's own server
// listens on, so clients pointed at the upstream port reach it. Runtimes that
// share a port are told apart by detection.
func TestDefaultPortsMatchUpstream(t *testing.T) {
	t.Parallel()

	s := Resolve(FromEnv(fakeEnv(nil)), Layer{}, Defaults())
	for _, tc := range []struct {
		server    string
		got, want int
	}{
		{"llama-server", s.LlamaServerPort, 8080},
		{"ninfer-serve", s.NInferServerPort, 8080},
		{"vllm serve", s.VLLMServerPort, 8000},
		{"koboldcpp", s.KoboldCppPort, 5001},
		{"omlx serve", s.OMLXPort, 8000},
		{"splash serve", s.SplashPort, 8000},
		{"mlx_lm.server", s.MLXLMPort, 8080},
		{"mlx_vlm.server", s.MLXVLMPort, 8080},
	} {
		if tc.got != tc.want {
			t.Errorf("%s default port = %d, want %d", tc.server, tc.got, tc.want)
		}
	}
}

func TestFromEnvReadsNInfer(t *testing.T) {
	t.Parallel()

	s := Resolve(FromEnv(fakeEnv(map[string]string{
		EnvNInferPath:       "/opt/ninfer/",
		EnvNInferServerPort: "19000",
		EnvNInferServerHost: "0.0.0.0",
	})), Defaults())

	if s.NInferPath != "/opt/ninfer" || s.NInferServerPort != 19000 || s.NInferServerHost != "0.0.0.0" {
		t.Errorf("ninfer env not applied: %+v", s)
	}
}

func TestFromEnvReadsOMLXAndSplash(t *testing.T) {
	t.Parallel()

	s := Resolve(FromEnv(fakeEnv(map[string]string{
		EnvOMLXPath:      "/Users/u/.omlx",
		EnvOMLXPort:      "8100",
		EnvOMLXModelDirs: "/a, /b/",
		EnvSplashHost:    "0.0.0.0",
		EnvSplashPort:    "8200",
	})), Layer{OMLXModelDirs: []string{"/from/app"}}, Defaults())

	if s.OMLXPath != "/Users/u/.omlx" || s.OMLXPort != 8100 || s.OMLXHost != DefaultOMLXHost {
		t.Errorf("oMLX = %q %q %d", s.OMLXPath, s.OMLXHost, s.OMLXPort)
	}
	if s.SplashHost != "0.0.0.0" || s.SplashPort != 8200 {
		t.Errorf("Splash = %q %d", s.SplashHost, s.SplashPort)
	}
	// The env list replaces the app's dirs rather than adding to them.
	if !slices.Equal(s.OMLXModelDirs, []string{"/a", "/b"}) {
		t.Errorf("OMLXModelDirs = %v", s.OMLXModelDirs)
	}
}

// mlx-lm's path, host, and port each come from the environment, then
// config.toml, then the built-in defaults, which match mlx_lm.server's own.
func TestResolveMLXLMPrecedence(t *testing.T) {
	t.Parallel()

	cfg := Layer{
		Origin:    OriginConfig,
		MLXLMPath: ptr("/from/config/bin"),
		MLXLMHost: ptr("10.0.0.1"),
		MLXLMPort: ptr(8181),
	}
	env := FromEnv(fakeEnv(map[string]string{
		EnvMLXLMPath: " /venv/bin/mlx_lm.server ",
		EnvMLXLMHost: "0.0.0.0",
		EnvMLXLMPort: "9191",
	}))

	s := Resolve(env, cfg, Defaults())
	if s.MLXLMPath != "/venv/bin/mlx_lm.server" || s.MLXLMHost != "0.0.0.0" || s.MLXLMPort != 9191 {
		t.Errorf("env should win: %q %q %d", s.MLXLMPath, s.MLXLMHost, s.MLXLMPort)
	}
	for f, want := range map[Field]string{FieldMLXLMPath: EnvMLXLMPath, FieldMLXLMHost: EnvMLXLMHost, FieldMLXLMPort: EnvMLXLMPort} {
		if got := s.Source(f).String(); got != want {
			t.Errorf("Source(%s) = %q, want %q", f.EnvVar(), got, want)
		}
	}

	s = Resolve(FromEnv(fakeEnv(nil)), cfg, Defaults())
	if s.MLXLMPath != "/from/config/bin" || s.MLXLMHost != "10.0.0.1" || s.MLXLMPort != 8181 {
		t.Errorf("config should win over defaults: %q %q %d", s.MLXLMPath, s.MLXLMHost, s.MLXLMPort)
	}
	if got := s.Source(FieldMLXLMPort).String(); got != "config" {
		t.Errorf("Source(MLX_LM_PORT) = %q, want config", got)
	}

	s = Resolve(FromEnv(fakeEnv(map[string]string{EnvMLXLMPort: "not-a-port"})), Layer{Origin: OriginConfig}, Defaults())
	if s.MLXLMPath != "" || s.MLXLMHost != DefaultMLXLMHost || s.MLXLMPort != DefaultMLXLMPort {
		t.Errorf("defaults should apply: %q %q %d", s.MLXLMPath, s.MLXLMHost, s.MLXLMPort)
	}
	if DefaultMLXLMHost != "127.0.0.1" {
		t.Errorf("DefaultMLXLMHost = %q, want mlx_lm.server's 127.0.0.1", DefaultMLXLMHost)
	}
	if got := s.Source(FieldMLXLMPort).String(); got != "default" {
		t.Errorf("an invalid port should fall through to the default, source %q", got)
	}
}

// mlx-vlm's path, host, and port each come from the environment, then
// config.toml, then the built-in defaults. The default host is loopback,
// although mlx_vlm.server's own is 0.0.0.0: llml always passes --host.
func TestResolveMLXVLMPrecedence(t *testing.T) {
	t.Parallel()

	cfg := Layer{
		Origin:     OriginConfig,
		MLXVLMPath: ptr("/from/config/bin"),
		MLXVLMHost: ptr("10.0.0.1"),
		MLXVLMPort: ptr(8181),
	}
	env := FromEnv(fakeEnv(map[string]string{
		EnvMLXVLMPath: " /venv/bin/mlx_vlm.server ",
		EnvMLXVLMHost: "0.0.0.0",
		EnvMLXVLMPort: "9191",
	}))

	s := Resolve(env, cfg, Defaults())
	if s.MLXVLMPath != "/venv/bin/mlx_vlm.server" || s.MLXVLMHost != "0.0.0.0" || s.MLXVLMPort != 9191 {
		t.Errorf("env should win: %q %q %d", s.MLXVLMPath, s.MLXVLMHost, s.MLXVLMPort)
	}
	for f, want := range map[Field]string{FieldMLXVLMPath: EnvMLXVLMPath, FieldMLXVLMHost: EnvMLXVLMHost, FieldMLXVLMPort: EnvMLXVLMPort} {
		if got := s.Source(f).String(); got != want {
			t.Errorf("Source(%s) = %q, want %q", f.EnvVar(), got, want)
		}
	}
	for want, got := range map[string]string{"MLX_VLM_PATH": EnvMLXVLMPath, "MLX_VLM_HOST": EnvMLXVLMHost, "MLX_VLM_PORT": EnvMLXVLMPort} {
		if got != want {
			t.Errorf("environment variable %q, want %q", got, want)
		}
	}

	s = Resolve(FromEnv(fakeEnv(nil)), cfg, Defaults())
	if s.MLXVLMPath != "/from/config/bin" || s.MLXVLMHost != "10.0.0.1" || s.MLXVLMPort != 8181 {
		t.Errorf("config should win over defaults: %q %q %d", s.MLXVLMPath, s.MLXVLMHost, s.MLXVLMPort)
	}
	if got := s.Source(FieldMLXVLMPort).String(); got != "config" {
		t.Errorf("Source(MLX_VLM_PORT) = %q, want config", got)
	}

	s = Resolve(FromEnv(fakeEnv(map[string]string{EnvMLXVLMPort: "not-a-port"})), Layer{Origin: OriginConfig}, Defaults())
	if s.MLXVLMPath != "" || s.MLXVLMHost != DefaultMLXVLMHost || s.MLXVLMPort != DefaultMLXVLMPort {
		t.Errorf("defaults should apply: %q %q %d", s.MLXVLMPath, s.MLXVLMHost, s.MLXVLMPort)
	}
	if DefaultMLXVLMHost != "127.0.0.1" {
		t.Errorf("DefaultMLXVLMHost = %q, want loopback, not mlx_vlm.server's 0.0.0.0", DefaultMLXVLMHost)
	}
	if got := s.Source(FieldMLXVLMPort).String(); got != "default" {
		t.Errorf("an invalid port should fall through to the default, source %q", got)
	}
}

func TestOMLXModelRoots(t *testing.T) {
	t.Parallel()

	if got := (Settings{}).OMLXModelRoots("/Users/u"); !slices.Equal(got, []string{"/Users/u/.omlx/models"}) {
		t.Errorf("default roots = %v", got)
	}
	if got := (Settings{}).OMLXModelRoots(""); got != nil {
		t.Errorf("no home should mean no default, got %v", got)
	}
	s := Settings{OMLXModelDirs: []string{"/x"}}
	if got := s.OMLXModelRoots("/Users/u"); !slices.Equal(got, []string{"/x"}) {
		t.Errorf("configured roots = %v", got)
	}
}

func TestNInferModelsDir(t *testing.T) {
	t.Parallel()

	tests := []struct {
		path, want string
	}{
		{"", ""},
		{"/opt/ninfer", "/opt/ninfer/models"},
		{"/opt/ninfer/build/apps", "/opt/ninfer/models"},
		{"/opt/ninfer/build/apps/ninfer-serve", "/opt/ninfer/models"},
		{"/usr/local/bin/ninfer-serve", "/usr/local/bin/models"},
	}
	for _, tt := range tests {
		if got := (Settings{NInferPath: tt.path}).NInferModelsDir(); got != tt.want {
			t.Errorf("NInferModelsDir(%q) = %q, want %q", tt.path, got, tt.want)
		}
	}
}

func TestDefaultSearchRootsIncludeNInferModels(t *testing.T) {
	t.Parallel()

	with := Settings{NInferPath: "/opt/ninfer"}.DefaultSearchRoots()
	if with != nil && !slices.Contains(with, "/opt/ninfer/models") {
		t.Errorf("roots %v missing the NInfer models dir", with)
	}
	for _, r := range (Settings{}).DefaultSearchRoots() {
		if filepath.Base(r) == "models" && filepath.Base(filepath.Dir(r)) == "ninfer" {
			t.Errorf("unexpected NInfer root %q without NInferPath", r)
		}
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
