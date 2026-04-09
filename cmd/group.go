package cmd

import (
	"fmt"
	"strings"

	"github.com/lutefd/weaver/internal/group"
	"github.com/lutefd/weaver/internal/ui"
	"github.com/spf13/cobra"
)

func init() {
	groupCmd.AddCommand(groupCreateCmd)
	groupCmd.AddCommand(groupAddCmd)
	groupCmd.AddCommand(groupRemoveCmd)
	groupCmd.AddCommand(groupListCmd)
	rootCmd.AddCommand(groupCmd)
}

var groupCmd = &cobra.Command{
	Use:   "group",
	Short: "Manage named compose groups",
}

var groupCreateCmd = &cobra.Command{
	Use:   "create <name> <branch...>",
	Short: "Create a named compose group",
	Args:  cobra.MinimumNArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		name, branches := args[0], args[1:]
		if err := group.NewStore(AppContext().Runner.RepoRoot()).Create(name, branches); err != nil {
			return err
		}
		term := terminalFor(cmd)
		if term.Styled() {
			writeLine(cmd.OutOrStdout(), renderActionCard(term, ui.ToneSuccess, "Group Created", "Named compose group saved", []ui.KeyValue{
				{Label: "name", Value: name},
				{Label: "branches", Value: strings.Join(branches, ", ")},
			}, nil))
			return nil
		}
		fmt.Fprintf(cmd.OutOrStdout(), "created group %s\n", name)
		return nil
	},
}

var groupAddCmd = &cobra.Command{
	Use:   "add <name> <branch...>",
	Short: "Add branches to a named compose group",
	Args:  cobra.MinimumNArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		name, branches := args[0], args[1:]
		if err := group.NewStore(AppContext().Runner.RepoRoot()).Add(name, branches); err != nil {
			return err
		}
		term := terminalFor(cmd)
		if term.Styled() {
			writeLine(cmd.OutOrStdout(), renderActionCard(term, ui.ToneSuccess, "Group Updated", "Branches added to compose group", []ui.KeyValue{
				{Label: "name", Value: name},
				{Label: "added", Value: strings.Join(branches, ", ")},
			}, nil))
			return nil
		}
		fmt.Fprintf(cmd.OutOrStdout(), "updated group %s\n", name)
		return nil
	},
}

var groupRemoveCmd = &cobra.Command{
	Use:   "remove <name> [branch...]",
	Short: "Remove branches from a group or delete the group entirely",
	Args:  cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]
		branches := []string(nil)
		if len(args) > 1 {
			branches = args[1:]
		}
		if err := group.NewStore(AppContext().Runner.RepoRoot()).Remove(name, branches); err != nil {
			return err
		}
		term := terminalFor(cmd)
		if term.Styled() {
			details := []ui.KeyValue{{Label: "name", Value: name}}
			if len(branches) > 0 {
				details = append(details, ui.KeyValue{Label: "removed", Value: strings.Join(branches, ", ")})
			} else {
				details = append(details, ui.KeyValue{Label: "removed", Value: "entire group"})
			}
			writeLine(cmd.OutOrStdout(), renderActionCard(term, ui.ToneSuccess, "Group Updated", "Compose group removal applied", details, nil))
			return nil
		}
		fmt.Fprintf(cmd.OutOrStdout(), "updated group %s\n", name)
		return nil
	},
}

var groupListCmd = &cobra.Command{
	Use:   "list",
	Short: "List named compose groups",
	RunE: func(cmd *cobra.Command, _ []string) error {
		store := group.NewStore(AppContext().Runner.RepoRoot())
		names, err := store.Names()
		if err != nil {
			return err
		}
		if len(names) == 0 {
			fmt.Fprintln(cmd.OutOrStdout(), "no groups")
			return nil
		}

		groups, err := store.List()
		if err != nil {
			return err
		}
		term := terminalFor(cmd)
		if term.Styled() {
			writeLine(cmd.OutOrStdout(), renderGroupListStyled(term, groups))
			return nil
		}
		for _, name := range names {
			fmt.Fprintf(cmd.OutOrStdout(), "%s: %s\n", name, strings.Join(groups[name], ", "))
		}
		return nil
	},
}
