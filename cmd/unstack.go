package cmd

import (
	"context"
	"fmt"

	"github.com/lutefd/weaver/internal/deps"
	"github.com/lutefd/weaver/internal/ui"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(unstackCmd)
}

var unstackCmd = &cobra.Command{
	Use:   "unstack <branch>",
	Short: "Remove a branch dependency declaration",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		branch := args[0]
		source := deps.NewLocalSource(AppContext().Runner.RepoRoot())
		if err := source.Remove(context.Background(), branch); err != nil {
			return err
		}

		term := terminalFor(cmd)
		if term.Styled() {
			writeLine(cmd.OutOrStdout(), renderActionCard(term, ui.ToneSuccess, "Dependency Removed", "Branch no longer depends on a parent", []ui.KeyValue{{Label: "branch", Value: branch}}, nil))
			return nil
		}

		fmt.Fprintf(cmd.OutOrStdout(), "unstacked %s\n", branch)
		return nil
	},
}
