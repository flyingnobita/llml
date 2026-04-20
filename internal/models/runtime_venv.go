package models

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// VenvRootFromActivateScript returns the venv root directory given an activate script path
// (Unix: .../bin/activate, Windows: .../Scripts/activate.bat), or "" if activate is empty.
func VenvRootFromActivateScript(activate string) string {
	activate = strings.TrimSpace(activate)
	if activate == "" {
		return ""
	}
	parent := filepath.Dir(activate)
	return filepath.Dir(parent)
}

// venvActivateScriptPath returns the shell script path for a Python venv root (the directory
// that contains bin/activate on Unix or Scripts/activate.bat on Windows).
func venvActivateScriptPath(venvRoot string) string {
	venvRoot = filepath.Clean(venvRoot)
	if runtime.GOOS == "windows" {
		return filepath.Join(venvRoot, "Scripts", "activate.bat")
	}
	return filepath.Join(venvRoot, "bin", "activate")
}

func isRegularFile(path string) bool {
	st, err := os.Stat(path)
	return err == nil && !st.IsDir() && st.Mode().IsRegular()
}

// pickExistingFile returns unixPath on non-Windows or windowsPath on Windows, if the
// chosen path is a regular file. Returns "" if the file does not exist.
func pickExistingFile(unixPath, windowsPath string) string {
	if runtime.GOOS == "windows" {
		if isRegularFile(windowsPath) {
			return windowsPath
		}
		return ""
	}
	if isRegularFile(unixPath) {
		return unixPath
	}
	return ""
}

// activateAdjacentToVLLM returns the activate script in the same directory as the vllm
// executable (standard layout: .venv/bin/vllm next to .venv/bin/activate).
func activateAdjacentToVLLM(vllmBin string) string {
	if vllmBin == "" {
		return ""
	}
	dir := filepath.Dir(vllmBin)
	return pickExistingFile(
		filepath.Join(dir, "activate"),
		filepath.Join(dir, "activate.bat"),
	)
}

// ResolveVLLMActivateScript returns an activate script path to source before `vllm serve`, or ""
// when no venv should be activated. Resolution order:
//  1. activate next to vllm in the same bin/ directory (e.g. .venv/bin/activate + .venv/bin/vllm)
//  2. VLLM_VENV (venv root)
//  3. $VLLM_PATH/.venv (when VLLM_PATH is set)
//  4. dirname(vllmBin)/.venv (project-local venv when vllm is a top-level script)
func ResolveVLLMActivateScript(vllmBin string) string {
	if s := activateAdjacentToVLLM(vllmBin); s != "" {
		return s
	}
	try := func(venvRoot string) string {
		p := venvActivateScriptPath(venvRoot)
		if isRegularFile(p) {
			return p
		}
		return ""
	}
	if d := strings.TrimSpace(os.Getenv(EnvVLLMVenv)); d != "" {
		if s := try(d); s != "" {
			return s
		}
	}
	if d := strings.TrimSpace(os.Getenv(EnvVLLMPath)); d != "" {
		if s := try(filepath.Join(filepath.Clean(d), ".venv")); s != "" {
			return s
		}
	}
	if vllmBin != "" {
		if s := try(filepath.Join(filepath.Dir(vllmBin), ".venv")); s != "" {
			return s
		}
	}
	return ""
}

// vllmBinaryInVenvRoot returns $venvRoot/bin/vllm (Unix) or $venvRoot/Scripts/vllm.exe (Windows) if present.
func vllmBinaryInVenvRoot(venvRoot string) string {
	venvRoot = filepath.Clean(venvRoot)
	return pickExistingFile(
		filepath.Join(venvRoot, "bin", "vllm"),
		filepath.Join(venvRoot, "Scripts", "vllm.exe"),
	)
}

// vllmBinaryInProjectDotVenv returns $project/.venv/bin/vllm when that file exists.
func vllmBinaryInProjectDotVenv(projectRoot string) string {
	return vllmBinaryInVenvRoot(filepath.Join(filepath.Clean(projectRoot), ".venv"))
}
