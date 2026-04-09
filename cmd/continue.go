package cmd

import (
	"context"
	"fmt"

	gitrunner "github.com/lutefd/weaver/internal/git"
	"github.com/lutefd/weaver/internal/merger"
	"github.com/lutefd/weaver/internal/rebaser"
	"github.com/lutefd/weaver/internal/ui"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(continueCmd)
}

var continueCmd = &cobra.Command{
	Use:   "continue",
	Short: "Continue a paused stack sync after resolving conflicts",
	RunE: func(cmd *cobra.Command, _ []string) error {
		repoRoot := AppContext().Runner.RepoRoot()
		rebasePending := rebaser.NewStateStore(repoRoot).HasPending()
		mergePending := merger.NewStateStore(repoRoot).HasPending()

		switch {
		case rebasePending && mergePending:
			return fmt.Errorf("both rebase and merge stack sync state are pending; repair .git/weaver state before continuing")
		case mergePending:
			result, err := runTask(context.Background(), cmd, ui.TaskSpec{
				Title:    "Continuing Sync",
				Subtitle: "Resuming merge-based stack sync",
			}, func(ctx context.Context, runner gitrunner.Runner) (*merger.MergeResult, error) {
				return merger.New(runner).Continue(ctx)
			})
			if err != nil {
				return err
			}
			if result.Conflict {
				return fmt.Errorf("merge is still blocked at %s onto %s", result.Current, result.CurrentOnto)
			}
		case rebasePending:
			result, err := runTask(context.Background(), cmd, ui.TaskSpec{
				Title:    "Continuing Sync",
				Subtitle: "Resuming rebase-based stack sync",
			}, func(ctx context.Context, runner gitrunner.Runner) (*rebaser.RebaseResult, error) {
				return rebaser.New(runner).Continue(ctx)
			})
			if err != nil {
				return err
			}
			if result.Conflict {
				return fmt.Errorf("rebase is still blocked at %s onto %s", result.Current, result.CurrentOnto)
			}
		default:
			return fmt.Errorf("no pending stack sync found")
		}

		term := terminalFor(cmd)
		if term.Styled() {
			writeLine(cmd.OutOrStdout(), renderActionCard(term, ui.ToneSuccess, "Sync Continued", "Paused stack sync resumed successfully", nil, nil))
			return nil
		}

		fmt.Fprintln(cmd.OutOrStdout(), "continued stack sync")
		return nil
	},
}
