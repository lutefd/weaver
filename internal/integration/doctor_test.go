package integration

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	gitrunner "github.com/lutefd/weaver/internal/git"
	"github.com/lutefd/weaver/internal/stack"
)

func TestAnalyzerHealthyIntegration(t *testing.T) {
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

	dag, err := stack.NewDAG([]stack.Dependency{{Branch: "feature-b", Parent: "feature-a"}})
	if err != nil {
		t.Fatalf("NewDAG() error = %v", err)
	}

	report, err := NewAnalyzer(gitrunner.NewCLIRunner(repo, nil)).Analyze(context.Background(), dag, "integration", Recipe{
		Base:     "main",
		Branches: []string{"feature-b"},
	})
	if err != nil {
		t.Fatalf("Analyze() error = %v", err)
	}
	if report.HasFailures() {
		t.Fatalf("Analyze() failures = %#v", report.Checks)
	}
	if report.Summary.Warn != 0 {
		t.Fatalf("Analyze() warnings = %d, want 0", report.Summary.Warn)
	}
}

func TestAnalyzerDetectsForeignAncestry(t *testing.T) {
	t.Parallel()

	repo := initRepo(t)
	writeRepoFile(t, repo, "README.md", "base\n")
	runGit(t, repo, "add", "README.md")
	runGit(t, repo, "commit", "-m", "init")

	runGit(t, repo, "checkout", "-b", "feature-a")
	writeRepoFile(t, repo, "feature-a.txt", "feature-a\n")
	runGit(t, repo, "add", "feature-a.txt")
	runGit(t, repo, "commit", "-m", "feature-a")

	runGit(t, repo, "checkout", "-b", "feature-debug")
	writeRepoFile(t, repo, "feature-debug.txt", "feature-debug\n")
	runGit(t, repo, "add", "feature-debug.txt")
	runGit(t, repo, "commit", "-m", "feature-debug")
	runGit(t, repo, "checkout", "main")

	dag, err := stack.NewDAG(nil)
	if err != nil {
		t.Fatalf("NewDAG() error = %v", err)
	}

	report, err := NewAnalyzer(gitrunner.NewCLIRunner(repo, nil)).Analyze(context.Background(), dag, "integration", Recipe{
		Base:     "main",
		Branches: []string{"feature-a", "feature-debug"},
	})
	if err != nil {
		t.Fatalf("Analyze() error = %v", err)
	}
	if !report.HasFailures() {
		t.Fatalf("Analyze() expected failure, got %#v", report.Checks)
	}
	if !containsMessage(report, "foreign ancestry") {
		t.Fatalf("Analyze() checks = %#v, want foreign ancestry", report.Checks)
	}
}

func TestAnalyzerDetectsLargeDrift(t *testing.T) {
	t.Parallel()

	repo := initRepo(t)
	writeRepoFile(t, repo, "README.md", "base\n")
	runGit(t, repo, "add", "README.md")
	runGit(t, repo, "commit", "-m", "init")

	runGit(t, repo, "checkout", "-b", "feature-a")
	writeRepoFile(t, repo, "feature-a.txt", "feature-a\n")
	runGit(t, repo, "add", "feature-a.txt")
	runGit(t, repo, "commit", "-m", "feature-a")
	runGit(t, repo, "checkout", "main")

	for i := 0; i < excessiveBehindThreshold; i++ {
		writeRepoFile(t, repo, "main.txt", strings.Repeat("m", i+1))
		runGit(t, repo, "add", "main.txt")
		runGit(t, repo, "commit", "-m", "main-update")
	}

	dag, err := stack.NewDAG(nil)
	if err != nil {
		t.Fatalf("NewDAG() error = %v", err)
	}

	report, err := NewAnalyzer(gitrunner.NewCLIRunner(repo, nil)).Analyze(context.Background(), dag, "integration", Recipe{
		Base:     "main",
		Branches: []string{"feature-a"},
	})
	if err != nil {
		t.Fatalf("Analyze() error = %v", err)
	}
	if !report.HasFailures() {
		t.Fatalf("Analyze() expected failure, got %#v", report.Checks)
	}
	if !containsMessage(report, "behind expected parent") {
		t.Fatalf("Analyze() checks = %#v, want drift failure", report.Checks)
	}
}

func TestAnalyzerIgnoresBaseSyncMergeCommits(t *testing.T) {
	t.Parallel()

	repo := initRepo(t)
	writeRepoFile(t, repo, "README.md", "base\n")
	runGit(t, repo, "add", "README.md")
	runGit(t, repo, "commit", "-m", "init")

	runGit(t, repo, "checkout", "-b", "feature-a")
	writeRepoFile(t, repo, "feature-a.txt", "feature-a\n")
	runGit(t, repo, "add", "feature-a.txt")
	runGit(t, repo, "commit", "-m", "feature-a")

	runGit(t, repo, "checkout", "main")
	writeRepoFile(t, repo, "main.txt", "main-update\n")
	runGit(t, repo, "add", "main.txt")
	runGit(t, repo, "commit", "-m", "main-update")

	runGit(t, repo, "checkout", "feature-a")
	runGit(t, repo, "merge", "--no-ff", "-m", "merge main", "main")

	dag, err := stack.NewDAG(nil)
	if err != nil {
		t.Fatalf("NewDAG() error = %v", err)
	}

	report, err := NewAnalyzer(gitrunner.NewCLIRunner(repo, nil)).Analyze(context.Background(), dag, "integration", Recipe{
		Base:     "main",
		Branches: []string{"feature-a"},
	})
	if err != nil {
		t.Fatalf("Analyze() error = %v", err)
	}
	if report.HasFailures() {
		t.Fatalf("Analyze() failures = %#v", report.Checks)
	}
	if report.Summary.Warn != 0 {
		t.Fatalf("Analyze() warnings = %d, want 0 (%#v)", report.Summary.Warn, report.Checks)
	}
}

func TestAnalyzerWarnsOnForeignMergeCommit(t *testing.T) {
	t.Parallel()

	repo := initRepo(t)
	writeRepoFile(t, repo, "README.md", "base\n")
	runGit(t, repo, "add", "README.md")
	runGit(t, repo, "commit", "-m", "init")

	runGit(t, repo, "checkout", "-b", "feature-a")
	writeRepoFile(t, repo, "feature-a.txt", "feature-a\n")
	runGit(t, repo, "add", "feature-a.txt")
	runGit(t, repo, "commit", "-m", "feature-a")

	runGit(t, repo, "checkout", "main")
	runGit(t, repo, "checkout", "-b", "feature-side")
	writeRepoFile(t, repo, "feature-side.txt", "feature-side\n")
	runGit(t, repo, "add", "feature-side.txt")
	runGit(t, repo, "commit", "-m", "feature-side")

	runGit(t, repo, "checkout", "feature-a")
	runGit(t, repo, "merge", "--no-ff", "-m", "merge feature-side", "feature-side")

	dag, err := stack.NewDAG(nil)
	if err != nil {
		t.Fatalf("NewDAG() error = %v", err)
	}

	report, err := NewAnalyzer(gitrunner.NewCLIRunner(repo, nil)).Analyze(context.Background(), dag, "integration", Recipe{
		Base:     "main",
		Branches: []string{"feature-a"},
	})
	if err != nil {
		t.Fatalf("Analyze() error = %v", err)
	}
	if report.Summary.Warn == 0 {
		t.Fatalf("Analyze() warnings = %d, want warning (%#v)", report.Summary.Warn, report.Checks)
	}
	if !containsMessage(report, "contains merge commits from outside") {
		t.Fatalf("Analyze() checks = %#v, want suspicious merge warning", report.Checks)
	}
}

func TestAnalyzerForeignAncestryOnlyBlamesImportingBranch(t *testing.T) {
	t.Parallel()

	repo := initRepo(t)
	writeRepoFile(t, repo, "README.md", "base\n")
	runGit(t, repo, "add", "README.md")
	runGit(t, repo, "commit", "-m", "init")

	runGit(t, repo, "checkout", "-b", "feature-a")
	writeRepoFile(t, repo, "feature-a.txt", "feature-a\n")
	runGit(t, repo, "add", "feature-a.txt")
	runGit(t, repo, "commit", "-m", "feature-a")

	runGit(t, repo, "checkout", "main")
	runGit(t, repo, "checkout", "-b", "feature-debug")
	writeRepoFile(t, repo, "feature-debug.txt", "feature-debug\n")
	runGit(t, repo, "add", "feature-debug.txt")
	runGit(t, repo, "commit", "-m", "feature-debug")
	runGit(t, repo, "merge", "--no-ff", "-m", "merge feature-a", "feature-a")

	dag, err := stack.NewDAG(nil)
	if err != nil {
		t.Fatalf("NewDAG() error = %v", err)
	}

	report, err := NewAnalyzer(gitrunner.NewCLIRunner(repo, nil)).Analyze(context.Background(), dag, "integration", Recipe{
		Base:     "main",
		Branches: []string{"feature-a", "feature-debug"},
	})
	if err != nil {
		t.Fatalf("Analyze() error = %v", err)
	}

	if containsMessage(report, `branch "feature-a" has foreign ancestry`) {
		t.Fatalf("Analyze() checks = %#v, did not want feature-a foreign ancestry", report.Checks)
	}
	if !containsMessage(report, `branch "feature-debug" has foreign ancestry`) {
		t.Fatalf("Analyze() checks = %#v, want feature-debug foreign ancestry", report.Checks)
	}
}

func TestAnalyzerIgnoresMergeOnlySharedCommitForForeignAncestry(t *testing.T) {
	t.Parallel()

	repo := initRepo(t)
	writeRepoFile(t, repo, "README.md", "base\n")
	runGit(t, repo, "add", "README.md")
	runGit(t, repo, "commit", "-m", "init")

	runGit(t, repo, "checkout", "-b", "feature-shared")
	writeRepoFile(t, repo, "shared.txt", "shared\n")
	runGit(t, repo, "add", "shared.txt")
	runGit(t, repo, "commit", "-m", "feature-shared")

	runGit(t, repo, "checkout", "main")
	runGit(t, repo, "checkout", "-b", "feature-a")
	writeRepoFile(t, repo, "feature-a.txt", "feature-a\n")
	runGit(t, repo, "add", "feature-a.txt")
	runGit(t, repo, "commit", "-m", "feature-a")
	runGit(t, repo, "merge", "--no-ff", "-m", "merge feature-shared", "feature-shared")

	runGit(t, repo, "checkout", "main")
	runGit(t, repo, "checkout", "-b", "feature-b")
	writeRepoFile(t, repo, "feature-b.txt", "feature-b\n")
	runGit(t, repo, "add", "feature-b.txt")
	runGit(t, repo, "commit", "-m", "feature-b")
	runGit(t, repo, "merge", "--no-ff", "-m", "merge feature-shared", "feature-shared")

	dag, err := stack.NewDAG(nil)
	if err != nil {
		t.Fatalf("NewDAG() error = %v", err)
	}

	report, err := NewAnalyzer(gitrunner.NewCLIRunner(repo, nil)).Analyze(context.Background(), dag, "integration", Recipe{
		Base:     "main",
		Branches: []string{"feature-a", "feature-b"},
	})
	if err != nil {
		t.Fatalf("Analyze() error = %v", err)
	}

	if containsMessage(report, "has foreign ancestry") {
		t.Fatalf("Analyze() checks = %#v, did not want foreign ancestry failure", report.Checks)
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
	runGit(t, repo, "config", "user.name", "Integration Test")
	runGit(t, repo, "config", "user.email", "integration@example.com")
	return repo
}

func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()

	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=Integration Test",
		"GIT_AUTHOR_EMAIL=integration@example.com",
		"GIT_COMMITTER_NAME=Integration Test",
		"GIT_COMMITTER_EMAIL=integration@example.com",
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
