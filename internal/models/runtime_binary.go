package models

import (
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

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

func findVLLMBinary() string {
	if dir := os.Getenv(EnvVLLMPath); dir != "" {
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
	if d := strings.TrimSpace(os.Getenv(EnvVLLMVenv)); d != "" {
		if p := vllmBinaryInVenvRoot(d); p != "" {
			return p
		}
	}
	var common []string
	common = append(common,
		"/usr/local/bin",
		"/opt/homebrew/bin",
	)
	if home, err := os.UserHomeDir(); err == nil {
		common = append(common, filepath.Join(home, ".local", "bin"))
		if runtime.GOOS == "darwin" {
			// Common local layout for Apple Silicon / Metal vLLM installs.
			common = append(common, filepath.Join(home, ".venv-vllm-metal", "bin"))
		}
	}
	return findBinaryInEnvAndCommonDirs("vllm", "", common)
}

func findLlamaBinary(name string) string {
	var envDir string
	if d := os.Getenv(EnvLlamaCppPath); d != "" {
		envDir = filepath.Clean(d)
	}
	var common []string
	common = append(common,
		"/usr/local/bin",
		"/opt/homebrew/bin",
		"/opt/llama.cpp/build/bin",
	)
	if home, err := os.UserHomeDir(); err == nil {
		common = append(common, filepath.Join(home, ".local", "bin"))
	}
	return findBinaryInEnvAndCommonDirs(name, envDir, common)
}

func findOllamaBinary() string {
	var envDir string
	if d := os.Getenv(EnvOllamaPath); d != "" {
		envDir = filepath.Clean(d)
	}
	var common []string
	common = append(common,
		"/usr/local/bin",
		"/opt/homebrew/bin",
	)
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
// prefix to cover future or renamed variants.
func pickFirstKoboldCppInDir(dir string) string {
	ents, err := os.ReadDir(dir)
	if err != nil {
		return ""
	}
	for _, pref := range koboldcppKnownNames() {
		for _, e := range ents {
			if e.Type().IsRegular() && e.Name() == pref {
				return filepath.Join(dir, e.Name())
			}
		}
	}
	for _, e := range ents {
		if e.Type().IsRegular() && strings.HasPrefix(e.Name(), "koboldcpp") {
			return filepath.Join(dir, e.Name())
		}
	}
	return ""
}

func findKoboldCppBinary() string {
	var envDir string
	if d := os.Getenv(EnvKoboldCppPath); d != "" {
		envDir = filepath.Clean(d)
	}
	// 1) $KOBOLDCPP_PATH points directly to an executable — use it as-is.
	if envDir != "" {
		clean := filepath.Clean(envDir)
		if fi, err := os.Stat(clean); err == nil && fi.Mode().IsRegular() {
			return clean
		}
	}
	// 2) $KOBOLDCPP_PATH points to a directory — look for a koboldcpp binary.
	if envDir != "" {
		if p := pickFirstKoboldCppInDir(envDir); p != "" {
			return p
		}
	}
	// 3) Common directories.
	common := []string{"/usr/local/bin", "/opt/homebrew/bin"}
	if home, err := os.UserHomeDir(); err == nil {
		common = append(common, filepath.Join(home, ".local", "bin"))
	}
	for _, dir := range common {
		if p := pickFirstKoboldCppInDir(dir); p != "" {
			return p
		}
	}
	// 4) PATH fallback (exact name only).
	if p, err := exec.LookPath("koboldcpp"); err == nil {
		return p
	}
	return ""
}

// probeLlamaServerHealth GETs /health on 127.0.0.1 (avoids localhost IPv6/IPv4 ambiguity).
func probeLlamaServerHealth(port int) bool {
	url := fmt.Sprintf("http://127.0.0.1:%d/health", port)
	client := &http.Client{Timeout: 2 * time.Second}
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return false
	}
	resp, err := client.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}
