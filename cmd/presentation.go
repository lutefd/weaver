package cmd

import (
	"context"
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/lutefd/weaver/internal/composer"
	"github.com/lutefd/weaver/internal/doctor"
	gitrunner "github.com/lutefd/weaver/internal/git"
	weaverintegration "github.com/lutefd/weaver/internal/integration"
	"github.com/lutefd/weaver/internal/ui"
	"github.com/lutefd/weaver/internal/updater"
	"github.com/spf13/cobra"
)

func terminalFor(cmd *cobra.Command) ui.Terminal {
	return ui.NewTerminal(cmd.InOrStdin(), cmd.OutOrStdout())
}

func runTask[T any](ctx context.Context, cmd *cobra.Command, spec ui.TaskSpec, fn func(context.Context, gitrunner.Runner) (T, error)) (T, error) {
	return ui.RunTask(ctx, terminalFor(cmd), AppContext().Runner, spec, fn)
}

func writeLine(w io.Writer, text string) {
	fmt.Fprintln(w, text)
}

func renderActionCard(term ui.Terminal, tone ui.Tone, title, subtitle string, details []ui.KeyValue, notes []string) string {
	theme := ui.NewTheme(term)
	bodyParts := make([]string, 0, 3)
	bodyParts = append(bodyParts, theme.Badge(tone, title))
	if len(details) > 0 {
		bodyParts = append(bodyParts, theme.KeyValues(details))
	}
	if len(notes) > 0 {
		bodyParts = append(bodyParts, theme.List(notes))
	}
	return theme.Card(title, subtitle, lipgloss.JoinVertical(lipgloss.Left, bodyParts...))
}

func renderTreeCard(term ui.Terminal, title, subtitle, tree string) string {
	theme := ui.NewTheme(term)
	return theme.Card(title, subtitle, tree)
}

func renderDoctorReportStyled(term ui.Terminal, report *doctor.Report) string {
	theme := ui.NewTheme(term)
	bodyParts := []string{
		lipgloss.JoinHorizontal(
			lipgloss.Center,
			theme.Badge(ui.ToneSuccess, fmt.Sprintf("%d ok", report.Summary.OK)),
			" ",
			theme.Badge(ui.ToneWarn, fmt.Sprintf("%d warn", report.Summary.Warn)),
			" ",
			theme.Badge(ui.ToneDanger, fmt.Sprintf("%d fail", report.Summary.Fail)),
		),
	}

	for _, check := range report.Checks {
		bodyParts = append(bodyParts, renderDoctorCheck(theme, check))
	}

	return theme.Card(
		"Repository Doctor",
		"Repository and Weaver health checks",
		lipgloss.JoinVertical(lipgloss.Left, bodyParts...),
	)
}

func renderIntegrationDoctorReportStyled(term ui.Terminal, report *weaverintegration.Report) string {
	theme := ui.NewTheme(term)
	details := []ui.KeyValue{
		{Label: "integration", Value: report.Integration},
		{Label: "base", Value: report.Base},
	}
	if len(report.Order) > 0 {
		details = append(details, ui.KeyValue{Label: "order", Value: strings.Join(report.Order, " → ")})
	}

	bodyParts := []string{
		theme.KeyValues(details),
		lipgloss.JoinHorizontal(
			lipgloss.Center,
			theme.Badge(ui.ToneSuccess, fmt.Sprintf("%d ok", report.Summary.OK)),
			" ",
			theme.Badge(ui.ToneWarn, fmt.Sprintf("%d warn", report.Summary.Warn)),
			" ",
			theme.Badge(ui.ToneDanger, fmt.Sprintf("%d fail", report.Summary.Fail)),
		),
	}
	for _, check := range report.Checks {
		bodyParts = append(bodyParts, renderIntegrationCheck(theme, check))
	}

	return theme.Card(
		"Integration Doctor",
		"Saved integration health, drift, and ancestry checks",
		lipgloss.JoinVertical(lipgloss.Left, bodyParts...),
	)
}

func renderComposeResultStyled(term ui.Terminal, result *composer.ComposeResult) string {
	title := "Compose Complete"
	mode := "ephemeral"
	target := ""
	switch {
	case result.DryRun:
		title = "Compose Preview"
	case result.CreatedBranch != "":
		mode = "create"
		target = result.CreatedBranch
	case result.UpdatedBranch != "":
		mode = "update"
		target = result.UpdatedBranch
	}

	details := []ui.KeyValue{
		{Label: "mode", Value: mode},
		{Label: "base", Value: result.BaseBranch},
		{Label: "order", Value: strings.Join(result.Order, " → ")},
	}
	if target != "" {
		details = append(details, ui.KeyValue{Label: "target", Value: target})
	}

	notes := []string(nil)
	if len(result.Skipped) > 0 {
		notes = append(notes, "skipped for manual merge: "+strings.Join(result.Skipped, ", "))
	}

	return renderActionCard(term, ui.ToneSuccess, title, "Resolved stack compose summary", details, notes)
}

func renderSyncResultStyled(term ui.Terminal, mode, branch string, completed []string) string {
	details := []ui.KeyValue{
		{Label: "mode", Value: mode},
		{Label: "branch", Value: branch},
	}
	if len(completed) > 0 {
		details = append(details, ui.KeyValue{Label: "applied", Value: strings.Join(completed, " → ")})
	}
	return renderActionCard(term, ui.ToneSuccess, "Sync Complete", "Stack sync finished without conflicts", details, nil)
}

func renderUpdateResultStyled(term ui.Terminal, result *updater.UpdateResult) string {
	details := []ui.KeyValue{
		{Label: "starting", Value: result.OriginalBranch},
	}
	if len(result.Updated) > 0 {
		details = append(details, ui.KeyValue{Label: "updated", Value: strings.Join(result.Updated, ", ")})
	}
	if len(result.UpToDate) > 0 {
		details = append(details, ui.KeyValue{Label: "current", Value: strings.Join(result.UpToDate, ", ")})
	}
	if len(details) == 1 {
		details = append(details, ui.KeyValue{Label: "status", Value: "no branches changed"})
	}
	return renderActionCard(term, ui.ToneSuccess, "Update Complete", "Upstream fast-forward status", details, nil)
}

func renderIntegrationRecipeStyled(term ui.Terminal, name string, recipe weaverintegration.Recipe) string {
	details := []ui.KeyValue{
		{Label: "name", Value: name},
		{Label: "base", Value: recipe.Base},
	}
	if len(recipe.Branches) > 0 {
		details = append(details, ui.KeyValue{Label: "branches", Value: strings.Join(recipe.Branches, " → ")})
	}
	return renderActionCard(term, ui.ToneInfo, "Integration Strategy", "Saved integration definition", details, nil)
}

func renderIntegrationListStyled(term ui.Terminal, recipes map[string]weaverintegration.Recipe) string {
	names := make([]string, 0, len(recipes))
	for name := range recipes {
		names = append(names, name)
	}
	sort.Strings(names)

	lines := make([]string, 0, len(names))
	for _, name := range names {
		recipe := recipes[name]
		lines = append(lines, fmt.Sprintf("%s  %s", name, strings.Join(recipe.Branches, " → ")))
	}
	return renderActionCard(term, ui.ToneInfo, "Integrations", "Saved integration strategies", []ui.KeyValue{{Label: "count", Value: fmt.Sprintf("%d", len(names))}}, lines)
}

func renderGroupListStyled(term ui.Terminal, groups map[string][]string) string {
	names := make([]string, 0, len(groups))
	for name := range groups {
		names = append(names, name)
	}
	sort.Strings(names)

	lines := make([]string, 0, len(names))
	for _, name := range names {
		lines = append(lines, fmt.Sprintf("%s  %s", name, strings.Join(groups[name], ", ")))
	}
	return renderActionCard(term, ui.ToneInfo, "Groups", "Named compose groups", []ui.KeyValue{{Label: "count", Value: fmt.Sprintf("%d", len(names))}}, lines)
}

func renderDoctorCheck(theme ui.Theme, check doctor.Check) string {
	return renderCheck(theme, string(check.Level), check.Message, check.Hint)
}

func renderIntegrationCheck(theme ui.Theme, check weaverintegration.Check) string {
	return renderCheck(theme, string(check.Level), check.Message, check.Hint)
}

func renderCheck(theme ui.Theme, level, message, hint string) string {
	tone := ui.ToneMuted
	switch level {
	case "ok":
		tone = ui.ToneSuccess
	case "warn":
		tone = ui.ToneWarn
	case "fail":
		tone = ui.ToneDanger
	}

	line := lipgloss.JoinHorizontal(lipgloss.Top, theme.Badge(tone, level), " ", theme.Value(message))
	if hint == "" {
		return line
	}
	return lipgloss.JoinVertical(
		lipgloss.Left,
		line,
		lipgloss.JoinHorizontal(lipgloss.Top, "   ", theme.Muted("fix"), "  ", theme.Value(hint)),
	)
}
