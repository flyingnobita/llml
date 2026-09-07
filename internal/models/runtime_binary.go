package models

import (
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"time"
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

// probeHealthEndpoint GETs /health on host:port. Used by both llama-server and KoboldCpp.
func probeHealthEndpoint(host string, port int) bool {
	url := fmt.Sprintf("http://%s:%d/health", host, port)
	client := &http.Client{Timeout: 2 * time.Second}
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return false
	}
	resp, err := client.Do(req)
	if err != nil {
		return false
	}
	defer func() { _ = resp.Body.Close() }()
	return resp.StatusCode == http.StatusOK
}

// defaultProbeHost is the loopback address used for health probes that have no
// configurable host of their own (KoboldCpp).
const defaultProbeHost = "127.0.0.1"

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

// ResolveKoboldCppPath returns the detected koboldcpp binary path, or the first match on PATH.
func ResolveKoboldCppPath(r RuntimeInfo) string {
	return resolvePath(r.KoboldCppPath, "koboldcpp")
}
