package config

import (
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/BurntSushi/toml"

	"github.com/flyingnobita/llml/internal/fsutil"
	"github.com/flyingnobita/llml/internal/models"
)

// CacheSchemaVersion is the current on-disk format for cache/models.toml.
const CacheSchemaVersion = 1

// CacheFile is the machine-owned discovery cache, stored separately from
// config.toml so a background scan never rewrites the file the user hand-edits.
// It is disposable: deleting it costs one rescan.
type CacheFile struct {
	SchemaVersion int          `toml:"schema_version"`
	LastScan      time.Time    `toml:"last_scan"`
	Models        []ModelEntry `toml:"models"`
}

// CachePath returns {UserConfigDir}/llml/cache/models.toml.
func CachePath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "llml", "cache", "models.toml"), nil
}

// ReadCache reads and parses cache/models.toml. A missing file returns the
// underlying os.ErrNotExist, which callers treat as "no cache".
//
//nolint:gosec // G304: path from CachePath() using os.UserConfigDir — trusted source.
func ReadCache() (CacheFile, error) {
	path, err := CachePath()
	if err != nil {
		return CacheFile{}, err
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return CacheFile{}, err
	}
	var c CacheFile
	if _, err := toml.Decode(string(b), &c); err != nil {
		return CacheFile{}, err
	}
	return c, nil
}

// WriteCache writes cache/models.toml atomically. Unlike [WriteFile] it takes no
// backup: the cache is reproducible by rescanning, so snapshots would only
// accumulate.
func WriteCache(c CacheFile) error {
	path, err := CachePath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	c.SchemaVersion = CacheSchemaVersion
	var buf strings.Builder
	if err := toml.NewEncoder(&buf).Encode(c); err != nil {
		return err
	}
	return fsutil.WriteFileAtomic(path, []byte(buf.String()), 0o644)
}

// ValidForCache reports whether the cache is usable for instant startup
// (skipping the filesystem walk).
func (c CacheFile) ValidForCache() bool {
	return c.SchemaVersion == CacheSchemaVersion && len(c.Models) > 0
}

// CacheFromFiles builds a cache document from a completed scan.
func CacheFromFiles(files []models.ModelFile, lastScan time.Time) CacheFile {
	c := CacheFile{SchemaVersion: CacheSchemaVersion, LastScan: lastScan}
	for _, f := range files {
		c.Models = append(c.Models, ModelEntryFromFile(f))
	}
	return c
}
