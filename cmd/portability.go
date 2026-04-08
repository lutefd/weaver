package cmd

import (
	"fmt"

	"github.com/lutefd/weaver/internal/portability"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(exportCmd)
	rootCmd.AddCommand(importCmd)
}

var exportCmd = &cobra.Command{
	Use:   "export",
	Short: "Export local deps, groups, and integrations as portable JSON",
	RunE: func(cmd *cobra.Command, _ []string) error {
		state, err := portability.New(AppContext().Runner.RepoRoot()).Export()
		if err != nil {
			return err
		}
		return portability.Encode(cmd.OutOrStdout(), state)
	},
}

var importCmd = &cobra.Command{
	Use:   "import <file>",
	Short: "Import local deps, groups, and integrations from portable JSON",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		state, err := portability.LoadFile(args[0])
		if err != nil {
			return err
		}
		if err := portability.New(AppContext().Runner.RepoRoot()).Import(state); err != nil {
			return err
		}
		fmt.Fprintf(cmd.OutOrStdout(), "imported %s\n", args[0])
		return nil
	},
}
