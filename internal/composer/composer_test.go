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
		Persisted:  false,
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
		Persisted:      false,
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

func TestComposePersistsBaseBranchWhenRequested(t *testing.T) {
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
	got, err := New(runner).Compose(context.Background(), dag, []string{"feature-b"}, "integration", ComposeOpts{Persist: true})
	if err != nil {
		t.Fatalf("Compose() error = %v", err)
	}
	want := &ComposeResult{
		OriginalBranch: "topic",
		BaseBranch:     "integration",
		Order:          []string{"feature-a", "feature-b"},
		Persisted:      true,
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Compose() = %#v, want %#v", got, want)
	}

	wantCalls := []string{
		"branch --show-current",
		"checkout --detach integration",
		"merge --no-ff --no-edit feature-a",
		"merge --no-ff --no-edit feature-b",
		"branch -f integration HEAD",
		"checkout topic",
	}
	if !reflect.DeepEqual(runner.calls, wantCalls) {
		t.Fatalf("calls = %#v, want %#v", runner.calls, wantCalls)
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

	wantCalls := []string{
		"branch --show-current",
		"checkout --detach main",
		"merge --no-ff --no-edit feature-a",
		"merge --no-ff --no-edit feature-b",
		"merge --abort",
		"checkout topic",
	}
	if !reflect.DeepEqual(runner.calls, wantCalls) {
		t.Fatalf("calls = %#v, want %#v", runner.calls, wantCalls)
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
