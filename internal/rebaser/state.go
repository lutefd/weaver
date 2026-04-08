package rebaser

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/lutefd/weaver/internal/config"
	"gopkg.in/yaml.v3"
)

const stateFileName = "rebase-state.yaml"

type State struct {
	Version        int       `yaml:"version"`
	StartedAt      time.Time `yaml:"started_at"`
	OriginalBranch string    `yaml:"original_branch"`
	BaseBranch     string    `yaml:"base_branch"`
	AllBranches    []string  `yaml:"all_branches"`
	Completed      []string  `yaml:"completed"`
	Current        string    `yaml:"current"`
	CurrentOnto    string    `yaml:"current_onto"`
}

type StateStore struct {
	repoRoot string
}

func NewStateStore(repoRoot string) *StateStore {
	return &StateStore{repoRoot: repoRoot}
}

func (s *StateStore) Load() (*State, error) {
	data, err := os.ReadFile(s.path())
	if err != nil {
		return nil, err
	}

	var state State
	if err := yaml.Unmarshal(data, &state); err != nil {
		return nil, fmt.Errorf("decode rebase state: %w", err)
	}
	if state.Version == 0 {
		state.Version = config.VersionOne
	}

	return &state, nil
}

func (s *StateStore) Save(state *State) error {
	if err := os.MkdirAll(s.dir(), 0o755); err != nil {
		return fmt.Errorf("create rebase state directory: %w", err)
	}

	state.Version = config.VersionOne
	data, err := yaml.Marshal(state)
	if err != nil {
		return fmt.Errorf("marshal rebase state: %w", err)
	}

	if err := os.WriteFile(s.path(), data, 0o644); err != nil {
		return fmt.Errorf("write rebase state: %w", err)
	}

	return nil
}

func (s *StateStore) Clear() error {
	if err := os.Remove(s.path()); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove rebase state: %w", err)
	}
	return nil
}

func (s *StateStore) HasPending() bool {
	_, err := os.Stat(s.path())
	return err == nil
}

func (s *StateStore) dir() string {
	return filepath.Join(s.repoRoot, config.DirName)
}

func (s *StateStore) path() string {
	return filepath.Join(s.dir(), stateFileName)
}
