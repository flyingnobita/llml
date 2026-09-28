package settings

// Field names one runtime value: a key of config.toml's [runtime] table and
// the environment variable that overrides it. [Settings.Source] reports which
// layer supplied each one.
type Field int

// Runtime fields, one per key of config.toml's [runtime] table.
const (
	FieldLlamaCppPath Field = iota
	FieldLlamaServerHost
	FieldLlamaServerPort
	FieldVLLMPath
	FieldVLLMVenv
	FieldVLLMServerHost
	FieldVLLMServerPort
	FieldOllamaPath
	FieldOllamaHost
	FieldKoboldCppPath
	FieldKoboldCppPort
	FieldNInferPath
	FieldNInferServerHost
	FieldNInferServerPort
	FieldOMLXPath
	FieldOMLXHost
	FieldOMLXPort
	FieldSplashPath
	FieldSplashHost
	FieldSplashPort
	FieldMLXLMPath
	FieldMLXLMHost
	FieldMLXLMPort
	FieldMLXVLMPath
	FieldMLXVLMHost
	FieldMLXVLMPort

	fieldCount
)

var fieldEnvVars = [fieldCount]string{
	FieldLlamaCppPath:     EnvLlamaCppPath,
	FieldLlamaServerHost:  EnvLlamaServerHost,
	FieldLlamaServerPort:  EnvLlamaServerPort,
	FieldVLLMPath:         EnvVLLMPath,
	FieldVLLMVenv:         EnvVLLMVenv,
	FieldVLLMServerHost:   EnvVLLMServerHost,
	FieldVLLMServerPort:   EnvVLLMServerPort,
	FieldOllamaPath:       EnvOllamaPath,
	FieldOllamaHost:       EnvOllamaHost,
	FieldKoboldCppPath:    EnvKoboldCppPath,
	FieldKoboldCppPort:    EnvKoboldCppPort,
	FieldNInferPath:       EnvNInferPath,
	FieldNInferServerHost: EnvNInferServerHost,
	FieldNInferServerPort: EnvNInferServerPort,
	FieldOMLXPath:         EnvOMLXPath,
	FieldOMLXHost:         EnvOMLXHost,
	FieldOMLXPort:         EnvOMLXPort,
	FieldSplashPath:       EnvSplashPath,
	FieldSplashHost:       EnvSplashHost,
	FieldSplashPort:       EnvSplashPort,
	FieldMLXLMPath:        EnvMLXLMPath,
	FieldMLXLMHost:        EnvMLXLMHost,
	FieldMLXLMPort:        EnvMLXLMPort,
	FieldMLXVLMPath:       EnvMLXVLMPath,
	FieldMLXVLMHost:       EnvMLXVLMHost,
	FieldMLXVLMPort:       EnvMLXVLMPort,
}

// EnvVar returns the environment variable that sets f.
func (f Field) EnvVar() string { return fieldEnvVars[f] }

// Origin identifies the kind of layer a value came from.
type Origin int

const (
	// OriginDefault is the built-in default, and also what an unset path
	// reports: its default is empty. It is the zero value, so a [Layer] built
	// without an Origin reports its values as defaults.
	OriginDefault Origin = iota
	// OriginConfig is config.toml.
	OriginConfig
	// OriginEnv is the process environment.
	OriginEnv
)

// Source says where a resolved value came from.
type Source struct {
	Origin Origin
	// EnvVar names the variable that supplied the value when Origin is
	// OriginEnv, and is empty otherwise.
	EnvVar string
}

// String returns the variable name for an environment source, and "config" or
// "default" otherwise.
func (s Source) String() string {
	switch s.Origin {
	case OriginEnv:
		return s.EnvVar
	case OriginConfig:
		return "config"
	default:
		return "default"
	}
}

func (o Origin) source(f Field) Source {
	if o == OriginEnv {
		return Source{Origin: OriginEnv, EnvVar: f.EnvVar()}
	}
	return Source{Origin: o}
}

// Source reports which layer supplied f in the [Resolve] call that produced s.
// Settings edited after resolution keep the sources [Resolve] recorded.
func (s Settings) Source(f Field) Source { return s.sources[f] }
