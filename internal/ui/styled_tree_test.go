package ui

import (
	"strings"
	"testing"

	"github.com/lutefd/weaver/internal/stack"
)

func TestRenderStyledStatusTreeIncludesBadges(t *testing.T) {
	t.Parallel()

	dag, err := stack.NewDAG([]stack.Dependency{
		{Branch: "feature-b", Parent: "feature-a"},
		{Branch: "feature-c", Parent: "feature-b"},
	})
	if err != nil {
		t.Fatalf("NewDAG() error = %v", err)
	}

	got := RenderStyledStatusTree(Terminal{width: 90}, dag, "main", map[string]stack.StackHealth{
		"feature-a": {State: stack.HealthClean},
		"feature-b": {State: stack.HealthOutdated, Behind: 3},
		"feature-c": {State: stack.HealthConflictRisk, Behind: 2},
	})

	for _, needle := range []string{"main", "feature-a", "feature-b", "feature-c", "CLEAN", "NEEDS SYNC", "3 BEHIND PARENT", "CONFLICT RISK", "2 BEHIND PARENT"} {
		if !strings.Contains(got, needle) {
			t.Fatalf("RenderStyledStatusTree() missing %q in %q", needle, got)
		}
	}
}
