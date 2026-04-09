package cmd

import (
	"context"
	"fmt"

	"github.com/lutefd/weaver/internal/deps"
	gitrunner "github.com/lutefd/weaver/internal/git"
	"github.com/lutefd/weaver/internal/resolver"
	"github.com/lutefd/weaver/internal/stack"
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
		dag, err := runTask(ctx, cmd, ui.TaskSpec{
			Title:    "Loading Dependencies",
			Subtitle: "Resolving the current stack graph",
		}, func(ctx context.Context, runner gitrunner.Runner) (*stack.DAG, error) {
			return resolver.New(deps.NewLocalSource(runner.RepoRoot())).Resolve(ctx)
		})
		if err != nil {
			return err
		}

		base := AppContext().Config.DefaultBase
		term := terminalFor(cmd)
		if len(args) == 1 {
			if term.Styled() {
				chain, err := ui.RenderStyledChain(term, dag, base, args[0])
				if err != nil {
					return err
				}
				writeLine(cmd.OutOrStdout(), renderTreeCard(term, "Dependency Chain", "Ancestors from base to branch", chain))
				return nil
			}

			chain, err := ui.RenderChain(dag, base, args[0])
			if err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), chain)
			return nil
		}

		if term.Styled() {
			writeLine(cmd.OutOrStdout(), renderTreeCard(term, "Dependency Tree", "Tracked branch relationships", ui.RenderStyledTree(term, dag, base)))
			return nil
		}

		fmt.Fprintln(cmd.OutOrStdout(), ui.RenderTree(dag, base))
		return nil
	},
}
