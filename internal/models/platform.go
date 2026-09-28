package models

import "runtime"

// Platform is the operating system and CPU architecture a runtime runs on. It
// is carried on [RuntimeInfo] rather than read from package runtime at each
// use so that views which depend on it can be tested for any platform.
type Platform struct {
	GOOS   string
	GOARCH string
}

// CurrentPlatform returns the platform llml is running on.
func CurrentPlatform() Platform {
	return Platform{GOOS: runtime.GOOS, GOARCH: runtime.GOARCH}
}

// Supports reports whether backend b can run on p. oMLX and Splash are built
// on Apple's MLX and Metal and exist only for Apple Silicon macOS; NInfer
// targets CUDA on Linux. mlx-lm runs wherever MLX ships a backend: see
// [Platform.runsMLX]. Every other backend is portable.
//
// The zero Platform supports everything, so a RuntimeInfo that has not been
// probed yet hides nothing.
func (p Platform) Supports(b ModelBackend) bool {
	if p == (Platform{}) {
		return true
	}
	switch b {
	case BackendOMLX, BackendSplash:
		return p.GOOS == "darwin" && p.GOARCH == "arm64"
	case BackendNInfer:
		return p.GOOS == "linux"
	case BackendMLXLM:
		return p.runsMLX()
	default:
		return true
	}
}

// runsMLX reports whether MLX publishes a backend for p: Metal on Apple
// Silicon macOS, and CUDA or CPU on Linux x86_64 and aarch64. Intel Macs and
// Windows are left out.
func (p Platform) runsMLX() bool {
	switch p.GOOS {
	case "darwin":
		return p.GOARCH == "arm64"
	case "linux":
		return p.GOARCH == "amd64" || p.GOARCH == "arm64"
	default:
		return false
	}
}
