package cmd

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/lutefd/weaver/internal/updater"
	"github.com/spf13/cobra"
)

func init() {
	updateCmd.Flags().String("group", "", "update all branches in a named group")
	updateCmd.Flags().String("integration", "", "update all branches in a saved integration strategy")
	updateCmd.Flags().Bool("all", false, "update every tracked branch")
	rootCmd.AddCommand(updateCmd)
}

var updateCmd = &cobra.Command{
	Use:   "update [branch...]",
	Short: "Fast-forward local branches to their upstream refs",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()
		selection, err := resolveBranchSelection(AppContext().Runner.RepoRoot(), args, cmd)
		if err != nil {
			return err
		}

		result, err := updater.New(AppContext().Runner).Update(ctx, selection.Branches)
		if err != nil {
			var missingBranchErr updater.MissingBranchError
			if errors.As(err, &missingBranchErr) {
				return err
			}
			var missingUpstreamErr updater.MissingUpstreamError
			if errors.As(err, &missingUpstreamErr) {
				return err
			}
			var ffErr updater.FastForwardError
			if errors.As(err, &ffErr) {
				return fmt.Errorf("update stopped at %s: branch is not a fast-forward of %s", ffErr.Branch, ffErr.Upstream)
			}
			return err
		}

		parts := make([]string, 0, 2)
		if len(result.Updated) > 0 {
			parts = append(parts, fmt.Sprintf("updated %s", strings.Join(result.Updated, ", ")))
		}
		if len(result.UpToDate) > 0 {
			parts = append(parts, fmt.Sprintf("already current %s", strings.Join(result.UpToDate, ", ")))
		}
		if len(parts) == 0 {
			parts = append(parts, "no branches changed")
		}

		fmt.Fprintln(cmd.OutOrStdout(), strings.Join(parts, "; "))
		return nil
	},
}
