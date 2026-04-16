package stack

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	gitrunner "github.com/lutefd/weaver/internal/git"
)

func TestComputeUpstreamHealth(t *testing.T) {
	t.Parallel()

	dag, err := NewDAG([]Dependency{
		{Branch: "feature-b", Parent: "feature-a"},
		{Branch: "feature-c", Parent: "feature-b"},
		{Branch: "feature-d", Parent: "feature-c"},
		{Branch: "feature-e", Parent: "feature-d"},
	})
	if err != nil {
		t.Fatalf("NewDAG() error = %v", err)
	}

	runner := fakeRunner{
		results: map[string]gitrunner.Result{
			"for-each-ref --format=%(refname:short)%09%(upstream:short) refs/heads/feature-a": {Stdout: "feature-a\torigin/feature-a"},
			"rev-list --left-right --count feature-a...origin/feature-a":                      {Stdout: "0 0"},
			"for-each-ref --format=%(refname:short)%09%(upstream:short) refs/heads/feature-b": {Stdout: "feature-b\torigin/feature-b"},
			"rev-list --left-right --count feature-b...origin/feature-b":                      {Stdout: "0 3"},
			"for-each-ref --format=%(refname:short)%09%(upstream:short) refs/heads/feature-c": {Stdout: "feature-c\torigin/feature-c"},
			"rev-list --left-right --count feature-c...origin/feature-c":                      {Stdout: "2 0"},
			"for-each-ref --format=%(refname:short)%09%(upstream:short) refs/heads/feature-d": {Stdout: "feature-d\torigin/feature-d"},
			"rev-list --left-right --count feature-d...origin/feature-d":                      {Stdout: "2 4"},
			"for-each-ref --format=%(refname:short)%09%(upstream:short) refs/heads/feature-e": {Stdout: "feature-e\t"},
		},
	}

	got, err := ComputeUpstreamHealth(context.Background(), runner, dag, "main")
	if err != nil {
		t.Fatalf("ComputeUpstreamHealth() error = %v", err)
	}

	want := map[string]UpstreamHealth{
		"feature-a": {State: UpstreamCurrent},
		"feature-b": {State: UpstreamBehind, Behind: 3},
		"feature-c": {State: UpstreamAhead, Ahead: 2},
		"feature-d": {State: UpstreamDiverged, Ahead: 2, Behind: 4},
		"feature-e": {State: UpstreamMissing},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ComputeUpstreamHealth() = %#v, want %#v", got, want)
	}
}

func TestClassifyUpstreamHealthErrors(t *testing.T) {
	t.Parallel()

	_, err := classifyUpstreamHealth(context.Background(), fakeRunner{
		errs: map[string]error{
			"for-each-ref --format=%(refname:short)%09%(upstream:short) refs/heads/feature-a": errors.New("boom"),
		},
	}, "feature-a")
	if err == nil || !strings.Contains(err.Error(), "resolve upstream for feature-a") {
		t.Fatalf("classifyUpstreamHealth(upstream) error = %v", err)
	}

	_, err = classifyUpstreamHealth(context.Background(), fakeRunner{
		results: map[string]gitrunner.Result{
			"for-each-ref --format=%(refname:short)%09%(upstream:short) refs/heads/feature-a": {Stdout: "feature-a\torigin/feature-a"},
		},
		errs: map[string]error{
			"rev-list --left-right --count feature-a...origin/feature-a": errors.New("boom"),
		},
	}, "feature-a")
	if err == nil || !strings.Contains(err.Error(), "resolve upstream ahead/behind for feature-a on origin/feature-a") {
		t.Fatalf("classifyUpstreamHealth(rev-list) error = %v", err)
	}

	_, err = classifyUpstreamHealth(context.Background(), fakeRunner{
		results: map[string]gitrunner.Result{
			"for-each-ref --format=%(refname:short)%09%(upstream:short) refs/heads/feature-a": {Stdout: "feature-a\torigin/feature-a"},
			"rev-list --left-right --count feature-a...origin/feature-a":                      {Stdout: "oops"},
		},
	}, "feature-a")
	if err == nil || !strings.Contains(err.Error(), "parse upstream ahead/behind for feature-a on origin/feature-a") {
		t.Fatalf("classifyUpstreamHealth(parse) error = %v", err)
	}
}

func TestUpstreamForBranch(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		runner       fakeRunner
		wantUpstream string
		wantHas      bool
		wantErr      string
	}{
		{
			name: "ok",
			runner: fakeRunner{results: map[string]gitrunner.Result{
				"for-each-ref --format=%(refname:short)%09%(upstream:short) refs/heads/feature-a": {Stdout: "feature-a\torigin/feature-a"},
			}},
			wantUpstream: "origin/feature-a",
			wantHas:      true,
		},
		{
			name: "empty output",
			runner: fakeRunner{results: map[string]gitrunner.Result{
				"for-each-ref --format=%(refname:short)%09%(upstream:short) refs/heads/feature-a": {Stdout: ""},
			}},
		},
		{
			name: "missing branch field",
			runner: fakeRunner{results: map[string]gitrunner.Result{
				"for-each-ref --format=%(refname:short)%09%(upstream:short) refs/heads/feature-a": {Stdout: "\torigin/feature-a"},
			}},
		},
		{
			name: "missing upstream",
			runner: fakeRunner{results: map[string]gitrunner.Result{
				"for-each-ref --format=%(refname:short)%09%(upstream:short) refs/heads/feature-a": {Stdout: "feature-a\t"},
			}},
		},
		{
			name: "runner error",
			runner: fakeRunner{errs: map[string]error{
				"for-each-ref --format=%(refname:short)%09%(upstream:short) refs/heads/feature-a": errors.New("boom"),
			}},
			wantErr: "boom",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, has, err := upstreamForBranch(context.Background(), tt.runner, "feature-a")
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("upstreamForBranch() error = %v, want %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("upstreamForBranch() error = %v", err)
			}
			if got != tt.wantUpstream || has != tt.wantHas {
				t.Fatalf("upstreamForBranch() = %q, %v want %q, %v", got, has, tt.wantUpstream, tt.wantHas)
			}
		})
	}
}

func TestComputeUpstreamHealthPropagatesErrors(t *testing.T) {
	t.Parallel()

	dag, err := NewDAG([]Dependency{{Branch: "feature-a", Parent: "main"}})
	if err != nil {
		t.Fatalf("NewDAG() error = %v", err)
	}

	wantErr := errors.New("boom")
	_, err = ComputeUpstreamHealth(context.Background(), fakeRunner{
		errs: map[string]error{
			"for-each-ref --format=%(refname:short)%09%(upstream:short) refs/heads/feature-a": wantErr,
		},
	}, dag, "main")
	if !errors.Is(err, wantErr) {
		t.Fatalf("ComputeUpstreamHealth() error = %v, want %v", err, wantErr)
	}
}
