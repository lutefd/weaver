package cmd

import (
	"fmt"

	"github.com/lutefd/weaver/internal/config"
	"github.com/lutefd/weaver/internal/ui"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(initCmd)
}

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize Weaver metadata for this repository",
	RunE: func(cmd *cobra.Command, _ []string) error {
		repoRoot := AppContext().Runner.RepoRoot()
		created, err := config.Initialize(repoRoot)
		if err != nil {
			return err
		}

		term := terminalFor(cmd)
		if created {
			if term.Styled() {
				writeLine(cmd.OutOrStdout(), renderActionCard(term, ui.ToneSuccess, "Weaver Initialized", "Repository metadata is ready", []ui.KeyValue{{Label: "repo", Value: repoRoot}}, nil))
				return nil
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Initialized Weaver in %s\n", repoRoot)
			return nil
		}

		if term.Styled() {
			writeLine(cmd.OutOrStdout(), renderActionCard(term, ui.ToneInfo, "Already Initialized", "Repository metadata already exists", []ui.KeyValue{{Label: "repo", Value: repoRoot}}, nil))
			return nil
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Weaver already initialized in %s\n", repoRoot)
		return nil
	},
}
