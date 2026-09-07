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

// scanResult is everything one discovery pass produces. It replaces a
// seven-value return that mixed two errors with two note strings and had to
// carry a //nolint directive because callers unpacked it by position.
type scanResult struct {
	runtime models.RuntimeInfo
	// settings are the values this pass resolved; the model adopts them.
	settings settings.Settings
	files    []models.ModelFile
	lastScan time.Time
	// configPaths are the extra roots the pass used, echoed back for the UI.
	configPaths []string
	// writeErr is a failed cache write. The scan still succeeded; only the
	// caching of its result did not.
	writeErr error
	// ollamaNote and ollamaWarn report what happened while making the Ollama
	// daemon available, if anything.
	ollamaNote string
	ollamaWarn string
}

// runDiscoveryScan performs one pass: make Ollama available if it is installed
// but stopped, walk the filesystem, merge Ollama rows, and cache the result.
// The returned error means discovery itself failed; everything else is reported
// in the scanResult.
func (svc services) runDiscoveryScan(ctx context.Context, plan discoveryScanPlan) (scanResult, error) {
	res := scanResult{
		runtime:     plan.runtime,
		settings:    plan.settings,
		configPaths: plan.fromFile,
	}
	debugf("runDiscoveryScan: start haveCfg=%t ollamaPath=%q ollamaRunning=%t", plan.haveCfg, res.runtime.OllamaPath, res.runtime.OllamaRunning)

	if res.runtime.OllamaPath != "" && !res.runtime.OllamaRunning {
		spec := discoveryOllamaSpec(res.runtime)
		debugf("runDiscoveryScan: ensuring Ollama ready via bin=%q host=%q", spec.bin, spec.host)
		ready, err := svc.ensureOllamaReady(ctx, spec)
		switch {
		case err != nil:
			res.ollamaWarn = err.Error()
			debugf("runDiscoveryScan: ensureOllamaReady failed: %v", err)
		case ready.Started:
			res.runtime = svc.discoverRuntime(ctx, plan.settings)
			res.ollamaNote = fmt.Sprintf("Started Ollama for model discovery on %s", spec.host)
			debugf("runDiscoveryScan: Ollama started successfully, refreshed runtime running=%t", res.runtime.OllamaRunning)
		}
	}

	files, err := svc.discoverModels(ctx, plan.opts)
	if err != nil {
		debugf("runDiscoveryScan: discoverModels failed: %v", err)
		return res, err
	}
	debugf("runDiscoveryScan: discoverModels returned %d files", len(files))

	// Ollama rows are fetched separately from the filesystem walk; a daemon
	// that is down must not fail the scan.
	if rows, oerr := svc.discoverOllama(ctx, plan.settings.OllamaHost); oerr == nil {
		files = append(files, rows...)
	} else {
		debugf("runDiscoveryScan: ollama discovery failed, continuing: %v", oerr)
	}
	files = mergeCachedOllamaRows(plan.cache, files, res.runtime)
	debugf("runDiscoveryScan: after cache merge -> %d files", len(files))

	res.files = files
	res.lastScan = time.Now()
	// Only the cache is written. config.toml belongs to the user and is
	// rewritten solely when the user saves from a panel.
	if res.writeErr = svc.writeCache(config.CacheFromFiles(files, res.lastScan)); res.writeErr != nil {
		debugf("runDiscoveryScan: writeCache failed: %v", res.writeErr)
	}
	debugf("runDiscoveryScan: done ollamaNote=%q ollamaWarn=%q", res.ollamaNote, res.ollamaWarn)
	return res, nil
}

// discoveryScanCmd runs one discovery pass. mode decides whether the refreshed
// runtime is applied to the model (a full pass) or only the model list is
// (a models-only rescan); the work itself is identical either way.
func (svc services) discoveryScanCmd(ctx context.Context, mode scanMode, explicitPaths ...string) tea.Cmd {
	plan := svc.prepareDiscoveryScan(ctx, explicitPaths)
	scanCmd := func() tea.Msg {
		debugf("discoveryScanCmd: executing scan mode=%v", mode)
		res, err := svc.runDiscoveryScan(ctx, plan)
		if err != nil {
			return modelsErrMsg{err: err}
		}
		return scanDoneMsg{mode: mode, result: res}
	}
	// Starting the daemon takes a moment, so tell the user before blocking.
	if plan.runtime.OllamaPath != "" && !plan.runtime.OllamaRunning {
		debugf("discoveryScanCmd: Ollama installed but stopped, sending startup note then scan")
		return tea.Batch(
			func() tea.Msg { return ollamaDiscoveryStartedMsg{note: discoveryStartNote(plan.runtime)} },
			scanCmd,
		)
	}
	debugf("discoveryScanCmd: no Ollama startup needed")
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
func (m Model) startScan(mode scanMode, explicitPaths ...string) (Model, tea.Cmd) {
	m = m.cancelInFlightScan()
	ctx, cancel := context.WithTimeout(context.Background(), scanTimeout)
	m.scanCancel = cancel
	return m, m.svc.discoveryScanCmd(ctx, mode, explicitPaths...)
}

// scanMode selects between a full pass, which also applies the refreshed
// runtime, and a models-only rescan, which leaves the runtime alone.
type scanMode int

const (
	scanModeFull scanMode = iota
	scanModeModelsOnly
)

// String makes scanMode readable in debug output.
func (s scanMode) String() string {
	if s == scanModeFull {
		return "full"
	}
	return "models-only"
}

// cancelInFlightScan cancels any scan still running and clears the handle.
// Safe to call when no scan is in flight.
func (m Model) cancelInFlightScan() Model {
	if m.scanCancel != nil {
		m.scanCancel()
		m.scanCancel = nil
	}
	return m
}
