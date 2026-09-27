package userdata

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

// testIsolatedUserConfig sets HOME (and OS-specific overrides) so os.UserConfigDir()
// resolves inside t.TempDir(). Note: on macOS, XDG_CONFIG_HOME is ignored by Go.
func testIsolatedUserConfig(t *testing.T) {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("UserProfile", home)
	switch runtime.GOOS {
	case "windows":
		t.Setenv("AppData", filepath.Join(home, "AppData", "Roaming"))
	default:
		if runtime.GOOS != "darwin" {
			t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
		}
	}
}

func TestBackupFileIfExists_createsBackup(t *testing.T) {
	testIsolatedUserConfig(t)

	cfg, err := ConfigTomlPath()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(cfg), 0o755); err != nil {
		t.Fatal(err)
	}
	content := []byte("schema_version = 3\n")
	if err := os.WriteFile(cfg, content, 0o644); err != nil {
		t.Fatal(err)
	}

	if err := BackupFileIfExists(cfg); err != nil {
		t.Fatal(err)
	}
	backupDir := filepath.Join(filepath.Dir(cfg), BackupDirName)
	entries, err := os.ReadDir(backupDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 backup, got %d", len(entries))
	}
	if !strings.HasPrefix(entries[0].Name(), "config.toml.") || !strings.HasSuffix(entries[0].Name(), ".bak") {
		t.Fatalf("unexpected backup name: %s", entries[0].Name())
	}
	b, err := os.ReadFile(filepath.Join(backupDir, entries[0].Name()))
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != string(content) {
		t.Fatal("backup content mismatch")
	}
}

func TestBackupFileIfExists_pruneKeepsNewest(t *testing.T) {
	dir := t.TempDir()
	backupDir := filepath.Join(dir, "backups")
	if err := os.MkdirAll(backupDir, 0o755); err != nil {
		t.Fatal(err)
	}
	base := "config.toml"
	// Create MaxBackupsPerFile+3 files; stagger mtimes so the last indices are newest.
	n := MaxBackupsPerFile + 3
	for i := range n {
		name := filepath.Join(backupDir, fmt.Sprintf("%s.%d.bak", base, i))
		if err := os.WriteFile(name, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
		old := time.Now().Add(-time.Duration(n-i) * time.Minute)
		if err := os.Chtimes(name, old, old); err != nil {
			t.Fatal(err)
		}
	}
	if err := PruneOldBackups(backupDir, base, MaxBackupsPerFile); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(backupDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != MaxBackupsPerFile {
		t.Fatalf("expected %d files after prune, got %d", MaxBackupsPerFile, len(entries))
	}
}

func TestBackupFileIfExists_missingNoOp(t *testing.T) {
	dir := t.TempDir()
	missing := filepath.Join(dir, "nope.toml")
	if err := BackupFileIfExists(missing); err != nil {
		t.Fatal(err)
	}
}

func TestMaybeBackupOnVersionChange_writesMarkerAndBacksUp(t *testing.T) {
	testIsolatedUserConfig(t)

	cfg, _ := ConfigTomlPath()
	_ = os.MkdirAll(filepath.Dir(cfg), 0o755)
	if err := os.WriteFile(cfg, []byte("k=v\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := MaybeBackupOnVersionChange("0.9.0"); err != nil {
		t.Fatal(err)
	}
	llml, _ := LlmlDir()
	marker := filepath.Join(llml, lastRunVersionFile)
	b, err := os.ReadFile(marker)
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(string(b)) != "0.9.0" {
		t.Fatalf("marker: %q", b)
	}

	if err := MaybeBackupOnVersionChange("0.9.0"); err != nil {
		t.Fatal(err)
	}
}

func TestMaybeBackupOnVersionChange_devNoBackup(t *testing.T) {
	testIsolatedUserConfig(t)

	cfg, _ := ConfigTomlPath()
	_ = os.MkdirAll(filepath.Dir(cfg), 0o755)
	_ = os.WriteFile(cfg, []byte("x\n"), 0o644)

	if err := MaybeBackupOnVersionChange("dev"); err != nil {
		t.Fatal(err)
	}
	backupDir := filepath.Join(filepath.Dir(cfg), BackupDirName)
	if _, err := os.Stat(backupDir); err == nil {
		entries, _ := os.ReadDir(backupDir)
		if len(entries) > 0 {
			t.Fatal("dev should not create version-triggered backups")
		}
	}
}

func TestMaybeBackupOnVersionChange_snapshotsRuntimeStates(t *testing.T) {
	testIsolatedUserConfig(t)

	rs, err := RuntimeStatesPath()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(rs), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(rs, []byte("[runtimes]\nllama = false\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := MaybeBackupOnVersionChange("1.2.3"); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(filepath.Join(filepath.Dir(rs), BackupDirName))
	if err != nil {
		t.Fatal(err)
	}
	var found bool
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), "runtimes.toml.") {
			found = true
		}
	}
	if !found {
		t.Errorf("version change did not snapshot runtimes.toml: %v", entries)
	}
}
