package stack

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"

	gitrunner "github.com/lutefd/weaver/internal/git"
)

func TestComputeHealth(t *testing.T) {
	t.Parallel()

	dag, err := NewDAG([]Dependency{
		{Branch: "feature-b", Parent: "feature-a"},
		{Branch: "feature-c", Parent: "feature-b"},
		{Branch: "feature-d", Parent: "main"},
	})
	if err != nil {
		t.Fatalf("NewDAG() error = %v", err)
	}

	runner := fakeRunner{
		results: map[string]gitrunner.Result{
			"merge-base feature-a main":                           {Stdout: "sha-main"},
			"rev-parse main":                                      {Stdout: "sha-main"},
			"merge-base feature-b feature-a":                      {Stdout: "sha-old"},
			"rev-parse feature-a":                                 {Stdout: "sha-new"},
			"merge-tree --write-tree --quiet feature-a feature-b": {},
			"merge-base feature-c feature-b":                      {Stdout: "sha-older"},
			"rev-parse feature-b":                                 {Stdout: "sha-newer"},
			"merge-tree --write-tree --quiet feature-b feature-c": {ExitCode: 1},
			"merge-base feature-d main":                           {Stdout: "sha-main"},
		},
		errs: map[string]error{
			"merge-tree --write-tree --quiet feature-b feature-c": errors.New("exit status 1"),
		},
	}

	got, err := ComputeHealth(context.Background(), runner, dag, "main")
	if err != nil {
		t.Fatalf("ComputeHealth() error = %v", err)
	}

	want := map[string]StackHealth{
		"feature-a": HealthClean,
		"feature-b": HealthNeedsRebase,
		"feature-c": HealthConflictRisk,
		"feature-d": HealthClean,
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ComputeHealth() = %#v, want %#v", got, want)
	}
}

func TestComputeHealthPropagatesErrors(t *testing.T) {
	t.Parallel()

	dag, err := NewDAG([]Dependency{{Branch: "feature-b", Parent: "feature-a"}})
	if err != nil {
		t.Fatalf("NewDAG() error = %v", err)
	}

	wantErr := errors.New("boom")
	_, err = ComputeHealth(context.Background(), fakeRunner{
		errs: map[string]error{
			"merge-base feature-a main": wantErr,
		},
	}, dag, "main")
	if !errors.Is(err, wantErr) {
		t.Fatalf("ComputeHealth() error = %v, want %v", err, wantErr)
	}
}

type fakeRunner struct {
	results map[string]gitrunner.Result
	errs    map[string]error
}

func (f fakeRunner) Run(_ context.Context, args ...string) (gitrunner.Result, error) {
	key := strings.Join(args, " ")
	result := f.results[key]
	if err, ok := f.errs[key]; ok {
		if result.ExitCode == 0 {
			result.ExitCode = 1
		}
		return result, err
	}
	if _, ok := f.results[key]; !ok {
		return gitrunner.Result{}, fmt.Errorf("unexpected git args: %s", key)
	}
	return result, nil
}

func (f fakeRunner) RepoRoot() string {
	return ""
}
