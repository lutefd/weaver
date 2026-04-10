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
	gitrunner "github.com/lutefd/weaver/internal/git"
	weaverintegration "github.com/lutefd/weaver/internal/integration"
	"github.com/lutefd/weaver/internal/resolver"
	"github.com/lutefd/weaver/internal/stack"
	"github.com/lutefd/weaver/internal/ui"
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

		dag, err := runTask(ctx, cmd, ui.TaskSpec{
			Title:    "Resolving Compose Inputs",
			Subtitle: "Building the stack graph for this compose run",
		}, func(ctx context.Context, runner gitrunner.Runner) (*stack.DAG, error) {
			return resolver.New(deps.NewLocalSource(runner.RepoRoot())).Resolve(ctx)
		})
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
		if err := recordComposeIntegrationBranch(selection, result); err != nil {
			return err
		}

		if result.DryRun {
			term := terminalFor(cmd)
			if term.Styled() {
				writeLine(cmd.OutOrStdout(), renderComposeResultStyled(term, result))
				return nil
			}
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

		term := terminalFor(cmd)
		if term.Styled() {
			writeLine(cmd.OutOrStdout(), renderComposeResultStyled(term, result))
			renderManualMergeSummary(cmd.OutOrStdout(), result)
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

func recordComposeIntegrationBranch(selection branchSelection, result *composer.ComposeResult) error {
	if result == nil || result.DryRun {
		return nil
	}

	record := weaverintegration.BranchRecord{
		Base:        result.BaseBranch,
		Branches:    append([]string(nil), result.Order...),
		Skipped:     append([]string(nil), result.Skipped...),
		Integration: selection.IntegrationName,
	}
	store := weaverintegration.NewBranchStore(AppContext().Runner.RepoRoot())

	switch {
	case result.CreatedBranch != "":
		if err := store.Track(result.CreatedBranch, record); err != nil {
			return fmt.Errorf("track integration branch %q: %w", result.CreatedBranch, err)
		}
	case result.UpdatedBranch != "":
		if err := store.Track(result.UpdatedBranch, record); err != nil {
			return fmt.Errorf("track integration branch %q: %w", result.UpdatedBranch, err)
		}
	}

	return nil
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
	for {
		var (
			result *composer.ComposeResult
			err    error
		)
		if opts.DryRun {
			result, err = composer.New(AppContext().Runner).Compose(ctx, dag, branches, base, opts)
		} else {
			result, err = runTask(ctx, cmd, ui.TaskSpec{
				Title:    "Running Compose",
				Subtitle: fmt.Sprintf("Composing onto %s", base),
				TotalOps: estimateComposeOps(dag, branches, base, opts),
			}, func(ctx context.Context, runner gitrunner.Runner) (*composer.ComposeResult, error) {
				return composer.New(runner).Compose(ctx, dag, branches, base, opts)
			})
		}
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
		term := terminalFor(cmd)
		if term.Styled() {
			writeLine(cmd.OutOrStdout(), renderActionCard(term, ui.ToneWarn, "Retrying Compose", "Branch skipped after merge conflict", []ui.KeyValue{{Label: "branch", Value: conflictErr.Branch}}, []string{"merge it manually later"}))
			continue
		}

		fmt.Fprintf(cmd.OutOrStdout(), "skipping %s and retrying compose; merge it manually later\n", conflictErr.Branch)
	}
}

func estimateComposeOps(dag *stack.DAG, branches []string, base string, opts composer.ComposeOpts) int {
	if opts.DryRun {
		return 0
	}

	order, err := composer.ResolveOrder(dag, branches, base)
	if err != nil {
		return 0
	}

	skipSet := make(map[string]struct{}, len(opts.SkipBranches))
	for _, branch := range opts.SkipBranches {
		if branch == "" {
			continue
		}
		skipSet[branch] = struct{}{}
	}

	mergeCount := 0
	for _, branch := range order {
		if _, skipped := skipSet[branch]; skipped {
			continue
		}
		mergeCount++
	}

	total := 1 + 1 + mergeCount + 1
	if opts.CreateBranch != "" || opts.UpdateBranch != "" {
		total++
	}
	return total
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
		term := terminalFor(cmd)
		if term.Styled() {
			details := []ui.KeyValue{{Label: "branch", Value: conflictErr.Branch}}
			if len(conflictErr.Files) > 0 {
				details = append(details, ui.KeyValue{Label: "files", Value: strings.Join(conflictErr.Files, ", ")})
			}
			writeLine(cmd.OutOrStdout(), renderActionCard(term, ui.ToneDanger, "Compose Conflict", "Choose whether to skip or abort this branch", details, []string{"type skip to continue", "type abort to stop"}))
		} else if len(conflictErr.Files) > 0 {
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
