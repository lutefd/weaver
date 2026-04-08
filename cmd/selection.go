package cmd

import (
	"context"
	"fmt"

	"github.com/lutefd/weaver/internal/deps"
	"github.com/lutefd/weaver/internal/group"
	weaverintegration "github.com/lutefd/weaver/internal/integration"
	"github.com/spf13/cobra"
)

type branchSelection struct {
	Branches        []string
	IntegrationName string
	Base            string
}

func resolveBranchSelection(repoRoot string, args []string, cmd *cobra.Command) (branchSelection, error) {
	groupName, err := cmd.Flags().GetString("group")
	if err != nil {
		return branchSelection{}, err
	}
	all, err := cmd.Flags().GetBool("all")
	if err != nil {
		return branchSelection{}, err
	}
	integrationName := ""
	if flag := cmd.Flags().Lookup("integration"); flag != nil {
		integrationName, err = cmd.Flags().GetString("integration")
		if err != nil {
			return branchSelection{}, err
		}
	}

	return resolveBranchSelectionMode(repoRoot, args, groupName, integrationName, all)
}

func resolveBranchSelectionMode(repoRoot string, args []string, groupName string, integrationName string, all bool) (branchSelection, error) {
	selectedModes := 0
	if len(args) > 0 {
		selectedModes++
	}
	if groupName != "" {
		selectedModes++
	}
	if integrationName != "" {
		selectedModes++
	}
	if all {
		selectedModes++
	}
	if selectedModes != 1 {
		return branchSelection{}, markUsage(fmt.Errorf("provide explicit branches, --group, --integration, or --all"))
	}

	if len(args) > 0 {
		return branchSelection{Branches: append([]string(nil), args...)}, nil
	}
	if groupName != "" {
		branches, ok, err := group.NewStore(repoRoot).Get(groupName)
		if err != nil {
			return branchSelection{}, err
		}
		if !ok {
			return branchSelection{}, fmt.Errorf("group %q does not exist", groupName)
		}
		if len(branches) == 0 {
			return branchSelection{}, fmt.Errorf("group %q is empty", groupName)
		}
		return branchSelection{Branches: branches}, nil
	}
	if integrationName != "" {
		recipe, ok, err := weaverintegration.NewStore(repoRoot).Get(integrationName)
		if err != nil {
			return branchSelection{}, err
		}
		if !ok {
			return branchSelection{}, fmt.Errorf("integration %q does not exist", integrationName)
		}
		return branchSelection{
			Branches:        append([]string(nil), recipe.Branches...),
			IntegrationName: integrationName,
			Base:            recipe.Base,
		}, nil
	}

	dependencies, err := deps.NewLocalSource(repoRoot).Load(context.Background())
	if err != nil {
		return branchSelection{}, err
	}
	if len(dependencies) == 0 {
		return branchSelection{}, fmt.Errorf("no tracked branches found")
	}

	branches := make([]string, 0, len(dependencies))
	for _, dependency := range dependencies {
		branches = append(branches, dependency.Branch)
	}
	return branchSelection{Branches: branches}, nil
}
