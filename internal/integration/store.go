package integration

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/lutefd/weaver/internal/config"
	"gopkg.in/yaml.v3"
)

const fileName = "integrations.yaml"

type Recipe struct {
	Base     string   `yaml:"base" json:"base"`
	Branches []string `yaml:"branches" json:"branches"`
}

type Store struct {
	repoRoot string
}

type file struct {
	Version      int               `yaml:"version"`
	Integrations map[string]Recipe `yaml:"integrations"`
}

func NewStore(repoRoot string) *Store {
	return &Store{repoRoot: repoRoot}
}

func (s *Store) Save(name string, recipe Recipe) error {
	if err := validateRecipe(name, recipe); err != nil {
		return err
	}

	data, err := s.read()
	if err != nil {
		return err
	}
	data.Integrations[name] = normalizeRecipe(recipe)
	return s.write(data)
}

func (s *Store) Get(name string) (Recipe, bool, error) {
	data, err := s.read()
	if err != nil {
		return Recipe{}, false, err
	}

	recipe, ok := data.Integrations[name]
	if !ok {
		return Recipe{}, false, nil
	}
	return cloneRecipe(recipe), true, nil
}

func (s *Store) Remove(name string) error {
	data, err := s.read()
	if err != nil {
		return err
	}
	if _, ok := data.Integrations[name]; !ok {
		return fmt.Errorf("integration %q does not exist", name)
	}
	delete(data.Integrations, name)
	return s.write(data)
}

func (s *Store) List() (map[string]Recipe, error) {
	data, err := s.read()
	if err != nil {
		return nil, err
	}
	return cloneRecipes(data.Integrations), nil
}

func (s *Store) Names() ([]string, error) {
	integrations, err := s.List()
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(integrations))
	for name := range integrations {
		names = append(names, name)
	}
	sort.Strings(names)
	return names, nil
}

func (s *Store) Replace(integrations map[string]Recipe) error {
	data := file{
		Version:      config.VersionOne,
		Integrations: map[string]Recipe{},
	}
	for name, recipe := range integrations {
		if err := validateRecipe(name, recipe); err != nil {
			return err
		}
		data.Integrations[name] = normalizeRecipe(recipe)
	}
	return s.write(data)
}

func (s *Store) read() (file, error) {
	result := file{
		Version:      config.VersionOne,
		Integrations: map[string]Recipe{},
	}

	data, err := os.ReadFile(s.path())
	if err != nil {
		if os.IsNotExist(err) {
			return result, nil
		}
		return file{}, fmt.Errorf("read integrations file: %w", err)
	}

	if err := yaml.Unmarshal(data, &result); err != nil {
		return file{}, fmt.Errorf("decode integrations file: %w", err)
	}
	if result.Version == 0 {
		result.Version = config.VersionOne
	}
	if result.Integrations == nil {
		result.Integrations = map[string]Recipe{}
	}
	for name, recipe := range result.Integrations {
		if err := validateRecipe(name, recipe); err != nil {
			return file{}, fmt.Errorf("invalid integration %q: %w", name, err)
		}
		result.Integrations[name] = normalizeRecipe(recipe)
	}

	return result, nil
}

func (s *Store) write(data file) error {
	if err := os.MkdirAll(s.dir(), 0o755); err != nil {
		return fmt.Errorf("create integrations directory: %w", err)
	}

	data.Version = config.VersionOne
	if data.Integrations == nil {
		data.Integrations = map[string]Recipe{}
	}

	payload, err := yaml.Marshal(data)
	if err != nil {
		return fmt.Errorf("marshal integrations file: %w", err)
	}
	if err := os.WriteFile(s.path(), payload, 0o644); err != nil {
		return fmt.Errorf("write integrations file: %w", err)
	}
	return nil
}

func (s *Store) dir() string {
	return filepath.Join(s.repoRoot, config.DirName)
}

func (s *Store) path() string {
	return filepath.Join(s.dir(), fileName)
}

func validateRecipe(name string, recipe Recipe) error {
	if name == "" {
		return fmt.Errorf("integration name is required")
	}
	if recipe.Base == "" {
		return fmt.Errorf("integration %q base is required", name)
	}
	branches := normalizeBranches(recipe.Branches)
	if len(branches) == 0 {
		return fmt.Errorf("integration %q requires at least one branch", name)
	}
	return nil
}

func normalizeRecipe(recipe Recipe) Recipe {
	return Recipe{
		Base:     recipe.Base,
		Branches: normalizeBranches(recipe.Branches),
	}
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

func cloneRecipe(recipe Recipe) Recipe {
	return Recipe{
		Base:     recipe.Base,
		Branches: append([]string(nil), recipe.Branches...),
	}
}

func cloneRecipes(integrations map[string]Recipe) map[string]Recipe {
	cloned := make(map[string]Recipe, len(integrations))
	for name, recipe := range integrations {
		cloned[name] = cloneRecipe(recipe)
	}
	return cloned
}
