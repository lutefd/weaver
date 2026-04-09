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
	rootCmd.AddCommand(abortCmd)
}

var abortCmd = &cobra.Command{
	Use:   "abort",
	Short: "Abort a paused stack sync and restore the original branch",
	RunE: func(cmd *cobra.Command, _ []string) error {
		repoRoot := AppContext().Runner.RepoRoot()
		rebasePending := rebaser.NewStateStore(repoRoot).HasPending()
		mergePending := merger.NewStateStore(repoRoot).HasPending()

		switch {
		case rebasePending && mergePending:
			return fmt.Errorf("both rebase and merge stack sync state are pending; repair .git/weaver state before aborting")
		case mergePending:
			if _, err := runTask(context.Background(), cmd, ui.TaskSpec{
				Title:    "Aborting Sync",
				Subtitle: "Restoring branch after merge-based sync",
			}, func(ctx context.Context, runner gitrunner.Runner) (struct{}, error) {
				return struct{}{}, merger.New(runner).Abort(ctx)
			}); err != nil {
				return err
			}
		case rebasePending:
			if _, err := runTask(context.Background(), cmd, ui.TaskSpec{
				Title:    "Aborting Sync",
				Subtitle: "Restoring branch after rebase-based sync",
			}, func(ctx context.Context, runner gitrunner.Runner) (struct{}, error) {
				return struct{}{}, rebaser.New(runner).Abort(ctx)
			}); err != nil {
				return err
			}
		default:
			return fmt.Errorf("no pending stack sync found")
		}

		term := terminalFor(cmd)
		if term.Styled() {
			writeLine(cmd.OutOrStdout(), renderActionCard(term, ui.ToneWarn, "Sync Aborted", "Paused stack sync was cancelled", nil, nil))
			return nil
		}

		fmt.Fprintln(cmd.OutOrStdout(), "aborted stack sync")
		return nil
	},
}
