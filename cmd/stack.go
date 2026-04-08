package cmd

import (
	"context"
	"fmt"

	"github.com/lutefd/weaver/internal/deps"
	stackpkg "github.com/lutefd/weaver/internal/stack"
	"github.com/spf13/cobra"
)

func init() {
	stackCmd.Flags().String("on", "", "parent branch")
	_ = stackCmd.MarkFlagRequired("on")
	rootCmd.AddCommand(stackCmd)
}

var stackCmd = &cobra.Command{
	Use:   "stack <branch> --on <parent>",
	Short: "Declare that a branch depends on another branch",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		branch := args[0]
		parent, err := cmd.Flags().GetString("on")
		if err != nil {
			return err
		}
		if branch == parent {
			return markUsage(fmt.Errorf("branch %q cannot depend on itself", branch))
		}

		ctx := context.Background()
		source := deps.NewLocalSource(AppContext().Runner.RepoRoot())
		existing, err := source.Load(ctx)
		if err != nil {
			return err
		}

		proposed := stackpkg.UpsertDependency(existing, stackpkg.Dependency{
			Branch: branch,
			Parent: parent,
		})
		if _, err := stackpkg.NewDAG(proposed); err != nil {
			return err
		}

		if err := source.Set(ctx, branch, parent); err != nil {
			return err
		}

		fmt.Fprintf(cmd.OutOrStdout(), "stacked %s on %s\n", branch, parent)
		return nil
	},
}
