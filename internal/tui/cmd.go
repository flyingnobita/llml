package tui

import (
	"context"
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

func mergeCachedOllamaRows(cached config.CacheFile, files []models.ModelFile, rt models.RuntimeInfo) []models.ModelFile {
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
	for _, f := range config.ModelFilesFromEntries(cached.Models) {
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
	cache    config.CacheFile
	haveCfg  bool
	fromFile []string
	settings settings.Settings
	opts     models.Options
	runtime  models.RuntimeInfo
}

func (svc services) prepareDiscoveryScan(ctx context.Context, explicitPaths []string) discoveryScanPlan {
	cfg, err := svc.readConfig()
	haveCfg := err == nil
	s := svc.resolveSettings(cfg, haveCfg, explicitPaths)
	cache, _ := svc.readCache() // a missing cache is normal; the scan repopulates it

	fromFile := explicitPaths
	if len(fromFile) == 0 && haveCfg {
		fromFile = cfg.Discovery.ExtraModelPaths
	}
	opts := models.Options{Settings: s}
	debugf("prepareDiscoveryScan: haveCfg=%t explicitPaths=%v fromFile=%v extraRoots=%v", haveCfg, explicitPaths, fromFile, s.ExtraModelPaths)
	rt := svc.discoverRuntime(ctx, s)
	debugf("prepareDiscoveryScan: runtime ollamaPath=%q ollamaHost=%q ollamaRunning=%t", rt.OllamaPath, rt.OllamaHost, rt.OllamaRunning)
	return discoveryScanPlan{
		cfg:      cfg,
		cache:    cache,
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
func (svc services) runDiscoveryScan(ctx context.Context, plan discoveryScanPlan) (models.RuntimeInfo, []models.ModelFile, time.Time, error, error, string, string) {
	rt := plan.runtime
	var ollamaNote, ollamaWarn string
	debugf("runDiscoveryScan: start haveCfg=%t ollamaPath=%q ollamaRunning=%t", plan.haveCfg, rt.OllamaPath, rt.OllamaRunning)
	if rt.OllamaPath != "" && !rt.OllamaRunning {
		spec := discoveryOllamaSpec(rt)
		debugf("runDiscoveryScan: ensuring Ollama ready via bin=%q host=%q", spec.bin, spec.host)
		ready, err := svc.ensureOllamaReady(ctx, spec)
		if err != nil {
			ollamaWarn = err.Error()
			debugf("runDiscoveryScan: ensureOllamaReady failed: %v", err)
		} else if ready.Started {
			rt = svc.discoverRuntime(ctx, plan.settings)
			ollamaNote = fmt.Sprintf("Started Ollama for model discovery on %s", spec.host)
			debugf("runDiscoveryScan: Ollama started successfully, refreshed runtime running=%t", rt.OllamaRunning)
		}
	}
	files, derr := svc.discoverModels(ctx, plan.opts)
	if derr != nil {
		debugf("runDiscoveryScan: discoverModels failed: %v", derr)
		return rt, nil, time.Time{}, derr, nil, ollamaNote, ollamaWarn
	}
	debugf("runDiscoveryScan: discoverModels returned %d files", len(files))
	// Ollama rows are fetched separately from the filesystem walk; a daemon
	// that is down must not fail the scan.
	if rows, oerr := svc.discoverOllama(ctx, plan.settings.OllamaHost); oerr == nil {
		files = append(files, rows...)
	} else {
		debugf("runDiscoveryScan: ollama discovery failed, continuing: %v", oerr)
	}
	files = mergeCachedOllamaRows(plan.cache, files, rt)
	debugf("runDiscoveryScan: after cache merge -> %d files", len(files))

	now := time.Now()
	// Only the cache is written. config.toml belongs to the user and is
	// rewritten solely when the user saves from a panel.
	werr := svc.writeCache(config.CacheFromFiles(files, now))
	if werr != nil {
		debugf("runDiscoveryScan: writeCache failed: %v", werr)
	}
	debugf("runDiscoveryScan: done ollamaNote=%q ollamaWarn=%q", ollamaNote, ollamaWarn)
	return rt, files, now, nil, werr, ollamaNote, ollamaWarn
}

// applyAndFullScanCmd applies [runtime] from config.toml when present, then runs a full discovery and writes config.toml.
func (svc services) applyAndFullScanCmd(ctx context.Context, explicitPaths ...string) tea.Cmd {
	plan := svc.prepareDiscoveryScan(ctx, explicitPaths)
	scanCmd := func() tea.Msg {
		debugf("applyAndFullScanCmd: executing scan")
		rt, files, now, derr, werr, ollamaNote, ollamaWarn := svc.runDiscoveryScan(ctx, plan)
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
func (svc services) rescanModelsCmd(ctx context.Context, explicitPaths ...string) tea.Cmd {
	plan := svc.prepareDiscoveryScan(ctx, explicitPaths)
	scanCmd := func() tea.Msg {
		debugf("rescanModelsCmd: executing scan")
		_, files, now, derr, werr, ollamaNote, ollamaWarn := svc.runDiscoveryScan(ctx, plan)
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
		ctx, cancel := context.WithTimeout(context.Background(), runtimeProbeTimeout)
		defer cancel()
		s := svc.resolveSettings(cfg, true, nil)
		return runtimeReadyMsg{runtime: svc.discoverRuntime(ctx, s), settings: s}
	}
}

// startupCmd tries the on-disk cache; on miss runs a full scan.
func (svc services) startupCmd() tea.Cmd {
	return func() tea.Msg {
		// Startup owns its context end to end: it runs once, and the model has
		// no handle to cancel it, so the timeout is what bounds it.
		ctx, cancel := context.WithTimeout(context.Background(), scanTimeout)
		defer cancel()
		cfg, cfgErr := svc.readConfig()
		s := svc.resolveSettings(cfg, cfgErr == nil, nil)
		cached, err := svc.readCache()
		if err != nil || !cached.ValidForCache() {
			debugf("startupCmd: no valid cache, falling back to full scan err=%v valid=%t", err, err == nil && cached.ValidForCache())
			return startupNeedFullScanMsg{}
		}
		rt := svc.discoverRuntime(ctx, s)
		debugf("startupCmd: cache valid, runtime ollamaPath=%q ollamaRunning=%t cachedModels=%d", rt.OllamaPath, rt.OllamaRunning, len(cached.Models))
		if rt.OllamaPath != "" && !rt.OllamaRunning {
			debugf("startupCmd: Ollama installed but stopped, forcing full scan")
			return startupNeedFullScanMsg{}
		}
		files := svc.filterExisting(svc.modelFilesFromCfg(cached.Models))
		if len(files) == 0 {
			debugf("startupCmd: cache had no surviving files, forcing full scan")
			return startupNeedFullScanMsg{}
		}
		var writeErr error
		if rt.OllamaRunning {
			liveOllama, err := svc.discoverOllama(ctx, s.OllamaHost)
			if err != nil {
				debugf("startupCmd: live Ollama refresh failed, keeping cache: %v", err)
			} else {
				files = mergeLiveOllamaRows(files, liveOllama)
				debugf("startupCmd: merged %d live Ollama rows into cache hit", len(liveOllama))
				writeErr = svc.writeCache(config.CacheFromFiles(files, cached.LastScan))
				if writeErr != nil {
					debugf("startupCmd: writeCache after live Ollama refresh failed: %v", writeErr)
				}
			}
		}
		debugf("startupCmd: using cache hit with %d files", len(files))
		return startupCacheHitMsg{
			runtime:     rt,
			settings:    s,
			files:       files,
			lastScan:    cached.LastScan,
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

// startScan begins a discovery pass, storing the pass's CancelFunc on the model
// so quitting (or starting another scan) can abandon it. A scan makes network
// calls and walks the filesystem, so leaving one running after the user quits
// would keep the process alive doing work nobody is waiting for.
func (m Model) startScan(mode scanStartMode, explicitPaths ...string) (Model, tea.Cmd) {
	m = m.cancelInFlightScan()
	ctx, cancel := context.WithTimeout(context.Background(), scanTimeout)
	m.scanCancel = cancel
	if mode == scanStartFull {
		return m, m.svc.applyAndFullScanCmd(ctx, explicitPaths...)
	}
	return m, m.svc.rescanModelsCmd(ctx, explicitPaths...)
}

// scanStartMode selects between a full pass (runtime probe plus models) and a
// models-only rescan.
type scanStartMode int

const (
	scanStartFull scanStartMode = iota
	scanStartModelsOnly
)

// cancelInFlightScan cancels any scan still running and clears the handle.
// Safe to call when no scan is in flight.
func (m Model) cancelInFlightScan() Model {
	if m.scanCancel != nil {
		m.scanCancel()
		m.scanCancel = nil
	}
	return m
}
