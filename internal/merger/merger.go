package merger

import (
	"context"
	"fmt"
	"strings"
	"time"

	gitrunner "github.com/lutefd/weaver/internal/git"
	"github.com/lutefd/weaver/internal/stack"
)

type Merger interface {
	MergeStack(ctx context.Context, dag *stack.DAG, branches []string, base string) (*MergeResult, error)
	Continue(ctx context.Context) (*MergeResult, error)
	Abort(ctx context.Context) error
	HasPending() bool
}

type MergeResult struct {
	OriginalBranch string
	Completed      []string
	Current        string
	CurrentOnto    string
	Conflict       bool
}

type SafeMerger struct {
	runner gitrunner.Runner
	store  *StateStore
}

func New(runner gitrunner.Runner) *SafeMerger {
	return &SafeMerger{
		runner: runner,
		store:  NewStateStore(runner.RepoRoot()),
	}
}

func (m *SafeMerger) MergeStack(ctx context.Context, dag *stack.DAG, branches []string, base string) (*MergeResult, error) {
	if len(branches) == 0 {
		return nil, fmt.Errorf("at least one branch is required")
	}
	if m.HasPending() {
		return nil, fmt.Errorf("a merge sync is already in progress")
	}

	originalBranch, err := currentBranch(ctx, m.runner)
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
	if err := m.store.Save(state); err != nil {
		return nil, err
	}

	return m.runFrom(ctx, state, 0, false)
}

func (m *SafeMerger) Continue(ctx context.Context) (*MergeResult, error) {
	state, err := m.store.Load()
	if err != nil {
		return nil, fmt.Errorf("load merge state: %w", err)
	}

	return m.runFrom(ctx, state, len(state.Completed), true)
}

func (m *SafeMerger) Abort(ctx context.Context) error {
	state, err := m.store.Load()
	if err != nil {
		return fmt.Errorf("load merge state: %w", err)
	}

	if _, err := m.runner.Run(ctx, "merge", "--abort"); err != nil {
		if !isNoMergeInProgress(err) {
			return err
		}
	}
	if _, err := m.runner.Run(ctx, "checkout", state.OriginalBranch); err != nil {
		return err
	}

	return m.store.Clear()
}

func (m *SafeMerger) HasPending() bool {
	return m.store.HasPending()
}

func (m *SafeMerger) runFrom(ctx context.Context, state *State, start int, continuing bool) (*MergeResult, error) {
	completed := append([]string(nil), state.Completed...)
	if continuing {
		if state.Current == "" {
			return nil, fmt.Errorf("merge state is missing the current branch")
		}
		if err := m.store.Save(state); err != nil {
			return nil, err
		}
		if _, err := m.runner.Run(ctx, "merge", "--continue"); err != nil {
			return &MergeResult{
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
		if err := m.store.Save(state); err != nil {
			return nil, err
		}

		if _, err := m.runner.Run(ctx, "checkout", branch); err != nil {
			return nil, err
		}
		if _, err := m.runner.Run(ctx, "merge", "--no-edit", onto); err != nil {
			return &MergeResult{
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

	if _, err := m.runner.Run(ctx, "checkout", state.OriginalBranch); err != nil {
		return nil, err
	}
	if err := m.store.Clear(); err != nil {
		return nil, err
	}

	return &MergeResult{
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

func isNoMergeInProgress(err error) bool {
	message := err.Error()
	return strings.Contains(message, "There is no merge to abort") || strings.Contains(message, "MERGE_HEAD missing")
}
