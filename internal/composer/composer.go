package composer

import (
	"context"
	"fmt"
	"strings"

	gitrunner "github.com/lutefd/weaver/internal/git"
	"github.com/lutefd/weaver/internal/stack"
)

type Composer interface {
	Compose(ctx context.Context, dag *stack.DAG, branches []string, base string, opts ComposeOpts) (*ComposeResult, error)
}

type ComposeOpts struct {
	DryRun       bool
	CreateBranch string
	UpdateBranch string
	SkipBranches []string
}

type ComposeResult struct {
	OriginalBranch string
	BaseBranch     string
	Order          []string
	Skipped        []string
	DryRun         bool
	CreatedBranch  string
	UpdatedBranch  string
}

type ConflictError struct {
	Branch string
	Files  []string
	Err    error
}

func (e ConflictError) Error() string {
	return fmt.Sprintf("compose conflict on %s: %v", e.Branch, e.Err)
}

func (e ConflictError) Unwrap() error {
	return e.Err
}

type Engine struct {
	runner gitrunner.Runner
}

func New(runner gitrunner.Runner) *Engine {
	return &Engine{runner: runner}
}

func (e *Engine) Compose(ctx context.Context, dag *stack.DAG, branches []string, base string, opts ComposeOpts) (*ComposeResult, error) {
	writeModes := 0
	if opts.CreateBranch != "" {
		writeModes++
	}
	if opts.UpdateBranch != "" {
		writeModes++
	}
	if writeModes > 1 {
		return nil, fmt.Errorf("cannot combine create branch and update branch modes")
	}

	order, err := resolveComposeOrder(dag, branches, base)
	if err != nil {
		return nil, err
	}
	order, skipped := applySkippedBranches(order, opts.SkipBranches)

	result := &ComposeResult{
		BaseBranch:    base,
		Order:         order,
		Skipped:       skipped,
		DryRun:        opts.DryRun,
		CreatedBranch: opts.CreateBranch,
		UpdatedBranch: opts.UpdateBranch,
	}
	if opts.DryRun {
		return result, nil
	}

	originalBranch, err := currentBranch(ctx, e.runner)
	if err != nil {
		return nil, err
	}
	result.OriginalBranch = originalBranch

	if _, err := e.runner.Run(ctx, "checkout", "--detach", base); err != nil {
		return nil, err
	}

	for _, branch := range order {
		if _, err := e.runner.Run(ctx, "merge", "--no-ff", "--no-edit", branch); err != nil {
			conflictFiles, filesErr := conflictedFiles(ctx, e.runner)
			_ = abortMerge(ctx, e.runner)
			_ = restoreBranch(ctx, e.runner, originalBranch)
			if filesErr != nil {
				return result, ConflictError{Branch: branch, Err: err}
			}
			return result, ConflictError{Branch: branch, Files: conflictFiles, Err: err}
		}
	}

	if opts.CreateBranch != "" {
		if _, err := e.runner.Run(ctx, "branch", opts.CreateBranch, "HEAD"); err != nil {
			_ = restoreBranch(ctx, e.runner, originalBranch)
			return nil, err
		}
	} else if opts.UpdateBranch != "" {
		if _, err := e.runner.Run(ctx, "branch", "-f", opts.UpdateBranch, "HEAD"); err != nil {
			_ = restoreBranch(ctx, e.runner, originalBranch)
			return nil, err
		}
	}

	if err := restoreBranch(ctx, e.runner, originalBranch); err != nil {
		return nil, err
	}

	return result, nil
}

func resolveComposeOrder(dag *stack.DAG, branches []string, base string) ([]string, error) {
	if len(branches) == 0 {
		return nil, fmt.Errorf("at least one branch is required")
	}

	seen := make(map[string]struct{})
	preferred := make([]string, 0)
	for _, branch := range branches {
		chain, err := dag.Ancestors(branch)
		if err != nil {
			return nil, err
		}
		for _, candidate := range chain {
			if candidate == base {
				continue
			}
			if _, ok := seen[candidate]; ok {
				continue
			}
			seen[candidate] = struct{}{}
			preferred = append(preferred, candidate)
		}
	}

	index := make(map[string]int, len(preferred))
	for idx, branch := range preferred {
		index[branch] = idx
	}

	inDegree := make(map[string]int, len(preferred))
	children := make(map[string][]string)
	for _, branch := range preferred {
		inDegree[branch] = 0
	}
	for _, dep := range dag.Dependencies() {
		if dep.Parent == base {
			continue
		}
		if _, childIncluded := inDegree[dep.Branch]; !childIncluded {
			continue
		}
		if _, parentIncluded := inDegree[dep.Parent]; !parentIncluded {
			continue
		}
		inDegree[dep.Branch]++
		children[dep.Parent] = append(children[dep.Parent], dep.Branch)
	}

	queue := make([]string, 0)
	for _, branch := range preferred {
		if inDegree[branch] == 0 {
			queue = append(queue, branch)
		}
	}

	order := make([]string, 0, len(preferred))
	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		order = append(order, current)

		for _, child := range children[current] {
			inDegree[child]--
			if inDegree[child] == 0 {
				queue = insertStable(queue, child, index)
			}
		}
	}

	if len(order) != len(preferred) {
		return nil, fmt.Errorf("dependency cycle detected")
	}

	return order, nil
}

func ResolveOrder(dag *stack.DAG, branches []string, base string) ([]string, error) {
	return resolveComposeOrder(dag, branches, base)
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

func abortMerge(ctx context.Context, runner gitrunner.Runner) error {
	if _, err := runner.Run(ctx, "merge", "--abort"); err != nil && !strings.Contains(err.Error(), "MERGE_HEAD") {
		return err
	}
	return nil
}

func restoreBranch(ctx context.Context, runner gitrunner.Runner, branch string) error {
	_, err := runner.Run(ctx, "checkout", branch)
	return err
}

func insertStable(queue []string, branch string, index map[string]int) []string {
	queue = append(queue, branch)
	for i := len(queue) - 1; i > 0; i-- {
		if index[queue[i-1]] <= index[queue[i]] {
			break
		}
		queue[i-1], queue[i] = queue[i], queue[i-1]
	}
	return queue
}

func conflictedFiles(ctx context.Context, runner gitrunner.Runner) ([]string, error) {
	result, err := runner.Run(ctx, "diff", "--name-only", "--diff-filter=U")
	if err != nil {
		return nil, err
	}
	if result.Stdout == "" {
		return nil, nil
	}
	return strings.Fields(result.Stdout), nil
}

func applySkippedBranches(order []string, skipBranches []string) ([]string, []string) {
	if len(skipBranches) == 0 {
		return order, nil
	}

	skipSet := make(map[string]struct{}, len(skipBranches))
	for _, branch := range skipBranches {
		if branch == "" {
			continue
		}
		skipSet[branch] = struct{}{}
	}

	filtered := make([]string, 0, len(order))
	skipped := make([]string, 0, len(skipSet))
	for _, branch := range order {
		if _, ok := skipSet[branch]; ok {
			skipped = append(skipped, branch)
			continue
		}
		filtered = append(filtered, branch)
	}
	return filtered, skipped
}
