package cmd

import (
	"context"
	"fmt"

	"github.com/lutefd/weaver/internal/deps"
	"github.com/lutefd/weaver/internal/merger"
	"github.com/lutefd/weaver/internal/rebaser"
	"github.com/lutefd/weaver/internal/resolver"
	"github.com/spf13/cobra"
)

func init() {
	syncCmd.Flags().Bool("merge", false, "merge each parent into the stack instead of rebasing it")
	rootCmd.AddCommand(syncCmd)
}

var syncCmd = &cobra.Command{
	Use:   "sync [branch]",
	Short: "Sync a branch stack in dependency order",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()
		source := deps.NewLocalSource(AppContext().Runner.RepoRoot())
		dag, err := resolver.New(source).Resolve(ctx)
		if err != nil {
			return err
		}

		branch := ""
		if len(args) == 1 {
			branch = args[0]
		} else {
			branch, err = currentBranchName(ctx)
			if err != nil {
				return err
			}
		}

		useMerge, err := cmd.Flags().GetBool("merge")
		if err != nil {
			return err
		}
		if useMerge {
			result, err := merger.New(AppContext().Runner).MergeStack(ctx, dag, []string{branch}, AppContext().Config.DefaultBase)
			if err != nil {
				return err
			}
			if result.Conflict {
				return fmt.Errorf("merge stopped at %s onto %s; resolve conflicts and run `weaver continue` or `weaver abort`", result.Current, result.CurrentOnto)
			}
		} else {
			result, err := rebaser.New(AppContext().Runner).RebaseStack(ctx, dag, []string{branch}, AppContext().Config.DefaultBase)
			if err != nil {
				return err
			}

			if result.Conflict {
				return fmt.Errorf("rebase stopped at %s onto %s; resolve conflicts and run `weaver continue` or `weaver abort`", result.Current, result.CurrentOnto)
			}
		}

		fmt.Fprintf(cmd.OutOrStdout(), "synced %s\n", branch)
		return nil
	},
}
