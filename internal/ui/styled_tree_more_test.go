package ui

import (
	"strings"
	"testing"

	"github.com/lutefd/weaver/internal/stack"
)

func TestRenderStyledChainAndTree(t *testing.T) {
	t.Parallel()

	dag, err := stack.NewDAG([]stack.Dependency{
		{Branch: "feature-b", Parent: "feature-a"},
		{Branch: "feature-c", Parent: "feature-b"},
	})
	if err != nil {
		t.Fatalf("NewDAG() error = %v", err)
	}

	term := Terminal{width: 80}
	chain, err := RenderStyledChain(term, dag, "main", "feature-c")
	if err != nil {
		t.Fatalf("RenderStyledChain() error = %v", err)
	}
	for _, want := range []string{"main", "feature-a", "feature-b", "feature-c"} {
		if !strings.Contains(chain, want) {
			t.Fatalf("RenderStyledChain() missing %q", want)
		}
	}

	tree := RenderStyledTree(term, dag, "main")
	for _, want := range []string{"main", "feature-a", "feature-b", "feature-c"} {
		if !strings.Contains(tree, want) {
			t.Fatalf("RenderStyledTree() missing %q", want)
		}
	}
}
