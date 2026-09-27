package tui

import (
	"fmt"
	"os"
	"testing"
)

// TestMain points every user-directory lookup at a throwaway directory before
// any test runs. Tests that build a Model with [New] get the real services,
// and profile saves go straight to disk, so without this a test run rewrites
// the developer's own runtimes.toml and model-params.json.
//
// Setting the environment here is safe: no test is running yet, so nothing
// can race it. Tests that need their own directory still call t.Setenv.
func TestMain(m *testing.M) {
	os.Exit(runIsolated(m))
}

func runIsolated(m *testing.M) int {
	dir, err := os.MkdirTemp("", "llml-tui-test-")
	if err != nil {
		fmt.Fprintln(os.Stderr, "tui tests: cannot create isolated config dir:", err)
		return 1
	}
	defer func() { _ = os.RemoveAll(dir) }()
	// HOME covers macOS and the XDG fallback, XDG_CONFIG_HOME covers Linux,
	// and AppData / USERPROFILE cover Windows.
	for _, k := range []string{"HOME", "XDG_CONFIG_HOME", "AppData", "USERPROFILE"} {
		if err := os.Setenv(k, dir); err != nil {
			fmt.Fprintln(os.Stderr, "tui tests: cannot isolate", k+":", err)
			return 1
		}
	}
	return m.Run()
}
