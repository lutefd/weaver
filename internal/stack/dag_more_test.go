package stack

import (
	"reflect"
	"strings"
	"testing"
)

func TestDAGHelpers(t *testing.T) {
	t.Parallel()

	dag, err := NewDAG([]Dependency{
		{Branch: "feature-b", Parent: "feature-a"},
		{Branch: "feature-c", Parent: "feature-b"},
		{Branch: "feature-e", Parent: "feature-d"},
	})
	if err != nil {
		t.Fatalf("NewDAG() error = %v", err)
	}

	if parent, ok := dag.Parent("feature-b"); !ok || parent != "feature-a" {
		t.Fatalf("Parent() = %q, %v", parent, ok)
	}
	if children := dag.Children("feature-a"); !reflect.DeepEqual(children, []string{"feature-b"}) {
		t.Fatalf("Children() = %#v", children)
	}
	if branches := dag.Branches(); !reflect.DeepEqual(branches, []string{"feature-a", "feature-b", "feature-c", "feature-d", "feature-e"}) {
		t.Fatalf("Branches() = %#v", branches)
	}
	if deps := dag.Dependencies(); !reflect.DeepEqual(deps, []Dependency{
		{Branch: "feature-b", Parent: "feature-a"},
		{Branch: "feature-c", Parent: "feature-b"},
		{Branch: "feature-e", Parent: "feature-d"},
	}) {
		t.Fatalf("Dependencies() = %#v", deps)
	}
	if roots := dag.Roots(); !reflect.DeepEqual(roots, []string{"feature-a", "feature-d"}) {
		t.Fatalf("Roots() = %#v", roots)
	}
	if !dag.Contains("feature-d") || dag.Contains("missing") {
		t.Fatalf("Contains() returned unexpected result")
	}
}

func TestDAGAddValidation(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name   string
		branch string
		parent string
		want   string
		setup  func(*DAG)
	}{
		{name: "missing branch", parent: "main", want: "dependency branch is required"},
		{name: "missing parent", branch: "feature-a", want: "dependency parent is required"},
		{name: "self dependency", branch: "feature-a", parent: "feature-a", want: `branch "feature-a" cannot depend on itself`},
		{
			name:   "duplicate parent",
			branch: "feature-a",
			parent: "main",
			want:   `branch "feature-a" already depends on "develop"`,
			setup: func(dag *DAG) {
				dag.parents["feature-a"] = "develop"
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dag := &DAG{
				nodes:    make(map[string]struct{}),
				parents:  make(map[string]string),
				children: make(map[string][]string),
			}
			if tc.setup != nil {
				tc.setup(dag)
			}

			err := dag.Add(tc.branch, tc.parent)
			if err == nil || err.Error() != tc.want {
				t.Fatalf("Add() error = %v, want %q", err, tc.want)
			}
		})
	}
}

func TestDAGAncestorsValidation(t *testing.T) {
	t.Parallel()

	dag, err := NewDAG([]Dependency{{Branch: "feature-b", Parent: "feature-a"}})
	if err != nil {
		t.Fatalf("NewDAG() error = %v", err)
	}

	if _, err := dag.Ancestors(""); err == nil || err.Error() != "branch is required" {
		t.Fatalf("Ancestors(\"\") error = %v", err)
	}

	got, err := dag.Ancestors("missing")
	if err != nil {
		t.Fatalf("Ancestors(missing) error = %v", err)
	}
	if !reflect.DeepEqual(got, []string{"missing"}) {
		t.Fatalf("Ancestors(missing) = %#v", got)
	}
}

func TestAppendUnique(t *testing.T) {
	t.Parallel()

	values := appendUnique([]string{"feature-a"}, "feature-a")
	if !reflect.DeepEqual(values, []string{"feature-a"}) {
		t.Fatalf("appendUnique(existing) = %#v", values)
	}

	values = appendUnique(values, "feature-b")
	if !reflect.DeepEqual(values, []string{"feature-a", "feature-b"}) {
		t.Fatalf("appendUnique(new) = %#v", values)
	}
}

func TestUpsertDependencyAppends(t *testing.T) {
	t.Parallel()

	got := UpsertDependency(nil, Dependency{Branch: "feature-a", Parent: "main"})
	want := []Dependency{{Branch: "feature-a", Parent: "main"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("UpsertDependency() = %#v, want %#v", got, want)
	}
}

func TestNewDAGRejectsCycle(t *testing.T) {
	t.Parallel()

	_, err := NewDAG([]Dependency{
		{Branch: "feature-a", Parent: "feature-b"},
		{Branch: "feature-b", Parent: "feature-a"},
	})
	if err == nil || !strings.Contains(err.Error(), "dependency cycle detected") {
		t.Fatalf("NewDAG() error = %v, want cycle error", err)
	}
}
