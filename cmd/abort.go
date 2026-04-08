package cmd

import (
	"context"
	"fmt"

	"github.com/lutefd/weaver/internal/merger"
	"github.com/lutefd/weaver/internal/rebaser"
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
			if err := merger.New(AppContext().Runner).Abort(context.Background()); err != nil {
				return err
			}
		case rebasePending:
			if err := rebaser.New(AppContext().Runner).Abort(context.Background()); err != nil {
				return err
			}
		default:
			return fmt.Errorf("no pending stack sync found")
		}

		fmt.Fprintln(cmd.OutOrStdout(), "aborted stack sync")
		return nil
	},
}
