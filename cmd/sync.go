package cmd

import (
	"context"
	"fmt"

	"github.com/lutefd/weaver/internal/deps"
	gitrunner "github.com/lutefd/weaver/internal/git"
	"github.com/lutefd/weaver/internal/merger"
	"github.com/lutefd/weaver/internal/rebaser"
	"github.com/lutefd/weaver/internal/resolver"
	"github.com/lutefd/weaver/internal/stack"
	"github.com/lutefd/weaver/internal/ui"
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
		dag, err := runTask(ctx, cmd, ui.TaskSpec{
			Title:    "Preparing Sync",
			Subtitle: "Resolving stack order before applying changes",
		}, func(ctx context.Context, runner gitrunner.Runner) (*stack.DAG, error) {
			return resolver.New(deps.NewLocalSource(runner.RepoRoot())).Resolve(ctx)
		})
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
		term := terminalFor(cmd)
		if useMerge {
			result, err := runTask(ctx, cmd, ui.TaskSpec{
				Title:    "Syncing Stack",
				Subtitle: fmt.Sprintf("Merging parents into %s", branch),
			}, func(ctx context.Context, runner gitrunner.Runner) (*merger.MergeResult, error) {
				return merger.New(runner).MergeStack(ctx, dag, []string{branch}, AppContext().Config.DefaultBase)
			})
			if err != nil {
				return err
			}
			if result.Conflict {
				return fmt.Errorf("merge stopped at %s onto %s; resolve conflicts and run `weaver continue` or `weaver abort`", result.Current, result.CurrentOnto)
			}
			if term.Styled() {
				writeLine(cmd.OutOrStdout(), renderSyncResultStyled(term, "merge", branch, result.Completed))
				return nil
			}
		} else {
			result, err := runTask(ctx, cmd, ui.TaskSpec{
				Title:    "Syncing Stack",
				Subtitle: fmt.Sprintf("Rebasing ancestors for %s", branch),
			}, func(ctx context.Context, runner gitrunner.Runner) (*rebaser.RebaseResult, error) {
				return rebaser.New(runner).RebaseStack(ctx, dag, []string{branch}, AppContext().Config.DefaultBase)
			})
			if err != nil {
				return err
			}

			if result.Conflict {
				return fmt.Errorf("rebase stopped at %s onto %s; resolve conflicts and run `weaver continue` or `weaver abort`", result.Current, result.CurrentOnto)
			}
			if term.Styled() {
				writeLine(cmd.OutOrStdout(), renderSyncResultStyled(term, "rebase", branch, result.Completed))
				return nil
			}
		}

		fmt.Fprintf(cmd.OutOrStdout(), "synced %s\n", branch)
		return nil
	},
}
