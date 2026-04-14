package llamacpp

import (
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

func tryStatBinary(path string) bool {
	st, err := os.Stat(path)
	return err == nil && !st.IsDir() && st.Mode().IsRegular()
}

// findBinaryInEnvAndCommonDirs resolves name as $envDir/name, then each of commonDirs/name,
// then [exec.LookPath]. envDir may be empty (skip that step).
func findBinaryInEnvAndCommonDirs(name, envDir string, commonDirs []string) string {
	if envDir != "" {
		candidate := filepath.Join(filepath.Clean(envDir), name)
		if tryStatBinary(candidate) {
			return candidate
		}
	}
	for _, dir := range commonDirs {
		candidate := filepath.Join(dir, name)
		if tryStatBinary(candidate) {
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
		candidate := filepath.Join(clean, "vllm")
		if tryStatBinary(candidate) {
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
