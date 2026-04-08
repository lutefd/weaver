package cmd

import (
	"fmt"
	"os"
	"strings"

	weaverintegration "github.com/lutefd/weaver/internal/integration"
	"github.com/spf13/cobra"
)

func init() {
	integrationSaveCmd.Flags().String("base", "", "base branch to compose onto")
	integrationExportCmd.Flags().Bool("json", false, "export integration strategy as JSON")

	integrationCmd.AddCommand(integrationSaveCmd)
	integrationCmd.AddCommand(integrationShowCmd)
	integrationCmd.AddCommand(integrationListCmd)
	integrationCmd.AddCommand(integrationRemoveCmd)
	integrationCmd.AddCommand(integrationExportCmd)
	integrationCmd.AddCommand(integrationImportCmd)
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

		for _, name := range names {
			recipe, ok, err := store.Get(name)
			if err != nil {
				return err
			}
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
		fmt.Fprintf(cmd.OutOrStdout(), "imported integration %s from %s\n", exported.Integration.Name, args[0])
		return nil
	},
}
