package tui

import (
	"time"

	"github.com/atotto/clipboard"

	"github.com/flyingnobita/llml/internal/config"
	"github.com/flyingnobita/llml/internal/models"
	"github.com/flyingnobita/llml/internal/settings"
)

// services collects every dependency the TUI reaches outside its own state:
// the config file, model and runtime discovery, the Ollama daemon, the process
// environment, and the system clipboard.
//
// It exists so those dependencies are visible in one place and injectable.
// Before this, they were package-level function variables that tests reassigned,
// which hid the real dependency graph and made every test that touched one of
// them unsafe to run in parallel.
type services struct {
	// Config file access.
	readConfig        func() (config.Config, error)
	writeConfig       func(config.Config) error
	discoveryConfig   func(configPaths []string, lastScan time.Time) config.DiscoveryConfig
	buildConfig       func(config.RuntimeConfig, config.DiscoveryConfig, []models.ModelFile) config.Config
	runtimeConfig     func(settings.Settings) config.RuntimeConfig
	modelFilesFromCfg func([]config.ModelEntry) []models.ModelFile
	filterExisting    func([]models.ModelFile) []models.ModelFile

	// Discovery.
	discoverRuntime func(settings.Settings) models.RuntimeInfo
	discoverModels  func(models.Options) ([]models.ModelFile, error)
	discoverOllama  func(host string) ([]models.ModelFile, error)

	// Ollama daemon lifecycle.
	startOllamaDaemon func(serverSpec) error
	waitForOllama     func(host string) bool
	probeOllama       func(host string) bool
	preloadOllama     func(host, modelID string) error

	// Environment and clipboard.
	getenv         settings.Getenv
	clipboardWrite func(string) error
}

// defaultServices wires the real implementations. [New] uses it; tests use
// [NewWithServices] with fakes.
func defaultServices() services {
	return services{
		readConfig:        config.ReadFile,
		writeConfig:       config.WriteFile,
		discoveryConfig:   config.DiscoveryConfigFromInputs,
		buildConfig:       config.BuildConfig,
		runtimeConfig:     config.RuntimeConfigFromSettings,
		modelFilesFromCfg: config.ModelFilesFromEntries,
		filterExisting:    config.FilterExistingPaths,

		discoverRuntime: models.DiscoverRuntime,
		discoverModels:  models.Discover,
		discoverOllama:  models.DiscoverOllamaModels,

		startOllamaDaemon: startOllamaDaemon,
		probeOllama:       models.ProbeOllama,
		preloadOllama:     models.PreloadOllamaModel,

		getenv:         settings.OSGetenv,
		clipboardWrite: clipboard.WriteAll,
	}
}

// waitForOllamaOrDefault returns the injected wait function, falling back to the
// real poll loop. The fallback is here rather than in defaultServices because
// the default implementation needs probeOllama from the same services value.
func (s services) waitForOllamaOrDefault(host string) bool {
	if s.waitForOllama != nil {
		return s.waitForOllama(host)
	}
	deadline := time.Now().Add(OllamaStartupTimeout)
	for time.Now().Before(deadline) {
		if s.probeOllama(host) {
			return true
		}
		time.Sleep(OllamaPollInterval)
	}
	return s.probeOllama(host)
}
