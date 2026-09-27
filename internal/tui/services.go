package tui

import (
	"context"
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
	buildConfig       func(config.RuntimeConfig, config.DiscoveryConfig) config.Config
	readCache         func() (config.CacheFile, error)
	writeCache        func(config.CacheFile) error
	runtimeConfig     func(settings.Settings) config.RuntimeConfig
	modelFilesFromCfg func([]config.ModelEntry) []models.ModelFile
	filterExisting    func([]models.ModelFile) []models.ModelFile

	// Runtime state store (runtimes.toml): which Runtimes are on.
	readRuntimeStates  func() (config.RuntimeStates, error)
	writeRuntimeStates func(config.RuntimeStates) error

	// Discovery. Every call that can block on I/O takes a context so an
	// abandoned scan stops instead of running to a fixed timeout. Runtime
	// detection sends no probe to a Runtime in its skip set.
	discoverRuntime func(ctx context.Context, s settings.Settings, skip models.BackendSet) models.RuntimeInfo
	discoverModels  func(context.Context, models.Options) ([]models.ModelFile, error)
	discoverOllama  func(ctx context.Context, host string) ([]models.ModelFile, error)

	// Ollama daemon lifecycle.
	startOllamaDaemon func(serverSpec) error
	waitForOllama     func(ctx context.Context, host string) bool
	probeOllama       func(ctx context.Context, host string) bool
	preloadOllama     func(ctx context.Context, host, modelID string) error

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
		buildConfig:       config.BuildConfig,
		readCache:         config.ReadCache,
		writeCache:        config.WriteCache,
		runtimeConfig:     config.RuntimeConfigFromSettings,
		modelFilesFromCfg: config.ModelFilesFromEntries,
		filterExisting:    config.FilterExistingPaths,

		readRuntimeStates:  config.ReadRuntimeStates,
		writeRuntimeStates: config.WriteRuntimeStates,

		discoverRuntime: models.DiscoverRuntime,
		discoverModels:  models.Discover,
		discoverOllama: func(ctx context.Context, host string) ([]models.ModelFile, error) {
			return models.NewOllamaClient(host).Tags(ctx)
		},

		startOllamaDaemon: startOllamaDaemon,
		probeOllama: func(ctx context.Context, host string) bool {
			return models.NewOllamaClient(host).Probe(ctx)
		},
		preloadOllama: func(ctx context.Context, host, modelID string) error {
			return models.NewOllamaClient(host).Preload(ctx, modelID)
		},

		getenv:         settings.OSGetenv,
		clipboardWrite: clipboard.WriteAll,
	}
}

// waitForOllamaOrDefault returns the injected wait function, falling back to the
// real poll loop. The fallback is here rather than in defaultServices because
// the default implementation needs probeOllama from the same services value.
func (s services) waitForOllamaOrDefault(ctx context.Context, host string) bool {
	if s.waitForOllama != nil {
		return s.waitForOllama(ctx, host)
	}
	ctx, cancel := context.WithTimeout(ctx, OllamaStartupTimeout)
	defer cancel()
	for {
		if s.probeOllama(ctx, host) {
			return true
		}
		select {
		case <-ctx.Done():
			return false
		case <-time.After(OllamaPollInterval):
		}
	}
}

// runtimeStatesRead is the runtime state store as one command read it. It
// travels back to the model with the detection it steered, so the model adopts
// the same on/off state detection used.
type runtimeStatesRead struct {
	states config.RuntimeStates
	// err is a failed read. The states are then empty: every Runtime is on.
	err error
}

// detectRuntime reads the runtime state store and runs detection, skipping the
// Disabled Runtimes.
func (s services) detectRuntime(ctx context.Context, set settings.Settings) (models.RuntimeInfo, runtimeStatesRead) {
	states, err := s.readRuntimeStates()
	if err != nil {
		states = config.RuntimeStates{}
	}
	return s.discoverRuntime(ctx, set, states.Disabled()), runtimeStatesRead{states: states, err: err}
}
