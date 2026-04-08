package cmd

import (
	"context"
	"fmt"

	"github.com/lutefd/weaver/internal/rebaser"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(abortCmd)
}

var abortCmd = &cobra.Command{
	Use:   "abort",
	Short: "Abort a paused stack rebase and restore the original branch",
	RunE: func(cmd *cobra.Command, _ []string) error {
		if err := rebaser.New(AppContext().Runner).Abort(context.Background()); err != nil {
			return err
		}

		fmt.Fprintln(cmd.OutOrStdout(), "aborted stack sync")
		return nil
	},
}
