package updater

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	gitrunner "github.com/lutefd/weaver/internal/git"
)

func TestUpdateFetchesAndFastForwardsBranches(t *testing.T) {
	t.Parallel()

	runner := &updateRunner{
		results: map[string]gitrunner.Result{
			"branch --show-current": {Stdout: "topic"},
			"for-each-ref --format=%(refname:short)%09%(upstream:short) refs/heads/feature-a": {Stdout: "feature-a\torigin/feature-a"},
			"for-each-ref --format=%(refname:short)%09%(upstream:short) refs/heads/feature-b": {Stdout: "feature-b\torigin/feature-b"},
			"rev-parse feature-a":        {Stdout: "a1"},
			"rev-parse origin/feature-a": {Stdout: "a2"},
			"rev-parse feature-b":        {Stdout: "b1"},
			"rev-parse origin/feature-b": {Stdout: "b1"},
		},
	}

	got, err := New(runner).Update(context.Background(), []string{"feature-a", "feature-b"})
	if err != nil {
		t.Fatalf("Update() error = %v", err)
	}

	want := &UpdateResult{
		OriginalBranch: "topic",
		Updated:        []string{"feature-a"},
		UpToDate:       []string{"feature-b"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Update() = %#v, want %#v", got, want)
	}

	wantCalls := []string{
		"branch --show-current",
		"fetch --all",
		"for-each-ref --format=%(refname:short)%09%(upstream:short) refs/heads/feature-a",
		"checkout feature-a",
		"rev-parse feature-a",
		"rev-parse origin/feature-a",
		"merge --ff-only origin/feature-a",
		"for-each-ref --format=%(refname:short)%09%(upstream:short) refs/heads/feature-b",
		"checkout feature-b",
		"rev-parse feature-b",
		"rev-parse origin/feature-b",
		"checkout topic",
	}
	if !reflect.DeepEqual(runner.calls, wantCalls) {
		t.Fatalf("calls = %#v, want %#v", runner.calls, wantCalls)
	}
}

func TestUpdateUsesCurrentBranchWithoutRedundantCheckout(t *testing.T) {
	t.Parallel()

	runner := &updateRunner{
		results: map[string]gitrunner.Result{
			"branch --show-current": {Stdout: "feature-a"},
			"for-each-ref --format=%(refname:short)%09%(upstream:short) refs/heads/feature-a": {Stdout: "feature-a\torigin/feature-a"},
			"rev-parse feature-a":        {Stdout: "a1"},
			"rev-parse origin/feature-a": {Stdout: "a2"},
		},
	}

	got, err := New(runner).Update(context.Background(), []string{"feature-a", "feature-a"})
	if err != nil {
		t.Fatalf("Update() error = %v", err)
	}
	if !reflect.DeepEqual(got.Updated, []string{"feature-a"}) {
		t.Fatalf("Updated = %#v, want [feature-a]", got.Updated)
	}

	wantCalls := []string{
		"branch --show-current",
		"fetch --all",
		"for-each-ref --format=%(refname:short)%09%(upstream:short) refs/heads/feature-a",
		"rev-parse feature-a",
		"rev-parse origin/feature-a",
		"merge --ff-only origin/feature-a",
	}
	if !reflect.DeepEqual(runner.calls, wantCalls) {
		t.Fatalf("calls = %#v, want %#v", runner.calls, wantCalls)
	}
}

func TestUpdateRejectsBranchWithoutUpstream(t *testing.T) {
	t.Parallel()

	runner := &updateRunner{
		results: map[string]gitrunner.Result{
			"branch --show-current": {Stdout: "topic"},
			"for-each-ref --format=%(refname:short)%09%(upstream:short) refs/heads/feature-a": {Stdout: "feature-a\t"},
		},
	}

	_, err := New(runner).Update(context.Background(), []string{"feature-a"})
	var upstreamErr MissingUpstreamError
	if !errors.As(err, &upstreamErr) {
		t.Fatalf("Update() error = %v, want MissingUpstreamError", err)
	}
}

func TestUpdateRestoresOriginalBranchOnFastForwardFailure(t *testing.T) {
	t.Parallel()

	runner := &updateRunner{
		results: map[string]gitrunner.Result{
			"branch --show-current": {Stdout: "topic"},
			"for-each-ref --format=%(refname:short)%09%(upstream:short) refs/heads/feature-a": {Stdout: "feature-a\torigin/feature-a"},
			"rev-parse feature-a":              {Stdout: "a1"},
			"rev-parse origin/feature-a":       {Stdout: "a2"},
			"merge --ff-only origin/feature-a": {ExitCode: 1},
		},
		errs: map[string]error{
			"merge --ff-only origin/feature-a": errors.New("exit status 1"),
		},
	}

	_, err := New(runner).Update(context.Background(), []string{"feature-a"})
	var ffErr FastForwardError
	if !errors.As(err, &ffErr) {
		t.Fatalf("Update() error = %v, want FastForwardError", err)
	}
	if ffErr.Branch != "feature-a" || ffErr.Upstream != "origin/feature-a" {
		t.Fatalf("FastForwardError = %#v, want feature-a/origin/feature-a", ffErr)
	}

	wantCalls := []string{
		"branch --show-current",
		"fetch --all",
		"for-each-ref --format=%(refname:short)%09%(upstream:short) refs/heads/feature-a",
		"checkout feature-a",
		"rev-parse feature-a",
		"rev-parse origin/feature-a",
		"merge --ff-only origin/feature-a",
		"checkout topic",
	}
	if !reflect.DeepEqual(runner.calls, wantCalls) {
		t.Fatalf("calls = %#v, want %#v", runner.calls, wantCalls)
	}
}

type updateRunner struct {
	results map[string]gitrunner.Result
	errs    map[string]error
	calls   []string
}

func (r *updateRunner) Run(_ context.Context, args ...string) (gitrunner.Result, error) {
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

func (r *updateRunner) RepoRoot() string {
	return ""
}
