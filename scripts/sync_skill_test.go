package scripts

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestCanonicalSkillStatesPortableRule(t *testing.T) {
	repoRoot := repoRoot(t)
	skillPath := filepath.Join(repoRoot, ".agents", "skills", "llml-import", "SKILL.md")
	docPath := filepath.Join(repoRoot, "docs", "profile-format.md")

	skill, err := os.ReadFile(skillPath)
	if err != nil {
		t.Fatalf("read skill: %v", err)
	}
	doc, err := os.ReadFile(docPath)
	if err != nil {
		t.Fatalf("read doc: %v", err)
	}

	skillText := string(skill)
	docText := string(doc)

	if !strings.Contains(docText, "Do not include the model path or the backend binary name; llml supplies those.") {
		t.Fatal("portable format doc lost the no-model-path rule")
	}
	if !strings.Contains(docText, "schema_version = 2") {
		t.Fatal("portable format doc did not move to schema version 2")
	}
	if !strings.Contains(docText, "[profiles.use_case]") || !strings.Contains(docText, "[profiles.hardware]") {
		t.Fatal("portable format doc lost structured metadata tables")
	}
	if !strings.Contains(skillText, "Do not extract model-location parameters into the portable profile.") {
		t.Fatal("skill does not mirror the portable no-model-location rule")
	}
	if !strings.Contains(skillText, "Import-time stripping remains a defensive backstop.") {
		t.Fatal("skill lost the import-time defensive backstop note")
	}
	if !strings.Contains(skillText, "Do not include model-location parameters in `args` or `env`.") {
		t.Fatal("fallback summary is missing the degraded-mode portability rule")
	}
	if !strings.Contains(skillText, "data = {'version': 3, 'models': {}}") {
		t.Fatal("skill still writes legacy local model-params version")
	}
	if !strings.Contains(skillText, "Portable `use_case` and `hardware` fields map into llml's local") {
		t.Fatal("fallback summary lost metadata mapping note")
	}
}

func TestCanonicalAndClaudeWorkspaceCopiesStayInSync(t *testing.T) {
	repoRoot := repoRoot(t)
	canonicalPath := filepath.Join(repoRoot, ".agents", "skills", "llml-import", "SKILL.md")
	claudePath := filepath.Join(repoRoot, ".claude", "skills", "llml-import", "SKILL.md")

	canonical, err := os.ReadFile(canonicalPath)
	if err != nil {
		t.Fatalf("read canonical skill: %v", err)
	}
	claude, err := os.ReadFile(claudePath)
	if err != nil {
		t.Fatalf("read claude skill: %v", err)
	}
	if string(canonical) != string(claude) {
		t.Fatal("canonical .agents skill and tracked .claude compatibility copy drifted")
	}
	if _, err := os.Stat(filepath.Join(repoRoot, ".claude", "skills", "llml-import", ".skill-sync-meta")); !os.IsNotExist(err) {
		t.Fatalf("expected no .skill-sync-meta for tracked claude workspace copy, got err=%v", err)
	}
}

func TestSyncSkillWorkspaceRequiresExplicitToolSelection(t *testing.T) {
	repo := testRepo(t)

	out := runSync(t, repo, nil, "--workspace")
	if !strings.Contains(out, "skill is already available at Workspace/repo scope") {
		t.Fatalf("expected workspace guidance, got: %s", out)
	}
	if _, err := os.Stat(filepath.Join(repo, ".claude", "skills", "llml-import")); !os.IsNotExist(err) {
		t.Fatalf("expected no implicit claude workspace install, got err=%v", err)
	}
}

func TestSyncSkillWorkspaceInstallStatusAndUninstall(t *testing.T) {
	repo := testRepo(t)
	runSync(t, repo, nil, "--workspace", "--tool", "claude")

	status := runSync(t, repo, nil, "--workspace", "--tool", "claude", "--status")
	if !strings.Contains(status, "workspace claude: installed (tracked compatibility copy)") {
		t.Fatalf("status missing claude install: %s", status)
	}

	runSync(t, repo, nil, "--workspace", "--tool", "claude", "--uninstall")
	path := filepath.Join(repo, ".claude", "skills", "llml-import")
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("expected removed path %s, got err=%v", path, err)
	}
}

func TestSyncSkillUserInstallDetectedToolAndProtectsUnmanagedTargets(t *testing.T) {
	repo := testRepo(t)
	home := t.TempDir()
	if err := os.MkdirAll(filepath.Join(home, ".codex"), 0o755); err != nil {
		t.Fatalf("mkdir codex home: %v", err)
	}

	out := runSync(t, repo, []string{"HOME=" + home}, "--user")
	if !strings.Contains(out, "synced user codex") {
		t.Fatalf("expected codex user sync, got: %s", out)
	}

	userSkill := filepath.Join(home, ".codex", "skills", "llml-import", "SKILL.md")
	if _, err := os.Stat(userSkill); err != nil {
		t.Fatalf("expected user skill install: %v", err)
	}

	unmanagedRepo := testRepo(t)
	unmanagedHome := t.TempDir()
	targetDir := filepath.Join(unmanagedRepo, ".claude", "skills", "llml-import")
	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		t.Fatalf("mkdir unmanaged target: %v", err)
	}
	if err := os.WriteFile(filepath.Join(targetDir, "SKILL.md"), []byte("manual"), 0o644); err != nil {
		t.Fatalf("write unmanaged skill: %v", err)
	}

	cmd := syncCmd(t, unmanagedRepo, []string{"HOME=" + unmanagedHome}, "--workspace", "--tool", "claude")
	outBytes, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("expected unmanaged overwrite refusal, got success: %s", outBytes)
	}
	if !strings.Contains(string(outBytes), "refusing to overwrite unmanaged target") {
		t.Fatalf("unexpected unmanaged overwrite error: %s", outBytes)
	}
}

// The tracked Claude copy is refreshed without --force once the canonical
// skill changes, since git holds the old version. An uncommitted edit to it
// is not in git, so it is still protected.
func TestSyncSkillWorkspaceRefreshesStaleTrackedClaudeCopy(t *testing.T) {
	repo := testRepo(t)
	runSync(t, repo, nil, "--workspace", "--tool", "claude")
	gitInit(t, repo)

	canonical := filepath.Join(repo, ".agents", "skills", "llml-import", "SKILL.md")
	tracked := filepath.Join(repo, ".claude", "skills", "llml-import", "SKILL.md")
	appendTo(t, canonical, "\nA canonical change.\n")

	status := runSync(t, repo, nil, "--workspace", "--tool", "claude", "--status")
	if !strings.Contains(status, "workspace claude: out of date (tracked compatibility copy)") {
		t.Fatalf("status should report the stale tracked copy: %s", status)
	}

	runSync(t, repo, nil, "--workspace", "--tool", "claude")
	want, _ := os.ReadFile(canonical)
	got, _ := os.ReadFile(tracked)
	if string(got) != string(want) {
		t.Fatal("tracked copy was not refreshed from the canonical skill")
	}
	if _, err := os.Stat(filepath.Join(filepath.Dir(tracked), ".skill-sync-meta")); !os.IsNotExist(err) {
		t.Fatalf("tracked copy should have no .skill-sync-meta, got err=%v", err)
	}

	gitRun(t, repo, "commit", "-qam", "refresh")
	appendTo(t, tracked, "\nA local edit.\n")
	appendTo(t, canonical, "\nAnother canonical change.\n")
	cmd := syncCmd(t, repo, nil, "--workspace", "--tool", "claude")
	out, err := cmd.CombinedOutput()
	if err == nil || !strings.Contains(string(out), "refusing to overwrite unmanaged target") {
		t.Fatalf("an uncommitted edit to the tracked copy should be refused, err=%v: %s", err, out)
	}
}

// gitInit makes repo a git repository with everything in it committed.
func gitInit(t *testing.T, repo string) {
	t.Helper()
	gitRun(t, repo, "init", "-q")
	gitRun(t, repo, "add", "-A")
	gitRun(t, repo, "commit", "-qm", "init")
}

// gitRun runs git in repo, isolated from the user's and system git config.
func gitRun(t *testing.T, repo string, args ...string) {
	t.Helper()
	base := []string{"-c", "user.name=test", "-c", "user.email=test@example.com", "-c", "commit.gpgsign=false"}
	cmd := exec.Command("git", append(base, args...)...)
	cmd.Dir = repo
	cmd.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL="+os.DevNull, "GIT_CONFIG_NOSYSTEM=1")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

func appendTo(t *testing.T, path, text string) {
	t.Helper()
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close() }()
	if _, err := f.WriteString(text); err != nil {
		t.Fatal(err)
	}
}

func testRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	for _, rel := range []string{
		".agents/skills/llml-import",
		"scripts",
		"docs",
	} {
		if err := os.MkdirAll(filepath.Join(dir, rel), 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", rel, err)
		}
	}

	repoRoot := repoRoot(t)
	copyFile(t, filepath.Join(repoRoot, "scripts", "sync-skill"), filepath.Join(dir, "scripts", "sync-skill"))
	copyFile(t, filepath.Join(repoRoot, "docs", "profile-format.md"), filepath.Join(dir, "docs", "profile-format.md"))
	copyFile(t, filepath.Join(repoRoot, ".agents", "skills", "llml-import", "SKILL.md"), filepath.Join(dir, ".agents", "skills", "llml-import", "SKILL.md"))
	if err := os.MkdirAll(filepath.Join(dir, ".claude", "skills"), 0o755); err != nil {
		t.Fatalf("mkdir .claude/skills: %v", err)
	}

	return dir
}

func repoRoot(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	return filepath.Dir(wd)
}

func copyFile(t *testing.T, src, dst string) {
	t.Helper()
	data, err := os.ReadFile(src)
	if err != nil {
		t.Fatalf("read %s: %v", src, err)
	}
	if err := os.WriteFile(dst, data, 0o755); err != nil {
		t.Fatalf("write %s: %v", dst, err)
	}
}

func runSync(t *testing.T, repo string, extraEnv []string, args ...string) string {
	t.Helper()
	cmd := syncCmd(t, repo, extraEnv, args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("sync-skill %v failed: %v\n%s", args, err, out)
	}
	return string(out)
}

func syncCmd(t *testing.T, repo string, extraEnv []string, args ...string) *exec.Cmd {
	t.Helper()
	cmd := exec.Command(filepath.Join(repo, "scripts", "sync-skill"), args...)
	cmd.Dir = repo
	cmd.Env = append(os.Environ(), extraEnv...)
	return cmd
}
