package cmd

import (
	"context"
	"fmt"

	"github.com/lutefd/weaver/internal/deps"
	"github.com/lutefd/weaver/internal/resolver"
	"github.com/lutefd/weaver/internal/ui"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(depsCmd)
}

var depsCmd = &cobra.Command{
	Use:   "deps [branch]",
	Short: "Show declared stack dependencies",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()
		source := deps.NewLocalSource(AppContext().Runner.RepoRoot())
		dag, err := resolver.New(source).Resolve(ctx)
		if err != nil {
			return err
		}

		base := AppContext().Config.DefaultBase
		if len(args) == 1 {
			chain, err := ui.RenderChain(dag, base, args[0])
			if err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), chain)
			return nil
		}

		fmt.Fprintln(cmd.OutOrStdout(), ui.RenderTree(dag, base))
		return nil
	},
}
