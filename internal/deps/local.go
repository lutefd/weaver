package deps

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/lutefd/weaver/internal/config"
	"github.com/lutefd/weaver/internal/stack"
	"gopkg.in/yaml.v3"
)

const depsFileName = "deps.yaml"

type LocalSource struct {
	repoRoot string
}

type localFile struct {
	Version      int               `yaml:"version"`
	Dependencies map[string]string `yaml:"dependencies"`
}

func NewLocalSource(repoRoot string) *LocalSource {
	return &LocalSource{repoRoot: repoRoot}
}

func (s *LocalSource) Load(_ context.Context) ([]stack.Dependency, error) {
	file, err := s.read()
	if err != nil {
		return nil, err
	}

	keys := make([]string, 0, len(file.Dependencies))
	for branch := range file.Dependencies {
		keys = append(keys, branch)
	}
	sort.Strings(keys)

	deps := make([]stack.Dependency, 0, len(keys))
	for _, branch := range keys {
		deps = append(deps, stack.Dependency{
			Branch: branch,
			Parent: file.Dependencies[branch],
		})
	}

	return deps, nil
}

func (s *LocalSource) Set(_ context.Context, branch, parent string) error {
	file, err := s.read()
	if err != nil {
		return err
	}

	if file.Dependencies == nil {
		file.Dependencies = make(map[string]string)
	}
	file.Dependencies[branch] = parent

	return s.write(file)
}

func (s *LocalSource) Remove(_ context.Context, branch string) error {
	file, err := s.read()
	if err != nil {
		return err
	}

	delete(file.Dependencies, branch)
	return s.write(file)
}

func (s *LocalSource) read() (localFile, error) {
	file := localFile{
		Version:      config.VersionOne,
		Dependencies: map[string]string{},
	}

	data, err := os.ReadFile(s.path())
	if err != nil {
		if os.IsNotExist(err) {
			return file, nil
		}
		return localFile{}, fmt.Errorf("read deps file: %w", err)
	}

	if err := yaml.Unmarshal(data, &file); err != nil {
		return localFile{}, fmt.Errorf("decode deps file: %w", err)
	}

	if file.Version == 0 {
		file.Version = config.VersionOne
	}
	if file.Dependencies == nil {
		file.Dependencies = map[string]string{}
	}

	return file, nil
}

func (s *LocalSource) write(file localFile) error {
	if file.Version == 0 {
		file.Version = config.VersionOne
	}

	if err := os.MkdirAll(s.dir(), 0o755); err != nil {
		return fmt.Errorf("create deps directory: %w", err)
	}

	data, err := yaml.Marshal(file)
	if err != nil {
		return fmt.Errorf("marshal deps file: %w", err)
	}

	if err := os.WriteFile(s.path(), data, 0o644); err != nil {
		return fmt.Errorf("write deps file: %w", err)
	}

	return nil
}

func (s *LocalSource) dir() string {
	return filepath.Join(s.repoRoot, config.DirName)
}

func (s *LocalSource) path() string {
	return filepath.Join(s.dir(), depsFileName)
}
