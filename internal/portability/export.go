package portability

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/lutefd/weaver/internal/deps"
	"github.com/lutefd/weaver/internal/group"
	weaverintegration "github.com/lutefd/weaver/internal/integration"
	"github.com/lutefd/weaver/internal/stack"
)

type State struct {
	Version             int                                       `json:"version"`
	ExportedAt          time.Time                                 `json:"exported_at"`
	Dependencies        map[string]string                         `json:"dependencies"`
	Groups              map[string][]string                       `json:"groups,omitempty"`
	Integrations        map[string]weaverintegration.Recipe       `json:"integrations,omitempty"`
	IntegrationBranches map[string]weaverintegration.BranchRecord `json:"integration_branches,omitempty"`
}

type Manager struct {
	repoRoot string
}

func New(repoRoot string) *Manager {
	return &Manager{repoRoot: repoRoot}
}

func (m *Manager) Export() (*State, error) {
	dependencySource := deps.NewLocalSource(m.repoRoot)
	groupStore := group.NewStore(m.repoRoot)
	integrationStore := weaverintegration.NewStore(m.repoRoot)
	integrationBranchStore := weaverintegration.NewBranchStore(m.repoRoot)

	dependencies, err := dependencySource.Map(nil)
	if err != nil {
		return nil, err
	}
	groups, err := groupStore.List()
	if err != nil {
		return nil, err
	}
	integrations, err := integrationStore.List()
	if err != nil {
		return nil, err
	}
	integrationBranches, err := integrationBranchStore.List()
	if err != nil {
		return nil, err
	}

	return &State{
		Version:             1,
		ExportedAt:          time.Now().UTC(),
		Dependencies:        dependencies,
		Groups:              groups,
		Integrations:        integrations,
		IntegrationBranches: integrationBranches,
	}, nil
}

func (m *Manager) Import(state *State) error {
	if state == nil {
		return fmt.Errorf("import state is required")
	}

	depsSlice := make([]stack.Dependency, 0, len(state.Dependencies))
	for branch, parent := range state.Dependencies {
		depsSlice = append(depsSlice, stack.Dependency{Branch: branch, Parent: parent})
	}
	if _, err := stack.NewDAG(depsSlice); err != nil {
		return err
	}

	if err := deps.NewLocalSource(m.repoRoot).Replace(state.Dependencies); err != nil {
		return err
	}
	if err := group.NewStore(m.repoRoot).Replace(state.Groups); err != nil {
		return err
	}
	if err := weaverintegration.NewStore(m.repoRoot).Replace(state.Integrations); err != nil {
		return err
	}
	if err := weaverintegration.NewBranchStore(m.repoRoot).Replace(state.IntegrationBranches); err != nil {
		return err
	}

	return nil
}

func Encode(w io.Writer, state *State) error {
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	return encoder.Encode(state)
}

func Decode(r io.Reader) (*State, error) {
	var state State
	if err := json.NewDecoder(r).Decode(&state); err != nil {
		return nil, fmt.Errorf("decode export: %w", err)
	}
	return &state, nil
}

func LoadFile(path string) (*State, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	return Decode(file)
}
