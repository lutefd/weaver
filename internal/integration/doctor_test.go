package integration

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
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

func TestAnalyzerWarnsOnLargeDrift(t *testing.T) {
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

	for i := 0; i < 10; i++ {
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
	if report.HasFailures() {
		t.Fatalf("Analyze() failures = %#v, want none for drift-only report", report.Checks)
	}
	if report.Summary.Warn == 0 {
		t.Fatalf("Analyze() warnings = %d, want drift warning", report.Summary.Warn)
	}
	if !containsMessage(report, "behind expected parent") {
		t.Fatalf("Analyze() checks = %#v, want drift warning", report.Checks)
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

func TestReportHelpersAndNewAnalyzer(t *testing.T) {
	t.Parallel()

	report := &Report{}
	report.add(LevelOK, "ok", "all good")
	report.addHint(LevelWarn, "warn", "fix it", "watch out")
	report.add(LevelFail, "fail", "broken")

	if report.Summary != (Summary{OK: 1, Warn: 1, Fail: 1}) {
		t.Fatalf("Summary = %#v", report.Summary)
	}
	if !report.HasFailures() {
		t.Fatal("HasFailures() = false, want true")
	}

	analyzer := NewAnalyzer(&scriptedIntegrationRunner{})
	if analyzer == nil {
		t.Fatal("NewAnalyzer() = nil")
	}
}

func TestAnalyzerHelperBranches(t *testing.T) {
	t.Parallel()

	t.Run("invalid recipe", func(t *testing.T) {
		dag, err := stack.NewDAG(nil)
		if err != nil {
			t.Fatalf("NewDAG() error = %v", err)
		}

		_, err = NewAnalyzer(&scriptedIntegrationRunner{}).Analyze(context.Background(), dag, "", Recipe{})
		if err == nil || err.Error() != "integration name is required" {
			t.Fatalf("Analyze() error = %v, want invalid recipe error", err)
		}
	})

	t.Run("missing base branch", func(t *testing.T) {
		dag, err := stack.NewDAG(nil)
		if err != nil {
			t.Fatalf("NewDAG() error = %v", err)
		}

		report, err := NewAnalyzer(&scriptedIntegrationRunner{
			results: map[string]gitrunner.Result{
				"show-ref --verify --quiet refs/heads/main": {ExitCode: 1},
			},
			errs: map[string]error{
				"show-ref --verify --quiet refs/heads/main": errors.New("exit status 1"),
			},
		}).Analyze(context.Background(), dag, "integration", Recipe{
			Base:     "main",
			Branches: []string{"feature-a"},
		})
		if err != nil {
			t.Fatalf("Analyze() error = %v", err)
		}
		if !containsMessage(report, `integration base "main" does not exist locally`) {
			t.Fatalf("Analyze() checks = %#v", report.Checks)
		}
	})
}

func TestAnalyzerUtilityMethods(t *testing.T) {
	t.Parallel()

	runner := &scriptedIntegrationRunner{
		results: map[string]gitrunner.Result{
			"show-ref --verify --quiet refs/heads/feature-a": {ExitCode: 1},
			"merge-tree --write-tree --messages --name-only main feature-a": {
				Stdout:   "Auto-merging app.go\nCONFLICT (content): merge conflict in app.go\napi/routes.go\nweb/view.tsx\n",
				ExitCode: 1,
			},
			"rev-list --reverse main..feature-a":      {Stdout: "a1\nb2"},
			"rev-list --first-parent feature-a":       {Stdout: "c3\nb2\na1"},
			"merge-base --is-ancestor deadbeef main":  {ExitCode: 1},
			"merge-base --is-ancestor deadbeef other": {},
		},
		errs: map[string]error{
			"show-ref --verify --quiet refs/heads/feature-a":                errors.New("exit status 1"),
			"merge-tree --write-tree --messages --name-only main feature-a": errors.New("exit status 1"),
			"merge-base --is-ancestor deadbeef main":                        errors.New("exit status 1"),
		},
	}
	analyzer := NewAnalyzer(runner)

	exists, err := analyzer.branchExists(context.Background(), "feature-a")
	if err != nil {
		t.Fatalf("branchExists() error = %v", err)
	}
	if exists {
		t.Fatal("branchExists() = true, want false")
	}

	files, risk, err := analyzer.predictConflict(context.Background(), "main", "feature-a")
	if err != nil {
		t.Fatalf("predictConflict() error = %v", err)
	}
	if !risk {
		t.Fatal("predictConflict() risk = false, want true")
	}
	if !reflect.DeepEqual(files, []string{"api/routes.go", "web/view.tsx"}) {
		t.Fatalf("predictConflict() files = %#v", files)
	}

	commits, err := analyzer.branchRange(context.Background(), "main", "feature-a")
	if err != nil {
		t.Fatalf("branchRange() error = %v", err)
	}
	if !reflect.DeepEqual(commits, []string{"a1", "b2"}) {
		t.Fatalf("branchRange() = %#v", commits)
	}

	firstParents, err := analyzer.firstParentSet(context.Background(), "feature-a")
	if err != nil {
		t.Fatalf("firstParentSet() error = %v", err)
	}
	if len(firstParents) != 3 {
		t.Fatalf("firstParentSet() = %#v", firstParents)
	}

	ok, err := analyzer.isAncestorOfAny(context.Background(), "deadbeef", []string{"", "main", "other"})
	if err != nil {
		t.Fatalf("isAncestorOfAny() error = %v", err)
	}
	if !ok {
		t.Fatal("isAncestorOfAny() = false, want true")
	}
}

func TestIntegrationFormattingHelpers(t *testing.T) {
	t.Parallel()

	dag, err := stack.NewDAG([]stack.Dependency{{Branch: "feature-b", Parent: "feature-a"}})
	if err != nil {
		t.Fatalf("NewDAG() error = %v", err)
	}

	if !relatedInDAG(dag, "feature-a", "feature-b") {
		t.Fatal("relatedInDAG() = false, want true")
	}
	if relatedInDAG(dag, "feature-a", "feature-x") {
		t.Fatal("relatedInDAG() = true, want false")
	}
	if got := shortRef("1234567890abcdef"); got != "1234567890ab" {
		t.Fatalf("shortRef() = %q", got)
	}
	if got := shortRefs([]string{"1234567890abcdef"}); !reflect.DeepEqual(got, []string{"1234567890ab"}) {
		t.Fatalf("shortRefs() = %#v", got)
	}
	if !strings.Contains(manualMergeHint("integration", "feature-a"), `"feature-a"`) {
		t.Fatalf("manualMergeHint() = %q", manualMergeHint("integration", "feature-a"))
	}
	if !strings.Contains(manualMergeHint("integration", ""), "repair the saved integration") {
		t.Fatalf("manualMergeHint() = %q", manualMergeHint("integration", ""))
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

type scriptedIntegrationRunner struct {
	results map[string]gitrunner.Result
	errs    map[string]error
}

func (r *scriptedIntegrationRunner) Run(_ context.Context, args ...string) (gitrunner.Result, error) {
	key := strings.Join(args, " ")
	if result, ok := r.results[key]; ok {
		if err, hasErr := r.errs[key]; hasErr {
			if result.ExitCode == 0 {
				result.ExitCode = 1
			}
			return result, err
		}
		return result, nil
	}
	if err, ok := r.errs[key]; ok {
		return gitrunner.Result{ExitCode: 1}, err
	}
	return gitrunner.Result{}, nil
}

func (r *scriptedIntegrationRunner) RepoRoot() string {
	return ""
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
