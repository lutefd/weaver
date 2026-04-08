package stack

import (
	"context"
	"fmt"

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
		return "", fmt.Errorf("resolve merge-base for %s on %s: %w", branch, parent, err)
	}

	parentRev, err := runner.Run(ctx, "rev-parse", parent)
	if err != nil {
		return "", fmt.Errorf("resolve parent revision for %s: %w", parent, err)
	}

	if mergeBase.Stdout == parentRev.Stdout {
		return HealthClean, nil
	}

	// A clean merge-tree means the branch is behind its parent but can be rebased cleanly.
	result, err := runner.Run(ctx, "merge-tree", "--write-tree", "--quiet", parent, branch)
	if err == nil {
		return HealthNeedsRebase, nil
	}
	if result.ExitCode != 0 {
		return HealthConflictRisk, nil
	}

	return "", fmt.Errorf("predict merge-tree for %s on %s: %w", branch, parent, err)
}
