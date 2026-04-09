package updater

import (
	"context"
	"errors"
	"strings"
	"testing"

	gitrunner "github.com/lutefd/weaver/internal/git"
)

func TestErrorTypes(t *testing.T) {
	t.Parallel()

	if got := (MissingBranchError{Branch: "feature-a"}).Error(); got != "branch feature-a does not exist" {
		t.Fatalf("MissingBranchError.Error() = %q", got)
	}
	if got := (MissingUpstreamError{Branch: "feature-a"}).Error(); got != "branch feature-a has no upstream configured" {
		t.Fatalf("MissingUpstreamError.Error() = %q", got)
	}

	root := errors.New("boom")
	ff := FastForwardError{Branch: "feature-a", Upstream: "origin/feature-a", Err: root}
	if got := ff.Error(); !strings.Contains(got, "cannot fast-forward feature-a from origin/feature-a") {
		t.Fatalf("FastForwardError.Error() = %q", got)
	}
	if !errors.Is(ff.Unwrap(), root) {
		t.Fatal("FastForwardError.Unwrap() did not return wrapped error")
	}
}

func TestUpdateRejectsMissingBranch(t *testing.T) {
	t.Parallel()

	runner := &updateRunner{
		results: map[string]gitrunner.Result{
			"branch --show-current": {Stdout: "topic"},
			"for-each-ref --format=%(refname:short)%09%(upstream:short) refs/heads/feature-a": {Stdout: ""},
		},
	}

	_, err := New(runner).Update(context.Background(), []string{"feature-a"})
	var branchErr MissingBranchError
	if !errors.As(err, &branchErr) {
		t.Fatalf("Update() error = %v, want MissingBranchError", err)
	}
}

func TestUpdatePropagatesFetchError(t *testing.T) {
	t.Parallel()

	wantErr := errors.New("fetch failed")
	runner := &updateRunner{
		results: map[string]gitrunner.Result{
			"branch --show-current": {Stdout: "topic"},
		},
		errs: map[string]error{
			"fetch --all": wantErr,
		},
	}

	_, err := New(runner).Update(context.Background(), []string{"feature-a"})
	if !errors.Is(err, wantErr) {
		t.Fatalf("Update() error = %v, want %v", err, wantErr)
	}
}

func TestCurrentBranchEmpty(t *testing.T) {
	t.Parallel()

	_, err := currentBranch(context.Background(), &updateRunner{
		results: map[string]gitrunner.Result{
			"branch --show-current": {},
		},
	})
	if err == nil || !strings.Contains(err.Error(), "empty branch name") {
		t.Fatalf("currentBranch() error = %v, want empty branch name", err)
	}
}

func TestUpstreamForBranchMissingBranch(t *testing.T) {
	t.Parallel()

	_, err := upstreamForBranch(context.Background(), &updateRunner{
		results: map[string]gitrunner.Result{
			"for-each-ref --format=%(refname:short)%09%(upstream:short) refs/heads/feature-a": {},
		},
	}, "feature-a")
	var branchErr MissingBranchError
	if !errors.As(err, &branchErr) {
		t.Fatalf("upstreamForBranch() error = %v, want MissingBranchError", err)
	}
}

func TestRevParseEmpty(t *testing.T) {
	t.Parallel()

	_, err := revParse(context.Background(), &updateRunner{
		results: map[string]gitrunner.Result{
			"rev-parse feature-a": {},
		},
	}, "feature-a")
	if err == nil || !strings.Contains(err.Error(), "empty revision") {
		t.Fatalf("revParse() error = %v, want empty revision", err)
	}
}
