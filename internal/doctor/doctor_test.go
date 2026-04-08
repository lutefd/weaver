package doctor

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lutefd/weaver/internal/config"
	"github.com/lutefd/weaver/internal/deps"
	gitrunner "github.com/lutefd/weaver/internal/git"
	"github.com/lutefd/weaver/internal/group"
	weaverintegration "github.com/lutefd/weaver/internal/integration"
	"github.com/lutefd/weaver/internal/rebaser"
)

func TestCheckerHealthyRepo(t *testing.T) {
	t.Parallel()

	repo := initRepo(t)
	writeRepoFile(t, repo, "README.md", "base\n")
	runGit(t, repo, "add", "README.md")
	runGit(t, repo, "commit", "-m", "init")

	runGit(t, repo, "checkout", "-b", "feature-a")
	writeRepoFile(t, repo, "feature-a.txt", "feature-a\n")
	runGit(t, repo, "add", "feature-a.txt")
	runGit(t, repo, "commit", "-m", "feature-a")

	runGit(t, repo, "checkout", "-b", "feature-b")
	writeRepoFile(t, repo, "feature-b.txt", "feature-b\n")
	runGit(t, repo, "add", "feature-b.txt")
	runGit(t, repo, "commit", "-m", "feature-b")
	runGit(t, repo, "checkout", "main")

	if _, err := config.Initialize(repo); err != nil {
		t.Fatalf("Initialize() error = %v", err)
	}
	if err := deps.NewLocalSource(repo).Set(context.Background(), "feature-b", "feature-a"); err != nil {
		t.Fatalf("Set() error = %v", err)
	}
	if err := group.NewStore(repo).Create("sprint-42", []string{"feature-a", "feature-b"}); err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if err := weaverintegration.NewStore(repo).Save("integration", weaverintegration.Recipe{
		Base:     "main",
		Branches: []string{"feature-a", "feature-b"},
	}); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	cfg := config.Default()
	report, err := New(gitrunner.NewCLIRunner(repo, nil), &cfg, nil).Run(context.Background())
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if report.HasFailures() {
		t.Fatalf("Run() reported failures: %#v", report.Checks)
	}
	if report.Summary.Warn != 0 {
		t.Fatalf("Run() warnings = %d, want none", report.Summary.Warn)
	}
}

func TestCheckerReportsMissingBranchAndDirtyTree(t *testing.T) {
	t.Parallel()

	repo := initRepo(t)
	writeRepoFile(t, repo, "README.md", "base\n")
	runGit(t, repo, "add", "README.md")
	runGit(t, repo, "commit", "-m", "init")

	if _, err := config.Initialize(repo); err != nil {
		t.Fatalf("Initialize() error = %v", err)
	}
	if err := deps.NewLocalSource(repo).Set(context.Background(), "feature-b", "feature-a"); err != nil {
		t.Fatalf("Set() error = %v", err)
	}
	if err := group.NewStore(repo).Create("sprint-42", []string{"feature-b"}); err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if err := weaverintegration.NewStore(repo).Save("integration", weaverintegration.Recipe{
		Base:     "main",
		Branches: []string{"feature-b"},
	}); err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	writeRepoFile(t, repo, "README.md", "dirty\n")

	cfg := config.Default()
	report, err := New(gitrunner.NewCLIRunner(repo, nil), &cfg, nil).Run(context.Background())
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if !report.HasFailures() {
		t.Fatal("Run() should report failures")
	}
	if report.Summary.Warn == 0 {
		t.Fatal("Run() should report warnings")
	}
	if !containsMessage(report, "does not exist locally") {
		t.Fatalf("Run() messages = %#v, want missing branch failure", report.Checks)
	}
	if !containsMessage(report, "working tree has") {
		t.Fatalf("Run() messages = %#v, want dirty working tree warning", report.Checks)
	}
}

func TestCheckerReportsConfigErrorAndBrokenRebaseState(t *testing.T) {
	t.Parallel()

	repo := initRepo(t)
	writeRepoFile(t, repo, "README.md", "base\n")
	runGit(t, repo, "add", "README.md")
	runGit(t, repo, "commit", "-m", "init")

	if _, err := config.Initialize(repo); err != nil {
		t.Fatalf("Initialize() error = %v", err)
	}

	store := rebaser.NewStateStore(repo)
	if err := store.Save(&rebaser.State{}); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	cfg := config.Default()
	report, err := New(gitrunner.NewCLIRunner(repo, nil), &cfg, config.Error{Message: "decode config: broken yaml"}).Run(context.Background())
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if !report.HasFailures() {
		t.Fatal("Run() should report failures")
	}
	if !containsMessage(report, "weaver config is invalid") {
		t.Fatalf("Run() messages = %#v, want config failure", report.Checks)
	}
	if !containsMessage(report, "pending stack sync state is missing") {
		t.Fatalf("Run() messages = %#v, want rebase-state failure", report.Checks)
	}
}

func TestCheckerReportsUpstreamDrift(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	remote := filepath.Join(root, "origin.git")
	runGit(t, root, "init", "--bare", "-b", "main", remote)

	seed := filepath.Join(root, "seed")
	runGit(t, root, "clone", remote, seed)
	runGit(t, seed, "config", "user.name", "Doctor Test")
	runGit(t, seed, "config", "user.email", "doctor@example.com")
	writeRepoFile(t, seed, "README.md", "base\n")
	runGit(t, seed, "add", "README.md")
	runGit(t, seed, "commit", "-m", "init")
	runGit(t, seed, "push", "-u", "origin", "main")
	runGit(t, seed, "checkout", "-b", "feature-a")
	writeRepoFile(t, seed, "feature-a.txt", "v1\n")
	runGit(t, seed, "add", "feature-a.txt")
	runGit(t, seed, "commit", "-m", "feature-a v1")
	runGit(t, seed, "push", "-u", "origin", "feature-a")

	repo := filepath.Join(root, "repo")
	runGit(t, root, "clone", remote, repo)
	runGit(t, repo, "config", "user.name", "Doctor Test")
	runGit(t, repo, "config", "user.email", "doctor@example.com")
	runGit(t, repo, "checkout", "-b", "feature-a", "--track", "origin/feature-a")
	runGit(t, repo, "checkout", "main")

	upstream := filepath.Join(root, "upstream")
	runGit(t, root, "clone", remote, upstream)
	runGit(t, upstream, "config", "user.name", "Doctor Test")
	runGit(t, upstream, "config", "user.email", "doctor@example.com")

	runGit(t, upstream, "checkout", "main")
	writeRepoFile(t, upstream, "README.md", "base\nremote-main\n")
	runGit(t, upstream, "add", "README.md")
	runGit(t, upstream, "commit", "-m", "main v2")
	runGit(t, upstream, "push", "origin", "main")

	runGit(t, upstream, "checkout", "feature-a")
	writeRepoFile(t, upstream, "feature-a.txt", "v2-remote\n")
	runGit(t, upstream, "add", "feature-a.txt")
	runGit(t, upstream, "commit", "-m", "feature-a remote")
	runGit(t, upstream, "push", "origin", "feature-a")

	runGit(t, repo, "fetch", "--all")
	runGit(t, repo, "checkout", "feature-a")
	writeRepoFile(t, repo, "feature-a.txt", "v2-local\n")
	runGit(t, repo, "add", "feature-a.txt")
	runGit(t, repo, "commit", "-m", "feature-a local")
	runGit(t, repo, "checkout", "main")

	if _, err := config.Initialize(repo); err != nil {
		t.Fatalf("Initialize() error = %v", err)
	}
	if err := deps.NewLocalSource(repo).Set(context.Background(), "feature-a", "main"); err != nil {
		t.Fatalf("Set() error = %v", err)
	}

	cfg := config.Default()
	report, err := New(gitrunner.NewCLIRunner(repo, nil), &cfg, nil).Run(context.Background())
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if !report.HasFailures() {
		t.Fatal("Run() should report upstream divergence as a failure")
	}
	if !containsMessage(report, `branch "main" is behind`) {
		t.Fatalf("Run() messages = %#v, want behind warning for main", report.Checks)
	}
	if !containsMessage(report, `branch "feature-a" has diverged`) {
		t.Fatalf("Run() messages = %#v, want divergence failure for feature-a", report.Checks)
	}
}

func containsMessage(report *Report, fragment string) bool {
	for _, check := range report.Checks {
		if strings.Contains(check.Message, fragment) {
			return true
		}
	}
	return false
}

func initRepo(t *testing.T) string {
	t.Helper()

	repo := t.TempDir()
	runGit(t, repo, "init", "-b", "main")
	runGit(t, repo, "config", "user.name", "Doctor Test")
	runGit(t, repo, "config", "user.email", "doctor@example.com")
	return repo
}

func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()

	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=Doctor Test",
		"GIT_AUTHOR_EMAIL=doctor@example.com",
		"GIT_COMMITTER_NAME=Doctor Test",
		"GIT_COMMITTER_EMAIL=doctor@example.com",
	)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v failed: %v\n%s", args, err, output)
	}
}

func writeRepoFile(t *testing.T, repo string, path string, contents string) {
	t.Helper()

	fullPath := filepath.Join(repo, path)
	if err := os.MkdirAll(filepath.Dir(fullPath), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(fullPath, []byte(contents), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}
