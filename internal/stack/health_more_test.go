package stack

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	gitrunner "github.com/lutefd/weaver/internal/git"
)

func TestComputeHealthUsesBaseForRootBranches(t *testing.T) {
	t.Parallel()

	dag, err := NewDAG([]Dependency{{Branch: "feature-b", Parent: "feature-a"}})
	if err != nil {
		t.Fatalf("NewDAG() error = %v", err)
	}

	runner := fakeRunner{
		results: map[string]gitrunner.Result{
			"merge-base feature-a main":                           {Stdout: "sha-main"},
			"rev-parse main":                                      {Stdout: "sha-main"},
			"rev-list --left-right --count feature-a...main":      {Stdout: "0 0"},
			"merge-base feature-b feature-a":                      {Stdout: "sha-main"},
			"rev-parse feature-a":                                 {Stdout: "sha-main"},
			"rev-list --left-right --count feature-b...feature-a": {Stdout: "0 0"},
		},
	}

	got, err := ComputeHealth(context.Background(), runner, dag, "main")
	if err != nil {
		t.Fatalf("ComputeHealth() error = %v", err)
	}
	if got["feature-a"].State != HealthClean {
		t.Fatalf("feature-a state = %q, want %q", got["feature-a"].State, HealthClean)
	}
}

func TestClassifyHealthErrorPaths(t *testing.T) {
	t.Parallel()

	t.Run("parent rev error", func(t *testing.T) {
		t.Parallel()

		wantErr := errors.New("boom")
		_, err := classifyHealth(context.Background(), fakeRunner{
			results: map[string]gitrunner.Result{
				"merge-base feature-b feature-a": {Stdout: "sha-main"},
			},
			errs: map[string]error{
				"rev-parse feature-a": wantErr,
			},
		}, "feature-b", "feature-a")
		if !errors.Is(err, wantErr) || !strings.Contains(err.Error(), "resolve parent revision") {
			t.Fatalf("classifyHealth() error = %v, want wrapped parent revision error", err)
		}
	})

	t.Run("ahead behind error", func(t *testing.T) {
		t.Parallel()

		wantErr := errors.New("boom")
		_, err := classifyHealth(context.Background(), fakeRunner{
			results: map[string]gitrunner.Result{
				"merge-base feature-b feature-a": {Stdout: "sha-main"},
				"rev-parse feature-a":            {Stdout: "sha-main"},
			},
			errs: map[string]error{
				"rev-list --left-right --count feature-b...feature-a": wantErr,
			},
		}, "feature-b", "feature-a")
		if !errors.Is(err, wantErr) || !strings.Contains(err.Error(), "resolve ahead/behind") {
			t.Fatalf("classifyHealth() error = %v, want wrapped ahead/behind error", err)
		}
	})

	t.Run("ahead behind parse error", func(t *testing.T) {
		t.Parallel()

		_, err := classifyHealth(context.Background(), fakeRunner{
			results: map[string]gitrunner.Result{
				"merge-base feature-b feature-a":                      {Stdout: "sha-main"},
				"rev-parse feature-a":                                 {Stdout: "sha-old"},
				"rev-list --left-right --count feature-b...feature-a": {Stdout: "not-valid"},
			},
		}, "feature-b", "feature-a")
		if err == nil || !strings.Contains(err.Error(), "parse ahead/behind") {
			t.Fatalf("classifyHealth() error = %v, want parse ahead/behind error", err)
		}
	})

	t.Run("unexpected merge tree error", func(t *testing.T) {
		t.Parallel()

		wantErr := errors.New("boom")
		_, err := classifyHealth(context.Background(), healthRunner{
			results: map[string]gitrunner.Result{
				"merge-base feature-b feature-a":                      {Stdout: "sha-old"},
				"rev-parse feature-a":                                 {Stdout: "sha-new"},
				"rev-list --left-right --count feature-b...feature-a": {Stdout: "1 2"},
				"merge-tree --write-tree --quiet feature-a feature-b": {ExitCode: 0},
			},
			errs: map[string]error{
				"merge-tree --write-tree --quiet feature-a feature-b": wantErr,
			},
		}, "feature-b", "feature-a")
		if !errors.Is(err, wantErr) || !strings.Contains(err.Error(), "predict merge-tree") {
			t.Fatalf("classifyHealth() error = %v, want wrapped merge-tree error", err)
		}
	})
}

type healthRunner struct {
	results map[string]gitrunner.Result
	errs    map[string]error
}

func (r healthRunner) Run(_ context.Context, args ...string) (gitrunner.Result, error) {
	key := strings.Join(args, " ")
	result, ok := r.results[key]
	if !ok {
		return gitrunner.Result{}, fmt.Errorf("unexpected git args: %s", key)
	}
	return result, r.errs[key]
}

func (r healthRunner) RepoRoot() string {
	return ""
}
