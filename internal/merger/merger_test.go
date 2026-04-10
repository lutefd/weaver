package merger

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

func TestSafeMergerMergeStack(t *testing.T) {
	t.Parallel()

	repoRoot := t.TempDir()
	merger := &SafeMerger{
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

	got, err := merger.MergeStack(context.Background(), dag, []string{"feature-c"}, "main")
	if err != nil {
		t.Fatalf("MergeStack() error = %v", err)
	}

	want := &MergeResult{
		OriginalBranch: "feature-c",
		Completed:      []string{"feature-a", "feature-b", "feature-c"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("MergeStack() = %#v, want %#v", got, want)
	}
	if merger.store.HasPending() {
		t.Fatal("HasPending() = true, want false")
	}

	wantCalls := []string{
		"branch --show-current",
		"checkout feature-a",
		"merge --no-edit main",
		"checkout feature-b",
		"merge --no-edit feature-a",
		"checkout feature-c",
		"merge --no-edit feature-b",
		"checkout feature-c",
	}
	if !reflect.DeepEqual(merger.runner.(*recordingRunner).calls, wantCalls) {
		t.Fatalf("calls = %#v, want %#v", merger.runner.(*recordingRunner).calls, wantCalls)
	}
}

func TestSafeMergerConflictPersistsState(t *testing.T) {
	t.Parallel()

	repoRoot := t.TempDir()
	runner := &recordingRunner{
		repoRoot: repoRoot,
		results: map[string]gitrunner.Result{
			"branch --show-current":     {Stdout: "feature-c"},
			"merge --no-edit feature-a": {ExitCode: 1},
		},
		errs: map[string]error{
			"merge --no-edit feature-a": errors.New("exit status 1"),
		},
	}
	merger := &SafeMerger{runner: runner, store: NewStateStore(repoRoot)}

	dag, err := stack.NewDAG([]stack.Dependency{
		{Branch: "feature-b", Parent: "feature-a"},
		{Branch: "feature-c", Parent: "feature-b"},
	})
	if err != nil {
		t.Fatalf("NewDAG() error = %v", err)
	}

	got, err := merger.MergeStack(context.Background(), dag, []string{"feature-c"}, "main")
	if err != nil {
		t.Fatalf("MergeStack() error = %v", err)
	}
	if !got.Conflict {
		t.Fatalf("Conflict = false, want true")
	}

	state, err := merger.store.Load()
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

func TestSafeMergerContinue(t *testing.T) {
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
	merger := &SafeMerger{runner: runner, store: store}

	got, err := merger.Continue(context.Background())
	if err != nil {
		t.Fatalf("Continue() error = %v", err)
	}

	want := &MergeResult{
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
		"merge --continue",
		"checkout feature-c",
		"merge --no-edit feature-b",
		"checkout feature-c",
	}
	if !reflect.DeepEqual(runner.calls, wantCalls) {
		t.Fatalf("calls = %#v, want %#v", runner.calls, wantCalls)
	}
}

func TestSafeMergerAbort(t *testing.T) {
	t.Parallel()

	repoRoot := t.TempDir()
	store := NewStateStore(repoRoot)
	if err := store.Save(&State{OriginalBranch: "feature-c"}); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	runner := &recordingRunner{repoRoot: repoRoot}
	merger := &SafeMerger{runner: runner, store: store}

	if err := merger.Abort(context.Background()); err != nil {
		t.Fatalf("Abort() error = %v", err)
	}
	if store.HasPending() {
		t.Fatal("HasPending() = true, want false")
	}

	wantCalls := []string{"merge --abort", "checkout feature-c"}
	if !reflect.DeepEqual(runner.calls, wantCalls) {
		t.Fatalf("calls = %#v, want %#v", runner.calls, wantCalls)
	}
}

func TestNew(t *testing.T) {
	t.Parallel()

	repoRoot := t.TempDir()
	merger := New(&recordingRunner{repoRoot: repoRoot})
	if merger == nil {
		t.Fatal("New() = nil")
	}
	if got, want := merger.store.path(), filepath.Join(repoRoot, ".git", "weaver", "merge-state.yaml"); got != want {
		t.Fatalf("store.path() = %q, want %q", got, want)
	}
}

func TestSafeMergerRejectsInvalidStart(t *testing.T) {
	t.Parallel()

	repoRoot := t.TempDir()
	merger := &SafeMerger{runner: &recordingRunner{repoRoot: repoRoot}, store: NewStateStore(repoRoot)}
	dag, err := stack.NewDAG([]stack.Dependency{{Branch: "feature-a", Parent: "main"}})
	if err != nil {
		t.Fatalf("NewDAG() error = %v", err)
	}

	if _, err := merger.MergeStack(context.Background(), dag, nil, "main"); err == nil || err.Error() != "at least one branch is required" {
		t.Fatalf("MergeStack() error = %v, want missing branch error", err)
	}

	if err := merger.store.Save(&State{}); err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	if _, err := merger.MergeStack(context.Background(), dag, []string{"feature-a"}, "main"); err == nil || err.Error() != "a merge sync is already in progress" {
		t.Fatalf("MergeStack() error = %v, want pending merge error", err)
	}
}

func TestSafeMergerContinueRequiresCurrentBranch(t *testing.T) {
	t.Parallel()

	repoRoot := t.TempDir()
	store := NewStateStore(repoRoot)
	if err := store.Save(&State{OriginalBranch: "feature-c"}); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	_, err := (&SafeMerger{runner: &recordingRunner{repoRoot: repoRoot}, store: store}).Continue(context.Background())
	if err == nil || err.Error() != "merge state is missing the current branch" {
		t.Fatalf("Continue() error = %v, want missing current branch error", err)
	}
}

func TestSafeMergerContinueConflictAndLoadError(t *testing.T) {
	t.Parallel()

	t.Run("conflict on merge continue", func(t *testing.T) {
		t.Parallel()

		repoRoot := t.TempDir()
		store := NewStateStore(repoRoot)
		if err := store.Save(&State{
			OriginalBranch: "feature-c",
			BaseBranch:     "main",
			AllBranches:    []string{"feature-a", "feature-b"},
			Completed:      []string{"feature-a"},
			Current:        "feature-b",
			CurrentOnto:    "feature-a",
		}); err != nil {
			t.Fatalf("Save() error = %v", err)
		}

		runner := &recordingRunner{
			repoRoot: repoRoot,
			results: map[string]gitrunner.Result{
				"merge --continue": {ExitCode: 1},
			},
			errs: map[string]error{
				"merge --continue": errors.New("exit status 1"),
			},
		}
		got, err := (&SafeMerger{runner: runner, store: store}).Continue(context.Background())
		if err != nil {
			t.Fatalf("Continue() error = %v", err)
		}
		want := &MergeResult{
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

		_, err := (&SafeMerger{
			runner: &recordingRunner{repoRoot: t.TempDir()},
			store:  NewStateStore(t.TempDir()),
		}).Continue(context.Background())
		if err == nil || !strings.Contains(err.Error(), "load merge state") {
			t.Fatalf("Continue() error = %v, want load merge state error", err)
		}
	})
}

func TestSafeMergerAbortIgnoresMissingMerge(t *testing.T) {
	t.Parallel()

	repoRoot := t.TempDir()
	store := NewStateStore(repoRoot)
	if err := store.Save(&State{OriginalBranch: "feature-c"}); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	runner := &recordingRunner{
		repoRoot: repoRoot,
		errs: map[string]error{
			"merge --abort": errors.New("There is no merge to abort"),
		},
	}
	if err := (&SafeMerger{runner: runner, store: store}).Abort(context.Background()); err != nil {
		t.Fatalf("Abort() error = %v", err)
	}
}

func TestSafeMergerAbortErrors(t *testing.T) {
	t.Parallel()

	t.Run("load error", func(t *testing.T) {
		t.Parallel()

		err := (&SafeMerger{
			runner: &recordingRunner{repoRoot: t.TempDir()},
			store:  NewStateStore(t.TempDir()),
		}).Abort(context.Background())
		if err == nil || !strings.Contains(err.Error(), "load merge state") {
			t.Fatalf("Abort() error = %v, want load merge state error", err)
		}
	})

	t.Run("merge abort error", func(t *testing.T) {
		t.Parallel()

		repoRoot := t.TempDir()
		store := NewStateStore(repoRoot)
		if err := store.Save(&State{OriginalBranch: "feature-c"}); err != nil {
			t.Fatalf("Save() error = %v", err)
		}

		err := (&SafeMerger{
			runner: &recordingRunner{
				repoRoot: repoRoot,
				errs: map[string]error{
					"merge --abort": errors.New("boom"),
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

		err := (&SafeMerger{
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

func TestResolveTargetsAndCurrentBranchErrors(t *testing.T) {
	t.Parallel()

	dag, err := stack.NewDAG([]stack.Dependency{{Branch: "feature-a", Parent: "main"}})
	if err != nil {
		t.Fatalf("NewDAG() error = %v", err)
	}
	got, err := resolveTargets(dag, "feature-a", "develop")
	if err != nil {
		t.Fatalf("resolveTargets() error = %v", err)
	}
	if !reflect.DeepEqual(got, []string{"main", "feature-a"}) {
		t.Fatalf("resolveTargets() = %#v", got)
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

	got, err = resolveTargets(dag, "feature-a", "main")
	if err != nil {
		t.Fatalf("resolveTargets() error = %v", err)
	}
	if !reflect.DeepEqual(got, []string{"feature-a"}) {
		t.Fatalf("resolveTargets() = %#v, want [feature-a]", got)
	}
}

func TestIsNoMergeInProgress(t *testing.T) {
	t.Parallel()

	if !isNoMergeInProgress(errors.New("There is no merge to abort")) {
		t.Fatal("isNoMergeInProgress() = false, want true for standard message")
	}
	if !isNoMergeInProgress(errors.New("MERGE_HEAD missing")) {
		t.Fatal("isNoMergeInProgress() = false, want true for missing MERGE_HEAD")
	}
	if isNoMergeInProgress(errors.New("boom")) {
		t.Fatal("isNoMergeInProgress() = true, want false")
	}
}

func TestStateStoreWritesInGitMetadata(t *testing.T) {
	t.Parallel()

	repoRoot := t.TempDir()
	store := NewStateStore(repoRoot)
	if got, want := store.path(), filepath.Join(repoRoot, ".git", "weaver", "merge-state.yaml"); got != want {
		t.Fatalf("path() = %q, want %q", got, want)
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
