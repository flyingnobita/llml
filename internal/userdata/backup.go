package userdata

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// MaxBackupsPerFile is how many timestamped backups to retain per logical file
// (config.toml and model-params.json each get their own cap).
const MaxBackupsPerFile = 10

const lastRunVersionFile = ".last-run-version"

// BackupDirName is the subdirectory under the llml config dir that holds copies.
const BackupDirName = "backups"

// BackupFileIfExists copies srcPath into {dir(srcPath)/backups/<basename>.<unixnano>.bak}
// when srcPath exists. It is a no-op when the file is missing. Creates the backups
// directory as needed.
//
//nolint:gosec // G304: file paths derived from os.UserConfigDir — trusted source.
func BackupFileIfExists(srcPath string) error {
	st, err := os.Stat(srcPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	if st.IsDir() {
		return fmt.Errorf("userdata: backup source is a directory: %s", srcPath)
	}

	backupDir := filepath.Join(filepath.Dir(srcPath), BackupDirName)
	if err := os.MkdirAll(backupDir, 0o700); err != nil {
		return err
	}

	base := filepath.Base(srcPath)
	dst := filepath.Join(backupDir, fmt.Sprintf("%s.%d.bak", base, time.Now().UnixNano()))

	in, err := os.Open(srcPath)
	if err != nil {
		return err
	}
	defer func() { _ = in.Close() }()

	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_EXCL, st.Mode()&0o777)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		_ = out.Close()
		_ = os.Remove(dst)
		return err
	}
	if err := out.Close(); err != nil {
		_ = os.Remove(dst)
		return err
	}

	return PruneOldBackups(backupDir, base, MaxBackupsPerFile)
}

// PruneOldBackups keeps the newest keep backups whose names start with base+"."
// and end with ".bak", deleting older files in backupDir.
func PruneOldBackups(backupDir, base string, keep int) error {
	entries, err := os.ReadDir(backupDir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}

	prefix := base + "."
	var matches []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if strings.HasPrefix(name, prefix) && strings.HasSuffix(name, ".bak") {
			matches = append(matches, name)
		}
	}
	if len(matches) <= keep {
		return nil
	}

	sort.Slice(matches, func(i, j int) bool {
		pi := filepath.Join(backupDir, matches[i])
		pj := filepath.Join(backupDir, matches[j])
		si, err1 := os.Stat(pi)
		sj, err2 := os.Stat(pj)
		if err1 != nil || err2 != nil {
			return matches[i] > matches[j]
		}
		return si.ModTime().After(sj.ModTime())
	})

	for _, name := range matches[keep:] {
		_ = os.Remove(filepath.Join(backupDir, name))
	}
	return nil
}

// MaybeBackupOnVersionChange compares currentVersion to .last-run-version. When the
// version changed (or the marker is missing), it backs up config.toml and
// model-params.json if they exist, then writes the marker. Skips version-based
// backup when currentVersion is empty or "dev" to avoid noisy backups during
// development.
//
//nolint:gosec // G304: marker path derived from os.UserConfigDir — trusted source.
func MaybeBackupOnVersionChange(currentVersion string) error {
	llml, err := LlmlDir()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(llml, 0o700); err != nil {
		return err
	}

	marker := filepath.Join(llml, lastRunVersionFile)
	prev, _ := os.ReadFile(marker)
	prevStr := strings.TrimSpace(string(prev))
	cur := strings.TrimSpace(currentVersion)
	if cur == "" || cur == "dev" {
		return writeVersionMarker(marker, cur)
	}
	if prevStr == cur {
		return nil
	}

	cfgPath, err := ConfigTomlPath()
	if err != nil {
		return err
	}
	if err := BackupFileIfExists(cfgPath); err != nil {
		return err
	}
	mpPath, err := ModelParamsPath()
	if err != nil {
		return err
	}
	if err := BackupFileIfExists(mpPath); err != nil {
		return err
	}

	return writeVersionMarker(marker, cur)
}

func writeVersionMarker(marker, version string) error {
	return os.WriteFile(marker, []byte(strings.TrimSpace(version)+"\n"), 0o600)
}
