package cmd

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/lutefd/weaver/internal/composer"
	"github.com/lutefd/weaver/internal/deps"
	"github.com/lutefd/weaver/internal/resolver"
	"github.com/lutefd/weaver/internal/stack"
	"github.com/spf13/cobra"
)

func init() {
	composeCmd.Flags().String("group", "", "compose all branches in a named group")
	composeCmd.Flags().String("integration", "", "compose branches from a saved integration strategy")
	composeCmd.Flags().Bool("all", false, "compose every tracked branch")
	composeCmd.Flags().String("base", "", "base branch to compose onto")
	composeCmd.Flags().String("create", "", "create a new branch with the composed result")
	composeCmd.Flags().String("update", "", "update an integration branch with a fresh composed result rebuilt from the base")
	composeCmd.Flags().StringSlice("skip", nil, "skip one or more resolved branches and leave them for manual merge later")
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

		result, err := runComposeWithRecovery(ctx, cmd, dag, selection.Branches, base, composeOpts)
		if err != nil {
			return err
		}

		if result.DryRun {
			switch {
			case result.CreatedBranch != "":
				fmt.Fprintf(cmd.OutOrStdout(), "dry-run compose on %s and create %s: %s%s\n", result.BaseBranch, result.CreatedBranch, strings.Join(result.Order, " -> "), formatSkipped(result.Skipped))
			case result.UpdatedBranch != "":
				fmt.Fprintf(cmd.OutOrStdout(), "dry-run compose on %s and update %s: %s%s\n", result.BaseBranch, result.UpdatedBranch, strings.Join(result.Order, " -> "), formatSkipped(result.Skipped))
			default:
				fmt.Fprintf(cmd.OutOrStdout(), "dry-run ephemeral compose on %s: %s%s\n", result.BaseBranch, strings.Join(result.Order, " -> "), formatSkipped(result.Skipped))
			}
			return nil
		}

		if result.CreatedBranch != "" {
			fmt.Fprintf(cmd.OutOrStdout(), "created %s from %s with: %s%s\n", result.CreatedBranch, result.BaseBranch, strings.Join(result.Order, " -> "), formatSkipped(result.Skipped))
			renderManualMergeSummary(cmd.OutOrStdout(), result)
			return nil
		}

		if result.UpdatedBranch != "" {
			fmt.Fprintf(cmd.OutOrStdout(), "updated %s from %s with: %s%s\n", result.UpdatedBranch, result.BaseBranch, strings.Join(result.Order, " -> "), formatSkipped(result.Skipped))
			renderManualMergeSummary(cmd.OutOrStdout(), result)
			return nil
		}

		fmt.Fprintf(cmd.OutOrStdout(), "composed ephemerally on %s: %s%s\n", result.BaseBranch, strings.Join(result.Order, " -> "), formatSkipped(result.Skipped))
		renderManualMergeSummary(cmd.OutOrStdout(), result)
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
	skipBranches, err := cmd.Flags().GetStringSlice("skip")
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
		SkipBranches: skipBranches,
	}, nil
}

func formatSkipped(skipped []string) string {
	if len(skipped) == 0 {
		return ""
	}
	return fmt.Sprintf(" (skipped: %s)", strings.Join(skipped, ", "))
}

func runComposeWithRecovery(ctx context.Context, cmd *cobra.Command, dag *stack.DAG, branches []string, base string, opts composer.ComposeOpts) (*composer.ComposeResult, error) {
	engine := composer.New(AppContext().Runner)
	for {
		result, err := engine.Compose(ctx, dag, branches, base, opts)
		if err == nil {
			return result, nil
		}

		var conflictErr composer.ConflictError
		if !errors.As(err, &conflictErr) {
			return nil, err
		}
		if opts.DryRun {
			return nil, formatComposeConflictError(conflictErr)
		}

		shouldSkip, promptErr := promptSkipOnComposeConflict(cmd, conflictErr)
		if promptErr != nil {
			return nil, promptErr
		}
		if !shouldSkip {
			return nil, formatComposeConflictError(conflictErr)
		}

		opts.SkipBranches = appendUniqueBranch(opts.SkipBranches, conflictErr.Branch)
		fmt.Fprintf(cmd.OutOrStdout(), "skipping %s and retrying compose; merge it manually later\n", conflictErr.Branch)
	}
}

func formatComposeConflictError(conflictErr composer.ConflictError) error {
	if len(conflictErr.Files) > 0 {
		return fmt.Errorf("compose failed while merging %s (conflicts: %s)", conflictErr.Branch, strings.Join(conflictErr.Files, ", "))
	}
	return fmt.Errorf("compose failed while merging %s", conflictErr.Branch)
}

func promptSkipOnComposeConflict(cmd *cobra.Command, conflictErr composer.ConflictError) (bool, error) {
	reader := bufio.NewReader(cmd.InOrStdin())
	for {
		if len(conflictErr.Files) > 0 {
			fmt.Fprintf(cmd.OutOrStdout(), "compose conflict while merging %s; conflicted files: %s\n", conflictErr.Branch, strings.Join(conflictErr.Files, ", "))
		} else {
			fmt.Fprintf(cmd.OutOrStdout(), "compose conflict while merging %s\n", conflictErr.Branch)
		}
		fmt.Fprintf(cmd.OutOrStdout(), "skip %s and continue, or abort? [skip/abort]: ", conflictErr.Branch)

		answer, err := reader.ReadString('\n')
		if err != nil && !errors.Is(err, io.EOF) {
			return false, err
		}
		answer = strings.ToLower(strings.TrimSpace(answer))
		switch answer {
		case "skip", "s":
			return true, nil
		case "abort", "a", "":
			return false, nil
		}
		fmt.Fprintf(cmd.OutOrStdout(), "please answer skip or abort\n")
		if errors.Is(err, io.EOF) {
			return false, nil
		}
	}
}

func renderManualMergeSummary(w io.Writer, result *composer.ComposeResult) {
	if result == nil || len(result.Skipped) == 0 {
		return
	}

	switch {
	case result.CreatedBranch != "":
		fmt.Fprintf(w, "manual merge required onto %s: %s\n", result.CreatedBranch, strings.Join(result.Skipped, ", "))
	case result.UpdatedBranch != "":
		fmt.Fprintf(w, "manual merge required onto %s: %s\n", result.UpdatedBranch, strings.Join(result.Skipped, ", "))
	default:
		fmt.Fprintf(w, "manual merge still required for skipped branches: %s\n", strings.Join(result.Skipped, ", "))
	}
}

func appendUniqueBranch(branches []string, next string) []string {
	for _, branch := range branches {
		if branch == next {
			return branches
		}
	}
	return append(branches, next)
}
