package rebaser

import (
	"context"
	"fmt"
	"strings"
	"time"

	gitrunner "github.com/lutefd/weaver/internal/git"
	"github.com/lutefd/weaver/internal/stack"
)

type Rebaser interface {
	RebaseStack(ctx context.Context, dag *stack.DAG, branches []string, base string) (*RebaseResult, error)
	Continue(ctx context.Context) (*RebaseResult, error)
	Abort(ctx context.Context) error
	HasPending() bool
}

type RebaseResult struct {
	OriginalBranch string
	Completed      []string
	Current        string
	CurrentOnto    string
	Conflict       bool
}

type SafeRebaser struct {
	runner gitrunner.Runner
	store  *StateStore
}

func New(runner gitrunner.Runner) *SafeRebaser {
	return &SafeRebaser{
		runner: runner,
		store:  NewStateStore(runner.RepoRoot()),
	}
}

func (r *SafeRebaser) RebaseStack(ctx context.Context, dag *stack.DAG, branches []string, base string) (*RebaseResult, error) {
	if len(branches) == 0 {
		return nil, fmt.Errorf("at least one branch is required")
	}
	if r.HasPending() {
		return nil, fmt.Errorf("a rebase is already in progress")
	}

	originalBranch, err := currentBranch(ctx, r.runner)
	if err != nil {
		return nil, err
	}

	targets, err := resolveTargets(dag, branches[0], base)
	if err != nil {
		return nil, err
	}

	state := &State{
		StartedAt:      time.Now().UTC(),
		OriginalBranch: originalBranch,
		BaseBranch:     base,
		AllBranches:    targets,
	}
	originalTips, err := branchTips(ctx, r.runner, targets)
	if err != nil {
		return nil, err
	}
	state.OriginalTips = originalTips
	if err := r.store.Save(state); err != nil {
		return nil, err
	}

	result, err := r.runFrom(ctx, state, 0, false)
	if err != nil {
		return nil, err
	}

	return result, nil
}

func (r *SafeRebaser) Continue(ctx context.Context) (*RebaseResult, error) {
	state, err := r.store.Load()
	if err != nil {
		return nil, fmt.Errorf("load rebase state: %w", err)
	}

	result, err := r.runFrom(ctx, state, len(state.Completed), true)
	if err != nil {
		return nil, err
	}

	return result, nil
}

func (r *SafeRebaser) Abort(ctx context.Context) error {
	state, err := r.store.Load()
	if err != nil {
		return fmt.Errorf("load rebase state: %w", err)
	}

	if _, err := r.runner.Run(ctx, "rebase", "--abort"); err != nil {
		if !isNoRebaseInProgress(err) {
			return err
		}
	}
	if _, err := r.runner.Run(ctx, "checkout", state.OriginalBranch); err != nil {
		return err
	}

	return r.store.Clear()
}

func (r *SafeRebaser) HasPending() bool {
	return r.store.HasPending()
}

func (r *SafeRebaser) runFrom(ctx context.Context, state *State, start int, continuing bool) (*RebaseResult, error) {
	completed := append([]string(nil), state.Completed...)
	if continuing {
		if state.Current == "" {
			return nil, fmt.Errorf("rebase state is missing the current branch")
		}
		if err := r.store.Save(state); err != nil {
			return nil, err
		}
		if _, err := r.runner.Run(ctx, "rebase", "--continue"); err != nil {
			return &RebaseResult{
				OriginalBranch: state.OriginalBranch,
				Completed:      completed,
				Current:        state.Current,
				CurrentOnto:    state.CurrentOnto,
				Conflict:       true,
			}, nil
		}
		completed = append(completed, state.Current)
		state.Completed = completed
		start++
	}

	for idx := start; idx < len(state.AllBranches); idx++ {
		branch := state.AllBranches[idx]
		onto := state.BaseBranch
		if idx > 0 {
			onto = state.AllBranches[idx-1]
		}

		state.Current = branch
		state.CurrentOnto = onto
		if err := r.store.Save(state); err != nil {
			return nil, err
		}

		if _, err := r.runner.Run(ctx, "checkout", branch); err != nil {
			return nil, err
		}
		rebaseArgs, err := rebaseArgsForIndex(state, idx, onto)
		if err != nil {
			return nil, err
		}
		if _, err := r.runner.Run(ctx, rebaseArgs...); err != nil {
			return &RebaseResult{
				OriginalBranch: state.OriginalBranch,
				Completed:      append([]string(nil), completed...),
				Current:        branch,
				CurrentOnto:    onto,
				Conflict:       true,
			}, nil
		}

		completed = append(completed, branch)
		state.Completed = completed
	}

	if _, err := r.runner.Run(ctx, "checkout", state.OriginalBranch); err != nil {
		return nil, err
	}
	if err := r.store.Clear(); err != nil {
		return nil, err
	}

	return &RebaseResult{
		OriginalBranch: state.OriginalBranch,
		Completed:      append([]string(nil), completed...),
	}, nil
}

func currentBranch(ctx context.Context, runner gitrunner.Runner) (string, error) {
	result, err := runner.Run(ctx, "branch", "--show-current")
	if err != nil {
		return "", fmt.Errorf("resolve current branch: %w", err)
	}
	if result.Stdout == "" {
		return "", fmt.Errorf("resolve current branch: empty branch name")
	}
	return result.Stdout, nil
}

func resolveTargets(dag *stack.DAG, branch, base string) ([]string, error) {
	chain, err := dag.Ancestors(branch)
	if err != nil {
		return nil, err
	}

	if len(chain) > 0 && chain[0] == base {
		return append([]string(nil), chain[1:]...), nil
	}
	return chain, nil
}

func isNoRebaseInProgress(err error) bool {
	return strings.Contains(err.Error(), "No rebase in progress")
}

func branchTips(ctx context.Context, runner gitrunner.Runner, branches []string) (map[string]string, error) {
	tips := make(map[string]string, len(branches))
	for _, branch := range branches {
		result, err := runner.Run(ctx, "rev-parse", branch)
		if err != nil {
			return nil, fmt.Errorf("resolve branch tip for %s: %w", branch, err)
		}
		if result.Stdout == "" {
			return nil, fmt.Errorf("resolve branch tip for %s: empty revision", branch)
		}
		tips[branch] = result.Stdout
	}
	return tips, nil
}

func rebaseArgsForIndex(state *State, idx int, onto string) ([]string, error) {
	if idx == 0 {
		return []string{"rebase", "--autostash", onto}, nil
	}

	parentBranch := state.AllBranches[idx-1]
	parentTip := state.OriginalTips[parentBranch]
	if parentTip == "" {
		return nil, fmt.Errorf("missing original tip for %s", parentBranch)
	}

	return []string{"rebase", "--autostash", "--onto", onto, parentTip}, nil
}
