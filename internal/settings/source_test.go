package settings

import "testing"

func TestSourceOfPathField(t *testing.T) {
	t.Parallel()

	cfg := Layer{Origin: OriginConfig, LlamaCppPath: ptr("/from/config")}

	fromEnv := Resolve(FromEnv(fakeEnv(map[string]string{EnvLlamaCppPath: "/from/env"})), cfg, Defaults())
	if got, want := fromEnv.Source(FieldLlamaCppPath), (Source{Origin: OriginEnv, EnvVar: EnvLlamaCppPath}); got != want {
		t.Errorf("env path: Source = %+v, want %+v", got, want)
	}

	fromConfig := Resolve(FromEnv(fakeEnv(nil)), cfg, Defaults())
	if got, want := fromConfig.Source(FieldLlamaCppPath), (Source{Origin: OriginConfig}); got != want {
		t.Errorf("config path: Source = %+v, want %+v", got, want)
	}

	unset := Resolve(FromEnv(fakeEnv(nil)), Layer{Origin: OriginConfig}, Defaults())
	if got, want := unset.Source(FieldLlamaCppPath), (Source{Origin: OriginDefault}); got != want {
		t.Errorf("unset path: Source = %+v, want %+v", got, want)
	}
}

func TestSourceOfHostAndPortFields(t *testing.T) {
	t.Parallel()

	env := FromEnv(fakeEnv(map[string]string{
		EnvVLLMServerHost:   "0.0.0.0",
		EnvNInferServerPort: "19000",
	}))
	cfg := Layer{
		Origin:           OriginConfig,
		SplashHost:       ptr("10.0.0.1"),
		KoboldCppPort:    ptr(5555),
		VLLMServerHost:   ptr("ignored"),
		NInferServerPort: ptr(1),
	}
	s := Resolve(env, cfg, Defaults())

	tests := []struct {
		field Field
		want  Source
	}{
		{FieldVLLMServerHost, Source{Origin: OriginEnv, EnvVar: EnvVLLMServerHost}},
		{FieldNInferServerPort, Source{Origin: OriginEnv, EnvVar: EnvNInferServerPort}},
		{FieldSplashHost, Source{Origin: OriginConfig}},
		{FieldKoboldCppPort, Source{Origin: OriginConfig}},
		{FieldLlamaServerHost, Source{Origin: OriginDefault}},
		{FieldOMLXPort, Source{Origin: OriginDefault}},
	}
	for _, tt := range tests {
		if got := s.Source(tt.field); got != tt.want {
			t.Errorf("Source(%s) = %+v, want %+v", tt.field.EnvVar(), got, tt.want)
		}
	}
}

func TestSourceOfOllamaHost(t *testing.T) {
	t.Parallel()

	cfg := Layer{Origin: OriginConfig, OllamaHost: ptr("cfg:11434")}

	s := Resolve(FromEnv(fakeEnv(map[string]string{EnvOllamaHost: "http://1.2.3.4:11434/"})), cfg, Defaults())
	if got, want := s.Source(FieldOllamaHost), (Source{Origin: OriginEnv, EnvVar: EnvOllamaHost}); got != want {
		t.Errorf("env: Source = %+v, want %+v", got, want)
	}

	// A scheme with no host is not a value, so config supplies it instead.
	s = Resolve(FromEnv(fakeEnv(map[string]string{EnvOllamaHost: "http://"})), cfg, Defaults())
	if s.OllamaHost != "cfg:11434" || s.Source(FieldOllamaHost) != (Source{Origin: OriginConfig}) {
		t.Errorf("hostless env: OllamaHost = %q from %v, want cfg:11434 from config", s.OllamaHost, s.Source(FieldOllamaHost))
	}

	s = Resolve(FromEnv(fakeEnv(nil)), Layer{Origin: OriginConfig}, Defaults())
	if s.OllamaHost != DefaultOllamaHost || s.Source(FieldOllamaHost) != (Source{Origin: OriginDefault}) {
		t.Errorf("unset: OllamaHost = %q from %v, want the default", s.OllamaHost, s.Source(FieldOllamaHost))
	}
}

func TestInvalidEnvValueReportsTheLayerThatSuppliedTheValue(t *testing.T) {
	t.Parallel()

	env := FromEnv(fakeEnv(map[string]string{
		EnvLlamaCppPath:    "   ",
		EnvLlamaServerPort: "70000",
		EnvVLLMServerPort:  "not-a-number",
		EnvOMLXHost:        "  ",
	}))
	cfg := Layer{
		Origin:          OriginConfig,
		LlamaCppPath:    ptr("/from/config"),
		LlamaServerPort: ptr(1234),
	}
	s := Resolve(env, cfg, Defaults())

	tests := []struct {
		field Field
		want  Source
	}{
		{FieldLlamaCppPath, Source{Origin: OriginConfig}},
		{FieldLlamaServerPort, Source{Origin: OriginConfig}},
		{FieldVLLMServerPort, Source{Origin: OriginDefault}},
		{FieldOMLXHost, Source{Origin: OriginDefault}},
	}
	for _, tt := range tests {
		if got := s.Source(tt.field); got != tt.want {
			t.Errorf("Source(%s) = %+v, want %+v", tt.field.EnvVar(), got, tt.want)
		}
	}
}

func TestEveryRuntimeFieldReportsItsOwnEnvVar(t *testing.T) {
	t.Parallel()

	// "8123" parses as a path, a host, an Ollama host, and a port alike.
	for f := range fieldCount {
		s := Resolve(FromEnv(fakeEnv(map[string]string{f.EnvVar(): "8123"})), Defaults())
		want := Source{Origin: OriginEnv, EnvVar: f.EnvVar()}
		if got := s.Source(f); got != want {
			t.Errorf("field %d: Source = %+v, want %+v", f, got, want)
		}
		for other := range fieldCount {
			if other != f && s.Source(other).Origin == OriginEnv {
				t.Errorf("setting %s also claimed field %d", f.EnvVar(), other)
			}
		}
	}
}

func TestSourceString(t *testing.T) {
	t.Parallel()

	tests := map[Source]string{
		{Origin: OriginEnv, EnvVar: EnvOMLXPort}: "OMLX_PORT",
		{Origin: OriginConfig}:                   "config",
		{Origin: OriginDefault}:                  "default",
	}
	for src, want := range tests {
		if got := src.String(); got != want {
			t.Errorf("%+v.String() = %q, want %q", src, got, want)
		}
	}
}
