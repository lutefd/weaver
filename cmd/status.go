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
	rootCmd.AddCommand(statusCmd)
}

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show the current stack tree and branch health",
	RunE: func(cmd *cobra.Command, _ []string) error {
		ctx := context.Background()
		type statusPayload struct {
			dag    *stack.DAG
			health map[string]stack.StackHealth
		}

		base := AppContext().Config.DefaultBase
		payload, err := runTask(ctx, cmd, ui.TaskSpec{
			Title:    "Checking Stack Status",
			Subtitle: "Calculating branch health and merge risk",
		}, func(ctx context.Context, runner gitrunner.Runner) (*statusPayload, error) {
			dag, err := resolver.New(deps.NewLocalSource(runner.RepoRoot())).Resolve(ctx)
			if err != nil {
				return nil, err
			}
			health, err := stack.ComputeHealth(ctx, runner, dag, base)
			if err != nil {
				return nil, err
			}
			return &statusPayload{dag: dag, health: health}, nil
		})
		if err != nil {
			return err
		}

		term := terminalFor(cmd)
		if term.Styled() {
			writeLine(cmd.OutOrStdout(), renderTreeCard(term, "Stack Status", "Dependency tree with health badges relative to each stack parent", ui.RenderStyledStatusTree(term, payload.dag, base, payload.health)))
			return nil
		}

		fmt.Fprintln(cmd.OutOrStdout(), ui.RenderStatusTree(payload.dag, base, payload.health))
		return nil
	},
}
