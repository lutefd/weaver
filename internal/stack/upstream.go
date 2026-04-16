package stack

import (
	"context"
	"fmt"
	"strings"

	gitparse "github.com/lutefd/weaver/internal/git"
	gitrunner "github.com/lutefd/weaver/internal/git"
)

func ComputeUpstreamHealth(ctx context.Context, runner gitrunner.Runner, dag *DAG, base string) (map[string]UpstreamHealth, error) {
	health := make(map[string]UpstreamHealth)
	for _, branch := range dag.Branches() {
		if branch == base {
			continue
		}

		status, err := classifyUpstreamHealth(ctx, runner, branch)
		if err != nil {
			return nil, err
		}
		health[branch] = status
	}
	return health, nil
}

func classifyUpstreamHealth(ctx context.Context, runner gitrunner.Runner, branch string) (UpstreamHealth, error) {
	upstream, hasUpstream, err := upstreamForBranch(ctx, runner, branch)
	if err != nil {
		return UpstreamHealth{}, fmt.Errorf("resolve upstream for %s: %w", branch, err)
	}
	if !hasUpstream {
		return UpstreamHealth{State: UpstreamMissing}, nil
	}

	aheadBehind, err := runner.Run(ctx, "rev-list", "--left-right", "--count", branch+"..."+upstream)
	if err != nil {
		return UpstreamHealth{}, fmt.Errorf("resolve upstream ahead/behind for %s on %s: %w", branch, upstream, err)
	}
	ahead, behind, err := gitparse.ParseAheadBehind(aheadBehind.Stdout)
	if err != nil {
		return UpstreamHealth{}, fmt.Errorf("parse upstream ahead/behind for %s on %s: %w", branch, upstream, err)
	}

	switch {
	case ahead == 0 && behind == 0:
		return UpstreamHealth{State: UpstreamCurrent}, nil
	case ahead > 0 && behind == 0:
		return UpstreamHealth{State: UpstreamAhead, Ahead: ahead}, nil
	case ahead == 0 && behind > 0:
		return UpstreamHealth{State: UpstreamBehind, Behind: behind}, nil
	default:
		return UpstreamHealth{State: UpstreamDiverged, Ahead: ahead, Behind: behind}, nil
	}
}

func upstreamForBranch(ctx context.Context, runner gitrunner.Runner, branch string) (string, bool, error) {
	result, err := runner.Run(ctx, "for-each-ref", "--format=%(refname:short)%09%(upstream:short)", "refs/heads/"+branch)
	if err != nil {
		return "", false, err
	}
	if result.Stdout == "" {
		return "", false, nil
	}

	parts := strings.SplitN(result.Stdout, "\t", 2)
	if len(parts) == 0 || parts[0] == "" {
		return "", false, nil
	}
	if len(parts) < 2 || parts[1] == "" {
		return "", false, nil
	}

	return parts[1], true, nil
}
