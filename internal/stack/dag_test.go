package stack

import (
	"reflect"
	"testing"
)

func TestNewDAGTopologicalSort(t *testing.T) {
	t.Parallel()

	dag, err := NewDAG([]Dependency{
		{Branch: "feature-b", Parent: "feature-a"},
		{Branch: "feature-c", Parent: "feature-b"},
		{Branch: "feature-e", Parent: "feature-d"},
	})
	if err != nil {
		t.Fatalf("NewDAG() error = %v", err)
	}

	got, err := dag.TopologicalSort()
	if err != nil {
		t.Fatalf("TopologicalSort() error = %v", err)
	}

	want := []string{"feature-a", "feature-b", "feature-c", "feature-d", "feature-e"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("TopologicalSort() = %#v, want %#v", got, want)
	}
}

func TestNewDAGCycle(t *testing.T) {
	t.Parallel()

	_, err := NewDAG([]Dependency{
		{Branch: "feature-a", Parent: "feature-c"},
		{Branch: "feature-b", Parent: "feature-a"},
		{Branch: "feature-c", Parent: "feature-b"},
	})
	if err == nil {
		t.Fatal("NewDAG() error = nil, want cycle error")
	}
}

func TestDAGAncestors(t *testing.T) {
	t.Parallel()

	dag, err := NewDAG([]Dependency{
		{Branch: "feature-b", Parent: "feature-a"},
		{Branch: "feature-c", Parent: "feature-b"},
	})
	if err != nil {
		t.Fatalf("NewDAG() error = %v", err)
	}

	got, err := dag.Ancestors("feature-c")
	if err != nil {
		t.Fatalf("Ancestors() error = %v", err)
	}

	want := []string{"feature-a", "feature-b", "feature-c"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Ancestors() = %#v, want %#v", got, want)
	}
}

func TestUpsertDependency(t *testing.T) {
	t.Parallel()

	got := UpsertDependency([]Dependency{
		{Branch: "feature-b", Parent: "feature-a"},
	}, Dependency{Branch: "feature-b", Parent: "main"})

	want := []Dependency{{Branch: "feature-b", Parent: "main"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("UpsertDependency() = %#v, want %#v", got, want)
	}
}
