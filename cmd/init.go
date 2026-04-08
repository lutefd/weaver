package cmd

import (
	"fmt"

	"github.com/lutefd/weaver/internal/config"
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

		if created {
			fmt.Fprintf(cmd.OutOrStdout(), "Initialized Weaver in %s\n", repoRoot)
			return nil
		}

		fmt.Fprintf(cmd.OutOrStdout(), "Weaver already initialized in %s\n", repoRoot)
		return nil
	},
}
