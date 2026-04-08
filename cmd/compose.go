package cmd

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/lutefd/weaver/internal/composer"
	"github.com/lutefd/weaver/internal/deps"
	"github.com/lutefd/weaver/internal/resolver"
	"github.com/spf13/cobra"
)

func init() {
	composeCmd.Flags().String("group", "", "compose all branches in a named group")
	composeCmd.Flags().String("integration", "", "compose branches from a saved integration strategy")
	composeCmd.Flags().Bool("all", false, "compose every tracked branch")
	composeCmd.Flags().String("base", "", "base branch to compose onto")
	composeCmd.Flags().String("create", "", "create a new branch with the composed result")
	composeCmd.Flags().String("update", "", "update an integration branch with a fresh composed result rebuilt from the base")
	composeCmd.Flags().Bool("dry-run", false, "print the compose order without mutating git state")
	rootCmd.AddCommand(composeCmd)
}

var composeCmd = &cobra.Command{
	Use:   "compose [branch...]",
	Short: "Compose one or more branches into an integration state",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()
		selection, err := resolveBranchSelection(AppContext().Runner.RepoRoot(), args, cmd)
		if err != nil {
			return err
		}

		dag, err := resolver.New(deps.NewLocalSource(AppContext().Runner.RepoRoot())).Resolve(ctx)
		if err != nil {
			return err
		}

		base, err := cmd.Flags().GetString("base")
		if err != nil {
			return err
		}
		if selection.IntegrationName != "" && base != "" {
			return markUsage(fmt.Errorf("saved integration %q already defines base %q; omit --base", selection.IntegrationName, selection.Base))
		}
		if base == "" {
			base = selection.Base
		}
		if base == "" {
			base = AppContext().Config.DefaultBase
		}

		composeOpts, err := resolveComposeOptions(cmd, base)
		if err != nil {
			return err
		}

		result, err := composer.New(AppContext().Runner).Compose(ctx, dag, selection.Branches, base, composeOpts)
		if err != nil {
			var conflictErr composer.ConflictError
			if errors.As(err, &conflictErr) {
				return fmt.Errorf("compose failed while merging %s", conflictErr.Branch)
			}
			return err
		}

		if result.DryRun {
			switch {
			case result.CreatedBranch != "":
				fmt.Fprintf(cmd.OutOrStdout(), "dry-run compose on %s and create %s: %s\n", result.BaseBranch, result.CreatedBranch, strings.Join(result.Order, " -> "))
			case result.UpdatedBranch != "":
				fmt.Fprintf(cmd.OutOrStdout(), "dry-run compose on %s and update %s: %s\n", result.BaseBranch, result.UpdatedBranch, strings.Join(result.Order, " -> "))
			default:
				fmt.Fprintf(cmd.OutOrStdout(), "dry-run ephemeral compose on %s: %s\n", result.BaseBranch, strings.Join(result.Order, " -> "))
			}
			return nil
		}

		if result.CreatedBranch != "" {
			fmt.Fprintf(cmd.OutOrStdout(), "created %s from %s with: %s\n", result.CreatedBranch, result.BaseBranch, strings.Join(result.Order, " -> "))
			return nil
		}

		if result.UpdatedBranch != "" {
			fmt.Fprintf(cmd.OutOrStdout(), "updated %s from %s with: %s\n", result.UpdatedBranch, result.BaseBranch, strings.Join(result.Order, " -> "))
			return nil
		}

		fmt.Fprintf(cmd.OutOrStdout(), "composed ephemerally on %s: %s\n", result.BaseBranch, strings.Join(result.Order, " -> "))
		return nil
	},
}

func resolveComposeOptions(cmd *cobra.Command, base string) (composer.ComposeOpts, error) {
	dryRun, err := cmd.Flags().GetBool("dry-run")
	if err != nil {
		return composer.ComposeOpts{}, err
	}
	createBranch, err := cmd.Flags().GetString("create")
	if err != nil {
		return composer.ComposeOpts{}, err
	}
	updateBranch, err := cmd.Flags().GetString("update")
	if err != nil {
		return composer.ComposeOpts{}, err
	}
	writeModes := 0
	if createBranch != "" {
		writeModes++
	}
	if updateBranch != "" {
		writeModes++
	}
	if writeModes > 1 {
		return composer.ComposeOpts{}, markUsage(fmt.Errorf("use only one of --create or --update"))
	}
	if createBranch == base {
		return composer.ComposeOpts{}, markUsage(fmt.Errorf("--create branch must differ from --base"))
	}
	if updateBranch == base {
		return composer.ComposeOpts{}, markUsage(fmt.Errorf("--update branch must differ from --base"))
	}

	return composer.ComposeOpts{
		DryRun:       dryRun,
		CreateBranch: createBranch,
		UpdateBranch: updateBranch,
	}, nil
}
