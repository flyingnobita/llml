#!/usr/bin/env bash
# Fail if the active Go toolchain does not match the `go` directive in go.mod.
# Both mise.toml and .github/workflows/release.yml resolve Go from go.mod, and
# golangci-lint is pinned to a release built against that same version. Drift
# makes the linter panic with "file requires newer Go version".
set -euo pipefail

want=$(awk '/^go /{print $2; exit}' go.mod)
# GOTOOLCHAIN=local reports the toolchain mise actually installed. Without it,
# Go silently downloads whatever go.mod asks for and the drift goes unnoticed
# even though golangci-lint is still the binary mise pinned.
got=$(GOTOOLCHAIN=local go env GOVERSION)
got=${got#go}

if [[ "$want" != "$got" ]]; then
  echo >&2 "Go toolchain mismatch: go.mod wants ${want}, active toolchain is ${got}."
  echo >&2 "Set the same version for \`go\` in mise.toml, then run: mise install"
  exit 1
fi
