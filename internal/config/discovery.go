package config

import (
	"context"
	"fmt"
	"slices"
	"time"

	"github.com/flyingnobita/llml/internal/models"
	"github.com/flyingnobita/llml/internal/settings"
)

// cacheMaxAge is the maximum age of a discovery cache before it's considered stale.
const cacheMaxAge = 24 * time.Hour

// RunDiscovery runs a full model scan (filesystem plus the Ollama API), writes
// the results to config.toml, and returns the discovered models. The extra
// search roots already carry both the environment and config.toml entries,
// because s was resolved from both. Callers that only need the cached models
// should use CachedModels instead.
func RunDiscovery(ctx context.Context, s settings.Settings) ([]models.ModelFile, error) {
	files, err := models.Discover(ctx, models.Options{Settings: s})
	if err != nil {
		return nil, fmt.Errorf("model discovery failed: %w", err)
	}
	// Ollama rows come from the daemon, separately from the filesystem walk. A
	// daemon that is down or slow must not fail or stall the whole scan.
	if rows, err := models.NewOllamaClient(s.OllamaHost).Tags(ctx); err == nil {
		files = append(files, rows...)
	}

	now := time.Now()
	disc := DiscoveryConfigFromInputs(s.ExtraModelPaths, now)
	cfg := BuildConfig(RuntimeConfigFromSettings(s), disc, files)
	if err := WriteFile(cfg); err != nil {
		return files, fmt.Errorf("writing config after discovery: %w", err)
	}

	return files, nil
}

// CachedModels returns the models from the discovery cache in config.toml.
// Returns (nil, nil) if config.toml doesn't exist.
// Returns (models, nil) if the cache is valid.
// Returns (models, CacheStaleError) if the cache exists but is stale (>24h).
func CachedModels() ([]models.ModelFile, error) {
	cfg, err := ReadFile()
	if err != nil {
		return nil, nil // no config file — return nil, no error
	}

	if !cfg.ValidForCache() {
		return nil, nil
	}

	files := ModelFilesFromEntries(cfg.Models)
	if len(files) == 0 {
		return nil, nil
	}

	if time.Since(cfg.Discovery.LastScan) > cacheMaxAge {
		return files, &CacheStaleError{LastScan: cfg.Discovery.LastScan}
	}

	return files, nil
}

// CacheStaleError signals that the discovery cache exists but is older than
// cacheMaxAge. Callers can use this to trigger a fresh scan or warn.
type CacheStaleError struct {
	LastScan time.Time
}

func (e *CacheStaleError) Error() string {
	return fmt.Sprintf("discovery cache is stale (last scan: %s)", e.LastScan.Format(time.RFC3339))
}

// FilterByBackend returns models whose backend matches any of the given backend
// name strings. Unknown backend strings are silently skipped.
func FilterByBackend(files []models.ModelFile, backends []string) []models.ModelFile {
	allowed := make(map[models.ModelBackend]bool, len(backends))
	for _, b := range backends {
		if mb, err := models.ParseBackend(b); err == nil {
			allowed[mb] = true
		}
	}
	if len(allowed) == 0 {
		return nil
	}
	var out []models.ModelFile
	for _, m := range files {
		if allowed[m.Backend] {
			out = append(out, m)
		}
	}
	return out
}

// ModelBackends returns the sorted, deduplicated backend name strings present in
// a model list. Used for error messages.
func ModelBackends(files []models.ModelFile) []string {
	seen := make(map[string]bool)
	for _, m := range files {
		seen[m.Backend.String()] = true
	}
	var out []string
	for b := range seen {
		out = append(out, b)
	}
	slices.Sort(out)
	return out
}
