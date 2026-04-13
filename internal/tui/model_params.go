package tui

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

const modelParamsFileVersion = 2

// EnvVar is one environment variable applied when launching the server for a model.
type EnvVar struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

// ModelParams holds extra environment variables and argv tokens for one parameter profile.
type ModelParams struct {
	Env  []EnvVar `json:"env"`
	Args []string `json:"args"`
}

// ParameterProfile is one named parameter profile: the env vars and command-line tokens used to run a model.
type ParameterProfile struct {
	Name string   `json:"name"`
	Env  []EnvVar `json:"env"`
	Args []string `json:"args"`
}

// modelEntry is stored per model path: several parameter profiles and which one to use when pressing R.
type modelEntry struct {
	Profiles    []ParameterProfile `json:"profiles"`
	ActiveIndex int                `json:"activeIndex"`
}

type modelParamsV1 struct {
	Env  []EnvVar `json:"env"`
	Args []string `json:"args"`
}

type modelParamsFile struct {
	Version int                        `json:"version"`
	Models  map[string]json.RawMessage `json:"models"`
}

func modelParamsConfigPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "llm-launch", "model-params.json"), nil
}

func parseModelEntry(raw json.RawMessage) (modelEntry, error) {
	var probe map[string]json.RawMessage
	if err := json.Unmarshal(raw, &probe); err != nil {
		return modelEntry{}, err
	}
	if _, ok := probe["profiles"]; ok {
		var e modelEntry
		if err := json.Unmarshal(raw, &e); err != nil {
			return modelEntry{}, err
		}
		if len(e.Profiles) == 0 {
			e.Profiles = []ParameterProfile{{Name: "default", Env: nil, Args: nil}}
		}
		for i := range e.Profiles {
			if strings.TrimSpace(e.Profiles[i].Name) == "" {
				e.Profiles[i].Name = fmt.Sprintf("parameter profile %d", i+1)
			}
		}
		e.ActiveIndex = clampInt(e.ActiveIndex, 0, len(e.Profiles)-1)
		return e, nil
	}
	var v1 modelParamsV1
	if err := json.Unmarshal(raw, &v1); err != nil {
		return modelEntry{}, err
	}
	return modelEntry{
		Profiles: []ParameterProfile{
			{Name: "default", Env: v1.Env, Args: v1.Args},
		},
		ActiveIndex: 0,
	}, nil
}

func clampInt(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func readParamsFile(path string) (modelParamsFile, error) {
	var f modelParamsFile
	b, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			f.Models = make(map[string]json.RawMessage)
			return f, nil
		}
		return f, err
	}
	if err := json.Unmarshal(b, &f); err != nil {
		return f, err
	}
	if f.Models == nil {
		f.Models = make(map[string]json.RawMessage)
	}
	return f, nil
}

// loadModelEntry returns stored parameter profiles for modelPath, or one empty "default" parameter profile if none.
func loadModelEntry(modelPath string) (modelEntry, error) {
	cfgPath, err := modelParamsConfigPath()
	if err != nil {
		return modelEntry{}, err
	}
	key := filepath.Clean(modelPath)
	f, err := readParamsFile(cfgPath)
	if err != nil {
		return modelEntry{}, err
	}
	raw, ok := f.Models[key]
	if !ok {
		return modelEntry{
			Profiles:    []ParameterProfile{{Name: "default", Env: nil, Args: nil}},
			ActiveIndex: 0,
		}, nil
	}
	return parseModelEntry(raw)
}

// saveModelEntry writes the entry for modelPath and preserves other models in the file.
func saveModelEntry(modelPath string, ent modelEntry) error {
	cfgPath, err := modelParamsConfigPath()
	if err != nil {
		return err
	}
	key := filepath.Clean(modelPath)
	if err := os.MkdirAll(filepath.Dir(cfgPath), 0o755); err != nil {
		return err
	}
	f, err := readParamsFile(cfgPath)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if f.Models == nil {
		f.Models = make(map[string]json.RawMessage)
	}
	f.Version = modelParamsFileVersion
	ent = normalizeModelEntry(ent)
	if len(ent.Profiles) == 0 {
		delete(f.Models, key)
	} else {
		raw, err := json.Marshal(ent)
		if err != nil {
			return err
		}
		f.Models[key] = raw
	}
	out, err := json.MarshalIndent(&f, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(cfgPath, out, 0o644)
}

func normalizeModelEntry(ent modelEntry) modelEntry {
	var profiles []ParameterProfile
	for i := range ent.Profiles {
		p := ent.Profiles[i]
		nm := normalizeModelParams(ModelParams{Env: p.Env, Args: p.Args})
		name := strings.TrimSpace(p.Name)
		if name == "" {
			name = fmt.Sprintf("parameter profile %d", len(profiles)+1)
		}
		profiles = append(profiles, ParameterProfile{Name: name, Env: nm.Env, Args: nm.Args})
	}
	if len(profiles) == 0 {
		return modelEntry{}
	}
	idx := clampInt(ent.ActiveIndex, 0, len(profiles)-1)
	return modelEntry{Profiles: profiles, ActiveIndex: idx}
}

// loadModelParamsForRun returns the active parameter profile's env/args for modelPath (for R / server launch).
func loadModelParamsForRun(modelPath string) (ModelParams, error) {
	ent, err := loadModelEntry(modelPath)
	if err != nil {
		return ModelParams{}, err
	}
	if len(ent.Profiles) == 0 {
		return ModelParams{}, nil
	}
	idx := clampInt(ent.ActiveIndex, 0, len(ent.Profiles)-1)
	p := ent.Profiles[idx]
	return normalizeModelParams(ModelParams{Env: p.Env, Args: p.Args}), nil
}

// mergeEnv overlays extra on base: keys present in extra replace any existing assignment.
func mergeEnv(base []string, extra []EnvVar) []string {
	drop := make(map[string]struct{})
	for _, e := range extra {
		if e.Key != "" {
			drop[e.Key] = struct{}{}
		}
	}
	var out []string
	for _, line := range base {
		k := line
		if i := strings.IndexByte(line, '='); i >= 0 {
			k = line[:i]
		}
		if _, ok := drop[k]; ok {
			continue
		}
		out = append(out, line)
	}
	for _, e := range extra {
		if e.Key != "" {
			out = append(out, e.Key+"="+e.Value)
		}
	}
	return out
}

// shellEnvPrefix emits VAR='value' assignments for a shell command prefix (empty if none).
func shellEnvPrefix(env []EnvVar) string {
	var b strings.Builder
	for _, e := range env {
		if e.Key == "" {
			continue
		}
		b.WriteString(e.Key)
		b.WriteByte('=')
		b.WriteString(shellSingleQuoted(e.Value))
		b.WriteByte(' ')
	}
	return b.String()
}

// shellWord prints a for POSIX sh: unquoted when the word is clearly safe, else
// single-quoted. This makes echoed commands readable (--max-model-len 4096 vs
// '--max-model-len' '4096') while remaining safe for typical flags and paths.
var shellSafeWord = regexp.MustCompile(`^[./a-zA-Z0-9_:=@,.+-]+$`)

func shellWord(a string) string {
	if a != "" && shellSafeWord.MatchString(a) {
		return a
	}
	return shellSingleQuoted(a)
}

func joinShellArgv(args []string) string {
	if len(args) == 0 {
		return ""
	}
	var b strings.Builder
	for i, a := range args {
		if i > 0 {
			b.WriteByte(' ')
		}
		b.WriteString(shellWord(a))
	}
	return b.String()
}

// collapseArgsForDisplay merges argv token pairs (flag + value) into one TUI line
// when the value is not another flag. Stored data remains a flat argv slice.
func collapseArgsForDisplay(tokens []string) []string {
	if len(tokens) == 0 {
		return nil
	}
	var out []string
	for i := 0; i < len(tokens); i++ {
		if strings.HasPrefix(tokens[i], "-") && i+1 < len(tokens) && !strings.HasPrefix(tokens[i+1], "-") {
			out = append(out, tokens[i]+" "+tokens[i+1])
			i++
		} else {
			out = append(out, tokens[i])
		}
	}
	return out
}

// flattenArgLines expands panel rows (each may be one token or "--flag value") to argv tokens.
func flattenArgLines(lines []string) []string {
	var out []string
	for _, line := range lines {
		out = append(out, expandArgLine(line)...)
	}
	return out
}

// normalizeModelParams trims keys and args for storage.
func normalizeModelParams(p ModelParams) ModelParams {
	var env []EnvVar
	for _, e := range p.Env {
		k := strings.TrimSpace(e.Key)
		if k == "" {
			continue
		}
		env = append(env, EnvVar{Key: k, Value: e.Value})
	}
	var args []string
	for _, a := range p.Args {
		a = strings.TrimSpace(a)
		if a == "" {
			continue
		}
		args = append(args, expandArgLine(a)...)
	}
	return ModelParams{Env: env, Args: args}
}

// expandArgLine maps one row from the parameter panel to argv tokens. A line
// that starts with '-' and contains a space is split on the first space only,
// so "--max-model-len 4096" becomes two tokens and "-m /path/with spaces" keeps
// the path as one value. Otherwise the line is a single token.
func expandArgLine(line string) []string {
	line = strings.TrimSpace(line)
	if line == "" {
		return nil
	}
	if !strings.HasPrefix(line, "-") || !strings.Contains(line, " ") {
		return []string{line}
	}
	i := strings.IndexByte(line, ' ')
	return []string{line[:i], strings.TrimSpace(line[i+1:])}
}
