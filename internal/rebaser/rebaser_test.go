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
				"rev-parse feature-a":   {Stdout: "sha-a"},
				"rev-parse feature-b":   {Stdout: "sha-b"},
				"rev-parse feature-c":   {Stdout: "sha-c"},
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
		"rev-parse feature-a",
		"rev-parse feature-b",
		"rev-parse feature-c",
		"checkout feature-a",
		"rebase --autostash main",
		"checkout feature-b",
		"rebase --autostash --onto feature-a sha-a",
		"checkout feature-c",
		"rebase --autostash --onto feature-b sha-b",
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
			"branch --show-current":                     {Stdout: "feature-c"},
			"rev-parse feature-a":                       {Stdout: "sha-a"},
			"rev-parse feature-b":                       {Stdout: "sha-b"},
			"rev-parse feature-c":                       {Stdout: "sha-c"},
			"rebase --autostash --onto feature-a sha-a": {ExitCode: 1},
		},
		errs: map[string]error{
			"rebase --autostash --onto feature-a sha-a": errors.New("exit status 1"),
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
	if state.OriginalTips["feature-a"] != "sha-a" {
		t.Fatalf("feature-a original tip = %q, want sha-a", state.OriginalTips["feature-a"])
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
		OriginalTips: map[string]string{
			"feature-a": "sha-a",
			"feature-b": "sha-b",
			"feature-c": "sha-c",
		},
		Completed:   []string{"feature-a"},
		Current:     "feature-b",
		CurrentOnto: "feature-a",
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
	wantCalls := []string{
		"rebase --continue",
		"checkout feature-c",
		"rebase --autostash --onto feature-b sha-b",
		"checkout feature-c",
	}
	if !reflect.DeepEqual(runner.calls, wantCalls) {
		t.Fatalf("calls = %#v, want %#v", runner.calls, wantCalls)
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

func TestNew(t *testing.T) {
	t.Parallel()

	repoRoot := t.TempDir()
	rebaser := New(&recordingRunner{repoRoot: repoRoot})
	if rebaser == nil {
		t.Fatal("New() = nil")
	}
	if got, want := rebaser.store.path(), filepath.Join(repoRoot, ".git", "weaver", "rebase-state.yaml"); got != want {
		t.Fatalf("store.path() = %q, want %q", got, want)
	}
}

func TestSafeRebaserRejectsInvalidStart(t *testing.T) {
	t.Parallel()

	repoRoot := t.TempDir()
	rebaser := &SafeRebaser{runner: &recordingRunner{repoRoot: repoRoot}, store: NewStateStore(repoRoot)}
	dag, err := stack.NewDAG([]stack.Dependency{{Branch: "feature-a", Parent: "main"}})
	if err != nil {
		t.Fatalf("NewDAG() error = %v", err)
	}

	if _, err := rebaser.RebaseStack(context.Background(), dag, nil, "main"); err == nil || err.Error() != "at least one branch is required" {
		t.Fatalf("RebaseStack() error = %v, want missing branch error", err)
	}

	if err := rebaser.store.Save(&State{}); err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	if _, err := rebaser.RebaseStack(context.Background(), dag, []string{"feature-a"}, "main"); err == nil || err.Error() != "a rebase is already in progress" {
		t.Fatalf("RebaseStack() error = %v, want pending rebase error", err)
	}
}

func TestSafeRebaserContinueRequiresCurrentBranch(t *testing.T) {
	t.Parallel()

	repoRoot := t.TempDir()
	store := NewStateStore(repoRoot)
	if err := store.Save(&State{OriginalBranch: "feature-c"}); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	_, err := (&SafeRebaser{runner: &recordingRunner{repoRoot: repoRoot}, store: store}).Continue(context.Background())
	if err == nil || err.Error() != "rebase state is missing the current branch" {
		t.Fatalf("Continue() error = %v, want missing current branch error", err)
	}
}

func TestSafeRebaserContinueConflictAndLoadError(t *testing.T) {
	t.Parallel()

	t.Run("conflict on rebase continue", func(t *testing.T) {
		t.Parallel()

		repoRoot := t.TempDir()
		store := NewStateStore(repoRoot)
		if err := store.Save(&State{
			OriginalBranch: "feature-c",
			BaseBranch:     "main",
			AllBranches:    []string{"feature-a", "feature-b"},
			OriginalTips: map[string]string{
				"feature-a": "sha-a",
				"feature-b": "sha-b",
			},
			Completed:   []string{"feature-a"},
			Current:     "feature-b",
			CurrentOnto: "feature-a",
		}); err != nil {
			t.Fatalf("Save() error = %v", err)
		}

		runner := &recordingRunner{
			repoRoot: repoRoot,
			results: map[string]gitrunner.Result{
				"rebase --continue": {ExitCode: 1},
			},
			errs: map[string]error{
				"rebase --continue": errors.New("exit status 1"),
			},
		}
		got, err := (&SafeRebaser{runner: runner, store: store}).Continue(context.Background())
		if err != nil {
			t.Fatalf("Continue() error = %v", err)
		}
		want := &RebaseResult{
			OriginalBranch: "feature-c",
			Completed:      []string{"feature-a"},
			Current:        "feature-b",
			CurrentOnto:    "feature-a",
			Conflict:       true,
		}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("Continue() = %#v, want %#v", got, want)
		}
	})

	t.Run("load error", func(t *testing.T) {
		t.Parallel()

		_, err := (&SafeRebaser{
			runner: &recordingRunner{repoRoot: t.TempDir()},
			store:  NewStateStore(t.TempDir()),
		}).Continue(context.Background())
		if err == nil || !strings.Contains(err.Error(), "load rebase state") {
			t.Fatalf("Continue() error = %v, want load rebase state error", err)
		}
	})
}

func TestSafeRebaserAbortIgnoresMissingRebase(t *testing.T) {
	t.Parallel()

	repoRoot := t.TempDir()
	store := NewStateStore(repoRoot)
	if err := store.Save(&State{OriginalBranch: "feature-c"}); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	runner := &recordingRunner{
		repoRoot: repoRoot,
		errs: map[string]error{
			"rebase --abort": errors.New("No rebase in progress"),
		},
	}
	if err := (&SafeRebaser{runner: runner, store: store}).Abort(context.Background()); err != nil {
		t.Fatalf("Abort() error = %v", err)
	}
}

func TestSafeRebaserAbortErrors(t *testing.T) {
	t.Parallel()

	t.Run("load error", func(t *testing.T) {
		t.Parallel()

		err := (&SafeRebaser{
			runner: &recordingRunner{repoRoot: t.TempDir()},
			store:  NewStateStore(t.TempDir()),
		}).Abort(context.Background())
		if err == nil || !strings.Contains(err.Error(), "load rebase state") {
			t.Fatalf("Abort() error = %v, want load rebase state error", err)
		}
	})

	t.Run("rebase abort error", func(t *testing.T) {
		t.Parallel()

		repoRoot := t.TempDir()
		store := NewStateStore(repoRoot)
		if err := store.Save(&State{OriginalBranch: "feature-c"}); err != nil {
			t.Fatalf("Save() error = %v", err)
		}

		err := (&SafeRebaser{
			runner: &recordingRunner{
				repoRoot: repoRoot,
				errs: map[string]error{
					"rebase --abort": errors.New("boom"),
				},
			},
			store: store,
		}).Abort(context.Background())
		if err == nil || err.Error() != "boom" {
			t.Fatalf("Abort() error = %v, want boom", err)
		}
	})

	t.Run("checkout error", func(t *testing.T) {
		t.Parallel()

		repoRoot := t.TempDir()
		store := NewStateStore(repoRoot)
		if err := store.Save(&State{OriginalBranch: "feature-c"}); err != nil {
			t.Fatalf("Save() error = %v", err)
		}

		err := (&SafeRebaser{
			runner: &recordingRunner{
				repoRoot: repoRoot,
				errs: map[string]error{
					"checkout feature-c": errors.New("checkout failed"),
				},
			},
			store: store,
		}).Abort(context.Background())
		if err == nil || err.Error() != "checkout failed" {
			t.Fatalf("Abort() error = %v, want checkout failed", err)
		}
	})
}

func TestBranchTipsAndCurrentBranchErrors(t *testing.T) {
	t.Parallel()

	got, err := branchTips(context.Background(), &recordingRunner{
		results: map[string]gitrunner.Result{
			"rev-parse feature-a": {Stdout: "sha-a"},
		},
	}, []string{"feature-a"})
	if err != nil {
		t.Fatalf("branchTips() error = %v", err)
	}
	if !reflect.DeepEqual(got, map[string]string{"feature-a": "sha-a"}) {
		t.Fatalf("branchTips() = %#v", got)
	}

	_, err = branchTips(context.Background(), &recordingRunner{}, []string{"feature-a"})
	if err == nil || err.Error() != "resolve branch tip for feature-a: empty revision" {
		t.Fatalf("branchTips() error = %v, want empty revision error", err)
	}

	_, err = branchTips(context.Background(), &recordingRunner{
		errs: map[string]error{"rev-parse feature-a": errors.New("boom")},
	}, []string{"feature-a"})
	if err == nil || err.Error() != "resolve branch tip for feature-a: boom" {
		t.Fatalf("branchTips() error = %v, want wrapped runner error", err)
	}

	_, err = currentBranch(context.Background(), &recordingRunner{
		errs: map[string]error{"branch --show-current": errors.New("boom")},
	})
	if err == nil || err.Error() != "resolve current branch: boom" {
		t.Fatalf("currentBranch() error = %v, want wrapped error", err)
	}

	_, err = currentBranch(context.Background(), &recordingRunner{})
	if err == nil || err.Error() != "resolve current branch: empty branch name" {
		t.Fatalf("currentBranch() error = %v, want empty branch error", err)
	}
}

func TestRebaseArgsForIndexAndIsNoRebaseInProgress(t *testing.T) {
	t.Parallel()

	args, err := rebaseArgsForIndex(&State{}, 0, "main")
	if err != nil {
		t.Fatalf("rebaseArgsForIndex() error = %v", err)
	}
	if !reflect.DeepEqual(args, []string{"rebase", "--autostash", "main"}) {
		t.Fatalf("rebaseArgsForIndex() = %#v", args)
	}

	_, err = rebaseArgsForIndex(&State{
		AllBranches: []string{"feature-a", "feature-b"},
		OriginalTips: map[string]string{
			"feature-b": "sha-b",
		},
	}, 1, "feature-a")
	if err == nil || err.Error() != "missing original tip for feature-a" {
		t.Fatalf("rebaseArgsForIndex() error = %v, want missing tip error", err)
	}

	if !isNoRebaseInProgress(errors.New("No rebase in progress")) {
		t.Fatal("isNoRebaseInProgress() = false, want true")
	}
	if isNoRebaseInProgress(errors.New("boom")) {
		t.Fatal("isNoRebaseInProgress() = true, want false")
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
