package tui

import (
	"errors"
	"os"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/flyingnobita/llml/internal/config"
	"github.com/flyingnobita/llml/internal/models"
)

const themeToastVisibleDuration = 2 * time.Second

func discoverRuntimeCmd() tea.Cmd {
	return func() tea.Msg {
		return runtimeReadyMsg{runtime: models.DiscoverRuntime()}
	}
}

// applyAndFullScanCmd applies [runtime] from config.toml when present, then runs a full discovery and writes config.toml.
func applyAndFullScanCmd(explicitPaths ...string) tea.Cmd {
	return func() tea.Msg {
		cfg, err := config.ReadFile()
		opts := models.Options{}
		var fromFile []string
		if err == nil {
			config.ApplyRuntimeFromConfig(&cfg.Runtime)
			fromFile = cfg.Discovery.ExtraModelPaths
		}
		if len(explicitPaths) > 0 {
			fromFile = explicitPaths
		}
		opts.ExtraRoots = config.MergeExtraRoots(fromFile, config.ExtraModelPathsFromEnv())
		rt := models.DiscoverRuntime()
		files, derr := models.Discover(opts)
		if derr != nil {
			return modelsErrMsg{err: derr}
		}
		now := time.Now()
		disc := config.DiscoveryConfigFromInputs(fromFile, now)
		werr := config.WriteFile(config.BuildConfig(config.RuntimeFromEnv(), disc, files))
		return fullScanDoneMsg{runtime: rt, files: files, writeErr: werr, lastScan: now, configPaths: fromFile}
	}
}

// rescanModelsCmd runs filesystem discovery only (S key); preserves current runtime env and merges discovery metadata into config.toml.
func rescanModelsCmd(explicitPaths ...string) tea.Cmd {
	return func() tea.Msg {
		cfg, err := config.ReadFile()
		opts := models.Options{}
		var fromFile []string
		if err == nil {
			fromFile = cfg.Discovery.ExtraModelPaths
		}
		if len(explicitPaths) > 0 {
			fromFile = explicitPaths
		}
		opts.ExtraRoots = config.MergeExtraRoots(fromFile, config.ExtraModelPathsFromEnv())
		files, derr := models.Discover(opts)
		if derr != nil {
			return modelsErrMsg{err: derr}
		}
		now := time.Now()
		disc := config.DiscoveryConfigFromInputs(fromFile, now)
		werr := config.WriteFile(config.BuildConfig(config.RuntimeFromEnv(), disc, files))
		return modelRescanDoneMsg{files: files, writeErr: werr, lastScan: now, configPaths: fromFile}
	}
}

// reloadRuntimeCmd re-reads [runtime] from config.toml and re-probes binaries (r key).
func reloadRuntimeCmd() tea.Cmd {
	return func() tea.Msg {
		cfg, err := config.ReadFile()
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				return runtimeReloadErrMsg{err: errors.New("config.toml not found — run a full scan first (restart llml or fix config path)")}
			}
			return runtimeReloadErrMsg{err: err}
		}
		config.ApplyRuntimeFromConfig(&cfg.Runtime)
		return runtimeReadyMsg{runtime: models.DiscoverRuntime()}
	}
}

// startupCmd tries the on-disk cache; on miss runs a full scan.
func startupCmd() tea.Cmd {
	return func() tea.Msg {
		cfg, err := config.ReadFile()
		if err != nil || !cfg.ValidForCache() {
			return startupNeedFullScanMsg{}
		}
		config.ApplyRuntimeFromConfig(&cfg.Runtime)
		rt := models.DiscoverRuntime()
		files := config.FilterExistingPaths(config.ModelFilesFromEntries(cfg.Models))
		if len(files) == 0 {
			return startupNeedFullScanMsg{}
		}
		return startupCacheHitMsg{runtime: rt, files: files, lastScan: cfg.Discovery.LastScan, configPaths: cfg.Discovery.ExtraModelPaths}
	}
}

// clearThemeToastAfterCmd schedules removal of the theme banner.
func clearThemeToastAfterCmd() tea.Cmd {
	return tea.Tick(themeToastVisibleDuration, func(time.Time) tea.Msg {
		return themeToastClearMsg{}
	})
}
