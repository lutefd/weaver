package ui

import (
	"context"
	"io"
	"regexp"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	gitrunner "github.com/lutefd/weaver/internal/git"
)

func TestFormatGitCommand(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		args []string
		want string
	}{
		{name: "current branch", args: []string{"branch", "--show-current"}, want: "reading current branch"},
		{name: "checkout", args: []string{"checkout", "feature-a"}, want: "checking out feature-a"},
		{name: "merge ancestry", args: []string{"merge-base", "--is-ancestor", "abc", "main"}, want: "checking ancestry"},
		{name: "ff only", args: []string{"merge", "--ff-only", "origin/main"}, want: "fast-forwarding from origin/main"},
		{name: "rev list drift", args: []string{"rev-list", "--left-right", "--count", "a...b"}, want: "measuring drift"},
		{name: "fallback", args: []string{"show-ref", "--verify", "refs/heads/feature-a"}, want: "git show-ref feature-a"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := formatGitCommand(tc.args); got != tc.want {
				t.Fatalf("formatGitCommand(%v) = %q, want %q", tc.args, got, tc.want)
			}
		})
	}
}

func TestFormatGitCommandAdditionalBranches(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		args []string
		want string
	}{
		{name: "no args", args: nil, want: "git"},
		{name: "fetch", args: []string{"fetch", "--all"}, want: "fetching remotes"},
		{name: "merge continue", args: []string{"merge", "--continue"}, want: "continuing merge"},
		{name: "merge abort", args: []string{"merge", "--abort"}, want: "aborting merge"},
		{name: "merge branch", args: []string{"merge", "--no-edit", "feature-a"}, want: "merging feature-a"},
		{name: "rebase continue", args: []string{"rebase", "--continue"}, want: "continuing rebase"},
		{name: "rebase abort", args: []string{"rebase", "--abort"}, want: "aborting rebase"},
		{name: "rebase onto", args: []string{"rebase", "--autostash", "main"}, want: "rebasing onto main"},
		{name: "rev parse", args: []string{"rev-parse", "refs/heads/feature-a"}, want: "resolving feature-a"},
		{name: "for each ref", args: []string{"for-each-ref"}, want: "finding branch upstream"},
		{name: "merge base", args: []string{"merge-base", "main", "feature-a"}, want: "resolving merge base"},
		{name: "rev list merges", args: []string{"rev-list", "--merges", "main"}, want: "scanning merge commits"},
		{name: "rev list parents", args: []string{"rev-list", "--parents", "main"}, want: "inspecting merge parents"},
		{name: "rev list reverse", args: []string{"rev-list", "--reverse", "main"}, want: "building commit order"},
		{name: "rev list first parent", args: []string{"rev-list", "--first-parent", "main"}, want: "walking first-parent history"},
		{name: "rev list generic", args: []string{"rev-list", "main"}, want: "walking commit history"},
		{name: "git fallback without plain args", args: []string{"status", "--short"}, want: "git status"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := formatGitCommand(tc.args); got != tc.want {
				t.Fatalf("formatGitCommand(%v) = %q, want %q", tc.args, got, tc.want)
			}
		})
	}
}

func TestCleanArg(t *testing.T) {
	t.Parallel()

	if got := cleanArg("refs/heads/feature-a"); got != "feature-a" {
		t.Fatalf("cleanArg(ref) = %q, want %q", got, "feature-a")
	}
	if got := cleanArg("570a756147fcaaa72d4bf855db7b75ee9b64a1fd"); got != "570a7561" {
		t.Fatalf("cleanArg(sha) = %q, want %q", got, "570a7561")
	}
	if got := cleanArg("feature-a\trefs/heads/main"); got != "feature-a main" {
		t.Fatalf("cleanArg(tabbed) = %q, want %q", got, "feature-a main")
	}
}

func TestStepSummary(t *testing.T) {
	t.Parallel()

	if got := stepSummary(0, 0, 0); got != "starting" {
		t.Fatalf("stepSummary starting = %q", got)
	}
	if got := stepSummary(3, 1, 10); got != "3/10 ops" {
		t.Fatalf("stepSummary counted = %q", got)
	}
	if got := stepSummary(12, 1, 10); got != "10/10 ops" {
		t.Fatalf("stepSummary clamped = %q", got)
	}
	if got := stepSummary(0, 1, 0); got != "1 op" {
		t.Fatalf("stepSummary one step = %q", got)
	}
	if got := stepSummary(0, 3, 0); got != "3 ops" {
		t.Fatalf("stepSummary many steps = %q", got)
	}
}

func TestProgressTargetForSteps(t *testing.T) {
	t.Parallel()

	first := progressTargetForSteps(1)
	later := progressTargetForSteps(20)
	capped := progressTargetForSteps(1000)

	if !(first > 0.10) {
		t.Fatalf("first target = %v, want > 0.10", first)
	}
	if !(later > first) {
		t.Fatalf("later target = %v, want > first %v", later, first)
	}
	if capped != 0.62 {
		t.Fatalf("capped target = %v, want 0.62", capped)
	}
}

func TestActivityModelCountedProgress(t *testing.T) {
	t.Parallel()

	model := newActivityModel(Terminal{width: 80}, TaskSpec{Title: "Compose", TotalOps: 10})
	model.startedAt = time.Now().Add(-2 * time.Second)

	next, _ := model.Update(taskShowMsg{})
	model = next.(activityModel)
	if !model.visible {
		t.Fatalf("visible = false, want true")
	}

	next, _ = model.Update(taskStepMsg{label: "merging feature-a", completed: 3, total: 10})
	model = next.(activityModel)
	if model.target != 0.3 {
		t.Fatalf("target = %v, want 0.3", model.target)
	}
	if model.lastStep != "merging feature-a" {
		t.Fatalf("lastStep = %q", model.lastStep)
	}

	next, _ = model.Update(pulseMsg(time.Now()))
	model = next.(activityModel)
	if model.progressV != 0.3 {
		t.Fatalf("progressV = %v, want 0.3", model.progressV)
	}

	view := model.View()
	plain := stripANSI(view)
	if !strings.Contains(plain, "Compose") || !strings.Contains(plain, "3/10 OPS") {
		t.Fatalf("View() = %q, want title and counted ops", view)
	}
}

func TestActivityModelDoneBeforeVisibleQuitsImmediately(t *testing.T) {
	t.Parallel()

	model := newActivityModel(Terminal{width: 80}, TaskSpec{Title: "Doctor"})
	next, cmd := model.Update(taskDoneMsg{})
	model = next.(activityModel)

	if !model.completed || model.progressV != 1 {
		t.Fatalf("done state = %#v", model)
	}
	if msg := cmd(); msg != tea.Quit() {
		t.Fatalf("done cmd = %#v, want tea.Quit()", msg)
	}
}

func TestActivityModelInitAndTicks(t *testing.T) {
	t.Parallel()

	model := newActivityModel(Terminal{width: 80}, TaskSpec{Title: "Sync"})

	if msg := model.Init()(); msg == nil {
		t.Fatal("Init() returned nil command result")
	}
	if _, ok := pulseTick()().(pulseMsg); !ok {
		t.Fatalf("pulseTick() did not return pulseMsg")
	}
	if _, ok := showTick()().(taskShowMsg); !ok {
		t.Fatalf("showTick() did not return taskShowMsg")
	}
	if _, ok := completeTick()().(taskClearMsg); !ok {
		t.Fatalf("completeTick() did not return taskClearMsg")
	}
	if _, ok := quitTick()().(taskQuitMsg); !ok {
		t.Fatalf("quitTick() did not return taskQuitMsg")
	}
}

func TestActivityModelUpdateBranches(t *testing.T) {
	t.Parallel()

	model := newActivityModel(Terminal{width: 80}, TaskSpec{Title: "Sync"})
	model.startedAt = time.Now().Add(-2 * time.Second)

	next, _ := model.Update(taskShowMsg{})
	model = next.(activityModel)
	next, _ = model.Update(pulseMsg(time.Now()))
	model = next.(activityModel)
	if model.target < 0.12 {
		t.Fatalf("target = %v, want delayed idle progress", model.target)
	}

	next, cmd := model.Update(taskDoneMsg{})
	model = next.(activityModel)
	if !model.completed || model.lastStep != "done" {
		t.Fatalf("done state = %#v", model)
	}
	if _, ok := cmd().(taskClearMsg); !ok {
		t.Fatalf("taskDoneMsg command did not clear after visible completion")
	}

	next, cmd = model.Update(taskClearMsg{})
	model = next.(activityModel)
	if !model.clearing {
		t.Fatalf("clearing = false, want true")
	}
	if _, ok := cmd().(taskQuitMsg); !ok {
		t.Fatalf("taskClearMsg command did not schedule quit")
	}

	if _, cmd = model.Update(taskQuitMsg{}); cmd == nil || cmd() != tea.Quit() {
		t.Fatalf("taskQuitMsg did not return tea.Quit()")
	}
}

func TestHumanizeClampMinAndQuietRunner(t *testing.T) {
	t.Parallel()

	if got := humanizeDuration(200 * time.Millisecond); got != "starting" {
		t.Fatalf("humanizeDuration(starting) = %q", got)
	}
	if got := humanizeDuration(1500 * time.Millisecond); got != "1.5s" {
		t.Fatalf("humanizeDuration(short) = %q", got)
	}
	if got := humanizeDuration(11*time.Second + 400*time.Millisecond); got != "11s" {
		t.Fatalf("humanizeDuration(long) = %q", got)
	}

	if got := clamp(-1, 0, 1); got != 0 {
		t.Fatalf("clamp(low) = %v", got)
	}
	if got := clamp(2, 0, 1); got != 1 {
		t.Fatalf("clamp(high) = %v", got)
	}
	if got := clamp(0.5, 0, 1); got != 0.5 {
		t.Fatalf("clamp(mid) = %v", got)
	}

	if got := min(2, 5); got != 2 {
		t.Fatalf("min(2, 5) = %d", got)
	}
	if got := min(5, 2); got != 2 {
		t.Fatalf("min(5, 2) = %d", got)
	}

	base := gitrunner.NewCLIRunner(t.TempDir(), io.Discard)
	quiet, ok := quietRunner(base).(*gitrunner.CLIRunner)
	if !ok || quiet == nil {
		t.Fatalf("quietRunner() did not return CLI runner")
	}
	if quiet == base {
		t.Fatal("quietRunner() returned original runner pointer")
	}
	if quiet.RepoRoot() != base.RepoRoot() {
		t.Fatalf("quietRunner() repo root = %q, want %q", quiet.RepoRoot(), base.RepoRoot())
	}

	stub := &stubRunner{}
	if quietRunner(stub) != stub {
		t.Fatal("quietRunner() should keep non-CLI runners unchanged")
	}
}

func TestRunTaskNonInteractiveRunsDirectly(t *testing.T) {
	t.Parallel()

	term := Terminal{}
	runner := &stubRunner{}
	called := false
	got, err := RunTask(context.Background(), term, runner, TaskSpec{Title: "Test"}, func(_ context.Context, passed gitrunner.Runner) (string, error) {
		called = true
		if passed != runner {
			t.Fatalf("runner was wrapped in non-interactive mode")
		}
		return "ok", nil
	})
	if err != nil {
		t.Fatalf("RunTask() error = %v", err)
	}
	if !called || got != "ok" {
		t.Fatalf("RunTask() = %q, called=%v", got, called)
	}
}

func TestRunTaskInteractive(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	term := Terminal{
		out:         io.Discard,
		width:       80,
		styled:      true,
		interactive: true,
	}
	runner := &recordingTaskRunner{}

	got, err := RunTask(ctx, term, runner, TaskSpec{Title: "Sync", TotalOps: 1}, func(ctx context.Context, passed gitrunner.Runner) (string, error) {
		time.Sleep(220 * time.Millisecond)
		if _, err := passed.Run(ctx, "fetch", "--all"); err != nil {
			return "", err
		}
		return "ok", nil
	})
	if err != nil {
		t.Fatalf("RunTask() error = %v", err)
	}
	if got != "ok" {
		t.Fatalf("RunTask() = %q, want %q", got, "ok")
	}
	if len(runner.calls) != 1 || runner.calls[0] != "fetch --all" {
		t.Fatalf("runner calls = %#v", runner.calls)
	}
}

type stubRunner struct{}

type recordingTaskRunner struct {
	calls []string
}

var ansiPattern = regexp.MustCompile(`\x1b\[[0-9;]*m`)

func stripANSI(s string) string {
	return ansiPattern.ReplaceAllString(s, "")
}

func (s *stubRunner) Run(_ context.Context, _ ...string) (gitrunner.Result, error) {
	return gitrunner.Result{}, nil
}

func (s *stubRunner) RepoRoot() string {
	return ""
}

func (r *recordingTaskRunner) Run(_ context.Context, args ...string) (gitrunner.Result, error) {
	r.calls = append(r.calls, strings.Join(args, " "))
	return gitrunner.Result{}, nil
}

func (r *recordingTaskRunner) RepoRoot() string {
	return ""
}
