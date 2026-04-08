package cmd

import (
	"context"
	"fmt"

	"github.com/lutefd/weaver/internal/deps"
	"github.com/lutefd/weaver/internal/group"
	"github.com/spf13/cobra"
)

func resolveBranchSelection(repoRoot string, args []string, cmd *cobra.Command) ([]string, error) {
	groupName, err := cmd.Flags().GetString("group")
	if err != nil {
		return nil, err
	}
	all, err := cmd.Flags().GetBool("all")
	if err != nil {
		return nil, err
	}

	return resolveBranchSelectionMode(repoRoot, args, groupName, all)
}

func resolveBranchSelectionMode(repoRoot string, args []string, groupName string, all bool) ([]string, error) {
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
