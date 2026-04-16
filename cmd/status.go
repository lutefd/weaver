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
	statusCmd.Flags().Bool("upstream", false, "fetch remotes and compare each tracked branch with its configured upstream")
	rootCmd.AddCommand(statusCmd)
}

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show the current stack tree and branch health",
	RunE: func(cmd *cobra.Command, _ []string) error {
		ctx := context.Background()
		type statusPayload struct {
			dag            *stack.DAG
			health         map[string]stack.StackHealth
			upstreamHealth map[string]stack.UpstreamHealth
		}

		base := AppContext().Config.DefaultBase
		upstreamMode, err := cmd.Flags().GetBool("upstream")
		if err != nil {
			return err
		}

		payload, err := runTask(ctx, cmd, ui.TaskSpec{
			Title:    statusTaskTitle(upstreamMode),
			Subtitle: statusTaskSubtitle(upstreamMode),
		}, func(ctx context.Context, runner gitrunner.Runner) (*statusPayload, error) {
			dag, err := resolver.New(deps.NewLocalSource(runner.RepoRoot())).Resolve(ctx)
			if err != nil {
				return nil, err
			}
			if upstreamMode {
				if _, err := runner.Run(ctx, "fetch", "--all"); err != nil {
					return nil, err
				}
				health, err := stack.ComputeUpstreamHealth(ctx, runner, dag, base)
				if err != nil {
					return nil, err
				}
				return &statusPayload{dag: dag, upstreamHealth: health}, nil
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
		if upstreamMode {
			if term.Styled() {
				writeLine(cmd.OutOrStdout(), renderTreeCard(term, "Upstream Status", "Dependency tree with badges relative to each branch upstream", ui.RenderStyledUpstreamStatusTree(term, payload.dag, base, payload.upstreamHealth)))
				return nil
			}
			fmt.Fprintln(cmd.OutOrStdout(), ui.RenderUpstreamStatusTree(payload.dag, base, payload.upstreamHealth))
			return nil
		}
		if term.Styled() {
			writeLine(cmd.OutOrStdout(), renderTreeCard(term, "Stack Status", "Dependency tree with health badges relative to each stack parent", ui.RenderStyledStatusTree(term, payload.dag, base, payload.health)))
			return nil
		}

		fmt.Fprintln(cmd.OutOrStdout(), ui.RenderStatusTree(payload.dag, base, payload.health))
		return nil
	},
}

func statusTaskTitle(upstream bool) string {
	if upstream {
		return "Checking Upstream Status"
	}
	return "Checking Stack Status"
}

func statusTaskSubtitle(upstream bool) string {
	if upstream {
		return "Fetching remotes and calculating branch drift against upstream"
	}
	return "Calculating branch health and merge risk"
}
