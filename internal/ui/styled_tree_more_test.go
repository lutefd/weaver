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

func TestRenderStyledChainAdditionalCases(t *testing.T) {
	t.Parallel()

	dag, err := stack.NewDAG([]stack.Dependency{
		{Branch: "feature-b", Parent: "feature-a"},
	})
	if err != nil {
		t.Fatalf("NewDAG() error = %v", err)
	}

	term := Terminal{width: 80}
	chain, err := RenderStyledChain(term, dag, "", "feature-b")
	if err != nil {
		t.Fatalf("RenderStyledChain() error = %v", err)
	}
	if strings.Contains(chain, "main") {
		t.Fatalf("RenderStyledChain() = %q, want no injected base", chain)
	}

	chain, err = RenderStyledChain(term, dag, "main", "missing")
	if err != nil {
		t.Fatalf("RenderStyledChain() error = %v", err)
	}
	for _, want := range []string{"main", "missing"} {
		if !strings.Contains(chain, want) {
			t.Fatalf("RenderStyledChain() missing %q in %q", want, chain)
		}
	}

	if _, err := RenderStyledChain(term, dag, "main", ""); err == nil {
		t.Fatal("RenderStyledChain() error = nil, want empty branch error")
	}
}
