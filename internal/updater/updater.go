package updater

import (
	"context"
	"fmt"
	"strings"

	gitrunner "github.com/lutefd/weaver/internal/git"
)

type Updater interface {
	Update(ctx context.Context, branches []string) (*UpdateResult, error)
}

type UpdateResult struct {
	OriginalBranch string
	Updated        []string
	UpToDate       []string
}

type MissingBranchError struct {
	Branch string
}

func (e MissingBranchError) Error() string {
	return fmt.Sprintf("branch %s does not exist", e.Branch)
}

type MissingUpstreamError struct {
	Branch string
}

func (e MissingUpstreamError) Error() string {
	return fmt.Sprintf("branch %s has no upstream configured", e.Branch)
}

type FastForwardError struct {
	Branch   string
	Upstream string
	Err      error
}

func (e FastForwardError) Error() string {
	return fmt.Sprintf("cannot fast-forward %s from %s: %v", e.Branch, e.Upstream, e.Err)
}

func (e FastForwardError) Unwrap() error {
	return e.Err
}

type Engine struct {
	runner gitrunner.Runner
}

func New(runner gitrunner.Runner) *Engine {
	return &Engine{runner: runner}
}

func (e *Engine) Update(ctx context.Context, branches []string) (*UpdateResult, error) {
	if len(branches) == 0 {
		return nil, fmt.Errorf("at least one branch is required")
	}

	originalBranch, err := currentBranch(ctx, e.runner)
	if err != nil {
		return nil, err
	}

	result := &UpdateResult{
		OriginalBranch: originalBranch,
	}

	if _, err := e.runner.Run(ctx, "fetch", "--all"); err != nil {
		return nil, err
	}

	selected := dedupe(branches)
	current := originalBranch
	for _, branch := range selected {
		upstream, err := upstreamForBranch(ctx, e.runner, branch)
		if err != nil {
			if current != originalBranch {
				_ = restoreBranch(ctx, e.runner, originalBranch)
			}
			return result, err
		}

		if current != branch {
			if _, err := e.runner.Run(ctx, "checkout", branch); err != nil {
				if current != originalBranch {
					_ = restoreBranch(ctx, e.runner, originalBranch)
				}
				return result, err
			}
			current = branch
		}

		branchRev, err := revParse(ctx, e.runner, branch)
		if err != nil {
			_ = restoreBranch(ctx, e.runner, originalBranch)
			return result, err
		}
		upstreamRev, err := revParse(ctx, e.runner, upstream)
		if err != nil {
			_ = restoreBranch(ctx, e.runner, originalBranch)
			return result, err
		}
		if branchRev == upstreamRev {
			result.UpToDate = append(result.UpToDate, branch)
			continue
		}

		if _, err := e.runner.Run(ctx, "merge", "--ff-only", upstream); err != nil {
			_ = restoreBranch(ctx, e.runner, originalBranch)
			return result, FastForwardError{Branch: branch, Upstream: upstream, Err: err}
		}
		result.Updated = append(result.Updated, branch)
	}

	if current != originalBranch {
		if err := restoreBranch(ctx, e.runner, originalBranch); err != nil {
			return nil, err
		}
	}

	return result, nil
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

func restoreBranch(ctx context.Context, runner gitrunner.Runner, branch string) error {
	_, err := runner.Run(ctx, "checkout", branch)
	return err
}

func upstreamForBranch(ctx context.Context, runner gitrunner.Runner, branch string) (string, error) {
	result, err := runner.Run(ctx, "for-each-ref", "--format=%(refname:short)%09%(upstream:short)", "refs/heads/"+branch)
	if err != nil {
		return "", err
	}
	if result.Stdout == "" {
		return "", MissingBranchError{Branch: branch}
	}

	parts := strings.SplitN(result.Stdout, "\t", 2)
	if len(parts) == 0 || parts[0] == "" {
		return "", MissingBranchError{Branch: branch}
	}
	if len(parts) < 2 || parts[1] == "" {
		return "", MissingUpstreamError{Branch: branch}
	}

	return parts[1], nil
}

func revParse(ctx context.Context, runner gitrunner.Runner, ref string) (string, error) {
	result, err := runner.Run(ctx, "rev-parse", ref)
	if err != nil {
		return "", err
	}
	if result.Stdout == "" {
		return "", fmt.Errorf("resolve ref %s: empty revision", ref)
	}
	return result.Stdout, nil
}

func dedupe(branches []string) []string {
	seen := make(map[string]struct{}, len(branches))
	out := make([]string, 0, len(branches))
	for _, branch := range branches {
		if _, ok := seen[branch]; ok {
			continue
		}
		seen[branch] = struct{}{}
		out = append(out, branch)
	}
	return out
}
