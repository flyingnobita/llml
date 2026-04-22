package tui

import (
	"errors"
	"fmt"
	"os"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/flyingnobita/llml/internal/config"
	"github.com/flyingnobita/llml/internal/models"
)

const themeToastVisibleDuration = 2 * time.Second

var (
	readConfigFileFn         = config.ReadFile
	writeConfigFileFn        = config.WriteFile
	applyRuntimeFromConfigFn = config.ApplyRuntimeFromConfig
	extraModelPathsFromEnvFn = config.ExtraModelPathsFromEnv
	mergeExtraRootsFn        = config.MergeExtraRoots
	discoveryConfigInputsFn  = config.DiscoveryConfigFromInputs
	buildConfigFn            = config.BuildConfig
	runtimeFromEnvFn         = config.RuntimeFromEnv
	modelFilesFromEntriesFn  = config.ModelFilesFromEntries
	filterExistingPathsFn    = config.FilterExistingPaths
	discoverRuntimeFn        = models.DiscoverRuntime
	discoverModelsFn         = models.Discover
	discoverOllamaModelsFn   = models.DiscoverOllamaModels
)

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
	opts     models.Options
	runtime  models.RuntimeInfo
}

func prepareDiscoveryScan(explicitPaths []string) discoveryScanPlan {
	cfg, err := readConfigFileFn()
	opts := models.Options{}
	var fromFile []string
	if err == nil {
		applyRuntimeFromConfigFn(&cfg.Runtime)
		fromFile = cfg.Discovery.ExtraModelPaths
	}
	if len(explicitPaths) > 0 {
		fromFile = explicitPaths
	}
	opts.ExtraRoots = mergeExtraRootsFn(fromFile, extraModelPathsFromEnvFn())
	debugf("prepareDiscoveryScan: haveCfg=%t explicitPaths=%v fromFile=%v extraRoots=%v", err == nil, explicitPaths, fromFile, opts.ExtraRoots)
	rt := discoverRuntimeFn()
	debugf("prepareDiscoveryScan: runtime ollamaPath=%q ollamaHost=%q ollamaRunning=%t", rt.OllamaPath, rt.OllamaHost, rt.OllamaRunning)
	return discoveryScanPlan{
		cfg:      cfg,
		haveCfg:  err == nil,
		fromFile: fromFile,
		opts:     opts,
		runtime:  rt,
	}
}

func discoveryStartNote(rt models.RuntimeInfo) string {
	spec := discoveryOllamaSpec(rt)
	return fmt.Sprintf("Starting Ollama on %s to discover models...", spec.host)
}

func runDiscoveryScan(plan discoveryScanPlan) (models.RuntimeInfo, []models.ModelFile, time.Time, error, error, string, string) {
	rt := plan.runtime
	var ollamaNote, ollamaWarn string
	debugf("runDiscoveryScan: start haveCfg=%t ollamaPath=%q ollamaRunning=%t", plan.haveCfg, rt.OllamaPath, rt.OllamaRunning)
	if rt.OllamaPath != "" && !rt.OllamaRunning {
		spec := discoveryOllamaSpec(rt)
		debugf("runDiscoveryScan: ensuring Ollama ready via bin=%q host=%q", spec.bin, spec.host)
		ready, err := ensureOllamaReady(spec)
		if err != nil {
			ollamaWarn = err.Error()
			debugf("runDiscoveryScan: ensureOllamaReady failed: %v", err)
		} else if ready.Started {
			rt = discoverRuntimeFn()
			ollamaNote = fmt.Sprintf("Started Ollama for model discovery on %s", spec.host)
			debugf("runDiscoveryScan: Ollama started successfully, refreshed runtime running=%t", rt.OllamaRunning)
		}
	}
	files, derr := discoverModelsFn(plan.opts)
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
	disc := discoveryConfigInputsFn(plan.fromFile, now)
	werr := writeConfigFileFn(buildConfigFn(runtimeFromEnvFn(), disc, files))
	if werr != nil {
		debugf("runDiscoveryScan: writeConfig failed: %v", werr)
	}
	debugf("runDiscoveryScan: done ollamaNote=%q ollamaWarn=%q", ollamaNote, ollamaWarn)
	return rt, files, now, nil, werr, ollamaNote, ollamaWarn
}

// applyAndFullScanCmd applies [runtime] from config.toml when present, then runs a full discovery and writes config.toml.
func applyAndFullScanCmd(explicitPaths ...string) tea.Cmd {
	plan := prepareDiscoveryScan(explicitPaths)
	scanCmd := func() tea.Msg {
		debugf("applyAndFullScanCmd: executing scan")
		rt, files, now, derr, werr, ollamaNote, ollamaWarn := runDiscoveryScan(plan)
		if derr != nil {
			return modelsErrMsg{err: derr}
		}
		return fullScanDoneMsg{
			runtime:     rt,
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
func rescanModelsCmd(explicitPaths ...string) tea.Cmd {
	plan := prepareDiscoveryScan(explicitPaths)
	scanCmd := func() tea.Msg {
		debugf("rescanModelsCmd: executing scan")
		_, files, now, derr, werr, ollamaNote, ollamaWarn := runDiscoveryScan(plan)
		if derr != nil {
			return modelsErrMsg{err: derr}
		}
		return modelRescanDoneMsg{
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
func reloadRuntimeCmd() tea.Cmd {
	return func() tea.Msg {
		cfg, err := readConfigFileFn()
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				return runtimeReloadErrMsg{err: errors.New("config.toml not found — run a full scan first (restart llml or fix config path)")}
			}
			return runtimeReloadErrMsg{err: err}
		}
		applyRuntimeFromConfigFn(&cfg.Runtime)
		return runtimeReadyMsg{runtime: discoverRuntimeFn()}
	}
}

// startupCmd tries the on-disk cache; on miss runs a full scan.
func startupCmd() tea.Cmd {
	return func() tea.Msg {
		cfg, err := readConfigFileFn()
		if err != nil || !cfg.ValidForCache() {
			debugf("startupCmd: no valid cache, falling back to full scan err=%v valid=%t", err, err == nil && cfg.ValidForCache())
			return startupNeedFullScanMsg{}
		}
		applyRuntimeFromConfigFn(&cfg.Runtime)
		rt := discoverRuntimeFn()
		debugf("startupCmd: cache valid, runtime ollamaPath=%q ollamaRunning=%t cachedModels=%d", rt.OllamaPath, rt.OllamaRunning, len(cfg.Models))
		if rt.OllamaPath != "" && !rt.OllamaRunning {
			debugf("startupCmd: Ollama installed but stopped, forcing full scan")
			return startupNeedFullScanMsg{}
		}
		files := filterExistingPathsFn(modelFilesFromEntriesFn(cfg.Models))
		if len(files) == 0 {
			debugf("startupCmd: cache had no surviving files, forcing full scan")
			return startupNeedFullScanMsg{}
		}
		var writeErr error
		if rt.OllamaRunning {
			liveOllama, err := discoverOllamaModelsFn()
			if err != nil {
				debugf("startupCmd: live Ollama refresh failed, keeping cache: %v", err)
			} else {
				files = mergeLiveOllamaRows(files, liveOllama)
				debugf("startupCmd: merged %d live Ollama rows into cache hit", len(liveOllama))
				writeErr = writeConfigFileFn(buildConfigFn(runtimeFromEnvFn(), cfg.Discovery, files))
				if writeErr != nil {
					debugf("startupCmd: writeConfig after live Ollama refresh failed: %v", writeErr)
				}
			}
		}
		debugf("startupCmd: using cache hit with %d files", len(files))
		return startupCacheHitMsg{
			runtime:     rt,
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
