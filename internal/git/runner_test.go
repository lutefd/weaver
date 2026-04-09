package git

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestDiscoverRepoRoot(t *testing.T) {
	t.Parallel()

	repoRoot := t.TempDir()
	runGit(t, repoRoot, "init", "-b", "main")
	want := evalPath(t, repoRoot)

	got, err := DiscoverRepoRoot(context.Background(), repoRoot)
	if err != nil {
		t.Fatalf("DiscoverRepoRoot() error = %v", err)
	}
	if got != want {
		t.Fatalf("DiscoverRepoRoot() = %q, want %q", got, want)
	}
}

func TestCLIRunnerRun(t *testing.T) {
	t.Parallel()

	repoRoot := t.TempDir()
	runGit(t, repoRoot, "init", "-b", "main")
	want := evalPath(t, repoRoot)

	runner := NewCLIRunner(repoRoot, nil)
	result, err := runner.Run(context.Background(), "rev-parse", "--show-toplevel")
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if result.Stdout != want {
		t.Fatalf("Stdout = %q, want %q", result.Stdout, want)
	}
}

func TestCLIRunnerRunMutatingTraceAndError(t *testing.T) {
	t.Parallel()

	repoRoot := t.TempDir()
	runGit(t, repoRoot, "init", "-b", "main")

	var trace bytes.Buffer
	runner := NewCLIRunner(repoRoot, &trace)
	_, err := runner.Run(context.Background(), "checkout", "missing-branch")
	if err == nil {
		t.Fatal("Run() error = nil, want error")
	}
	if !strings.Contains(trace.String(), "+ git checkout missing-branch") {
		t.Fatalf("trace = %q, want git trace", trace.String())
	}
}

func TestCLIRunnerHelpers(t *testing.T) {
	t.Parallel()

	runner := NewCLIRunner("/tmp/repo", nil)
	if got := runner.RepoRoot(); got != "/tmp/repo" {
		t.Fatalf("RepoRoot() = %q", got)
	}
	withStdout := runner.WithStdout(&bytes.Buffer{})
	if withStdout == runner || withStdout.RepoRoot() != runner.RepoRoot() {
		t.Fatalf("WithStdout() did not preserve repo root correctly")
	}
	if (*CLIRunner)(nil).WithStdout(nil) != nil {
		t.Fatal("nil WithStdout() != nil")
	}
}

func TestIsMutating(t *testing.T) {
	t.Parallel()

	cases := []struct {
		args []string
		want bool
	}{
		{args: nil, want: false},
		{args: []string{"checkout", "main"}, want: true},
		{args: []string{"branch", "-f", "topic", "HEAD"}, want: true},
		{args: []string{"branch", "--show-current"}, want: false},
	}
	for _, tc := range cases {
		if got := isMutating(tc.args); got != tc.want {
			t.Fatalf("isMutating(%v) = %v, want %v", tc.args, got, tc.want)
		}
	}
}

func TestWithObserver(t *testing.T) {
	t.Parallel()

	base := &observerStubRunner{result: Result{Stdout: "ok"}}
	var beforeCalled, afterCalled bool
	runner := WithObserver(base, func(event CommandEvent) {
		beforeCalled = true
		if !event.Mutating || strings.Join(event.Args, " ") != "checkout main" {
			t.Fatalf("before event = %#v", event)
		}
	}, func(event CommandEvent, result Result, err error) {
		afterCalled = true
		if result.Stdout != "ok" || err != nil {
			t.Fatalf("after got result=%#v err=%v", result, err)
		}
	})

	got, err := runner.Run(context.Background(), "checkout", "main")
	if err != nil || got.Stdout != "ok" {
		t.Fatalf("Run() = %#v, %v", got, err)
	}
	if !beforeCalled || !afterCalled {
		t.Fatalf("observer hooks called before=%v after=%v", beforeCalled, afterCalled)
	}
	if gotRoot := runner.RepoRoot(); gotRoot != "/tmp/stub" {
		t.Fatalf("RepoRoot() = %q", gotRoot)
	}
	if WithObserver(nil, nil, nil) != nil {
		t.Fatal("WithObserver(nil) != nil")
	}
}

type observerStubRunner struct {
	result Result
	err    error
}

func (r *observerStubRunner) Run(_ context.Context, _ ...string) (Result, error) {
	return r.result, r.err
}

func (r *observerStubRunner) RepoRoot() string {
	return "/tmp/stub"
}

func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()

	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=Test",
		"GIT_AUTHOR_EMAIL=test@example.com",
		"GIT_COMMITTER_NAME=Test",
		"GIT_COMMITTER_EMAIL=test@example.com",
	)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v failed: %v\n%s", args, err, output)
	}
}

func writeFile(t *testing.T, path string, data string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func evalPath(t *testing.T, path string) string {
	t.Helper()

	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		t.Fatalf("EvalSymlinks(%q): %v", path, err)
	}

	return resolved
}
