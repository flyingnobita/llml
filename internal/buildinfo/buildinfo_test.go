package buildinfo

import (
	"runtime/debug"
	"testing"
)

func vcs(rev, when, modified string) []debug.BuildSetting {
	var s []debug.BuildSetting
	if rev != "" {
		s = append(s, debug.BuildSetting{Key: "vcs.revision", Value: rev})
	}
	if when != "" {
		s = append(s, debug.BuildSetting{Key: "vcs.time", Value: when})
	}
	if modified != "" {
		s = append(s, debug.BuildSetting{Key: "vcs.modified", Value: modified})
	}
	return s
}

func TestIdentity(t *testing.T) {
	t.Parallel()
	const rev = "16728e6aca52a748eaabf9b78569d22151460ad9"
	const when = "2026-09-28T10:28:07Z"
	tests := []struct {
		name    string
		stamped string
		bi      *debug.BuildInfo
		want    string
	}{
		{
			name:    "goreleaser release is the bare version",
			stamped: "0.7.1",
			bi:      &debug.BuildInfo{Main: debug.Module{Version: "v0.7.1"}, Settings: vcs(rev, when, "false")},
			want:    "0.7.1",
		},
		{
			name:    "plain go build is dev with commit and date",
			stamped: "dev",
			bi:      &debug.BuildInfo{Main: debug.Module{Version: "v0.7.2-0.20260928102807-16728e6aca52"}, Settings: vcs(rev, when, "false")},
			want:    "dev (16728e6, 2026-09-28)",
		},
		{
			name:    "dirty go build is marked modified",
			stamped: "dev",
			bi:      &debug.BuildInfo{Main: debug.Module{Version: "v0.7.1+dirty"}, Settings: vcs("1c3ca6532919709408a3045b99b8079f5789b93e", "2026-06-05T13:52:26Z", "true")},
			want:    "dev (1c3ca65, 2026-06-05, modified)",
		},
		{
			name:    "stamped build from a dirty tree keeps the details",
			stamped: "0.7.2-SNAPSHOT-16728e6",
			bi:      &debug.BuildInfo{Settings: vcs(rev, when, "true")},
			want:    "0.7.2-SNAPSHOT-16728e6 (16728e6, 2026-09-28, modified)",
		},
		{
			name:    "go install at a tag uses the module version",
			stamped: "dev",
			bi:      &debug.BuildInfo{Main: debug.Module{Version: "v0.7.1"}},
			want:    "v0.7.1",
		},
		{
			name:    "devel module without vcs info is plain dev",
			stamped: "dev",
			bi:      &debug.BuildInfo{Main: debug.Module{Version: "(devel)"}},
			want:    "dev",
		},
		{
			name:    "no build info at all",
			stamped: "dev",
			bi:      nil,
			want:    "dev",
		},
		{
			name:    "empty stamp counts as dev",
			stamped: " ",
			bi:      &debug.BuildInfo{Settings: vcs(rev, "", "")},
			want:    "dev (16728e6)",
		},
		{
			name:    "unparseable time is left out",
			stamped: "dev",
			bi:      &debug.BuildInfo{Settings: vcs(rev, "yesterday", "false")},
			want:    "dev (16728e6)",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := identity(tt.stamped, tt.bi); got != tt.want {
				t.Errorf("identity(%q) = %q, want %q", tt.stamped, got, tt.want)
			}
		})
	}
}

func TestIdentityReadsRunningBinary(t *testing.T) {
	t.Parallel()
	// Reading the test binary's real build info must not panic or come back empty.
	if got := Identity("dev"); got == "" {
		t.Fatal("Identity returned an empty string")
	}
}
