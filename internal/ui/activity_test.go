package ui

import (
	"context"
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

type stubRunner struct{}

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
