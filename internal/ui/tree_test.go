package ui

import (
	"strings"
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

	want := "main\n`-- feature-a  clean\n    `-- feature-b  needs sync (3 behind parent)\n        `-- feature-c  conflict risk (2 behind parent)"
	if got != want {
		t.Fatalf("RenderStatusTree() = %q, want %q", got, want)
	}
}

func TestRenderChainErrorAndHealthHelpers(t *testing.T) {
	t.Parallel()

	dag, err := stack.NewDAG([]stack.Dependency{
		{Branch: "feature-b", Parent: "feature-a"},
	})
	if err != nil {
		t.Fatalf("NewDAG() error = %v", err)
	}

	chain, err := RenderChain(dag, "main", "missing")
	if err != nil {
		t.Fatalf("RenderChain() error = %v", err)
	}
	if chain != "main -> missing" {
		t.Fatalf("RenderChain() = %q, want %q", chain, "main -> missing")
	}
	if _, err := RenderChain(dag, "main", ""); err == nil {
		t.Fatal("RenderChain() error = nil, want empty branch error")
	}

	if got := formatHealth(stack.StackHealth{State: stack.HealthOutdated}); got != "needs sync" {
		t.Fatalf("formatHealth(outdated) = %q, want %q", got, "needs sync")
	}
	if got := formatHealth(stack.StackHealth{State: stack.HealthOutdated, Behind: 3}); got != "needs sync (3 behind parent)" {
		t.Fatalf("formatHealth(outdated behind) = %q, want %q", got, "needs sync (3 behind parent)")
	}
	if got := formatHealth(stack.StackHealth{State: stack.HealthConflictRisk}); got != "conflict risk" {
		t.Fatalf("formatHealth(conflict risk) = %q, want %q", got, "conflict risk")
	}
	if got := formatHealth(stack.StackHealth{State: stack.HealthConflictRisk, Behind: 2}); got != "conflict risk (2 behind parent)" {
		t.Fatalf("formatHealth(conflict risk behind) = %q, want %q", got, "conflict risk (2 behind parent)")
	}
	if got := formatHealth(stack.StackHealth{State: stack.StackHealthState("mystery")}); got != "mystery" {
		t.Fatalf("formatHealth(mystery) = %q, want %q", got, "mystery")
	}

	theme := NewTheme(Terminal{width: 80})
	badges := healthBadges(theme, stack.StackHealth{State: stack.HealthClean, Behind: 4})
	if strings.Contains(badges, "BEHIND") {
		t.Fatalf("healthBadges(clean) = %q, want no behind badge", badges)
	}
	badges = healthBadges(theme, stack.StackHealth{State: stack.HealthOutdated, Behind: 4})
	for _, want := range []string{"NEEDS SYNC", "4 BEHIND PARENT"} {
		if !strings.Contains(badges, want) {
			t.Fatalf("healthBadges(outdated) missing %q in %q", want, badges)
		}
	}

	primary := primaryHealthBadge(theme, stack.StackHealthState("mystery"))
	if !strings.Contains(primary, "MYSTERY") {
		t.Fatalf("primaryHealthBadge() = %q, want mystery badge", primary)
	}
	primary = primaryHealthBadge(theme, stack.HealthOutdated)
	if !strings.Contains(primary, "NEEDS SYNC") {
		t.Fatalf("primaryHealthBadge(outdated) = %q, want needs sync badge", primary)
	}
}
