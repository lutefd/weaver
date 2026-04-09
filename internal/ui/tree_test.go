package ui

import (
	"testing"

	"github.com/lutefd/weaver/internal/stack"
)

func TestRenderChain(t *testing.T) {
	t.Parallel()

	dag, err := stack.NewDAG([]stack.Dependency{
		{Branch: "feature-b", Parent: "feature-a"},
		{Branch: "feature-c", Parent: "feature-b"},
	})
	if err != nil {
		t.Fatalf("NewDAG() error = %v", err)
	}

	got, err := RenderChain(dag, "main", "feature-c")
	if err != nil {
		t.Fatalf("RenderChain() error = %v", err)
	}

	want := "main -> feature-a -> feature-b -> feature-c"
	if got != want {
		t.Fatalf("RenderChain() = %q, want %q", got, want)
	}
}

func TestRenderTree(t *testing.T) {
	t.Parallel()

	dag, err := stack.NewDAG([]stack.Dependency{
		{Branch: "feature-b", Parent: "feature-a"},
		{Branch: "feature-c", Parent: "feature-b"},
		{Branch: "feature-e", Parent: "feature-d"},
	})
	if err != nil {
		t.Fatalf("NewDAG() error = %v", err)
	}

	got := RenderTree(dag, "main")
	want := "main\n+-- feature-a\n|   `-- feature-b\n|       `-- feature-c\n`-- feature-d\n    `-- feature-e"
	if got != want {
		t.Fatalf("RenderTree() = %q, want %q", got, want)
	}
}

func TestRenderTreeSkipsBaseNode(t *testing.T) {
	t.Parallel()

	dag, err := stack.NewDAG([]stack.Dependency{
		{Branch: "feature-b", Parent: "main"},
	})
	if err != nil {
		t.Fatalf("NewDAG() error = %v", err)
	}

	got := RenderTree(dag, "main")
	want := "main\n`-- feature-b"
	if got != want {
		t.Fatalf("RenderTree() = %q, want %q", got, want)
	}
}

func TestRenderStatusTree(t *testing.T) {
	t.Parallel()

	dag, err := stack.NewDAG([]stack.Dependency{
		{Branch: "feature-b", Parent: "feature-a"},
		{Branch: "feature-c", Parent: "feature-b"},
	})
	if err != nil {
		t.Fatalf("NewDAG() error = %v", err)
	}

	got := RenderStatusTree(dag, "main", map[string]stack.StackHealth{
		"feature-a": {State: stack.HealthClean},
		"feature-b": {State: stack.HealthOutdated, Behind: 3},
		"feature-c": {State: stack.HealthConflictRisk, Behind: 2},
	})

	want := "main\n`-- feature-a  clean\n    `-- feature-b  outdated (3 behind)\n        `-- feature-c  conflict risk (2 behind)"
	if got != want {
		t.Fatalf("RenderStatusTree() = %q, want %q", got, want)
	}
}
