package config

import (
	"errors"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"

	"github.com/flyingnobita/llml/internal/fsutil"
	"github.com/flyingnobita/llml/internal/models"
	"github.com/flyingnobita/llml/internal/userdata"
)

// RuntimeStatesSchemaVersion is the current on-disk format for runtimes.toml.
const RuntimeStatesSchemaVersion = 1

// RuntimeStates is the runtime state store: whether each Runtime llml has seen
// is on. A Runtime with no entry has not been seen yet.
//
// It lives in its own file rather than config.toml because llml records it on
// its own, and config.toml is rewritten only when the user saves (see the ADR
// "Disabled runtimes keep their models and live in their own state file").
// Unlike the discovery cache it is not disposable, so it is backed up before
// every overwrite.
//
// A RuntimeStates is a value: [RuntimeStates.With] returns a changed copy and
// never modifies the receiver, so copies held by different Model values stay
// independent.
type RuntimeStates struct {
	// on is keyed by the backend's profile name ([models.ModelBackend.String]),
	// so an entry this version does not know survives a read and write.
	on map[string]bool
}

// runtimeStatesDoc is the on-disk shape of runtimes.toml.
type runtimeStatesDoc struct {
	SchemaVersion int             `toml:"schema_version"`
	Runtimes      map[string]bool `toml:"runtimes"`
}

// Lookup reports whether b is on, and whether llml has seen it at all. When
// seen is false, on is meaningless.
func (s RuntimeStates) Lookup(b models.ModelBackend) (on, seen bool) {
	on, seen = s.on[b.String()]
	return on, seen
}

// Enabled reports whether b is on. A Runtime not seen yet counts as on.
func (s RuntimeStates) Enabled(b models.ModelBackend) bool {
	on, seen := s.Lookup(b)
	return on || !seen
}

// With returns a copy of s that records b as on or off.
func (s RuntimeStates) With(b models.ModelBackend, on bool) RuntimeStates {
	next := make(map[string]bool, len(s.on)+1)
	maps.Copy(next, s.on)
	next[b.String()] = on
	return RuntimeStates{on: next}
}

// Disabled returns the Runtimes recorded as off: the Disabled Runtimes, which
// runtime detection skips. It never includes a Runtime not seen yet.
func (s RuntimeStates) Disabled() models.BackendSet {
	out := models.BackendSet{}
	for name, on := range s.on {
		if on {
			continue
		}
		if b, err := models.ParseBackend(name); err == nil && name != "" {
			out[b] = struct{}{}
		}
	}
	return out
}

// RuntimeStatesPath returns {UserConfigDir}/llml/runtimes.toml.
func RuntimeStatesPath() (string, error) {
	return userdata.RuntimeStatesPath()
}

// ReadRuntimeStates reads runtimes.toml. A missing file is not an error: it
// means no Runtime has been seen yet.
//
//nolint:gosec // G304: path from RuntimeStatesPath() using os.UserConfigDir — trusted source.
func ReadRuntimeStates() (RuntimeStates, error) {
	path, err := RuntimeStatesPath()
	if err != nil {
		return RuntimeStates{}, err
	}
	b, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return RuntimeStates{}, nil
	}
	if err != nil {
		return RuntimeStates{}, err
	}
	var doc runtimeStatesDoc
	if _, err := toml.Decode(string(b), &doc); err != nil {
		return RuntimeStates{}, err
	}
	return RuntimeStates{on: doc.Runtimes}, nil
}

// WriteRuntimeStates writes runtimes.toml atomically, first copying the
// previous file into backups/ (pruned like config.toml and model-params.json).
func WriteRuntimeStates(s RuntimeStates) error {
	path, err := RuntimeStatesPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	// Best effort, like config.toml: a failed snapshot must not stop the user
	// from saving their choices.
	_ = userdata.BackupFileIfExists(path)
	doc := runtimeStatesDoc{SchemaVersion: RuntimeStatesSchemaVersion, Runtimes: s.on}
	if doc.Runtimes == nil {
		doc.Runtimes = map[string]bool{}
	}
	var buf strings.Builder
	if err := toml.NewEncoder(&buf).Encode(doc); err != nil {
		return err
	}
	return fsutil.WriteFileAtomic(path, []byte(buf.String()), 0o600)
}
