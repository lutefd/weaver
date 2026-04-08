package cmd

import (
	"context"
	"fmt"

	"github.com/lutefd/weaver/internal/deps"
	"github.com/lutefd/weaver/internal/resolver"
	"github.com/lutefd/weaver/internal/stack"
	"github.com/lutefd/weaver/internal/ui"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(statusCmd)
}

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show the current stack tree and branch health",
	RunE: func(cmd *cobra.Command, _ []string) error {
		ctx := context.Background()
		source := deps.NewLocalSource(AppContext().Runner.RepoRoot())
		dag, err := resolver.New(source).Resolve(ctx)
		if err != nil {
			return err
		}

		base := AppContext().Config.DefaultBase
		health, err := stack.ComputeHealth(ctx, AppContext().Runner, dag, base)
		if err != nil {
			return err
		}

		fmt.Fprintln(cmd.OutOrStdout(), ui.RenderStatusTree(dag, base, health))
		return nil
	},
}
