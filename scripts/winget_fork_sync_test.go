package scripts

import (
	"errors"
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// These tests run the step scripts of .github/workflows/winget-fork-sync.yml
// under bash, the way Actions does, with a fake gh on PATH. The steps take all
// their inputs from env, so no ${{ }} expression needs expanding here.

const (
	syncStep  = "Sync fork with microsoft/winget-pkgs"
	trackStep = "Open, update, or close the tracking issue"
)

// gh's own message when the token cannot merge upstream workflow changes.
const workflowScopeMessage = "failed to sync: Upstream commits contain workflow changes, which require the `workflow` scope or permission to merge. To request it, run: gh auth refresh -s workflow"

// syncEnv is the sync step's env on a scheduled run with a token, with
// overrides applied.
func syncEnv(overrides map[string]string) map[string]string {
	env := map[string]string{
		"GH_TOKEN":          "token",
		"FORK":              "flyingnobita/winget-pkgs",
		"GITHUB_EVENT_NAME": "schedule",
	}
	maps.Copy(env, overrides)
	return env
}

type stepRun struct {
	out     string
	exit    int
	calls   []string
	outputs string
}

func TestWingetSyncMissingWorkflowScopeNamesTheFix(t *testing.T) {
	run := runWingetStep(t, syncStep, syncEnv(map[string]string{
		"FAKE_GH_SYNC_OUTPUT": workflowScopeMessage,
		"FAKE_GH_SYNC_STATUS": "1",
	}))

	if run.exit == 0 {
		t.Fatalf("sync should fail, got success: %s", run.out)
	}
	annotation := errorAnnotation(run.out)
	if annotation == "" {
		t.Fatalf("expected an ::error:: annotation, got: %s", run.out)
	}
	for _, want := range []string{"WINGET_GITHUB_TOKEN", "workflow", "https://github.com/settings/tokens"} {
		if !strings.Contains(annotation, want) {
			t.Errorf("annotation should mention %q: %s", want, annotation)
		}
	}
}

func TestWingetSyncOtherFailureKeepsGhMessageAndStatus(t *testing.T) {
	const ghMessage = "failed to sync: HTTP 502: Bad Gateway"
	run := runWingetStep(t, syncStep, syncEnv(map[string]string{
		"FAKE_GH_SYNC_OUTPUT": ghMessage,
		"FAKE_GH_SYNC_STATUS": "4",
	}))

	if run.exit != 4 {
		t.Fatalf("sync should exit with gh's status 4, got %d: %s", run.exit, run.out)
	}
	if !strings.Contains(run.out, ghMessage) {
		t.Errorf("gh's message should be in the log: %s", run.out)
	}
	if strings.Contains(run.out, "workflow scope") {
		t.Errorf("an unrelated failure must not blame the workflow scope: %s", run.out)
	}
}

// With no token a scheduled run skips cleanly, and says so, so the tracking
// step does not take the skip for a green sync.
func TestWingetSyncScheduledRunSkipsWithoutToken(t *testing.T) {
	run := runWingetStep(t, syncStep, syncEnv(map[string]string{
		"GH_TOKEN": "",
	}))

	if run.exit != 0 {
		t.Fatalf("scheduled run without a token should succeed, got %d: %s", run.exit, run.out)
	}
	if !strings.Contains(run.outputs, "skipped=true") {
		t.Errorf("step outputs should record the skip, got %q", run.outputs)
	}
	if len(run.calls) != 0 {
		t.Errorf("a skipped sync should not call gh, got %q", run.calls)
	}
}

// A manual run can fail on purpose, so the tracking issue can be tested
// without breaking the token, and it fails before touching the fork.
func TestWingetSyncForcedFailureFailsWithoutSyncing(t *testing.T) {
	run := runWingetStep(t, syncStep, syncEnv(map[string]string{
		"FORCE_FAILURE": "true",
	}))

	if run.exit == 0 {
		t.Fatalf("a forced failure should fail the step: %s", run.out)
	}
	if !strings.Contains(errorAnnotation(run.out), "force_failure") {
		t.Errorf("the annotation should say the failure was forced: %s", run.out)
	}
	if len(run.calls) != 0 {
		t.Errorf("a forced failure should not call gh, got %q", run.calls)
	}
}

const runURL = "https://github.com/flyingnobita/llml/actions/runs/123"

func trackEnv(result, skipped, openIssue string) map[string]string {
	return map[string]string{
		"GH_TOKEN":           "github-token",
		"GH_REPO":            "flyingnobita/llml",
		"SYNC_RESULT":        result,
		"SYNC_SKIPPED":       skipped,
		"RUN_URL":            runURL,
		"FAKE_GH_OPEN_ISSUE": openIssue,
	}
}

// callsTo returns the recorded gh calls that start with prefix.
func callsTo(calls []string, prefix string) []string {
	var matched []string
	for _, c := range calls {
		if strings.HasPrefix(c, prefix) {
			matched = append(matched, c)
		}
	}
	return matched
}

func TestWingetTrackFirstFailureOpensIssueForAHuman(t *testing.T) {
	run := runWingetStep(t, trackStep, trackEnv("failure", "", ""))

	if run.exit != 0 {
		t.Fatalf("tracking step failed: %s", run.out)
	}
	created := callsTo(run.calls, "issue create")
	if len(created) != 1 {
		t.Fatalf("expected one issue create, got calls %q", run.calls)
	}
	for _, want := range []string{runURL, "--label ready-for-human"} {
		if !strings.Contains(created[0], want) {
			t.Errorf("issue create should include %q: %s", want, created[0])
		}
	}
}

func TestWingetTrackLaterFailureCommentsOnOpenIssue(t *testing.T) {
	run := runWingetStep(t, trackStep, trackEnv("failure", "", "42"))

	if run.exit != 0 {
		t.Fatalf("tracking step failed: %s", run.out)
	}
	if created := callsTo(run.calls, "issue create"); len(created) != 0 {
		t.Fatalf("an open issue must not be duplicated, got %q", created)
	}
	commented := callsTo(run.calls, "issue comment 42")
	if len(commented) != 1 || !strings.Contains(commented[0], runURL) {
		t.Fatalf("expected one comment on #42 linking the run, got calls %q", run.calls)
	}
}

func TestWingetTrackSuccessClosesOpenIssue(t *testing.T) {
	run := runWingetStep(t, trackStep, trackEnv("success", "", "42"))

	if run.exit != 0 {
		t.Fatalf("tracking step failed: %s", run.out)
	}
	closed := callsTo(run.calls, "issue close 42")
	if len(closed) != 1 || !strings.Contains(closed[0], runURL) {
		t.Fatalf("expected #42 closed with a comment linking the run, got calls %q", run.calls)
	}
}

// A skipped sync (no token) proves nothing, and a cancelled one never
// finished, so neither touches the tracking issue.
func TestWingetTrackSkippedOrCancelledSyncLeavesIssueAlone(t *testing.T) {
	for name, env := range map[string]map[string]string{
		"skipped":   trackEnv("success", "true", "42"),
		"cancelled": trackEnv("cancelled", "", "42"),
	} {
		t.Run(name, func(t *testing.T) {
			run := runWingetStep(t, trackStep, env)
			if run.exit != 0 {
				t.Fatalf("tracking step failed: %s", run.out)
			}
			for _, c := range run.calls {
				if strings.HasPrefix(c, "issue ") && !strings.HasPrefix(c, "issue list") {
					t.Errorf("expected no issue change, got %q", c)
				}
			}
		})
	}
}

// errorAnnotation returns the last ::error:: line in out, or "".
func errorAnnotation(out string) string {
	last := ""
	for line := range strings.SplitSeq(out, "\n") {
		if strings.HasPrefix(line, "::error::") {
			last = line
		}
	}
	return last
}

// runWingetStep runs the named step's script with env and a fake gh, and
// returns what it printed, its exit status, and the gh calls it made.
func runWingetStep(t *testing.T, step string, env map[string]string) stepRun {
	t.Helper()
	dir := t.TempDir()
	script := filepath.Join(dir, "step.sh")
	if err := os.WriteFile(script, []byte(stepScript(t, step)), 0o600); err != nil {
		t.Fatal(err)
	}
	bin := filepath.Join(dir, "bin")
	if err := os.MkdirAll(bin, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(bin, "gh"), []byte(fakeGH), 0o755); err != nil {
		t.Fatal(err)
	}
	logPath := filepath.Join(dir, "gh.log")
	outputPath := filepath.Join(dir, "github_output")

	// Actions runs `run:` blocks as `bash --noprofile --norc -eo pipefail {0}`.
	cmd := exec.CommandContext(t.Context(), "bash", "--noprofile", "--norc", "-eo", "pipefail", script)
	cmd.Env = []string{
		"PATH=" + bin + string(os.PathListSeparator) + os.Getenv("PATH"),
		"HOME=" + dir,
		"FAKE_GH_LOG=" + logPath,
		"GITHUB_OUTPUT=" + outputPath,
	}
	for k, v := range env {
		cmd.Env = append(cmd.Env, k+"="+v)
	}
	outBytes, err := cmd.CombinedOutput()
	run := stepRun{out: string(outBytes)}
	var exitErr *exec.ExitError
	switch {
	case errors.As(err, &exitErr):
		run.exit = exitErr.ExitCode()
	case err != nil:
		t.Fatalf("run step %q: %v", step, err)
	}
	if log, err := os.ReadFile(logPath); err == nil {
		run.calls = strings.Split(strings.TrimSpace(string(log)), "\n")
	}
	if outputs, err := os.ReadFile(outputPath); err == nil {
		run.outputs = string(outputs)
	}
	return run
}

// fakeGH records each call and answers the few commands the workflow uses.
const fakeGH = `#!/usr/bin/env bash
printf '%s\n' "$*" >>"${FAKE_GH_LOG}"
case "$1 $2" in
"repo sync")
	printf '%s\n' "${FAKE_GH_SYNC_OUTPUT:-}" >&2
	exit "${FAKE_GH_SYNC_STATUS:-0}"
	;;
"issue list") printf '%s' "${FAKE_GH_OPEN_ISSUE:-}" ;;
esac
exit 0
`

// stepScript returns the `run: |` block of the workflow step named step.
func stepScript(t *testing.T, step string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(repoRoot(t), ".github", "workflows", "winget-fork-sync.yml"))
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(string(data), "\n")
	start := -1
	for i, line := range lines {
		if strings.TrimSpace(line) == "- name: "+step {
			start = i
			break
		}
	}
	if start < 0 {
		t.Fatalf("workflow has no step named %q", step)
	}
	for i := start + 1; i < len(lines); i++ {
		trimmed := strings.TrimSpace(lines[i])
		if strings.HasPrefix(trimmed, "- name: ") {
			break
		}
		if trimmed != "run: |" {
			continue
		}
		keyIndent := indentOf(lines[i])
		var body []string
		for _, line := range lines[i+1:] {
			if strings.TrimSpace(line) != "" && indentOf(line) <= keyIndent {
				break
			}
			body = append(body, line)
		}
		script := strings.Join(body, "\n") + "\n"
		if strings.Contains(script, "${{") {
			t.Fatalf("step %q expands ${{ }} inside run:; pass the value through env instead", step)
		}
		return script
	}
	t.Fatalf("step %q has no `run: |` block", step)
	return ""
}

func indentOf(line string) int {
	return len(line) - len(strings.TrimLeft(line, " "))
}
