package integration

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/lutefd/weaver/internal/config"
	"gopkg.in/yaml.v3"
)

const branchesFileName = "integration-branches.yaml"

type BranchRecord struct {
	Base        string   `yaml:"base" json:"base"`
	Branches    []string `yaml:"branches,omitempty" json:"branches,omitempty"`
	Skipped     []string `yaml:"skipped,omitempty" json:"skipped,omitempty"`
	Integration string   `yaml:"integration,omitempty" json:"integration,omitempty"`
}

type BranchStore struct {
	repoRoot string
}

type branchFile struct {
	Version  int                     `yaml:"version"`
	Branches map[string]BranchRecord `yaml:"branches"`
}

func NewBranchStore(repoRoot string) *BranchStore {
	return &BranchStore{repoRoot: repoRoot}
}

func (s *BranchStore) Track(name string, record BranchRecord) error {
	if err := validateBranchRecord(name, record); err != nil {
		return err
	}

	data, err := s.read()
	if err != nil {
		return err
	}
	data.Branches[name] = normalizeBranchRecord(record)
	return s.write(data)
}

func (s *BranchStore) Get(name string) (BranchRecord, bool, error) {
	data, err := s.read()
	if err != nil {
		return BranchRecord{}, false, err
	}

	record, ok := data.Branches[name]
	if !ok {
		return BranchRecord{}, false, nil
	}
	return cloneBranchRecord(record), true, nil
}

func (s *BranchStore) Remove(name string) error {
	data, err := s.read()
	if err != nil {
		return err
	}
	if _, ok := data.Branches[name]; !ok {
		return fmt.Errorf("integration branch %q does not exist", name)
	}
	delete(data.Branches, name)
	return s.write(data)
}

func (s *BranchStore) List() (map[string]BranchRecord, error) {
	data, err := s.read()
	if err != nil {
		return nil, err
	}
	return cloneBranchRecords(data.Branches), nil
}

func (s *BranchStore) Names() ([]string, error) {
	branches, err := s.List()
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(branches))
	for name := range branches {
		names = append(names, name)
	}
	sort.Strings(names)
	return names, nil
}

func (s *BranchStore) Replace(branches map[string]BranchRecord) error {
	data := branchFile{
		Version:  config.VersionOne,
		Branches: map[string]BranchRecord{},
	}
	for name, record := range branches {
		if err := validateBranchRecord(name, record); err != nil {
			return err
		}
		data.Branches[name] = normalizeBranchRecord(record)
	}
	return s.write(data)
}

func (s *BranchStore) read() (branchFile, error) {
	result := branchFile{
		Version:  config.VersionOne,
		Branches: map[string]BranchRecord{},
	}

	data, err := os.ReadFile(s.path())
	if err != nil {
		if os.IsNotExist(err) {
			return result, nil
		}
		return branchFile{}, fmt.Errorf("read integration branches file: %w", err)
	}

	if err := yaml.Unmarshal(data, &result); err != nil {
		return branchFile{}, fmt.Errorf("decode integration branches file: %w", err)
	}
	if result.Version == 0 {
		result.Version = config.VersionOne
	}
	if result.Branches == nil {
		result.Branches = map[string]BranchRecord{}
	}
	for name, record := range result.Branches {
		if err := validateBranchRecord(name, record); err != nil {
			return branchFile{}, fmt.Errorf("invalid integration branch %q: %w", name, err)
		}
		result.Branches[name] = normalizeBranchRecord(record)
	}

	return result, nil
}

func (s *BranchStore) write(data branchFile) error {
	if err := os.MkdirAll(s.dir(), 0o755); err != nil {
		return fmt.Errorf("create integration branches directory: %w", err)
	}

	data.Version = config.VersionOne
	if data.Branches == nil {
		data.Branches = map[string]BranchRecord{}
	}

	payload, err := yaml.Marshal(data)
	if err != nil {
		return fmt.Errorf("marshal integration branches file: %w", err)
	}
	if err := os.WriteFile(s.path(), payload, 0o644); err != nil {
		return fmt.Errorf("write integration branches file: %w", err)
	}
	return nil
}

func (s *BranchStore) dir() string {
	return filepath.Join(s.repoRoot, config.DirName)
}

func (s *BranchStore) path() string {
	return filepath.Join(s.dir(), branchesFileName)
}

func validateBranchRecord(name string, record BranchRecord) error {
	if name == "" {
		return fmt.Errorf("integration branch name is required")
	}
	if record.Base == "" {
		return fmt.Errorf("integration branch %q base is required", name)
	}
	branches := normalizeBranches(record.Branches)
	skipped := normalizeBranches(record.Skipped)
	if len(branches) == 0 && len(skipped) == 0 {
		return fmt.Errorf("integration branch %q requires at least one composed or skipped branch", name)
	}
	return nil
}

func normalizeBranchRecord(record BranchRecord) BranchRecord {
	return BranchRecord{
		Base:        record.Base,
		Branches:    normalizeBranches(record.Branches),
		Skipped:     normalizeBranches(record.Skipped),
		Integration: record.Integration,
	}
}

func cloneBranchRecord(record BranchRecord) BranchRecord {
	return BranchRecord{
		Base:        record.Base,
		Branches:    append([]string(nil), record.Branches...),
		Skipped:     append([]string(nil), record.Skipped...),
		Integration: record.Integration,
	}
}

func cloneBranchRecords(branches map[string]BranchRecord) map[string]BranchRecord {
	cloned := make(map[string]BranchRecord, len(branches))
	for name, record := range branches {
		cloned[name] = cloneBranchRecord(record)
	}
	return cloned
}
