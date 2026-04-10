package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/lutefd/weaver/internal/deps"
	gitrunner "github.com/lutefd/weaver/internal/git"
	weaverintegration "github.com/lutefd/weaver/internal/integration"
	"github.com/lutefd/weaver/internal/resolver"
	"github.com/lutefd/weaver/internal/ui"
	"github.com/spf13/cobra"
)

func init() {
	integrationSaveCmd.Flags().String("base", "", "base branch to compose onto")
	integrationExportCmd.Flags().Bool("json", false, "export integration strategy as JSON")
	integrationDoctorCmd.Flags().Bool("json", false, "print the integration doctor report as JSON")

	integrationCmd.AddCommand(integrationSaveCmd)
	integrationCmd.AddCommand(integrationShowCmd)
	integrationCmd.AddCommand(integrationListCmd)
	integrationCmd.AddCommand(integrationRemoveCmd)
	integrationCmd.AddCommand(integrationDoctorCmd)
	integrationCmd.AddCommand(integrationExportCmd)
	integrationCmd.AddCommand(integrationImportCmd)
	integrationCmd.AddCommand(integrationBranchCmd)
	rootCmd.AddCommand(integrationCmd)
}

var integrationCmd = &cobra.Command{
	Use:   "integration",
	Short: "Manage saved integration compose strategies",
}

var integrationSaveCmd = &cobra.Command{
	Use:   "save <name> <branch...>",
	Short: "Save or update an integration compose strategy",
	Args:  cobra.MinimumNArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		base, err := cmd.Flags().GetString("base")
		if err != nil {
			return err
		}
		if base == "" {
			base = AppContext().Config.DefaultBase
		}

		name := args[0]
		recipe := weaverintegration.Recipe{
			Base:     base,
			Branches: args[1:],
		}
		if err := weaverintegration.NewStore(AppContext().Runner.RepoRoot()).Save(name, recipe); err != nil {
			return err
		}

		term := terminalFor(cmd)
		if term.Styled() {
			writeLine(cmd.OutOrStdout(), renderActionCard(term, ui.ToneSuccess, "Integration Saved", "Saved integration compose strategy", []ui.KeyValue{
				{Label: "name", Value: name},
				{Label: "base", Value: base},
				{Label: "branches", Value: strings.Join(recipe.Branches, " → ")},
			}, nil))
			return nil
		}

		fmt.Fprintf(cmd.OutOrStdout(), "saved integration %s on %s with: %s\n", name, base, strings.Join(recipe.Branches, " -> "))
		return nil
	},
}

var integrationShowCmd = &cobra.Command{
	Use:   "show <name>",
	Short: "Show a saved integration compose strategy",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		recipe, ok, err := weaverintegration.NewStore(AppContext().Runner.RepoRoot()).Get(args[0])
		if err != nil {
			return err
		}
		if !ok {
			return fmt.Errorf("integration %q does not exist", args[0])
		}

		term := terminalFor(cmd)
		if term.Styled() {
			writeLine(cmd.OutOrStdout(), renderIntegrationRecipeStyled(term, args[0], recipe))
			return nil
		}

		fmt.Fprintf(cmd.OutOrStdout(), "%s\n", args[0])
		fmt.Fprintf(cmd.OutOrStdout(), "base: %s\n", recipe.Base)
		fmt.Fprintf(cmd.OutOrStdout(), "branches: %s\n", strings.Join(recipe.Branches, " -> "))
		return nil
	},
}

var integrationListCmd = &cobra.Command{
	Use:   "list",
	Short: "List saved integration compose strategies",
	RunE: func(cmd *cobra.Command, _ []string) error {
		store := weaverintegration.NewStore(AppContext().Runner.RepoRoot())
		names, err := store.Names()
		if err != nil {
			return err
		}
		if len(names) == 0 {
			fmt.Fprintln(cmd.OutOrStdout(), "no integrations")
			return nil
		}

		recipes := make(map[string]weaverintegration.Recipe, len(names))
		for _, name := range names {
			recipe, ok, err := store.Get(name)
			if err != nil {
				return err
			}
			if !ok {
				continue
			}
			recipes[name] = recipe
		}

		term := terminalFor(cmd)
		if term.Styled() {
			writeLine(cmd.OutOrStdout(), renderIntegrationListStyled(term, recipes))
			return nil
		}

		for _, name := range names {
			recipe, ok := recipes[name]
			if !ok {
				continue
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%s: base=%s branches=%s\n", name, recipe.Base, strings.Join(recipe.Branches, ", "))
		}
		return nil
	},
}

var integrationRemoveCmd = &cobra.Command{
	Use:   "remove <name>",
	Short: "Remove a saved integration compose strategy",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := weaverintegration.NewStore(AppContext().Runner.RepoRoot()).Remove(args[0]); err != nil {
			return err
		}
		term := terminalFor(cmd)
		if term.Styled() {
			writeLine(cmd.OutOrStdout(), renderActionCard(term, ui.ToneSuccess, "Integration Removed", "Saved integration deleted", []ui.KeyValue{{Label: "name", Value: args[0]}}, nil))
			return nil
		}

		fmt.Fprintf(cmd.OutOrStdout(), "removed integration %s\n", args[0])
		return nil
	},
}

var integrationExportCmd = &cobra.Command{
	Use:   "export <name>",
	Short: "Export one saved integration compose strategy",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		asJSON, err := cmd.Flags().GetBool("json")
		if err != nil {
			return err
		}

		recipe, ok, err := weaverintegration.NewStore(AppContext().Runner.RepoRoot()).Get(args[0])
		if err != nil {
			return err
		}
		if !ok {
			return fmt.Errorf("integration %q does not exist", args[0])
		}

		if !asJSON {
			term := terminalFor(cmd)
			if term.Styled() {
				writeLine(cmd.OutOrStdout(), renderIntegrationRecipeStyled(term, args[0], recipe))
				return nil
			}

			fmt.Fprintf(cmd.OutOrStdout(), "%s\n", args[0])
			fmt.Fprintf(cmd.OutOrStdout(), "base: %s\n", recipe.Base)
			fmt.Fprintf(cmd.OutOrStdout(), "branches: %s\n", strings.Join(recipe.Branches, " -> "))
			return nil
		}

		exported, err := weaverintegration.NewExport(args[0], recipe)
		if err != nil {
			return err
		}
		return weaverintegration.EncodeExport(cmd.OutOrStdout(), exported)
	},
}

var integrationDoctorCmd = &cobra.Command{
	Use:   "doctor <name>",
	Short: "Inspect one saved integration compose strategy for drift, foreign ancestry, and suspicious merges",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()
		store := weaverintegration.NewStore(AppContext().Runner.RepoRoot())
		recipe, ok, err := store.Get(args[0])
		if err != nil {
			return err
		}
		if !ok {
			return fmt.Errorf("integration %q does not exist", args[0])
		}

		asJSON, err := cmd.Flags().GetBool("json")
		if err != nil {
			return err
		}

		report, err := runTask(ctx, cmd, ui.TaskSpec{
			Title:    "Checking Integration",
			Subtitle: fmt.Sprintf("Analyzing %s for drift and suspicious merges", args[0]),
		}, func(ctx context.Context, runner gitrunner.Runner) (*weaverintegration.Report, error) {
			dag, err := resolver.New(deps.NewLocalSource(runner.RepoRoot())).Resolve(ctx)
			if err != nil {
				return nil, err
			}
			return weaverintegration.NewAnalyzer(runner).Analyze(ctx, dag, args[0], recipe)
		})
		if err != nil {
			return err
		}
		if asJSON {
			encoder := json.NewEncoder(cmd.OutOrStdout())
			encoder.SetIndent("", "  ")
			if err := encoder.Encode(report); err != nil {
				return err
			}
		} else {
			term := terminalFor(cmd)
			if term.Styled() {
				writeLine(cmd.OutOrStdout(), renderIntegrationDoctorReportStyled(term, report))
			} else {
				renderIntegrationDoctorReport(cmd.OutOrStdout(), report)
			}
		}

		if report.HasFailures() {
			return fmt.Errorf("integration doctor found %d failure(s)", report.Summary.Fail)
		}
		return nil
	},
}

var integrationImportCmd = &cobra.Command{
	Use:   "import <file>",
	Short: "Import one saved integration compose strategy from JSON",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		file, err := os.Open(args[0])
		if err != nil {
			return err
		}
		defer file.Close()

		exported, err := weaverintegration.DecodeExport(file)
		if err != nil {
			return err
		}
		if err := weaverintegration.NewStore(AppContext().Runner.RepoRoot()).Save(exported.Integration.Name, exported.Integration.Recipe); err != nil {
			return err
		}
		term := terminalFor(cmd)
		if term.Styled() {
			writeLine(cmd.OutOrStdout(), renderActionCard(term, ui.ToneSuccess, "Integration Imported", "Imported saved integration strategy", []ui.KeyValue{
				{Label: "name", Value: exported.Integration.Name},
				{Label: "file", Value: args[0]},
			}, nil))
			return nil
		}

		fmt.Fprintf(cmd.OutOrStdout(), "imported integration %s from %s\n", exported.Integration.Name, args[0])
		return nil
	},
}

func renderIntegrationDoctorReport(w io.Writer, report *weaverintegration.Report) {
	fmt.Fprintf(w, "integration: %s\n", report.Integration)
	fmt.Fprintf(w, "base: %s\n", report.Base)
	if len(report.Order) > 0 {
		fmt.Fprintf(w, "order: %s\n", strings.Join(report.Order, " -> "))
	}

	for _, check := range report.Checks {
		fmt.Fprintf(w, "%-4s %s\n", strings.ToUpper(string(check.Level)), check.Message)
		if check.Hint != "" {
			fmt.Fprintf(w, "     fix: %s\n", check.Hint)
		}
	}
	fmt.Fprintf(w, "summary: %d ok, %d warn, %d fail\n", report.Summary.OK, report.Summary.Warn, report.Summary.Fail)
}
