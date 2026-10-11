package models

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeStrataConfig writes body to dir/name, creating dir.
func writeStrataConfig(t *testing.T, dir, name, body string) string {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

// strataInstall builds a Strata install at dir with a serve/server.py and the
// given engine configs. It returns the install directory.
func strataInstall(t *testing.T, dir string, configs map[string]string) string {
	t.Helper()
	writeStrataConfig(t, filepath.Join(dir, "serve"), "server.py", "# strata server\n")
	for name, body := range configs {
		writeStrataConfig(t, dir, name, body)
	}
	return dir
}

func TestDiscoverStrata(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	native := writeStrataConfig(t, filepath.Join(dir, "models", "IQ3_S"), "w-00001.gguf", "0123456789")
	ple := writeStrataConfig(t, filepath.Join(dir, "models", "IQ3_S"), "w-00002.gguf", "0123456789ab")
	cfg := `{"exe":"` + filepath.Join(dir, "engine", "strata") + `",` +
		`"args":["--pack","` + filepath.Join(dir, "packs", "iq3_s") + `",` +
		`"--native","` + native + `","--ple-gguf","` + ple + `","--max-context","262144"],` +
		`"model_name":"qwen3.8-flash-next-iq3_s","port":9003}`
	strataInstall(t, dir, map[string]string{
		"strata-iq3_s.json":                 cfg,
		"strata-iq3_s.json.bak":             cfg,
		"strata-iq3_s.shared-settings.json": `{"port":9003}`,
		"other.json":                        cfg,
	})

	rows := discoverStrata([]string{dir})
	if len(rows) != 1 {
		t.Fatalf("discoverStrata returned %d rows, want 1: %+v", len(rows), rows)
	}
	row := rows[0]
	if row.Backend != BackendStrata {
		t.Errorf("Backend = %v, want BackendStrata", row.Backend)
	}
	if row.Name != "qwen3.8-flash-next-iq3_s" {
		t.Errorf("Name = %q, want the model_name", row.Name)
	}
	if filepath.Base(row.Path) != "strata-iq3_s.json" {
		t.Errorf("Path = %q, want the engine config", row.Path)
	}
	if want := int64(10 + 12); row.Size != want {
		t.Errorf("Size = %d, want %d (the two weight shards)", row.Size, want)
	}
	if got, want := row.Parameters, "strata · IQ3_S · ctx 262144"; got != want {
		t.Errorf("Parameters = %q, want %q", got, want)
	}
}

func TestDiscoverStrataRejectsNonConfigs(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	strataInstall(t, dir, map[string]string{
		// Right prefix and suffix, but not a launchable engine config.
		"strata-shared.json": `{"port":9003,"installs":["/x"]}`,
		"strata-bad.json":    `not json`,
	})
	if rows := discoverStrata([]string{dir}); len(rows) != 0 {
		t.Errorf("discoverStrata = %+v, want no rows", rows)
	}
	if rows := discoverStrata([]string{filepath.Join(dir, "missing")}); len(rows) != 0 {
		t.Errorf("missing root: discoverStrata = %+v, want no rows", rows)
	}
}

func TestDiscoverStrataConfigSubdir(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	strataInstall(t, dir, nil)
	writeStrataConfig(t, filepath.Join(dir, "config"), "strata-iq2_xs.json",
		`{"exe":"/x/engine/strata","args":["--max-context","8192"],"model_name":"qwen3.8-flash-next-iq2_xs"}`)
	rows := discoverStrata([]string{dir})
	if len(rows) != 1 || rows[0].Name != "qwen3.8-flash-next-iq2_xs" {
		t.Fatalf("discoverStrata = %+v, want the config/ pack", rows)
	}
	if got, want := rows[0].Parameters, "strata · IQ2_XS · ctx 8192"; got != want {
		t.Errorf("Parameters = %q, want %q", got, want)
	}
}

func TestStrataSizeLabel(t *testing.T) {
	t.Parallel()

	cases := map[string]string{
		"qwen3.8-flash-next-iq3_s": "IQ3_S",
		"qwen3.8-flash-next":       "NEXT",
		"Q2_0":                     "Q2_0",
		"":                         "",
	}
	for in, want := range cases {
		if got := strataSizeLabel(in); got != want {
			t.Errorf("strataSizeLabel(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestStrataPython(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	strataInstall(t, dir, nil)
	// No venv: the expected platform path is returned anyway, so a launch
	// preview still shows the command it would run.
	python := StrataPython(filepath.Join(dir, "serve", "server.py"))
	if !strings.Contains(python, filepath.Join(dir, ".venv")) || !strings.Contains(python, "python") {
		t.Errorf("StrataPython = %q, want a python under %s/.venv", python, dir)
	}
	if StrataPython("") != "" {
		t.Error("StrataPython(\"\") should be empty")
	}
}

func TestProbeStrata(t *testing.T) {
	t.Parallel()

	strata := fakeServer{health: `{"status":"ok","service":"strata","model":"q"}`}
	host, port := strata.start(t)
	if !probeStrata(context.Background(), host, port) {
		t.Error("a server naming service=strata was not detected")
	}

	notStrata := fakeServer{health: `{"status":"ok"}`}
	host, port = notStrata.start(t)
	if probeStrata(context.Background(), host, port) {
		t.Error("a server without service=strata was detected as Strata")
	}
}
