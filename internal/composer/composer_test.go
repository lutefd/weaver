package composer

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	gitrunner "github.com/lutefd/weaver/internal/git"
	"github.com/lutefd/weaver/internal/stack"
)

func TestResolveComposeOrder(t *testing.T) {
	t.Parallel()

	dag, err := stack.NewDAG([]stack.Dependency{
		{Branch: "feature-b", Parent: "feature-a"},
		{Branch: "feature-c", Parent: "feature-b"},
		{Branch: "feature-e", Parent: "feature-d"},
	})
	if err != nil {
		t.Fatalf("NewDAG() error = %v", err)
	}

	got, err := resolveComposeOrder(dag, []string{"feature-c", "feature-e"}, "main")
	if err != nil {
		t.Fatalf("resolveComposeOrder() error = %v", err)
	}

	want := []string{"feature-a", "feature-b", "feature-c", "feature-d", "feature-e"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("resolveComposeOrder() = %#v, want %#v", got, want)
	}
}

func TestComposeDryRun(t *testing.T) {
	t.Parallel()

	dag, err := stack.NewDAG([]stack.Dependency{{Branch: "feature-b", Parent: "feature-a"}})
	if err != nil {
		t.Fatalf("NewDAG() error = %v", err)
	}

	runner := &composeRunner{}
	got, err := New(runner).Compose(context.Background(), dag, []string{"feature-b"}, "main", ComposeOpts{DryRun: true})
	if err != nil {
		t.Fatalf("Compose() error = %v", err)
	}
	want := &ComposeResult{
		BaseBranch: "main",
		Order:      []string{"feature-a", "feature-b"},
		DryRun:     true,
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Compose() = %#v, want %#v", got, want)
	}
	if len(runner.calls) != 0 {
		t.Fatalf("calls = %#v, want none", runner.calls)
	}
}

func TestComposeRunsMergesAndRestoresBranch(t *testing.T) {
	t.Parallel()

	dag, err := stack.NewDAG([]stack.Dependency{{Branch: "feature-b", Parent: "feature-a"}})
	if err != nil {
		t.Fatalf("NewDAG() error = %v", err)
	}

	runner := &composeRunner{
		results: map[string]gitrunner.Result{
			"branch --show-current": {Stdout: "topic"},
		},
	}
	got, err := New(runner).Compose(context.Background(), dag, []string{"feature-b"}, "main", ComposeOpts{})
	if err != nil {
		t.Fatalf("Compose() error = %v", err)
	}
	want := &ComposeResult{
		OriginalBranch: "topic",
		BaseBranch:     "main",
		Order:          []string{"feature-a", "feature-b"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Compose() = %#v, want %#v", got, want)
	}

	wantCalls := []string{
		"branch --show-current",
		"checkout --detach main",
		"merge --no-ff --no-edit feature-a",
		"merge --no-ff --no-edit feature-b",
		"checkout topic",
	}
	if !reflect.DeepEqual(runner.calls, wantCalls) {
		t.Fatalf("calls = %#v, want %#v", runner.calls, wantCalls)
	}
}

func TestComposeCreatesNewBranchWhenRequested(t *testing.T) {
	t.Parallel()

	dag, err := stack.NewDAG([]stack.Dependency{{Branch: "feature-b", Parent: "feature-a"}})
	if err != nil {
		t.Fatalf("NewDAG() error = %v", err)
	}

	runner := &composeRunner{
		results: map[string]gitrunner.Result{
			"branch --show-current": {Stdout: "topic"},
		},
	}
	got, err := New(runner).Compose(context.Background(), dag, []string{"feature-b"}, "main", ComposeOpts{CreateBranch: "integration"})
	if err != nil {
		t.Fatalf("Compose() error = %v", err)
	}
	want := &ComposeResult{
		OriginalBranch: "topic",
		BaseBranch:     "main",
		Order:          []string{"feature-a", "feature-b"},
		CreatedBranch:  "integration",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Compose() = %#v, want %#v", got, want)
	}

	wantCalls := []string{
		"branch --show-current",
		"checkout --detach main",
		"merge --no-ff --no-edit feature-a",
		"merge --no-ff --no-edit feature-b",
		"branch integration HEAD",
		"checkout topic",
	}
	if !reflect.DeepEqual(runner.calls, wantCalls) {
		t.Fatalf("calls = %#v, want %#v", runner.calls, wantCalls)
	}
}

func TestComposeUpdatesBranchWhenRequested(t *testing.T) {
	t.Parallel()

	dag, err := stack.NewDAG([]stack.Dependency{{Branch: "feature-b", Parent: "feature-a"}})
	if err != nil {
		t.Fatalf("NewDAG() error = %v", err)
	}

	runner := &composeRunner{
		results: map[string]gitrunner.Result{
			"branch --show-current": {Stdout: "topic"},
		},
	}
	got, err := New(runner).Compose(context.Background(), dag, []string{"feature-b"}, "main", ComposeOpts{UpdateBranch: "integration"})
	if err != nil {
		t.Fatalf("Compose() error = %v", err)
	}
	want := &ComposeResult{
		OriginalBranch: "topic",
		BaseBranch:     "main",
		Order:          []string{"feature-a", "feature-b"},
		UpdatedBranch:  "integration",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Compose() = %#v, want %#v", got, want)
	}

	wantCalls := []string{
		"branch --show-current",
		"checkout --detach main",
		"merge --no-ff --no-edit feature-a",
		"merge --no-ff --no-edit feature-b",
		"branch -f integration HEAD",
		"checkout topic",
	}
	if !reflect.DeepEqual(runner.calls, wantCalls) {
		t.Fatalf("calls = %#v, want %#v", runner.calls, wantCalls)
	}
}

func TestComposeRejectsCombinedWriteModes(t *testing.T) {
	t.Parallel()

	dag, err := stack.NewDAG([]stack.Dependency{{Branch: "feature-a", Parent: "main"}})
	if err != nil {
		t.Fatalf("NewDAG() error = %v", err)
	}

	_, err = New(&composeRunner{}).Compose(context.Background(), dag, []string{"feature-a"}, "main", ComposeOpts{
		CreateBranch: "integration",
		UpdateBranch: "integration-preview",
	})
	if err == nil {
		t.Fatal("Compose() error = nil, want error")
	}
	if !strings.Contains(err.Error(), "cannot combine create branch and update branch modes") {
		t.Fatalf("Compose() error = %v, want write mode validation", err)
	}
}

func TestComposeSkipsRequestedBranches(t *testing.T) {
	t.Parallel()

	dag, err := stack.NewDAG([]stack.Dependency{
		{Branch: "feature-b", Parent: "feature-a"},
		{Branch: "feature-c", Parent: "feature-b"},
	})
	if err != nil {
		t.Fatalf("NewDAG() error = %v", err)
	}

	got, err := New(&composeRunner{}).Compose(context.Background(), dag, []string{"feature-c"}, "main", ComposeOpts{
		DryRun:       true,
		SkipBranches: []string{"feature-b"},
	})
	if err != nil {
		t.Fatalf("Compose() error = %v", err)
	}
	want := &ComposeResult{
		BaseBranch: "main",
		Order:      []string{"feature-a", "feature-c"},
		Skipped:    []string{"feature-b"},
		DryRun:     true,
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Compose() = %#v, want %#v", got, want)
	}
}

func TestComposeConflictAbortsAndRestoresBranch(t *testing.T) {
	t.Parallel()

	dag, err := stack.NewDAG([]stack.Dependency{{Branch: "feature-b", Parent: "feature-a"}})
	if err != nil {
		t.Fatalf("NewDAG() error = %v", err)
	}

	runner := &composeRunner{
		results: map[string]gitrunner.Result{
			"branch --show-current":             {Stdout: "topic"},
			"merge --no-ff --no-edit feature-b": {ExitCode: 1},
			"diff --name-only --diff-filter=U":  {Stdout: "app/service.go\napp/ui.tsx"},
		},
		errs: map[string]error{
			"merge --no-ff --no-edit feature-b": errors.New("exit status 1"),
		},
	}
	_, err = New(runner).Compose(context.Background(), dag, []string{"feature-b"}, "main", ComposeOpts{})
	var conflictErr ConflictError
	if !errors.As(err, &conflictErr) {
		t.Fatalf("Compose() error = %v, want ConflictError", err)
	}
	if conflictErr.Branch != "feature-b" {
		t.Fatalf("ConflictError.Branch = %q, want feature-b", conflictErr.Branch)
	}
	if !reflect.DeepEqual(conflictErr.Files, []string{"app/service.go", "app/ui.tsx"}) {
		t.Fatalf("ConflictError.Files = %#v, want conflict file list", conflictErr.Files)
	}

	wantCalls := []string{
		"branch --show-current",
		"checkout --detach main",
		"merge --no-ff --no-edit feature-a",
		"merge --no-ff --no-edit feature-b",
		"diff --name-only --diff-filter=U",
		"merge --abort",
		"checkout topic",
	}
	if !reflect.DeepEqual(runner.calls, wantCalls) {
		t.Fatalf("calls = %#v, want %#v", runner.calls, wantCalls)
	}
}

func TestConflictError(t *testing.T) {
	t.Parallel()

	err := ConflictError{
		Branch: "feature-a",
		Files:  []string{"app.go"},
		Err:    errors.New("exit status 1"),
	}

	if got := err.Error(); got != "compose conflict on feature-a: exit status 1" {
		t.Fatalf("Error() = %q", got)
	}
	if !errors.Is(err, err.Err) {
		t.Fatalf("Unwrap() did not expose inner error")
	}
}

func TestResolveOrder(t *testing.T) {
	t.Parallel()

	dag, err := stack.NewDAG([]stack.Dependency{{Branch: "feature-b", Parent: "feature-a"}})
	if err != nil {
		t.Fatalf("NewDAG() error = %v", err)
	}

	got, err := ResolveOrder(dag, []string{"feature-b"}, "main")
	if err != nil {
		t.Fatalf("ResolveOrder() error = %v", err)
	}
	want := []string{"feature-a", "feature-b"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ResolveOrder() = %#v, want %#v", got, want)
	}
}

func TestCurrentBranch(t *testing.T) {
	t.Parallel()

	t.Run("ok", func(t *testing.T) {
		got, err := currentBranch(context.Background(), &composeRunner{
			results: map[string]gitrunner.Result{
				"branch --show-current": {Stdout: "topic"},
			},
		})
		if err != nil {
			t.Fatalf("currentBranch() error = %v", err)
		}
		if got != "topic" {
			t.Fatalf("currentBranch() = %q, want %q", got, "topic")
		}
	})

	t.Run("runner error", func(t *testing.T) {
		_, err := currentBranch(context.Background(), &composeRunner{
			errs: map[string]error{"branch --show-current": errors.New("boom")},
		})
		if err == nil || err.Error() != "resolve current branch: boom" {
			t.Fatalf("currentBranch() error = %v, want wrapped runner error", err)
		}
	})

	t.Run("empty branch", func(t *testing.T) {
		_, err := currentBranch(context.Background(), &composeRunner{})
		if err == nil || err.Error() != "resolve current branch: empty branch name" {
			t.Fatalf("currentBranch() error = %v, want empty branch error", err)
		}
	})
}

func TestAbortMerge(t *testing.T) {
	t.Parallel()

	if err := abortMerge(context.Background(), &composeRunner{
		errs: map[string]error{"merge --abort": errors.New("MERGE_HEAD missing")},
	}); err != nil {
		t.Fatalf("abortMerge() error = %v", err)
	}

	err := abortMerge(context.Background(), &composeRunner{
		errs: map[string]error{"merge --abort": errors.New("boom")},
	})
	if err == nil || err.Error() != "boom" {
		t.Fatalf("abortMerge() error = %v, want hard failure", err)
	}
}

func TestConflictedFiles(t *testing.T) {
	t.Parallel()

	got, err := conflictedFiles(context.Background(), &composeRunner{
		results: map[string]gitrunner.Result{
			"diff --name-only --diff-filter=U": {Stdout: "app.go\nweb.ts"},
		},
	})
	if err != nil {
		t.Fatalf("conflictedFiles() error = %v", err)
	}
	if !reflect.DeepEqual(got, []string{"app.go", "web.ts"}) {
		t.Fatalf("conflictedFiles() = %#v", got)
	}

	got, err = conflictedFiles(context.Background(), &composeRunner{})
	if err != nil {
		t.Fatalf("conflictedFiles() error = %v", err)
	}
	if got != nil {
		t.Fatalf("conflictedFiles() = %#v, want nil", got)
	}
}

type composeRunner struct {
	results map[string]gitrunner.Result
	errs    map[string]error
	calls   []string
}

func (r *composeRunner) Run(_ context.Context, args ...string) (gitrunner.Result, error) {
	key := strings.Join(args, " ")
	r.calls = append(r.calls, key)
	if result, ok := r.results[key]; ok {
		if err, hasErr := r.errs[key]; hasErr {
			return result, err
		}
		return result, nil
	}
	if err, ok := r.errs[key]; ok {
		return gitrunner.Result{ExitCode: 1}, err
	}
	return gitrunner.Result{}, nil
}

func (r *composeRunner) RepoRoot() string {
	return ""
}
