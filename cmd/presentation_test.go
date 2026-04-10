package cmd

import (
	"bytes"
	"strings"
	"testing"

	"github.com/lutefd/weaver/internal/composer"
	"github.com/lutefd/weaver/internal/doctor"
	weaverintegration "github.com/lutefd/weaver/internal/integration"
	"github.com/lutefd/weaver/internal/ui"
	"github.com/lutefd/weaver/internal/updater"
)

func TestWriteLine(t *testing.T) {
	t.Parallel()

	var out bytes.Buffer
	writeLine(&out, "hello")
	if got := out.String(); got != "hello\n" {
		t.Fatalf("writeLine() = %q, want %q", got, "hello\n")
	}
}

func TestRenderHelpers(t *testing.T) {
	t.Parallel()

	term := ui.NewTerminal(bytes.NewBuffer(nil), &bytes.Buffer{})

	cases := []struct {
		name string
		got  string
		want []string
	}{
		{
			name: "action",
			got: renderActionCard(term, ui.ToneSuccess, "Saved", "done", []ui.KeyValue{
				{Label: "branch", Value: "feature-a"},
			}, []string{"one", "two"}),
			want: []string{"Saved", "branch", "feature-a", "one", "two"},
		},
		{
			name: "tree",
			got:  renderTreeCard(term, "Dependency Tree", "Tracked branch relationships", "main\n└─ feature-a"),
			want: []string{"Dependency Tree", "Tracked branch relationships", "feature-a"},
		},
		{
			name: "doctor styled",
			got: renderDoctorReportStyled(term, &doctor.Report{
				Checks: []doctor.Check{{Level: doctor.LevelWarn, Message: "drift", Hint: "sync it"}},
				Summary: doctor.Summary{
					OK:   1,
					Warn: 1,
				},
			}),
			want: []string{"Repository Doctor", "drift", "sync it"},
		},
		{
			name: "integration doctor styled",
			got: renderIntegrationDoctorReportStyled(term, &weaverintegration.Report{
				Integration: "prox",
				Base:        "main",
				Order:       []string{"a", "b"},
				Checks:      []weaverintegration.Check{{Level: weaverintegration.LevelFail, Message: "bad", Hint: "fix"}},
				Summary:     weaverintegration.Summary{Fail: 1},
			}),
			want: []string{"Integration Doctor", "prox", "a → b", "bad", "fix"},
		},
		{
			name: "compose result",
			got: renderComposeResultStyled(term, &composer.ComposeResult{
				BaseBranch:    "main",
				Order:         []string{"a", "b"},
				CreatedBranch: "integration",
				Skipped:       []string{"c"},
			}),
			want: []string{"Compose Complete", "create", "integration", "skipped for manual merge"},
		},
		{
			name: "sync result",
			got:  renderSyncResultStyled(term, "merge", "feature-c", []string{"feature-a", "feature-b"}),
			want: []string{"Sync Complete", "merge", "feature-c", "feature-a → feature-b"},
		},
		{
			name: "update result",
			got: renderUpdateResultStyled(term, &updater.UpdateResult{
				OriginalBranch: "topic",
				Updated:        []string{"a"},
				UpToDate:       []string{"b"},
			}),
			want: []string{"Update Complete", "topic", "a", "b"},
		},
		{
			name: "integration recipe",
			got: renderIntegrationRecipeStyled(term, "prox", weaverintegration.Recipe{
				Base:     "main",
				Branches: []string{"a", "b"},
			}),
			want: []string{"Integration Strategy", "prox", "main", "a → b"},
		},
		{
			name: "integration list",
			got: renderIntegrationListStyled(term, map[string]weaverintegration.Recipe{
				"prox": {Base: "main", Branches: []string{"a", "b"}},
			}),
			want: []string{"Integrations", "count", "prox", "a → b"},
		},
		{
			name: "integration branch list",
			got: renderTrackedIntegrationBranchListStyled(term, []trackedIntegrationBranchEntry{
				{
					Name:   "release-1",
					Exists: true,
					Record: weaverintegration.BranchRecord{Base: "main", Branches: []string{"a", "b"}, Skipped: []string{"c"}, Integration: "staging"},
				},
			}),
			want: []string{"Integration Branches", "count", "release-1", "[present]", "branches=a, b", "skipped=c", "integration=staging"},
		},
		{
			name: "group list",
			got: renderGroupListStyled(term, map[string][]string{
				"sprint": {"a", "b"},
			}),
			want: []string{"Groups", "count", "sprint", "a, b"},
		},
		{
			name: "check",
			got:  renderCheck(ui.NewTheme(term), "warn", "message", "hint"),
			want: []string{"WARN", "message", "hint"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			for _, want := range tc.want {
				if !strings.Contains(tc.got, want) {
					t.Fatalf("render missing %q in %q", want, tc.got)
				}
			}
		})
	}
}

func TestRenderComposeResultStyledModes(t *testing.T) {
	t.Parallel()

	term := ui.NewTerminal(bytes.NewBuffer(nil), &bytes.Buffer{})

	preview := renderComposeResultStyled(term, &composer.ComposeResult{
		DryRun:        true,
		BaseBranch:    "main",
		Order:         []string{"feature-a", "feature-b"},
		CreatedBranch: "integration",
	})
	for _, want := range []string{"Compose Preview", "ephemeral", "feature-a → feature-b"} {
		if !strings.Contains(preview, want) {
			t.Fatalf("renderComposeResultStyled() preview missing %q in %q", want, preview)
		}
	}
	if strings.Contains(preview, "target") {
		t.Fatalf("renderComposeResultStyled() preview unexpectedly rendered target: %q", preview)
	}

	update := renderComposeResultStyled(term, &composer.ComposeResult{
		BaseBranch:    "main",
		Order:         []string{"feature-a"},
		UpdatedBranch: "integration",
	})
	for _, want := range []string{"Compose Complete", "update", "integration"} {
		if !strings.Contains(update, want) {
			t.Fatalf("renderComposeResultStyled() update missing %q in %q", want, update)
		}
	}
}

func TestRenderUpdateResultStyledNoChanges(t *testing.T) {
	t.Parallel()

	term := ui.NewTerminal(bytes.NewBuffer(nil), &bytes.Buffer{})
	got := renderUpdateResultStyled(term, &updater.UpdateResult{OriginalBranch: "topic"})
	for _, want := range []string{"Update Complete", "topic", "no branches changed"} {
		if !strings.Contains(got, want) {
			t.Fatalf("renderUpdateResultStyled() missing %q in %q", want, got)
		}
	}
}

func TestRenderCheckLevelsWithoutHint(t *testing.T) {
	t.Parallel()

	theme := ui.NewTheme(ui.NewTerminal(bytes.NewBuffer(nil), &bytes.Buffer{}))
	cases := []struct {
		level string
		want  string
	}{
		{level: "ok", want: "OK"},
		{level: "fail", want: "FAIL"},
		{level: "mystery", want: "MYSTERY"},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.level, func(t *testing.T) {
			t.Parallel()

			got := renderCheck(theme, tc.level, "message", "")
			if !strings.Contains(got, tc.want) || !strings.Contains(got, "message") {
				t.Fatalf("renderCheck() = %q, want level %q and message", got, tc.want)
			}
			if strings.Contains(got, "fix") {
				t.Fatalf("renderCheck() = %q, want no fix hint", got)
			}
		})
	}
}
