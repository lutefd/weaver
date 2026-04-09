package stack

import (
	"context"
	"fmt"

	gitparse "github.com/lutefd/weaver/internal/git"
	gitrunner "github.com/lutefd/weaver/internal/git"
)

func ComputeHealth(ctx context.Context, runner gitrunner.Runner, dag *DAG, base string) (map[string]StackHealth, error) {
	health := make(map[string]StackHealth)
	for _, branch := range dag.Branches() {
		if branch == base {
			continue
		}

		parent, ok := dag.Parent(branch)
		if !ok {
			if len(dag.Children(branch)) == 0 {
				continue
			}
			parent = base
		}

		status, err := classifyHealth(ctx, runner, branch, parent)
		if err != nil {
			return nil, err
		}
		health[branch] = status
	}

	return health, nil
}

func classifyHealth(ctx context.Context, runner gitrunner.Runner, branch, parent string) (StackHealth, error) {
	mergeBase, err := runner.Run(ctx, "merge-base", branch, parent)
	if err != nil {
		return StackHealth{}, fmt.Errorf("resolve merge-base for %s on %s: %w", branch, parent, err)
	}

	parentRev, err := runner.Run(ctx, "rev-parse", parent)
	if err != nil {
		return StackHealth{}, fmt.Errorf("resolve parent revision for %s: %w", parent, err)
	}

	aheadBehind, err := runner.Run(ctx, "rev-list", "--left-right", "--count", branch+"..."+parent)
	if err != nil {
		return StackHealth{}, fmt.Errorf("resolve ahead/behind for %s on %s: %w", branch, parent, err)
	}
	_, behind, err := gitparse.ParseAheadBehind(aheadBehind.Stdout)
	if err != nil {
		return StackHealth{}, fmt.Errorf("parse ahead/behind for %s on %s: %w", branch, parent, err)
	}

	if mergeBase.Stdout == parentRev.Stdout {
		return StackHealth{State: HealthClean, Behind: behind}, nil
	}

	// A clean merge-tree means the branch is behind its parent but can be rebased cleanly.
	result, err := runner.Run(ctx, "merge-tree", "--write-tree", "--quiet", parent, branch)
	if err == nil {
		return StackHealth{State: HealthOutdated, Behind: behind}, nil
	}
	if result.ExitCode != 0 {
		return StackHealth{State: HealthConflictRisk, Behind: behind}, nil
	}

	return StackHealth{}, fmt.Errorf("predict merge-tree for %s on %s: %w", branch, parent, err)
}
