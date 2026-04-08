package rebaser

import (
	"context"
	"errors"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	gitrunner "github.com/lutefd/weaver/internal/git"
	"github.com/lutefd/weaver/internal/stack"
)

func TestSafeRebaserRebaseStack(t *testing.T) {
	t.Parallel()

	repoRoot := t.TempDir()
	rebaser := &SafeRebaser{
		runner: &recordingRunner{
			repoRoot: repoRoot,
			results: map[string]gitrunner.Result{
				"branch --show-current": {Stdout: "feature-c"},
			},
		},
		store: NewStateStore(repoRoot),
	}

	dag, err := stack.NewDAG([]stack.Dependency{
		{Branch: "feature-b", Parent: "feature-a"},
		{Branch: "feature-c", Parent: "feature-b"},
	})
	if err != nil {
		t.Fatalf("NewDAG() error = %v", err)
	}

	got, err := rebaser.RebaseStack(context.Background(), dag, []string{"feature-c"}, "main")
	if err != nil {
		t.Fatalf("RebaseStack() error = %v", err)
	}

	want := &RebaseResult{
		OriginalBranch: "feature-c",
		Completed:      []string{"feature-a", "feature-b", "feature-c"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("RebaseStack() = %#v, want %#v", got, want)
	}
	if rebaser.store.HasPending() {
		t.Fatal("HasPending() = true, want false")
	}

	calls := rebaser.runner.(*recordingRunner).calls
	wantCalls := []string{
		"branch --show-current",
		"checkout feature-a",
		"rebase --autostash main",
		"checkout feature-b",
		"rebase --autostash feature-a",
		"checkout feature-c",
		"rebase --autostash feature-b",
		"checkout feature-c",
	}
	if !reflect.DeepEqual(calls, wantCalls) {
		t.Fatalf("calls = %#v, want %#v", calls, wantCalls)
	}
}

func TestSafeRebaserConflictPersistsState(t *testing.T) {
	t.Parallel()

	repoRoot := t.TempDir()
	runner := &recordingRunner{
		repoRoot: repoRoot,
		results: map[string]gitrunner.Result{
			"branch --show-current":        {Stdout: "feature-c"},
			"rebase --autostash feature-a": {ExitCode: 1},
		},
		errs: map[string]error{
			"rebase --autostash feature-a": errors.New("exit status 1"),
		},
	}
	rebaser := &SafeRebaser{runner: runner, store: NewStateStore(repoRoot)}

	dag, err := stack.NewDAG([]stack.Dependency{
		{Branch: "feature-b", Parent: "feature-a"},
		{Branch: "feature-c", Parent: "feature-b"},
	})
	if err != nil {
		t.Fatalf("NewDAG() error = %v", err)
	}

	got, err := rebaser.RebaseStack(context.Background(), dag, []string{"feature-c"}, "main")
	if err != nil {
		t.Fatalf("RebaseStack() error = %v", err)
	}
	if !got.Conflict {
		t.Fatalf("Conflict = false, want true")
	}

	state, err := rebaser.store.Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if !reflect.DeepEqual(state.Completed, []string{"feature-a"}) {
		t.Fatalf("Completed = %#v, want [feature-a]", state.Completed)
	}
	if state.Current != "feature-b" || state.CurrentOnto != "feature-a" {
		t.Fatalf("state current = %s onto %s, want feature-b onto feature-a", state.Current, state.CurrentOnto)
	}
}

func TestSafeRebaserContinue(t *testing.T) {
	t.Parallel()

	repoRoot := t.TempDir()
	store := NewStateStore(repoRoot)
	if err := store.Save(&State{
		OriginalBranch: "feature-c",
		BaseBranch:     "main",
		AllBranches:    []string{"feature-a", "feature-b", "feature-c"},
		Completed:      []string{"feature-a"},
		Current:        "feature-b",
		CurrentOnto:    "feature-a",
	}); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	runner := &recordingRunner{repoRoot: repoRoot}
	rebaser := &SafeRebaser{runner: runner, store: store}

	got, err := rebaser.Continue(context.Background())
	if err != nil {
		t.Fatalf("Continue() error = %v", err)
	}

	want := &RebaseResult{
		OriginalBranch: "feature-c",
		Completed:      []string{"feature-a", "feature-b", "feature-c"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Continue() = %#v, want %#v", got, want)
	}
	if store.HasPending() {
		t.Fatal("HasPending() = true, want false")
	}
}

func TestSafeRebaserAbort(t *testing.T) {
	t.Parallel()

	repoRoot := t.TempDir()
	store := NewStateStore(repoRoot)
	if err := store.Save(&State{OriginalBranch: "feature-c"}); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	runner := &recordingRunner{repoRoot: repoRoot}
	rebaser := &SafeRebaser{runner: runner, store: store}

	if err := rebaser.Abort(context.Background()); err != nil {
		t.Fatalf("Abort() error = %v", err)
	}
	if store.HasPending() {
		t.Fatal("HasPending() = true, want false")
	}

	wantCalls := []string{"rebase --abort", "checkout feature-c"}
	if !reflect.DeepEqual(runner.calls, wantCalls) {
		t.Fatalf("calls = %#v, want %#v", runner.calls, wantCalls)
	}
}

type recordingRunner struct {
	repoRoot string
	results  map[string]gitrunner.Result
	errs     map[string]error
	calls    []string
}

func (r *recordingRunner) Run(_ context.Context, args ...string) (gitrunner.Result, error) {
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

func (r *recordingRunner) RepoRoot() string {
	return r.repoRoot
}

func TestResolveTargets(t *testing.T) {
	t.Parallel()

	dag, err := stack.NewDAG([]stack.Dependency{{Branch: "feature-b", Parent: "main"}})
	if err != nil {
		t.Fatalf("NewDAG() error = %v", err)
	}

	got, err := resolveTargets(dag, "feature-b", "main")
	if err != nil {
		t.Fatalf("resolveTargets() error = %v", err)
	}

	want := []string{"feature-b"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("resolveTargets() = %#v, want %#v", got, want)
	}
}

func TestStateStoreWritesInGitMetadata(t *testing.T) {
	t.Parallel()

	repoRoot := t.TempDir()
	store := NewStateStore(repoRoot)
	if got, want := store.path(), filepath.Join(repoRoot, ".git", "weaver", "rebase-state.yaml"); got != want {
		t.Fatalf("path() = %q, want %q", got, want)
	}
}
