package cmd

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/lutefd/weaver/internal/composer"
	"github.com/lutefd/weaver/internal/deps"
	"github.com/lutefd/weaver/internal/group"
	"github.com/lutefd/weaver/internal/resolver"
	"github.com/spf13/cobra"
)

func init() {
	composeCmd.Flags().String("group", "", "compose all branches in a named group")
	composeCmd.Flags().Bool("all", false, "compose every tracked branch")
	composeCmd.Flags().String("base", "", "base branch to compose onto")
	composeCmd.Flags().String("create", "", "create a new branch with the composed result")
	composeCmd.Flags().Bool("dry-run", false, "print the compose order without mutating git state")
	composeCmd.Flags().Bool("persist", false, "update the base branch to the composed result")
	rootCmd.AddCommand(composeCmd)
}

var composeCmd = &cobra.Command{
	Use:   "compose [branch...]",
	Short: "Compose one or more branches into an integration state",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()
		selected, err := resolveComposeBranches(AppContext().Runner.RepoRoot(), args, cmd)
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
		if base == "" {
			base = AppContext().Config.DefaultBase
		}

		composeOpts, err := resolveComposeOptions(cmd, base)
		if err != nil {
			return err
		}

		result, err := composer.New(AppContext().Runner).Compose(ctx, dag, selected, base, composeOpts)
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
			case composeOpts.Persist:
				fmt.Fprintf(cmd.OutOrStdout(), "dry-run persistent compose on %s: %s\n", result.BaseBranch, strings.Join(result.Order, " -> "))
			default:
				fmt.Fprintf(cmd.OutOrStdout(), "dry-run ephemeral compose on %s: %s\n", result.BaseBranch, strings.Join(result.Order, " -> "))
			}
			return nil
		}

		if result.CreatedBranch != "" {
			fmt.Fprintf(cmd.OutOrStdout(), "created %s from %s with: %s\n", result.CreatedBranch, result.BaseBranch, strings.Join(result.Order, " -> "))
			return nil
		}

		if result.Persisted {
			fmt.Fprintf(cmd.OutOrStdout(), "updated %s with: %s\n", result.BaseBranch, strings.Join(result.Order, " -> "))
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
	persist, err := cmd.Flags().GetBool("persist")
	if err != nil {
		return composer.ComposeOpts{}, err
	}
	createBranch, err := cmd.Flags().GetString("create")
	if err != nil {
		return composer.ComposeOpts{}, err
	}
	if persist && createBranch != "" {
		return composer.ComposeOpts{}, markUsage(fmt.Errorf("use either --persist or --create"))
	}
	if createBranch == base {
		return composer.ComposeOpts{}, markUsage(fmt.Errorf("--create branch must differ from --base"))
	}

	return composer.ComposeOpts{
		DryRun:       dryRun,
		Persist:      persist,
		CreateBranch: createBranch,
	}, nil
}

func resolveComposeBranches(repoRoot string, args []string, cmd *cobra.Command) ([]string, error) {
	groupName, err := cmd.Flags().GetString("group")
	if err != nil {
		return nil, err
	}
	all, err := cmd.Flags().GetBool("all")
	if err != nil {
		return nil, err
	}

	selectedModes := 0
	if len(args) > 0 {
		selectedModes++
	}
	if groupName != "" {
		selectedModes++
	}
	if all {
		selectedModes++
	}
	if selectedModes != 1 {
		return nil, markUsage(fmt.Errorf("provide explicit branches, --group, or --all"))
	}

	if len(args) > 0 {
		return append([]string(nil), args...), nil
	}
	if groupName != "" {
		branches, ok, err := group.NewStore(repoRoot).Get(groupName)
		if err != nil {
			return nil, err
		}
		if !ok {
			return nil, fmt.Errorf("group %q does not exist", groupName)
		}
		if len(branches) == 0 {
			return nil, fmt.Errorf("group %q is empty", groupName)
		}
		return branches, nil
	}

	dependencies, err := deps.NewLocalSource(repoRoot).Load(context.Background())
	if err != nil {
		return nil, err
	}
	if len(dependencies) == 0 {
		return nil, fmt.Errorf("no tracked branches found")
	}

	branches := make([]string, 0, len(dependencies))
	for _, dependency := range dependencies {
		branches = append(branches, dependency.Branch)
	}
	return branches, nil
}
