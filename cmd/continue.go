package cmd

import (
	"context"
	"fmt"

	"github.com/lutefd/weaver/internal/merger"
	"github.com/lutefd/weaver/internal/rebaser"
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
			result, err := merger.New(AppContext().Runner).Continue(context.Background())
			if err != nil {
				return err
			}
			if result.Conflict {
				return fmt.Errorf("merge is still blocked at %s onto %s", result.Current, result.CurrentOnto)
			}
		case rebasePending:
			result, err := rebaser.New(AppContext().Runner).Continue(context.Background())
			if err != nil {
				return err
			}
			if result.Conflict {
				return fmt.Errorf("rebase is still blocked at %s onto %s", result.Current, result.CurrentOnto)
			}
		default:
			return fmt.Errorf("no pending stack sync found")
		}

		fmt.Fprintln(cmd.OutOrStdout(), "continued stack sync")
		return nil
	},
}
