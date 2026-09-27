package models

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
)

var commonBinaryDirs = []string{
	"/usr/local/bin",
	"/opt/homebrew/bin",
}

// findBinaryInEnvAndCommonDirs resolves name as $envDir/name, then each of commonDirs/name,
// then [exec.LookPath]. envDir may be empty (skip that step).
func findBinaryInEnvAndCommonDirs(name, envDir string, commonDirs []string) string {
	if envDir != "" {
		clean := filepath.Clean(envDir)
		if isRegularFile(clean) && filepath.Base(clean) == name {
			return clean
		}
		candidate := filepath.Join(clean, name)
		if isRegularFile(candidate) {
			return candidate
		}
	}
	for _, dir := range commonDirs {
		candidate := filepath.Join(dir, name)
		if isRegularFile(candidate) {
			return candidate
		}
	}
	if p, err := exec.LookPath(name); err == nil {
		return p
	}
	return ""
}

// findVLLMBinary resolves the vllm executable from vllmPath, then venvRoot, then
// common install directories, then PATH. Both parameters may be empty.
func findVLLMBinary(vllmPath, venvRoot string) string {
	if dir := vllmPath; dir != "" {
		clean := filepath.Clean(dir)
		if isRegularFile(clean) && filepath.Base(clean) == "vllm" {
			return clean
		}
		candidate := filepath.Join(clean, "vllm")
		if isRegularFile(candidate) {
			return candidate
		}
		// vllm often lives only at $VLLM_PATH/.venv/bin/vllm until the venv is activated.
		if p := vllmBinaryInProjectDotVenv(clean); p != "" {
			return p
		}
	}
	if d := strings.TrimSpace(venvRoot); d != "" {
		if p := vllmBinaryInVenvRoot(d); p != "" {
			return p
		}
	}
	common := slices.Clone(commonBinaryDirs)
	if home, err := os.UserHomeDir(); err == nil {
		common = append(common, filepath.Join(home, ".local", "bin"))
		if runtime.GOOS == "darwin" {
			// Common local layout for Apple Silicon / Metal vLLM installs.
			common = append(common, filepath.Join(home, ".venv-vllm-metal", "bin"))
		}
	}
	return findBinaryInEnvAndCommonDirs("vllm", "", common)
}

// findLlamaBinary resolves name from llamaCppPath, then common install
// directories, then PATH. llamaCppPath may be empty.
func findLlamaBinary(name, llamaCppPath string) string {
	var envDir string
	if d := llamaCppPath; d != "" {
		envDir = filepath.Clean(d)
	}
	common := slices.Clone(commonBinaryDirs)
	common = append(common, "/opt/llama.cpp/build/bin")
	if home, err := os.UserHomeDir(); err == nil {
		common = append(common, filepath.Join(home, ".local", "bin"))
	}
	return findBinaryInEnvAndCommonDirs(name, envDir, common)
}

// findOllamaBinary resolves the ollama executable from ollamaPath, then common
// install directories, then PATH. ollamaPath may be empty.
func findOllamaBinary(ollamaPath string) string {
	var envDir string
	if d := ollamaPath; d != "" {
		envDir = filepath.Clean(d)
	}
	common := slices.Clone(commonBinaryDirs)
	if home, err := os.UserHomeDir(); err == nil {
		common = append(common, filepath.Join(home, ".local", "bin"))
	}
	return findBinaryInEnvAndCommonDirs("ollama", envDir, common)
}

// koboldcppKnownNames lists every binary name published in upstream releases, in
// preference order for the current platform (primary CUDA variant first).
//
// Source: https://github.com/LostRuins/koboldcpp/releases
func koboldcppKnownNames() []string {
	switch runtime.GOOS {
	case "windows":
		return []string{"koboldcpp.exe", "koboldcpp-nocuda.exe", "koboldcpp-oldpc.exe"}
	case "darwin":
		return []string{"koboldcpp-mac-arm64"}
	default:
		return []string{"koboldcpp-linux-x64", "koboldcpp-linux-x64-nocuda", "koboldcpp-linux-x64-oldpc"}
	}
}

// pickFirstKoboldCppInDir returns the best koboldcpp binary from dir, preferring
// the primary platform variant. Falls back to any regular file with a "koboldcpp"
// prefix to cover future or renamed variants. Uses isRegularFile (os.Stat) so
// symlinks are followed, matching the other backends.
func pickFirstKoboldCppInDir(dir string) string {
	ents, err := os.ReadDir(dir)
	if err != nil {
		return ""
	}
	for _, pref := range koboldcppKnownNames() {
		for _, e := range ents {
			p := filepath.Join(dir, e.Name())
			if isRegularFile(p) && e.Name() == pref {
				return p
			}
		}
	}
	for _, e := range ents {
		p := filepath.Join(dir, e.Name())
		if isRegularFile(p) && strings.HasPrefix(e.Name(), "koboldcpp") {
			return p
		}
	}
	return ""
}

// isExecutableFile reports whether path is a regular file with at least one
// execute bit (Unix) or is a regular file (Windows).
func isExecutableFile(path string) bool {
	fi, err := os.Stat(path)
	if err != nil {
		return false
	}
	if !fi.Mode().IsRegular() {
		return false
	}
	if runtime.GOOS == "windows" {
		return true
	}
	return fi.Mode().Perm()&0o111 != 0
}

// findKoboldCppBinary resolves a koboldcpp executable from koboldCppPath (a
// directory or a direct file path), then common install directories, then PATH.
// koboldCppPath may be empty.
func findKoboldCppBinary(koboldCppPath string) string {
	var envDir string
	if d := koboldCppPath; d != "" {
		envDir = filepath.Clean(d)
	}
	// 1) $KOBOLDCPP_PATH points directly to a file — use it only when the
	//    base name matches a known koboldcpp pattern.
	if envDir != "" {
		clean := filepath.Clean(envDir)
		if isRegularFile(clean) && strings.HasPrefix(filepath.Base(clean), "koboldcpp") && isExecutableFile(clean) {
			return clean
		}
	}
	// 2) $KOBOLDCPP_PATH points to a directory — look for a koboldcpp binary.
	if envDir != "" {
		if p := pickFirstKoboldCppInDir(envDir); p != "" && isExecutableFile(p) {
			return p
		}
	}
	// 3) Common directories.
	common := slices.Clone(commonBinaryDirs)
	if home, err := os.UserHomeDir(); err == nil {
		common = append(common, filepath.Join(home, ".local", "bin"))
	}
	for _, dir := range common {
		if p := pickFirstKoboldCppInDir(dir); p != "" && isExecutableFile(p) {
			return p
		}
	}
	// 4) PATH fallback (already checks executability via LookPath).
	if p, err := exec.LookPath("koboldcpp"); err == nil {
		return p
	}
	return ""
}

// ninferServeName is the NInfer HTTP server executable.
const ninferServeName = "ninfer-serve"

// findNInferBinary resolves ninfer-serve from ninferPath, then common install
// directories, then PATH. ninferPath may be the binary itself, a directory
// containing it, or an NInfer checkout root: NInfer has no install target, so
// its build leaves the server at build/apps/ninfer-serve inside the checkout.
func findNInferBinary(ninferPath string) string {
	if d := ninferPath; d != "" {
		clean := filepath.Clean(d)
		if isRegularFile(clean) && filepath.Base(clean) == ninferServeName && isExecutableFile(clean) {
			return clean
		}
		for _, candidate := range []string{
			filepath.Join(clean, ninferServeName),
			filepath.Join(clean, "build", "apps", ninferServeName),
		} {
			if isExecutableFile(candidate) {
				return candidate
			}
		}
	}
	common := slices.Clone(commonBinaryDirs)
	if home, err := os.UserHomeDir(); err == nil {
		common = append(common, filepath.Join(home, ".local", "bin"))
	}
	return findBinaryInEnvAndCommonDirs(ninferServeName, "", common)
}

// findOMLXBinary resolves the omlx CLI from omlxPath, then the oMLX app's
// shim at ~/.omlx/bin/omlx, then common install directories, then PATH.
// omlxPath may be the CLI itself, a directory containing it, or the oMLX base
// directory, whose bin/ holds the shim the macOS app installs.
func findOMLXBinary(omlxPath string) string {
	const name = "omlx"
	var dirs []string
	if d := omlxPath; d != "" {
		clean := filepath.Clean(d)
		if filepath.Base(clean) == name && isExecutableFile(clean) {
			return clean
		}
		dirs = append(dirs, clean, filepath.Join(clean, "bin"))
	}
	if home, err := os.UserHomeDir(); err == nil {
		dirs = append(dirs, filepath.Join(home, ".omlx", "bin"))
	}
	for _, dir := range dirs {
		if p := filepath.Join(dir, name); isExecutableFile(p) {
			return p
		}
	}
	return findBinaryInEnvAndCommonDirs(name, "", commonBinaryDirs)
}

// findSplashBinary resolves the splash executable from splashPath (the binary
// or a directory containing it), then common install directories (Homebrew),
// then PATH.
func findSplashBinary(splashPath string) string {
	const name = "splash"
	if d := splashPath; d != "" {
		clean := filepath.Clean(d)
		if filepath.Base(clean) == name && isExecutableFile(clean) {
			return clean
		}
		if p := filepath.Join(clean, name); isExecutableFile(p) {
			return p
		}
	}
	return findBinaryInEnvAndCommonDirs(name, "", commonBinaryDirs)
}

// probeHealthEndpoint GETs /health on host:port, bounded by ctx. Used by
// llama-server, KoboldCpp, and ninfer-serve. It shares the package HTTP client
// so repeated probes reuse connections.
func probeHealthEndpoint(ctx context.Context, host string, port int) bool {
	url := fmt.Sprintf("http://%s:%d/health", host, port)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return false
	}
	resp, err := sharedHTTPClient.Do(req)
	if err != nil {
		return false
	}
	defer func() { _ = resp.Body.Close() }()
	// Drain so the connection returns to the pool.
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4<<10))
	return resp.StatusCode == http.StatusOK
}

// probeModelsOwner reports whether the OpenAI-compatible server on host:port
// lists a model owned by owner. oMLX and Splash both answer /health and both
// default to port 8000, so a health check alone cannot tell which one is up;
// each reports itself as the owner in /v1/models.
func probeModelsOwner(ctx context.Context, host string, port int, owner string) bool {
	url := fmt.Sprintf("http://%s:%d/v1/models", host, port)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return false
	}
	resp, err := sharedHTTPClient.Do(req)
	if err != nil {
		return false
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4<<10))
		return false
	}
	var list struct {
		Data []struct {
			OwnedBy string `json:"owned_by"`
		} `json:"data"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&list); err != nil {
		return false
	}
	for _, m := range list.Data {
		if m.OwnedBy == owner {
			return true
		}
	}
	return false
}

// defaultProbeHost is the loopback address used for health probes that have no
// configurable host of their own (KoboldCpp).
const defaultProbeHost = "127.0.0.1"

// probeHost returns the address to probe a server configured to listen on
// host. A wildcard listen address accepts loopback connections but is not a
// portable destination, so it is probed on loopback.
func probeHost(host string) string {
	switch strings.TrimSpace(host) {
	case "", "0.0.0.0", "::", "[::]":
		return defaultProbeHost
	default:
		return strings.TrimSpace(host)
	}
}

// resolvePath returns existing if non-empty, otherwise the first match for cmdName on PATH.
func resolvePath(existing, cmdName string) string {
	if existing != "" {
		return existing
	}
	if p, err := exec.LookPath(cmdName); err == nil {
		return p
	}
	return ""
}

// ResolveLlamaServerPath returns the detected llama-server binary path, or the first match on PATH.
func ResolveLlamaServerPath(r RuntimeInfo) string {
	return resolvePath(r.LlamaServerPath, "llama-server")
}

// ResolveVLLMPath returns the detected vllm binary path, or the first match on PATH.
func ResolveVLLMPath(r RuntimeInfo) string {
	return resolvePath(r.VLLMPath, "vllm")
}

// ResolveOllamaPath returns the detected ollama binary path, or the first match on PATH.
func ResolveOllamaPath(r RuntimeInfo) string {
	return resolvePath(r.OllamaPath, "ollama")
}

// ResolveNInferPath returns the detected ninfer-serve binary path, or the first match on PATH.
func ResolveNInferPath(r RuntimeInfo) string {
	return resolvePath(r.NInferPath, ninferServeName)
}

// ResolveOMLXPath returns the detected omlx CLI path, or the first match on PATH.
func ResolveOMLXPath(r RuntimeInfo) string {
	return resolvePath(r.OMLXPath, "omlx")
}

// ResolveSplashPath returns the detected splash path, or the first match on PATH.
func ResolveSplashPath(r RuntimeInfo) string {
	return resolvePath(r.SplashPath, "splash")
}

// ResolveKoboldCppPath returns the detected koboldcpp binary path, or the first match on PATH.
func ResolveKoboldCppPath(r RuntimeInfo) string {
	return resolvePath(r.KoboldCppPath, "koboldcpp")
}
