// Package profiles manages per-model launch parameters (model-params.json).
//
// Profiles are keyed by cleaned local model path and store named presets of extra
// environment variables, argv tokens, and structured metadata (backend, use case,
// hardware). The active profile is used when pressing R to launch a server.
//
// The shared profile catalog consumes this package's types to import/export
// portable TOML profiles; see docs/profile-format.md.
package profiles
