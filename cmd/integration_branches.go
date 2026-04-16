package cmd

import (
	"context"
	"fmt"
	"io"
	"strings"

	gitrunner "github.com/lutefd/weaver/internal/git"
	weaverintegration "github.com/lutefd/weaver/internal/integration"
	"github.com/lutefd/weaver/internal/ui"
	"github.com/spf13/cobra"
)

func init() {
	integrationBranchCmd.AddCommand(integrationBranchListCmd)
	integrationBranchCmd.AddCommand(integrationBranchDeleteCmd)
}

var integrationBranchCmd = &cobra.Command{
	Use:     "branch",
	Aliases: []string{"branches"},
	Short:   "Manage tracked integration branches created by compose --create",
}

var integrationBranchListCmd = &cobra.Command{
	Use:   "list",
	Short: "List tracked integration branches created by compose --create",
	RunE: func(cmd *cobra.Command, _ []string) error {
		ctx := context.Background()
		store := weaverintegration.NewBranchStore(AppContext().Runner.RepoRoot())
		entries, err := loadTrackedIntegrationBranchEntries(ctx, AppContext().Runner, store)
		if err != nil {
			return err
		}
		term := terminalFor(cmd)
		if term.Interactive() && len(entries) > 0 {
			return runTrackedIntegrationBranchBrowser(ctx, term, AppContext().Runner, store, entries)
		}
		return writeTrackedIntegrationBranchList(cmd.OutOrStdout(), term, term.Styled(), entries)
	},
}

var integrationBranchDeleteCmd = &cobra.Command{
	Use:     "delete <name>",
	Aliases: []string{"remove"},
	Short:   "Delete a tracked integration branch and remove it from Weaver metadata",
	Args:    cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()
		result, err := deleteTrackedIntegrationBranch(ctx, AppContext().Runner, weaverintegration.NewBranchStore(AppContext().Runner.RepoRoot()), args[0])
		if err != nil {
			return err
		}

		term := terminalFor(cmd)
		return writeTrackedIntegrationBranchDeleteResult(cmd.OutOrStdout(), term, term.Styled(), args[0], result)
	},
}

type trackedIntegrationBranchEntry struct {
	Name            string
	Record          weaverintegration.BranchRecord
	Exists          bool
	IncludedSkipped []string
	PendingSkipped  []string
}

func (e trackedIntegrationBranchEntry) Status() string {
	if !e.Exists {
		return "missing"
	}
	pending := e.pendingSkipped()
	included := e.includedSkipped()
	switch {
	case len(pending) == 0 && len(included) > 0:
		return "integrated"
	case len(pending) > 0 && len(included) > 0:
		return "partial"
	case len(pending) > 0:
		return "pending"
	default:
		return "present"
	}
}

func (e trackedIntegrationBranchEntry) pendingSkipped() []string {
	if len(e.PendingSkipped) == 0 && len(e.IncludedSkipped) == 0 && len(e.Record.Skipped) > 0 {
		return append([]string(nil), e.Record.Skipped...)
	}
	return append([]string(nil), e.PendingSkipped...)
}

func (e trackedIntegrationBranchEntry) includedSkipped() []string {
	return append([]string(nil), e.IncludedSkipped...)
}

type trackedIntegrationBranchDeleteResult struct {
	DeletedBranch bool
}

func loadTrackedIntegrationBranchEntries(ctx context.Context, runner gitrunner.Runner, store *weaverintegration.BranchStore) ([]trackedIntegrationBranchEntry, error) {
	names, err := store.Names()
	if err != nil {
		return nil, err
	}

	entries := make([]trackedIntegrationBranchEntry, 0, len(names))
	for _, name := range names {
		record, ok, err := store.Get(name)
		if err != nil {
			return nil, err
		}
		if !ok {
			continue
		}
		exists, err := gitBranchExists(ctx, runner, name)
		if err != nil {
			return nil, fmt.Errorf("check integration branch %q: %w", name, err)
		}
		includedSkipped, pendingSkipped, err := resolveTrackedIntegrationBranchProgress(ctx, runner, name, record, exists)
		if err != nil {
			return nil, err
		}
		entries = append(entries, trackedIntegrationBranchEntry{
			Name:            name,
			Record:          record,
			Exists:          exists,
			IncludedSkipped: includedSkipped,
			PendingSkipped:  pendingSkipped,
		})
	}

	return entries, nil
}

func resolveTrackedIntegrationBranchProgress(ctx context.Context, runner gitrunner.Runner, integrationBranch string, record weaverintegration.BranchRecord, exists bool) ([]string, []string, error) {
	pending := append([]string(nil), record.Skipped...)
	if !exists || len(record.Skipped) == 0 {
		return nil, pending, nil
	}

	included := make([]string, 0, len(record.Skipped))
	pending = make([]string, 0, len(record.Skipped))
	for _, branch := range record.Skipped {
		contains, err := integrationBranchContainsBranch(ctx, runner, integrationBranch, branch)
		if err != nil {
			return nil, nil, fmt.Errorf("check whether skipped branch %q is integrated into %q: %w", branch, integrationBranch, err)
		}
		if contains {
			included = append(included, branch)
			continue
		}
		pending = append(pending, branch)
	}
	return included, pending, nil
}

func writeTrackedIntegrationBranchList(w io.Writer, term ui.Terminal, styled bool, entries []trackedIntegrationBranchEntry) error {
	if len(entries) == 0 {
		fmt.Fprintln(w, "no integration branches")
		return nil
	}
	if styled {
		writeLine(w, renderTrackedIntegrationBranchListStyled(term, entries))
		return nil
	}

	for _, entry := range entries {
		fmt.Fprintf(w, "%s: status=%s base=%s branches=%s", entry.Name, entry.Status(), entry.Record.Base, formatTrackedBranchSlice(entry.Record.Branches))
		if included := entry.includedSkipped(); len(included) > 0 {
			fmt.Fprintf(w, " integrated=%s", strings.Join(included, ", "))
		}
		if pending := entry.pendingSkipped(); len(pending) > 0 {
			fmt.Fprintf(w, " skipped=%s", strings.Join(pending, ", "))
		}
		if entry.Record.Integration != "" {
			fmt.Fprintf(w, " integration=%s", entry.Record.Integration)
		}
		fmt.Fprintln(w)
	}
	return nil
}

func writeTrackedIntegrationBranchDeleteResult(w io.Writer, term ui.Terminal, styled bool, name string, result trackedIntegrationBranchDeleteResult) error {
	if styled {
		subtitle := "Tracked integration branch deleted"
		notes := []string(nil)
		if !result.DeletedBranch {
			subtitle = "Tracked integration branch removed from Weaver metadata"
			notes = append(notes, "git branch was already missing locally")
		}
		writeLine(w, renderActionCard(term, ui.ToneSuccess, "Integration Branch Deleted", subtitle, []ui.KeyValue{{Label: "name", Value: name}}, notes))
		return nil
	}

	if result.DeletedBranch {
		fmt.Fprintf(w, "deleted integration branch %s\n", name)
		return nil
	}

	fmt.Fprintf(w, "removed tracked integration branch %s (git branch already missing)\n", name)
	return nil
}

func deleteTrackedIntegrationBranch(ctx context.Context, runner gitrunner.Runner, store *weaverintegration.BranchStore, name string) (trackedIntegrationBranchDeleteResult, error) {
	if _, ok, err := store.Get(name); err != nil {
		return trackedIntegrationBranchDeleteResult{}, err
	} else if !ok {
		return trackedIntegrationBranchDeleteResult{}, fmt.Errorf("integration branch %q does not exist", name)
	}

	exists, err := gitBranchExists(ctx, runner, name)
	if err != nil {
		return trackedIntegrationBranchDeleteResult{}, fmt.Errorf("check integration branch %q: %w", name, err)
	}

	result := trackedIntegrationBranchDeleteResult{}
	if exists {
		current, err := currentBranchNameForRunner(ctx, runner)
		if err != nil {
			return trackedIntegrationBranchDeleteResult{}, err
		}
		if current == name {
			return trackedIntegrationBranchDeleteResult{}, fmt.Errorf("cannot delete current branch %q; checkout another branch first", name)
		}
		if _, err := runner.Run(ctx, "branch", "-D", name); err != nil {
			return trackedIntegrationBranchDeleteResult{}, err
		}
		result.DeletedBranch = true
	}

	if err := store.Remove(name); err != nil {
		return trackedIntegrationBranchDeleteResult{}, err
	}
	return result, nil
}

func gitBranchExists(ctx context.Context, runner gitrunner.Runner, branch string) (bool, error) {
	result, err := runner.Run(ctx, "show-ref", "--verify", "--quiet", "refs/heads/"+branch)
	if err != nil {
		if result.ExitCode != 0 {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func integrationBranchContainsBranch(ctx context.Context, runner gitrunner.Runner, integrationBranch, branch string) (bool, error) {
	exists, err := gitBranchExists(ctx, runner, branch)
	if err != nil {
		return false, err
	}
	if !exists {
		return false, nil
	}

	result, err := runner.Run(ctx, "merge-base", "--is-ancestor", branch, integrationBranch)
	if err != nil {
		if result.ExitCode == 1 {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func formatTrackedBranchSlice(branches []string) string {
	if len(branches) == 0 {
		return "(none)"
	}
	return strings.Join(branches, ", ")
}
