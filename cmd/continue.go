package cmd

import (
	"context"
	"fmt"

	"github.com/lutefd/weaver/internal/rebaser"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(continueCmd)
}

var continueCmd = &cobra.Command{
	Use:   "continue",
	Short: "Continue a paused stack rebase after resolving conflicts",
	RunE: func(cmd *cobra.Command, _ []string) error {
		result, err := rebaser.New(AppContext().Runner).Continue(context.Background())
		if err != nil {
			return err
		}

		if result.Conflict {
			return fmt.Errorf("rebase is still blocked at %s onto %s", result.Current, result.CurrentOnto)
		}

		fmt.Fprintln(cmd.OutOrStdout(), "continued stack sync")
		return nil
	},
}
