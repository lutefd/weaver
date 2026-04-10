//go:build integration

package main_test

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
)

var (
	buildBinaryOnce sync.Once
	buildBinaryPath string
	buildBinaryErr  error
)

func TestSyncContinueIntegration(t *testing.T) {
	repo := initGitRepo(t)
	writeRepoFile(t, repo, "shared.txt", "base\n")
	git(t, repo, "add", "shared.txt")
	git(t, repo, "commit", "-m", "init")

	git(t, repo, "checkout", "-b", "feature-a")
	writeRepoFile(t, repo, "shared.txt", "feature-a\n")
	git(t, repo, "add", "shared.txt")
	git(t, repo, "commit", "-m", "feature-a")

	git(t, repo, "checkout", "-b", "feature-b")
	writeRepoFile(t, repo, "feature-b.txt", "feature-b\n")
	git(t, repo, "add", "feature-b.txt")
	git(t, repo, "commit", "-m", "feature-b")

	git(t, repo, "checkout", "main")
	writeRepoFile(t, repo, "shared.txt", "main\n")
	git(t, repo, "add", "shared.txt")
	git(t, repo, "commit", "-m", "main-update")

	weaver(t, repo, "init")
	weaver(t, repo, "stack", "feature-b", "--on", "feature-a")
	git(t, repo, "checkout", "feature-b")

	result := weaverAllowError(t, repo, "sync", "feature-b")
	if result.ExitCode == 0 {
		t.Fatalf("sync exit code = 0, want conflict\n%s", result.Output)
	}
	if _, err := os.Stat(filepath.Join(repo, ".git", "weaver", "rebase-state.yaml")); err != nil {
		t.Fatalf("missing rebase state: %v", err)
	}

	writeRepoFile(t, repo, "shared.txt", "resolved\n")
	git(t, repo, "add", "shared.txt")

	continueResult := weaver(t, repo, "continue")
	if !strings.Contains(continueResult.Output, "continued stack sync") {
		t.Fatalf("continue output = %q", continueResult.Output)
	}
	assertCurrentBranch(t, repo, "feature-b")
	if _, err := os.Stat(filepath.Join(repo, ".git", "weaver", "rebase-state.yaml")); !os.IsNotExist(err) {
		t.Fatalf("rebase state still present: %v", err)
	}
	if got := strings.TrimSpace(showFileAtRef(t, repo, "feature-a", "shared.txt")); got != "resolved" {
		t.Fatalf("feature-a shared.txt = %q, want resolved", got)
	}
	if got := strings.TrimSpace(showFileAtRef(t, repo, "feature-b", "feature-b.txt")); got != "feature-b" {
		t.Fatalf("feature-b.txt = %q, want feature-b", got)
	}
}

func TestSyncAbortIntegration(t *testing.T) {
	repo := initGitRepo(t)
	writeRepoFile(t, repo, "shared.txt", "base\n")
	git(t, repo, "add", "shared.txt")
	git(t, repo, "commit", "-m", "init")

	git(t, repo, "checkout", "-b", "feature-a")
	writeRepoFile(t, repo, "shared.txt", "feature-a\n")
	git(t, repo, "add", "shared.txt")
	git(t, repo, "commit", "-m", "feature-a")

	git(t, repo, "checkout", "-b", "feature-b")
	writeRepoFile(t, repo, "feature-b.txt", "feature-b\n")
	git(t, repo, "add", "feature-b.txt")
	git(t, repo, "commit", "-m", "feature-b")

	git(t, repo, "checkout", "main")
	writeRepoFile(t, repo, "shared.txt", "main\n")
	git(t, repo, "add", "shared.txt")
	git(t, repo, "commit", "-m", "main-update")

	weaver(t, repo, "init")
	weaver(t, repo, "stack", "feature-b", "--on", "feature-a")
	git(t, repo, "checkout", "feature-b")

	result := weaverAllowError(t, repo, "sync", "feature-b")
	if result.ExitCode == 0 {
		t.Fatalf("sync exit code = 0, want conflict\n%s", result.Output)
	}

	abortResult := weaver(t, repo, "abort")
	if !strings.Contains(abortResult.Output, "aborted stack sync") {
		t.Fatalf("abort output = %q", abortResult.Output)
	}
	assertCurrentBranch(t, repo, "feature-b")
	if _, err := os.Stat(filepath.Join(repo, ".git", "weaver", "rebase-state.yaml")); !os.IsNotExist(err) {
		t.Fatalf("rebase state still present: %v", err)
	}
	if output := git(t, repo, "status", "--short", "--untracked-files=no"); strings.TrimSpace(output) != "" {
		t.Fatalf("working tree not clean after abort:\n%s", output)
	}
}

func TestComposeEphemeralIntegration(t *testing.T) {
	repo := setupComposeRepo(t)
	before := revParse(t, repo, "integration")

	result := weaver(t, repo, "compose", "feature-b", "--base", "integration")
	if !strings.Contains(result.Output, "composed ephemerally on integration") {
		t.Fatalf("compose output = %q", result.Output)
	}
	assertCurrentBranch(t, repo, "feature-b")
	after := revParse(t, repo, "integration")
	if before != after {
		t.Fatalf("integration changed in ephemeral compose: before=%s after=%s", before, after)
	}
}

func TestComposeCreateIntegration(t *testing.T) {
	repo := setupComposeRepo(t)
	integrationBefore := revParse(t, repo, "integration")

	result := weaver(t, repo, "compose", "feature-b", "--base", "integration", "--create", "release-1")
	if !strings.Contains(result.Output, "created release-1 from integration with: feature-a -> feature-b") {
		t.Fatalf("compose output = %q", result.Output)
	}
	assertCurrentBranch(t, repo, "feature-b")
	if got := strings.TrimSpace(git(t, repo, "branch", "--list", "release-1")); got != "release-1" && got != "* release-1" {
		t.Fatalf("release-1 branch missing: %q", got)
	}
	if got := strings.TrimSpace(showFileAtRef(t, repo, "release-1", "feature-b.txt")); got != "feature-b" {
		t.Fatalf("release-1 feature-b.txt = %q, want feature-b", got)
	}
	if integrationBefore != revParse(t, repo, "integration") {
		t.Fatalf("integration changed during --create compose")
	}

	listResult := weaver(t, repo, "integration", "branch", "list")
	if !strings.Contains(listResult.Output, "release-1: status=present base=integration branches=feature-a, feature-b") {
		t.Fatalf("integration branch list output = %q", listResult.Output)
	}
}

func TestComposeUpdateIntegrationRebuildsFromBase(t *testing.T) {
	repo := initGitRepo(t)
	writeRepoFile(t, repo, "README.md", "base\n")
	git(t, repo, "add", "README.md")
	git(t, repo, "commit", "-m", "init")
	git(t, repo, "branch", "integration")

	git(t, repo, "checkout", "-b", "feature-a", "main")
	writeRepoFile(t, repo, "feature-a.txt", "feature-a\n")
	git(t, repo, "add", "feature-a.txt")
	git(t, repo, "commit", "-m", "feature-a")

	git(t, repo, "checkout", "-b", "feature-b")
	writeRepoFile(t, repo, "feature-b.txt", "feature-b\n")
	git(t, repo, "add", "feature-b.txt")
	git(t, repo, "commit", "-m", "feature-b")

	weaver(t, repo, "init")
	weaver(t, repo, "stack", "feature-a", "--on", "main")
	weaver(t, repo, "stack", "feature-b", "--on", "feature-a")
	git(t, repo, "checkout", "feature-b")

	seedResult := weaver(t, repo, "compose", "feature-b", "--base", "main", "--update", "integration")
	if !strings.Contains(seedResult.Output, "updated integration from main with: feature-a -> feature-b") {
		t.Fatalf("compose output = %q", seedResult.Output)
	}

	git(t, repo, "checkout", "integration")
	writeRepoFile(t, repo, "integration-only.txt", "stale\n")
	git(t, repo, "add", "integration-only.txt")
	git(t, repo, "commit", "-m", "integration drift")
	driftedIntegration := revParse(t, repo, "integration")

	git(t, repo, "checkout", "main")
	writeRepoFile(t, repo, "main.txt", "main-update\n")
	git(t, repo, "add", "main.txt")
	git(t, repo, "commit", "-m", "main-update")

	git(t, repo, "checkout", "feature-b")
	syncResult := weaver(t, repo, "sync", "feature-b")
	if !strings.Contains(syncResult.Output, "synced feature-b") {
		t.Fatalf("sync output = %q", syncResult.Output)
	}

	result := weaver(t, repo, "compose", "feature-b", "--base", "main", "--update", "integration")
	if !strings.Contains(result.Output, "updated integration from main with: feature-a -> feature-b") {
		t.Fatalf("compose output = %q", result.Output)
	}
	assertCurrentBranch(t, repo, "feature-b")

	if after := revParse(t, repo, "integration"); after == driftedIntegration {
		t.Fatalf("integration did not change after update")
	}
	if fileExistsAtRef(t, repo, "integration", "integration-only.txt") {
		t.Fatal("integration-only.txt still present after update")
	}
	if got := strings.TrimSpace(showFileAtRef(t, repo, "integration", "main.txt")); got != "main-update" {
		t.Fatalf("integration main.txt = %q, want main-update", got)
	}
	if got := strings.TrimSpace(showFileAtRef(t, repo, "integration", "feature-a.txt")); got != "feature-a" {
		t.Fatalf("integration feature-a.txt = %q, want feature-a", got)
	}
	if got := strings.TrimSpace(showFileAtRef(t, repo, "integration", "feature-b.txt")); got != "feature-b" {
		t.Fatalf("integration feature-b.txt = %q, want feature-b", got)
	}

	listResult := weaver(t, repo, "integration", "branch", "list")
	if !strings.Contains(listResult.Output, "integration: status=present base=main branches=feature-a, feature-b") {
		t.Fatalf("integration branch list output = %q", listResult.Output)
	}
}

func TestIntegrationBranchDeleteRemovesTrackedBranchAndGitRef(t *testing.T) {
	repo := setupComposeRepo(t)

	createResult := weaver(t, repo, "compose", "feature-b", "--base", "integration", "--create", "release-1")
	if !strings.Contains(createResult.Output, "created release-1 from integration with: feature-a -> feature-b") {
		t.Fatalf("compose output = %q", createResult.Output)
	}

	deleteResult := weaver(t, repo, "integration", "branch", "delete", "release-1")
	if !strings.Contains(deleteResult.Output, "deleted integration branch release-1") {
		t.Fatalf("integration branch delete output = %q", deleteResult.Output)
	}

	if got := strings.TrimSpace(git(t, repo, "branch", "--list", "release-1")); got != "" {
		t.Fatalf("release-1 branch still present: %q", got)
	}

	listResult := weaver(t, repo, "integration", "branch", "list")
	if !strings.Contains(listResult.Output, "no integration branches") {
		t.Fatalf("integration branch list output = %q", listResult.Output)
	}
}

func TestUpdateIntegration(t *testing.T) {
	root := t.TempDir()
	remote := filepath.Join(root, "origin.git")
	git(t, root, "init", "--bare", "-b", "main", remote)

	seed := filepath.Join(root, "seed")
	git(t, root, "clone", remote, seed)
	configureGitUser(t, seed)
	writeRepoFile(t, seed, "README.md", "seed\n")
	git(t, seed, "add", "README.md")
	git(t, seed, "commit", "-m", "init")
	git(t, seed, "push", "-u", "origin", "main")
	git(t, seed, "checkout", "-b", "feature-a")
	writeRepoFile(t, seed, "feature-a.txt", "v1\n")
	git(t, seed, "add", "feature-a.txt")
	git(t, seed, "commit", "-m", "feature-a v1")
	git(t, seed, "push", "-u", "origin", "feature-a")

	repo := filepath.Join(root, "repo")
	git(t, root, "clone", remote, repo)
	configureGitUser(t, repo)
	git(t, repo, "checkout", "-b", "feature-a", "--track", "origin/feature-a")
	git(t, repo, "checkout", "main")

	upstream := filepath.Join(root, "upstream")
	git(t, root, "clone", remote, upstream)
	configureGitUser(t, upstream)
	git(t, upstream, "checkout", "feature-a")
	writeRepoFile(t, upstream, "feature-a.txt", "v2\n")
	git(t, upstream, "add", "feature-a.txt")
	git(t, upstream, "commit", "-m", "feature-a v2")
	git(t, upstream, "push", "origin", "feature-a")

	result := weaver(t, repo, "update", "feature-a")
	if !strings.Contains(result.Output, "updated feature-a") {
		t.Fatalf("update output = %q", result.Output)
	}
	assertCurrentBranch(t, repo, "main")
	if revParse(t, repo, "feature-a") != revParse(t, repo, "origin/feature-a") {
		t.Fatalf("feature-a did not fast-forward to origin/feature-a")
	}
}

func setupComposeRepo(t *testing.T) string {
	t.Helper()

	repo := initGitRepo(t)
	writeRepoFile(t, repo, "README.md", "base\n")
	git(t, repo, "add", "README.md")
	git(t, repo, "commit", "-m", "init")
	git(t, repo, "branch", "integration")
	git(t, repo, "checkout", "-b", "feature-a", "integration")
	writeRepoFile(t, repo, "feature-a.txt", "feature-a\n")
	git(t, repo, "add", "feature-a.txt")
	git(t, repo, "commit", "-m", "feature-a")
	git(t, repo, "checkout", "-b", "feature-b")
	writeRepoFile(t, repo, "feature-b.txt", "feature-b\n")
	git(t, repo, "add", "feature-b.txt")
	git(t, repo, "commit", "-m", "feature-b")

	weaver(t, repo, "init")
	weaver(t, repo, "stack", "feature-a", "--on", "integration")
	weaver(t, repo, "stack", "feature-b", "--on", "feature-a")
	git(t, repo, "checkout", "feature-b")

	return repo
}

func initGitRepo(t *testing.T) string {
	t.Helper()

	repo := t.TempDir()
	git(t, repo, "init", "-b", "main")
	configureGitUser(t, repo)
	return repo
}

func configureGitUser(t *testing.T, repo string) {
	t.Helper()

	git(t, repo, "config", "user.name", "Weaver Integration")
	git(t, repo, "config", "user.email", "integration@example.com")
}

type commandResult struct {
	Output   string
	ExitCode int
}

func weaver(t *testing.T, repo string, args ...string) commandResult {
	t.Helper()

	result, err := runCommand(repo, buildWeaverBinary(t), args...)
	if err != nil {
		t.Fatalf("weaver %v failed: %v\n%s", args, err, result.Output)
	}
	return result
}

func weaverAllowError(t *testing.T, repo string, args ...string) commandResult {
	t.Helper()

	result, _ := runCommand(repo, buildWeaverBinary(t), args...)
	return result
}

func git(t *testing.T, repo string, args ...string) string {
	t.Helper()

	result, err := runCommand(repo, "git", args...)
	if err != nil {
		t.Fatalf("git %v failed: %v\n%s", args, err, result.Output)
	}
	return strings.TrimSpace(result.Output)
}

func runCommand(dir string, name string, args ...string) (commandResult, error) {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=Weaver Integration",
		"GIT_AUTHOR_EMAIL=integration@example.com",
		"GIT_COMMITTER_NAME=Weaver Integration",
		"GIT_COMMITTER_EMAIL=integration@example.com",
		"GIT_EDITOR=true",
		"EDITOR=true",
		"GIT_TERMINAL_PROMPT=0",
	)
	output, err := cmd.CombinedOutput()

	result := commandResult{
		Output: strings.TrimSpace(string(output)),
	}
	if err == nil {
		return result, nil
	}

	if exitErr, ok := err.(*exec.ExitError); ok {
		result.ExitCode = exitErr.ExitCode()
	} else {
		result.ExitCode = 1
	}

	return result, err
}

func buildWeaverBinary(t *testing.T) string {
	t.Helper()

	buildBinaryOnce.Do(func() {
		dir, err := os.MkdirTemp("", "weaver-integration-bin-*")
		if err != nil {
			buildBinaryErr = err
			return
		}

		buildBinaryPath = filepath.Join(dir, "weaver")
		cmd := exec.Command("go", "build", "-o", buildBinaryPath, ".")
		cmd.Dir = repoRoot()
		output, err := cmd.CombinedOutput()
		if err != nil {
			buildBinaryErr = fmt.Errorf("go build failed: %w\n%s", err, strings.TrimSpace(string(output)))
		}
	})

	if buildBinaryErr != nil {
		t.Fatal(buildBinaryErr)
	}

	return buildBinaryPath
}

func repoRoot() string {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		panic("cannot resolve caller")
	}
	return filepath.Dir(file)
}

func writeRepoFile(t *testing.T, repo string, name string, contents string) {
	t.Helper()

	path := filepath.Join(repo, name)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir for %s: %v", name, err)
	}
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
}

func assertCurrentBranch(t *testing.T, repo string, want string) {
	t.Helper()

	if got := git(t, repo, "branch", "--show-current"); got != want {
		t.Fatalf("current branch = %q, want %q", got, want)
	}
}

func revParse(t *testing.T, repo string, ref string) string {
	t.Helper()
	return git(t, repo, "rev-parse", ref)
}

func showFileAtRef(t *testing.T, repo string, ref string, path string) string {
	t.Helper()
	return git(t, repo, "show", ref+":"+path)
}

func fileExistsAtRef(t *testing.T, repo string, ref string, path string) bool {
	t.Helper()

	result, err := runCommand(repo, "git", "cat-file", "-e", ref+":"+path)
	return err == nil && result.ExitCode == 0
}
