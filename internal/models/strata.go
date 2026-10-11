package models

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

// strataConfigPrefix is the name prefix Strata's setup gives each engine
// config: strata-<model>.json. The suffix keeps a launcher's siblings
// (strata-<model>.json.bak, the log, and the run scripts) out.
const strataConfigPrefix = "strata-"

// strataConfig is the part of a Strata engine config (strata-<model>.json)
// that llml reads. The rest (cwd, log, lib_dirs, gpu) is Strata's own.
type strataConfig struct {
	Exe       string   `json:"exe"`
	Args      []string `json:"args"`
	Tokenizer string   `json:"tokenizer"`
	ModelName string   `json:"model_name"`
	Port      int      `json:"port"`
}

// discoverStrata returns one row per Strata engine config in roots. Strata
// installs its configs in the install directory; a Docker install keeps them
// in a config/ subdirectory. Roots that do not exist are skipped, and a file
// that is not a usable engine config (the .shared-settings.json, a backup) is
// ignored. Only the configs are read: walking a whole install would traverse a
// multi-gigabyte model pack and surface its GSQ-RCO .gguf shards as llama.cpp
// rows.
func discoverStrata(roots []string) []ModelFile {
	var out []ModelFile
	seen := map[string]struct{}{}
	for _, root := range roots {
		for _, dir := range []string{root, filepath.Join(root, "config")} {
			ents, err := os.ReadDir(dir)
			if err != nil {
				continue
			}
			for _, ent := range ents {
				if ent.IsDir() || !isStrataConfigName(ent.Name()) {
					continue
				}
				path := filepath.Join(dir, ent.Name())
				if _, ok := seen[path]; ok {
					continue
				}
				if f, ok := buildStrataModel(path); ok {
					seen[path] = struct{}{}
					out = append(out, f)
				}
			}
		}
	}
	return out
}

// isStrataConfigName reports whether name is an engine config file
// (strata-<model>.json). A .json.bak backup and the .shared-settings.json
// fail the suffix test.
func isStrataConfigName(name string) bool {
	return strings.HasPrefix(name, strataConfigPrefix) && strings.HasSuffix(name, ".json")
}

// buildStrataModel reads one engine config and builds its row. It reports
// false for a file that does not parse or lacks the exe/args/model_name a
// launchable config always has.
//
//nolint:gosec // G304: path comes from the Strata install directory listing.
func buildStrataModel(path string) (ModelFile, bool) {
	data, err := os.ReadFile(path)
	if err != nil {
		return ModelFile{}, false
	}
	var cfg strataConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return ModelFile{}, false
	}
	if strings.TrimSpace(cfg.Exe) == "" || len(cfg.Args) == 0 || strings.TrimSpace(cfg.ModelName) == "" {
		return ModelFile{}, false
	}
	fi, err := os.Stat(path)
	if err != nil {
		return ModelFile{}, false
	}
	clean := filepath.Clean(path)
	return ModelFile{
		Backend:    BackendStrata,
		ID:         clean,
		Path:       clean,
		Location:   clean,
		Name:       cfg.ModelName,
		Size:       strataPackSize(cfg.Args),
		ModTime:    fi.ModTime(),
		Parameters: strataParamsSummary(cfg),
	}, true
}

// strataWeightFlags are the engine args naming the model's GGUF weight shards,
// whose sizes make up the Size column.
var strataWeightFlags = map[string]bool{"--native": true, "--ple-gguf": true}

// strataPackSize sums the sizes of the weight shards named in args. It is 0
// when none are readable.
func strataPackSize(args []string) int64 {
	var total int64
	for i, a := range args {
		if !strataWeightFlags[a] || i+1 >= len(args) {
			continue
		}
		if fi, err := os.Stat(args[i+1]); err == nil && fi.Mode().IsRegular() {
			total += fi.Size()
		}
	}
	return total
}

// strataParamsSummary returns the Parameters cell for a Strata config:
// "strata · <size> · ctx <n>", dropping the parts the config does not name.
func strataParamsSummary(cfg strataConfig) string {
	parts := []string{"strata"}
	if s := strataSizeLabel(cfg.ModelName); s != "" {
		parts = append(parts, s)
	}
	if ctx := strataArgValue(cfg.Args, "--max-context"); ctx != "" {
		parts = append(parts, "ctx "+ctx)
	}
	if len(parts) == 1 {
		return "strata · —"
	}
	return strings.Join(parts, " · ")
}

// strataSizeLabel is the pack size (IQ3_S, IQ2_XS, ...) taken from the last
// dash-separated token of the model name.
func strataSizeLabel(modelName string) string {
	name := strings.TrimSpace(modelName)
	if name == "" {
		return ""
	}
	if i := strings.LastIndexByte(name, '-'); i >= 0 && i+1 < len(name) {
		name = name[i+1:]
	}
	return strings.ToUpper(name)
}

// strataArgValue returns the value that follows flag in args, or "".
func strataArgValue(args []string, flag string) string {
	for i, a := range args {
		if a == flag && i+1 < len(args) {
			return args[i+1]
		}
	}
	return ""
}
