// Package buildinfo names the running llml build, so a stale hand-built binary
// can be told apart from a fresh one or a release.
package buildinfo

import (
	"runtime/debug"
	"strings"
	"time"
)

// devVersion is main.version when the build was not stamped with -ldflags.
const devVersion = "dev"

// shortCommitLen matches git's default abbreviated hash length.
const shortCommitLen = 7

// Identity returns what llml reports as its version. stamped is main.version,
// set by GoReleaser's -ldflags and "dev" otherwise.
//
// A release (stamped, built from a clean tree) is its bare version, "0.7.1".
// Any other build appends what the Go toolchain recorded about its source:
// "dev (16728e6, 2026-09-28, modified)". The date is the commit's, not the
// build's, because what matters is how old the code is.
func Identity(stamped string) string {
	bi, _ := debug.ReadBuildInfo() // nil when the binary carries no build info
	return identity(stamped, bi)
}

// identity is [Identity] with the build info passed in; bi may be nil.
func identity(stamped string, bi *debug.BuildInfo) string {
	stamped = strings.TrimSpace(stamped)
	if stamped == "" {
		stamped = devVersion
	}
	src := readSource(bi)
	if stamped != devVersion && !src.modified {
		return stamped
	}
	label := stamped
	// `go install …@v0.7.1` records the module version but no commit. A plain
	// `go build` records a commit and a pseudo-version, which the commit and
	// date already say more plainly, so it keeps the "dev" label.
	if label == devVersion && src.revision == "" && bi != nil {
		if v := bi.Main.Version; v != "" && v != "(devel)" {
			label = v
		}
	}
	var parts []string
	if src.revision != "" {
		parts = append(parts, shortCommit(src.revision))
	}
	if !src.time.IsZero() {
		parts = append(parts, src.time.UTC().Format(time.DateOnly))
	}
	if src.modified {
		parts = append(parts, "modified")
	}
	if len(parts) == 0 {
		return label
	}
	return label + " (" + strings.Join(parts, ", ") + ")"
}

// source is the version-control state the Go toolchain stamps into a binary
// built inside a repository.
type source struct {
	revision string
	time     time.Time
	modified bool
}

func readSource(bi *debug.BuildInfo) source {
	var s source
	if bi == nil {
		return s
	}
	for _, kv := range bi.Settings {
		switch kv.Key {
		case "vcs.revision":
			s.revision = kv.Value
		case "vcs.time":
			if t, err := time.Parse(time.RFC3339, kv.Value); err == nil {
				s.time = t
			}
		case "vcs.modified":
			s.modified = kv.Value == "true"
		}
	}
	return s
}

func shortCommit(rev string) string {
	if len(rev) > shortCommitLen {
		return rev[:shortCommitLen]
	}
	return rev
}
