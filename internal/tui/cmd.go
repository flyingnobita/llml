package tui

import (
	"errors"
	"fmt"
	"os"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/flyingnobita/llml/internal/config"
	"github.com/flyingnobita/llml/internal/models"
	"github.com/flyingnobita/llml/internal/settings"
)

const themeToastVisibleDuration = 2 * time.Second

// resolveSettings folds the environment, an optional config file, and the
// built-in defaults into one value, in that order of precedence. explicitPaths,
// when non-empty, replaces the extra model roots the config file carries;
// roots from the environment still apply.
func (svc services) resolveSettings(cfg config.Config, haveCfg bool, explicitPaths []string) settings.Settings {
	layers := []settings.Layer{settings.FromEnv(svc.getenv)}
	if haveCfg {
		layer := cfg.Layer()
		if len(explicitPaths) > 0 {
			layer.ExtraModelPaths = explicitPaths
		}
		layers = append(layers, layer)
	} else if len(explicitPaths) > 0 {
		layers = append(layers, settings.Layer{ExtraModelPaths: explicitPaths})
	}
	return settings.Resolve(append(layers, settings.Defaults())...)
}

func mergeCachedOllamaRows(cfg config.Config, files []models.ModelFile, rt models.RuntimeInfo) []models.ModelFile {
	if rt.OllamaRunning {
		return files
	}
	haveLive := false
	for _, f := range files {
		if f.Backend == models.BackendOllama {
			haveLive = true
			break
		}
	}
	if haveLive {
		return files
	}
	cached := config.ModelFilesFromEntries(cfg.Models)
	for _, f := range cached {
		if f.Backend == models.BackendOllama {
			files = append(files, f)
		}
	}
	return files
}

func mergeLiveOllamaRows(files []models.ModelFile, live []models.ModelFile) []models.ModelFile {
	out := make([]models.ModelFile, 0, len(files)+len(live))
	for _, f := range files {
		if f.Backend == models.BackendOllama {
			continue
		}
		out = append(out, f)
	}
	out = append(out, live...)
	return out
}

type discoveryScanPlan struct {
	cfg      config.Config
	haveCfg  bool
	fromFile []string
	settings settings.Settings
	opts     models.Options
	runtime  models.RuntimeInfo
}

func (svc services) prepareDiscoveryScan(explicitPaths []string) discoveryScanPlan {
	cfg, err := svc.readConfig()
	haveCfg := err == nil
	s := svc.resolveSettings(cfg, haveCfg, explicitPaths)

	fromFile := explicitPaths
	if len(fromFile) == 0 && haveCfg {
		fromFile = cfg.Discovery.ExtraModelPaths
	}
	opts := models.Options{Settings: s, IncludeOllama: true}
	debugf("prepareDiscoveryScan: haveCfg=%t explicitPaths=%v fromFile=%v extraRoots=%v", haveCfg, explicitPaths, fromFile, s.ExtraModelPaths)
	rt := svc.discoverRuntime(s)
	debugf("prepareDiscoveryScan: runtime ollamaPath=%q ollamaHost=%q ollamaRunning=%t", rt.OllamaPath, rt.OllamaHost, rt.OllamaRunning)
	return discoveryScanPlan{
		cfg:      cfg,
		haveCfg:  haveCfg,
		fromFile: fromFile,
		settings: s,
		opts:     opts,
		runtime:  rt,
	}
}

func discoveryStartNote(rt models.RuntimeInfo) string {
	spec := discoveryOllamaSpec(rt)
	return fmt.Sprintf("Starting Ollama on %s to discover models...", spec.host)
}

//nolint:staticcheck // ST1008: note strings follow error returns — caller unpacks by position.
func (svc services) runDiscoveryScan(plan discoveryScanPlan) (models.RuntimeInfo, []models.ModelFile, time.Time, error, error, string, string) {
	rt := plan.runtime
	var ollamaNote, ollamaWarn string
	debugf("runDiscoveryScan: start haveCfg=%t ollamaPath=%q ollamaRunning=%t", plan.haveCfg, rt.OllamaPath, rt.OllamaRunning)
	if rt.OllamaPath != "" && !rt.OllamaRunning {
		spec := discoveryOllamaSpec(rt)
		debugf("runDiscoveryScan: ensuring Ollama ready via bin=%q host=%q", spec.bin, spec.host)
		ready, err := svc.ensureOllamaReady(spec)
		if err != nil {
			ollamaWarn = err.Error()
			debugf("runDiscoveryScan: ensureOllamaReady failed: %v", err)
		} else if ready.Started {
			rt = svc.discoverRuntime(plan.settings)
			ollamaNote = fmt.Sprintf("Started Ollama for model discovery on %s", spec.host)
			debugf("runDiscoveryScan: Ollama started successfully, refreshed runtime running=%t", rt.OllamaRunning)
		}
	}
	files, derr := svc.discoverModels(plan.opts)
	if derr != nil {
		debugf("runDiscoveryScan: discoverModels failed: %v", derr)
		return rt, nil, time.Time{}, derr, nil, ollamaNote, ollamaWarn
	}
	debugf("runDiscoveryScan: discoverModels returned %d files", len(files))
	if plan.haveCfg {
		files = mergeCachedOllamaRows(plan.cfg, files, rt)
		debugf("runDiscoveryScan: after cache merge -> %d files", len(files))
	}
	now := time.Now()
	disc := svc.discoveryConfig(plan.fromFile, now)
	werr := svc.writeConfig(svc.buildConfig(svc.runtimeConfig(plan.settings), disc, files))
	if werr != nil {
		debugf("runDiscoveryScan: writeConfig failed: %v", werr)
	}
	debugf("runDiscoveryScan: done ollamaNote=%q ollamaWarn=%q", ollamaNote, ollamaWarn)
	return rt, files, now, nil, werr, ollamaNote, ollamaWarn
}

// applyAndFullScanCmd applies [runtime] from config.toml when present, then runs a full discovery and writes config.toml.
func (svc services) applyAndFullScanCmd(explicitPaths ...string) tea.Cmd {
	plan := svc.prepareDiscoveryScan(explicitPaths)
	scanCmd := func() tea.Msg {
		debugf("applyAndFullScanCmd: executing scan")
		rt, files, now, derr, werr, ollamaNote, ollamaWarn := svc.runDiscoveryScan(plan)
		if derr != nil {
			return modelsErrMsg{err: derr}
		}
		return fullScanDoneMsg{
			runtime:     rt,
			settings:    plan.settings,
			files:       files,
			writeErr:    werr,
			lastScan:    now,
			configPaths: plan.fromFile,
			ollamaNote:  ollamaNote,
			ollamaWarn:  ollamaWarn,
		}
	}
	if plan.runtime.OllamaPath != "" && !plan.runtime.OllamaRunning {
		debugf("applyAndFullScanCmd: Ollama installed but stopped, sending startup note then scan")
		return tea.Batch(
			func() tea.Msg { return ollamaDiscoveryStartedMsg{note: discoveryStartNote(plan.runtime)} },
			scanCmd,
		)
	}
	debugf("applyAndFullScanCmd: no Ollama startup needed")
	return scanCmd
}

// rescanModelsCmd runs filesystem discovery only (S key); preserves current runtime env and merges discovery metadata into config.toml.
func (svc services) rescanModelsCmd(explicitPaths ...string) tea.Cmd {
	plan := svc.prepareDiscoveryScan(explicitPaths)
	scanCmd := func() tea.Msg {
		debugf("rescanModelsCmd: executing scan")
		_, files, now, derr, werr, ollamaNote, ollamaWarn := svc.runDiscoveryScan(plan)
		if derr != nil {
			return modelsErrMsg{err: derr}
		}
		return modelRescanDoneMsg{
			settings:    plan.settings,
			files:       files,
			writeErr:    werr,
			lastScan:    now,
			configPaths: plan.fromFile,
			ollamaNote:  ollamaNote,
			ollamaWarn:  ollamaWarn,
		}
	}
	if plan.runtime.OllamaPath != "" && !plan.runtime.OllamaRunning {
		debugf("rescanModelsCmd: Ollama installed but stopped, sending startup note then scan")
		return tea.Batch(
			func() tea.Msg { return ollamaDiscoveryStartedMsg{note: discoveryStartNote(plan.runtime)} },
			scanCmd,
		)
	}
	debugf("rescanModelsCmd: no Ollama startup needed")
	return scanCmd
}

// reloadRuntimeCmd re-reads [runtime] from config.toml and re-probes binaries (r key).
func (svc services) reloadRuntimeCmd() tea.Cmd {
	return func() tea.Msg {
		cfg, err := svc.readConfig()
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				return runtimeReloadErrMsg{err: errors.New("config.toml not found — run a full scan first (restart llml or fix config path)")}
			}
			return runtimeReloadErrMsg{err: err}
		}
		s := svc.resolveSettings(cfg, true, nil)
		return runtimeReadyMsg{runtime: svc.discoverRuntime(s), settings: s}
	}
}

// startupCmd tries the on-disk cache; on miss runs a full scan.
func (svc services) startupCmd() tea.Cmd {
	return func() tea.Msg {
		cfg, err := svc.readConfig()
		s := svc.resolveSettings(cfg, err == nil, nil)
		if err != nil || !cfg.ValidForCache() {
			debugf("startupCmd: no valid cache, falling back to full scan err=%v valid=%t", err, err == nil && cfg.ValidForCache())
			return startupNeedFullScanMsg{}
		}
		rt := svc.discoverRuntime(s)
		debugf("startupCmd: cache valid, runtime ollamaPath=%q ollamaRunning=%t cachedModels=%d", rt.OllamaPath, rt.OllamaRunning, len(cfg.Models))
		if rt.OllamaPath != "" && !rt.OllamaRunning {
			debugf("startupCmd: Ollama installed but stopped, forcing full scan")
			return startupNeedFullScanMsg{}
		}
		files := svc.filterExisting(svc.modelFilesFromCfg(cfg.Models))
		if len(files) == 0 {
			debugf("startupCmd: cache had no surviving files, forcing full scan")
			return startupNeedFullScanMsg{}
		}
		var writeErr error
		if rt.OllamaRunning {
			liveOllama, err := svc.discoverOllama(s.OllamaHost)
			if err != nil {
				debugf("startupCmd: live Ollama refresh failed, keeping cache: %v", err)
			} else {
				files = mergeLiveOllamaRows(files, liveOllama)
				debugf("startupCmd: merged %d live Ollama rows into cache hit", len(liveOllama))
				writeErr = svc.writeConfig(svc.buildConfig(svc.runtimeConfig(s), cfg.Discovery, files))
				if writeErr != nil {
					debugf("startupCmd: writeConfig after live Ollama refresh failed: %v", writeErr)
				}
			}
		}
		debugf("startupCmd: using cache hit with %d files", len(files))
		return startupCacheHitMsg{
			runtime:     rt,
			settings:    s,
			files:       files,
			lastScan:    cfg.Discovery.LastScan,
			configPaths: cfg.Discovery.ExtraModelPaths,
			writeErr:    writeErr,
		}
	}
}

// clearThemeToastAfterCmd schedules removal of the theme banner.
func clearThemeToastAfterCmd() tea.Cmd {
	return tea.Tick(themeToastVisibleDuration, func(time.Time) tea.Msg {
		return themeToastClearMsg{}
	})
}

// clearLastRunNoteAfterCmd schedules removal of the footer status line (lastRunNote).
func clearLastRunNoteAfterCmd() tea.Cmd {
	return tea.Tick(lastRunNoteVisibleDuration, func(time.Time) tea.Msg {
		return lastRunNoteClearMsg{}
	})
}
