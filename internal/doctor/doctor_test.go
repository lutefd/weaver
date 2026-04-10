package doctor

import (
	"context"
	"errors"
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
	"github.com/lutefd/weaver/internal/merger"
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

func TestReportHelpersAndNew(t *testing.T) {
	t.Parallel()

	report := &Report{}
	report.add(LevelOK, "ok", "all good")
	report.addHint(LevelWarn, "warn", "do thing", "warning")
	report.add(LevelFail, "fail", "broken")

	if report.Summary != (Summary{OK: 1, Warn: 1, Fail: 1}) {
		t.Fatalf("Summary = %#v", report.Summary)
	}
	if !report.HasFailures() {
		t.Fatal("HasFailures() = false, want true")
	}

	checker := New(&scriptedRunner{repoRoot: t.TempDir()}, nil, nil)
	if checker.cfg.DefaultBase != "main" {
		t.Fatalf("DefaultBase = %q, want main", checker.cfg.DefaultBase)
	}
	if checker.branchCache == nil {
		t.Fatal("branchCache = nil")
	}
}

func TestCheckerHelperBranches(t *testing.T) {
	t.Parallel()

	t.Run("initialization missing", func(t *testing.T) {
		repoRoot := t.TempDir()
		report := &Report{}
		New(&scriptedRunner{repoRoot: repoRoot}, &config.Config{DefaultBase: "main"}, nil).checkInitialization(report)
		if !containsMessage(report, "weaver is not fully initialized") {
			t.Fatalf("checkInitialization() = %#v", report.Checks)
		}
	})

	t.Run("base branch unborn", func(t *testing.T) {
		report := &Report{}
		checker := New(&scriptedRunner{
			results: map[string]gitrunner.Result{
				"show-ref --verify --quiet refs/heads/main": {ExitCode: 1},
				"branch --show-current":                     {Stdout: "main"},
			},
			errs: map[string]error{
				"show-ref --verify --quiet refs/heads/main": errors.New("exit status 1"),
			},
		}, &config.Config{DefaultBase: "main"}, nil)
		checker.checkBaseBranch(context.Background(), report)
		if !containsMessage(report, "configured base branch is the current unborn branch") {
			t.Fatalf("checkBaseBranch() = %#v", report.Checks)
		}
	})

	t.Run("base branch missing", func(t *testing.T) {
		report := &Report{}
		checker := New(&scriptedRunner{
			results: map[string]gitrunner.Result{
				"show-ref --verify --quiet refs/heads/main": {ExitCode: 1},
				"branch --show-current":                     {Stdout: "feature-a"},
			},
			errs: map[string]error{
				"show-ref --verify --quiet refs/heads/main": errors.New("exit status 1"),
			},
		}, &config.Config{DefaultBase: "main"}, nil)
		checker.checkBaseBranch(context.Background(), report)
		if !containsMessage(report, `configured base branch "main" does not exist locally`) {
			t.Fatalf("checkBaseBranch() = %#v", report.Checks)
		}
	})

	t.Run("current branch detached", func(t *testing.T) {
		report := &Report{}
		New(&scriptedRunner{}, &config.Config{DefaultBase: "main"}, nil).checkCurrentBranch(context.Background(), report)
		if !containsMessage(report, "repository is in detached HEAD state") {
			t.Fatalf("checkCurrentBranch() = %#v", report.Checks)
		}
	})

	t.Run("current branch error", func(t *testing.T) {
		report := &Report{}
		New(&scriptedRunner{
			errs: map[string]error{"branch --show-current": errors.New("boom")},
		}, &config.Config{DefaultBase: "main"}, nil).checkCurrentBranch(context.Background(), report)
		if !containsMessage(report, "cannot resolve current branch: boom") {
			t.Fatalf("checkCurrentBranch() = %#v", report.Checks)
		}
	})
}

func TestCheckerGitHelpers(t *testing.T) {
	t.Parallel()

	repoRoot := t.TempDir()
	gitDir := filepath.Join(repoRoot, ".git")
	if err := os.MkdirAll(gitDir, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(gitDir, "MERGE_HEAD"), []byte("head\n"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	checker := New(&scriptedRunner{
		repoRoot: repoRoot,
		results: map[string]gitrunner.Result{
			"rev-parse --git-dir": {Stdout: ".git"},
		},
	}, &config.Config{DefaultBase: "main"}, nil)

	gotDir, err := checker.gitDir(context.Background())
	if err != nil {
		t.Fatalf("gitDir() error = %v", err)
	}
	if want := filepath.Join(repoRoot, ".git"); gotDir != want {
		t.Fatalf("gitDir() = %q, want %q", gotDir, want)
	}

	report := &Report{}
	checker.checkGitOperations(context.Background(), report)
	if !containsMessage(report, "in-progress git operations detected: merge") {
		t.Fatalf("checkGitOperations() = %#v", report.Checks)
	}

	emptyChecker := New(&scriptedRunner{
		repoRoot: repoRoot,
		results: map[string]gitrunner.Result{
			"rev-parse --git-dir": {Stdout: ""},
		},
	}, &config.Config{DefaultBase: "main"}, nil)
	if _, err := emptyChecker.gitDir(context.Background()); err == nil || err.Error() != "empty git dir" {
		t.Fatalf("gitDir() error = %v, want empty git dir", err)
	}
}

func TestCheckerStateAndBranchHelpers(t *testing.T) {
	t.Parallel()

	report := &Report{}
	tracked := map[string]struct{}{}
	checker := New(&scriptedRunner{}, &config.Config{DefaultBase: "main"}, nil)
	checker.checkPendingStateBranches(context.Background(), report, tracked, "", "", nil, ".git/weaver/rebase-state.yaml")
	if len(report.Checks) != 3 {
		t.Fatalf("checkPendingStateBranches() checks = %#v", report.Checks)
	}

	runner := &scriptedRunner{
		results: map[string]gitrunner.Result{
			"show-ref --verify --quiet refs/heads/feature-a": {ExitCode: 1},
			"for-each-ref --format=%(refname:short)%09%(upstream:short) refs/heads/feature-a": {
				Stdout: "feature-a\torigin/feature-a",
			},
			"rev-list --left-right --count feature-a...origin/feature-a": {Stdout: "3\t2"},
		},
		errs: map[string]error{
			"show-ref --verify --quiet refs/heads/feature-a": errors.New("exit status 1"),
		},
	}
	checker = New(runner, &config.Config{DefaultBase: "main"}, nil)

	exists, err := checker.branchExists(context.Background(), "feature-a")
	if err != nil {
		t.Fatalf("branchExists() error = %v", err)
	}
	if exists {
		t.Fatal("branchExists() = true, want false")
	}

	upstream, hasUpstream, err := checker.upstreamForBranch(context.Background(), "feature-a")
	if err != nil {
		t.Fatalf("upstreamForBranch() error = %v", err)
	}
	if !hasUpstream || upstream != "origin/feature-a" {
		t.Fatalf("upstreamForBranch() = %q, %v", upstream, hasUpstream)
	}

	ahead, behind, err := checker.aheadBehind(context.Background(), "feature-a", "origin/feature-a")
	if err != nil {
		t.Fatalf("aheadBehind() error = %v", err)
	}
	if ahead != 3 || behind != 2 {
		t.Fatalf("aheadBehind() = %d, %d", ahead, behind)
	}
}

func TestCheckerRebaseStateVariants(t *testing.T) {
	t.Parallel()

	t.Run("both pending", func(t *testing.T) {
		repoRoot := t.TempDir()
		if err := rebaser.NewStateStore(repoRoot).Save(&rebaser.State{Current: "feature-a"}); err != nil {
			t.Fatalf("Save() rebase error = %v", err)
		}
		if err := merger.NewStateStore(repoRoot).Save(&merger.State{Current: "feature-a"}); err != nil {
			t.Fatalf("Save() merge error = %v", err)
		}

		report := &Report{}
		New(&scriptedRunner{repoRoot: repoRoot}, &config.Config{DefaultBase: "main"}, nil).checkRebaseState(context.Background(), report, map[string]struct{}{})
		if !containsMessage(report, "both rebase and merge stack sync state files are present") {
			t.Fatalf("checkRebaseState() = %#v", report.Checks)
		}
	})

	t.Run("pending merge state", func(t *testing.T) {
		repoRoot := t.TempDir()
		if err := merger.NewStateStore(repoRoot).Save(&merger.State{
			OriginalBranch: "feature-b",
			BaseBranch:     "main",
			AllBranches:    []string{"feature-a", "feature-b"},
			Current:        "feature-b",
		}); err != nil {
			t.Fatalf("Save() merge error = %v", err)
		}

		runner := &scriptedRunner{
			repoRoot: repoRoot,
			results: map[string]gitrunner.Result{
				"show-ref --verify --quiet refs/heads/feature-b": {},
				"show-ref --verify --quiet refs/heads/main":      {},
				"show-ref --verify --quiet refs/heads/feature-a": {},
			},
		}
		report := &Report{}
		New(runner, &config.Config{DefaultBase: "main"}, nil).checkRebaseState(context.Background(), report, map[string]struct{}{})
		if !containsMessage(report, `pending merge stack sync state detected for "feature-b"`) {
			t.Fatalf("checkRebaseState() = %#v", report.Checks)
		}
	})
}

func containsMessage(report *Report, fragment string) bool {
	for _, check := range report.Checks {
		if strings.Contains(check.Message, fragment) {
			return true
		}
	}
	return false
}

type scriptedRunner struct {
	repoRoot string
	results  map[string]gitrunner.Result
	errs     map[string]error
	calls    []string
}

func (r *scriptedRunner) Run(_ context.Context, args ...string) (gitrunner.Result, error) {
	key := strings.Join(args, " ")
	r.calls = append(r.calls, key)

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

func (r *scriptedRunner) RepoRoot() string {
	return r.repoRoot
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
