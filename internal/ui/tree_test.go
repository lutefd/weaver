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

func TestRenderUpstreamStatusTree(t *testing.T) {
	t.Parallel()

	dag, err := stack.NewDAG([]stack.Dependency{
		{Branch: "feature-b", Parent: "feature-a"},
		{Branch: "feature-c", Parent: "feature-b"},
		{Branch: "feature-d", Parent: "feature-c"},
	})
	if err != nil {
		t.Fatalf("NewDAG() error = %v", err)
	}

	got := RenderUpstreamStatusTree(dag, "main", map[string]stack.UpstreamHealth{
		"feature-a": {State: stack.UpstreamCurrent},
		"feature-b": {State: stack.UpstreamBehind, Behind: 3},
		"feature-c": {State: stack.UpstreamAhead, Ahead: 2},
		"feature-d": {State: stack.UpstreamDiverged, Ahead: 1, Behind: 4},
	})

	want := "main\n`-- feature-a  up to date\n    `-- feature-b  behind upstream (3 behind)\n        `-- feature-c  ahead of upstream (2 ahead)\n            `-- feature-d  diverged (1 ahead, 4 behind)"
	if got != want {
		t.Fatalf("RenderUpstreamStatusTree() = %q, want %q", got, want)
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
	if got := formatUpstreamHealth(stack.UpstreamHealth{State: stack.UpstreamCurrent}); got != "up to date" {
		t.Fatalf("formatUpstreamHealth(current) = %q", got)
	}
	if got := formatUpstreamHealth(stack.UpstreamHealth{State: stack.UpstreamBehind}); got != "behind upstream" {
		t.Fatalf("formatUpstreamHealth(behind) = %q", got)
	}
	if got := formatUpstreamHealth(stack.UpstreamHealth{State: stack.UpstreamAhead}); got != "ahead of upstream" {
		t.Fatalf("formatUpstreamHealth(ahead) = %q", got)
	}
	if got := formatUpstreamHealth(stack.UpstreamHealth{State: stack.UpstreamAhead, Ahead: 2}); got != "ahead of upstream (2 ahead)" {
		t.Fatalf("formatUpstreamHealth(ahead count) = %q", got)
	}
	if got := formatUpstreamHealth(stack.UpstreamHealth{State: stack.UpstreamBehind, Behind: 3}); got != "behind upstream (3 behind)" {
		t.Fatalf("formatUpstreamHealth(behind count) = %q", got)
	}
	if got := formatUpstreamHealth(stack.UpstreamHealth{State: stack.UpstreamDiverged}); got != "diverged" {
		t.Fatalf("formatUpstreamHealth(diverged) = %q", got)
	}
	if got := formatUpstreamHealth(stack.UpstreamHealth{State: stack.UpstreamDiverged, Ahead: 2}); got != "diverged (2 ahead)" {
		t.Fatalf("formatUpstreamHealth(diverged ahead) = %q", got)
	}
	if got := formatUpstreamHealth(stack.UpstreamHealth{State: stack.UpstreamDiverged, Behind: 3}); got != "diverged (3 behind)" {
		t.Fatalf("formatUpstreamHealth(diverged behind) = %q", got)
	}
	if got := formatUpstreamHealth(stack.UpstreamHealth{State: stack.UpstreamDiverged, Ahead: 2, Behind: 3}); got != "diverged (2 ahead, 3 behind)" {
		t.Fatalf("formatUpstreamHealth(diverged both) = %q", got)
	}
	if got := formatUpstreamHealth(stack.UpstreamHealth{State: stack.UpstreamMissing}); got != "no upstream" {
		t.Fatalf("formatUpstreamHealth(missing) = %q", got)
	}
	if got := formatUpstreamHealth(stack.UpstreamHealth{State: stack.UpstreamHealthState("mystery")}); got != "mystery" {
		t.Fatalf("formatUpstreamHealth(mystery) = %q", got)
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
	for _, tc := range []struct {
		name   string
		status stack.UpstreamHealth
		want   []string
	}{
		{name: "current", status: stack.UpstreamHealth{State: stack.UpstreamCurrent}, want: []string{"UP TO DATE"}},
		{name: "behind", status: stack.UpstreamHealth{State: stack.UpstreamBehind, Behind: 4}, want: []string{"BEHIND UPSTREAM", "4 BEHIND"}},
		{name: "ahead", status: stack.UpstreamHealth{State: stack.UpstreamAhead, Ahead: 2}, want: []string{"AHEAD OF UPSTREAM", "2 AHEAD"}},
		{name: "diverged both", status: stack.UpstreamHealth{State: stack.UpstreamDiverged, Ahead: 2, Behind: 3}, want: []string{"DIVERGED", "2 AHEAD / 3 BEHIND"}},
		{name: "diverged ahead", status: stack.UpstreamHealth{State: stack.UpstreamDiverged, Ahead: 2}, want: []string{"DIVERGED", "2 AHEAD"}},
		{name: "diverged behind", status: stack.UpstreamHealth{State: stack.UpstreamDiverged, Behind: 3}, want: []string{"DIVERGED", "3 BEHIND"}},
		{name: "missing", status: stack.UpstreamHealth{State: stack.UpstreamMissing}, want: []string{"NO UPSTREAM"}},
	} {
		badges = upstreamHealthBadges(theme, tc.status)
		for _, want := range tc.want {
			if !strings.Contains(badges, want) {
				t.Fatalf("upstreamHealthBadges(%s) missing %q in %q", tc.name, want, badges)
			}
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
	for _, tc := range []struct {
		state stack.UpstreamHealthState
		want  string
	}{
		{state: stack.UpstreamCurrent, want: "UP TO DATE"},
		{state: stack.UpstreamBehind, want: "BEHIND UPSTREAM"},
		{state: stack.UpstreamAhead, want: "AHEAD OF UPSTREAM"},
		{state: stack.UpstreamDiverged, want: "DIVERGED"},
		{state: stack.UpstreamMissing, want: "NO UPSTREAM"},
		{state: stack.UpstreamHealthState("mystery"), want: "MYSTERY"},
	} {
		primary = primaryUpstreamHealthBadge(theme, tc.state)
		if !strings.Contains(primary, tc.want) {
			t.Fatalf("primaryUpstreamHealthBadge(%q) = %q, want %q", tc.state, primary, tc.want)
		}
	}
}
