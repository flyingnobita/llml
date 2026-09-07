package main

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// runCLI drives run with fake streams and returns the exit code plus what each
// stream received. Because run never calls os.Exit, this covers the real
// entrypoint rather than a stand-in for it.
func runCLI(t *testing.T, stdin string, args ...string) (int, string, string) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	code := run(args, strings.NewReader(stdin), &stdout, &stderr)
	return code, stdout.String(), stderr.String()
}

func TestRun_version(t *testing.T) {
	for _, flag := range []string{"--version", "-version", "-v"} {
		code, out, _ := runCLI(t, "", flag)
		if code != 0 {
			t.Errorf("%s: exit %d, want 0", flag, code)
		}
		if strings.TrimSpace(out) != version {
			t.Errorf("%s: stdout %q, want %q", flag, out, version)
		}
	}
}

func TestRun_helpListsSubcommands(t *testing.T) {
	for _, flag := range []string{"--help", "-h", "help"} {
		code, out, _ := runCLI(t, "", flag)
		if code != 0 {
			t.Errorf("%s: exit %d, want 0", flag, code)
		}
		for _, want := range []string{"llml export", "llml import", "--version"} {
			if !strings.Contains(out, want) {
				t.Errorf("%s: usage missing %q:\n%s", flag, want, out)
			}
		}
	}
}

func TestRun_subcommandHelpExitsZero(t *testing.T) {
	for _, sub := range []string{"export", "import"} {
		code, _, errOut := runCLI(t, "", sub, "--help")
		if code != 0 {
			t.Errorf("%s --help: exit %d, want 0", sub, code)
		}
		if !strings.Contains(errOut, "-target") && !strings.Contains(errOut, "-output") {
			t.Errorf("%s --help: expected flag usage, got:\n%s", sub, errOut)
		}
	}
}

func TestRun_importWithoutSourceFailsWithUsage(t *testing.T) {
	code, _, errOut := runCLI(t, "", "import")
	if code != 1 {
		t.Errorf("exit %d, want 1", code)
	}
	if !strings.Contains(errOut, "required") {
		t.Errorf("stderr should explain what is missing:\n%s", errOut)
	}
}

func TestRun_importUnknownFlagFails(t *testing.T) {
	code, _, errOut := runCLI(t, "", "import", "--nope", "x.toml")
	if code != 1 {
		t.Errorf("exit %d, want 1", code)
	}
	if !strings.Contains(errOut, "nope") {
		t.Errorf("stderr should name the bad flag:\n%s", errOut)
	}
}

func TestRun_importDryRunPrintsPreviewAndWritesNothing(t *testing.T) {
	dir := setupConfigDir(t)

	src := filepath.Join(dir, "p.toml")
	if err := os.WriteFile(src, []byte(`schema_version = 3

[[profiles]]
name = "cuda-fast"
backend = "llama"
args = ["--ctx-size 4096"]
`), 0o644); err != nil {
		t.Fatal(err)
	}

	code, out, errOut := runCLI(t, "", "import", "--dry-run", src)
	if code != 0 {
		t.Fatalf("exit %d, want 0 (stderr: %s)", code, errOut)
	}
	if !strings.Contains(out, "cuda-fast") {
		t.Errorf("preview should name the profile:\n%s", out)
	}
	if _, err := os.Stat(filepath.Join(dir, "llml", "model-params.json")); err == nil {
		t.Error("--dry-run must not write model-params.json")
	}
}

func TestRun_importMissingFileFails(t *testing.T) {
	setupConfigDir(t)

	code, _, errOut := runCLI(t, "", "import", "--target", "/m.gguf", "/nonexistent/p.toml")
	if code != 1 {
		t.Errorf("exit %d, want 1", code)
	}
	if !strings.Contains(errOut, "llml:") {
		t.Errorf("stderr should be prefixed:\n%s", errOut)
	}
}

func TestRun_importLocalFileWritesProfile(t *testing.T) {
	dir := setupConfigDir(t)

	src := filepath.Join(dir, "p.toml")
	if err := os.WriteFile(src, []byte(`schema_version = 3

[[profiles]]
name = "cuda-fast"
backend = "llama"
args = ["--ctx-size 4096"]
`), 0o644); err != nil {
		t.Fatal(err)
	}

	code, out, errOut := runCLI(t, "", "import", "--target", filepath.Join(dir, "a.gguf"), src)
	if code != 0 {
		t.Fatalf("exit %d, want 0 (stderr: %s)", code, errOut)
	}
	if !strings.Contains(out, "1 added") {
		t.Errorf("stdout should report the import:\n%s", out)
	}
	if _, err := os.Stat(filepath.Join(dir, "llml", "model-params.json")); err != nil {
		t.Errorf("model-params.json should exist: %v", err)
	}
}

func TestRun_exportWritesFile(t *testing.T) {
	dir := setupConfigDir(t)

	// Seed one profile so there is something to export.
	src := filepath.Join(dir, "p.toml")
	if err := os.WriteFile(src, []byte(`schema_version = 3

[[profiles]]
name = "cuda-fast"
backend = "llama"
args = ["--ctx-size 4096"]
`), 0o644); err != nil {
		t.Fatal(err)
	}
	if code, _, errOut := runCLI(t, "", "import", "--target", filepath.Join(dir, "a.gguf"), src); code != 0 {
		t.Fatalf("seed import failed: %s", errOut)
	}

	dest := filepath.Join(dir, "out.toml")
	code, out, errOut := runCLI(t, "", "export", "--output", dest)
	if code != 0 {
		t.Fatalf("exit %d, want 0 (stderr: %s)", code, errOut)
	}
	if !strings.Contains(out, "Exported") {
		t.Errorf("stdout should report the export:\n%s", out)
	}
	b, err := os.ReadFile(dest)
	if err != nil {
		t.Fatalf("export file: %v", err)
	}
	if !strings.Contains(string(b), "cuda-fast") {
		t.Errorf("exported file should contain the profile:\n%s", b)
	}
}

func TestRun_exportWithNoProfilesSaysSo(t *testing.T) {
	setupConfigDir(t)

	code, out, errOut := runCLI(t, "", "export")
	if code != 0 {
		t.Fatalf("exit %d, want 0 (stderr: %s)", code, errOut)
	}
	if !strings.Contains(out, "No profiles to export") {
		t.Errorf("stdout %q", out)
	}
}

// Declining the confirmation prompt is a choice, not a failure, so it exits 0.
func TestConfirmImport_declineIsCleanExit(t *testing.T) {
	t.Parallel()

	var out bytes.Buffer
	c := cli{stdin: strings.NewReader("n\n"), stdout: &out, stderr: io.Discard, isTerminal: notATerminal}
	if err := c.confirmImport(); err == nil {
		t.Fatal("declining should return an error to unwind the command")
	}
	if !strings.Contains(out.String(), "Cancelled") {
		t.Errorf("should tell the user:\n%s", out.String())
	}
}

func TestConfirmImport_accepts(t *testing.T) {
	t.Parallel()

	for _, in := range []string{"y\n", "yes\n", "Y\n"} {
		c := cli{stdin: strings.NewReader(in), stdout: io.Discard, stderr: io.Discard, isTerminal: notATerminal}
		if err := c.confirmImport(); err != nil {
			t.Errorf("%q should be accepted, got %v", in, err)
		}
	}
}
