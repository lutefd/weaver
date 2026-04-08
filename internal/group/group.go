package group

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/lutefd/weaver/internal/config"
	"gopkg.in/yaml.v3"
)

const fileName = "groups.yaml"

type Store struct {
	repoRoot string
}

type file struct {
	Version int                 `yaml:"version"`
	Groups  map[string][]string `yaml:"groups"`
}

func NewStore(repoRoot string) *Store {
	return &Store{repoRoot: repoRoot}
}

func (s *Store) Create(name string, branches []string) error {
	data, err := s.read()
	if err != nil {
		return err
	}
	if _, exists := data.Groups[name]; exists {
		return fmt.Errorf("group %q already exists", name)
	}
	data.Groups[name] = normalizeBranches(branches)
	return s.write(data)
}

func (s *Store) Add(name string, branches []string) error {
	data, err := s.read()
	if err != nil {
		return err
	}
	if _, exists := data.Groups[name]; !exists {
		return fmt.Errorf("group %q does not exist", name)
	}
	data.Groups[name] = normalizeBranches(append(data.Groups[name], branches...))
	return s.write(data)
}

func (s *Store) Remove(name string, branches []string) error {
	data, err := s.read()
	if err != nil {
		return err
	}
	existing, exists := data.Groups[name]
	if !exists {
		return fmt.Errorf("group %q does not exist", name)
	}
	if len(branches) == 0 {
		delete(data.Groups, name)
		return s.write(data)
	}

	removeSet := make(map[string]struct{}, len(branches))
	for _, branch := range branches {
		removeSet[branch] = struct{}{}
	}

	next := make([]string, 0, len(existing))
	for _, branch := range existing {
		if _, ok := removeSet[branch]; ok {
			continue
		}
		next = append(next, branch)
	}

	if len(next) == 0 {
		delete(data.Groups, name)
	} else {
		data.Groups[name] = next
	}

	return s.write(data)
}

func (s *Store) Get(name string) ([]string, bool, error) {
	data, err := s.read()
	if err != nil {
		return nil, false, err
	}
	branches, ok := data.Groups[name]
	if !ok {
		return nil, false, nil
	}
	return append([]string(nil), branches...), true, nil
}

func (s *Store) List() (map[string][]string, error) {
	data, err := s.read()
	if err != nil {
		return nil, err
	}

	return cloneGroups(data.Groups), nil
}

func (s *Store) Replace(groups map[string][]string) error {
	file := file{
		Version: config.VersionOne,
		Groups:  cloneGroups(groups),
	}
	for name, branches := range file.Groups {
		file.Groups[name] = normalizeBranches(branches)
	}
	return s.write(file)
}

func (s *Store) Names() ([]string, error) {
	groups, err := s.List()
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(groups))
	for name := range groups {
		names = append(names, name)
	}
	sort.Strings(names)
	return names, nil
}

func (s *Store) read() (file, error) {
	result := file{
		Version: config.VersionOne,
		Groups:  map[string][]string{},
	}

	data, err := os.ReadFile(s.path())
	if err != nil {
		if os.IsNotExist(err) {
			return result, nil
		}
		return file{}, fmt.Errorf("read groups file: %w", err)
	}

	if err := yaml.Unmarshal(data, &result); err != nil {
		return file{}, fmt.Errorf("decode groups file: %w", err)
	}
	if result.Version == 0 {
		result.Version = config.VersionOne
	}
	if result.Groups == nil {
		result.Groups = map[string][]string{}
	}
	for name, branches := range result.Groups {
		result.Groups[name] = normalizeBranches(branches)
	}

	return result, nil
}

func (s *Store) write(data file) error {
	if err := os.MkdirAll(s.dir(), 0o755); err != nil {
		return fmt.Errorf("create groups directory: %w", err)
	}

	data.Version = config.VersionOne
	if data.Groups == nil {
		data.Groups = map[string][]string{}
	}

	payload, err := yaml.Marshal(data)
	if err != nil {
		return fmt.Errorf("marshal groups file: %w", err)
	}

	if err := os.WriteFile(s.path(), payload, 0o644); err != nil {
		return fmt.Errorf("write groups file: %w", err)
	}

	return nil
}

func (s *Store) dir() string {
	return filepath.Join(s.repoRoot, config.DirName)
}

func (s *Store) path() string {
	return filepath.Join(s.dir(), fileName)
}

func normalizeBranches(branches []string) []string {
	seen := make(map[string]struct{}, len(branches))
	normalized := make([]string, 0, len(branches))
	for _, branch := range branches {
		if branch == "" {
			continue
		}
		if _, ok := seen[branch]; ok {
			continue
		}
		seen[branch] = struct{}{}
		normalized = append(normalized, branch)
	}
	return normalized
}

func cloneGroups(groups map[string][]string) map[string][]string {
	cloned := make(map[string][]string, len(groups))
	for name, branches := range groups {
		cloned[name] = append([]string(nil), branches...)
	}
	return cloned
}
