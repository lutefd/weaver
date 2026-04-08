package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/viper"
	"gopkg.in/yaml.v3"
)

const (
	FileName   = ".weaver.yaml"
	DirName    = ".git/weaver"
	VersionOne = 1
)

type Error struct {
	Message string
}

func (e Error) Error() string {
	return e.Message
}

type Config struct {
	Version     int    `mapstructure:"version" yaml:"version"`
	DefaultBase string `mapstructure:"default_base" yaml:"default_base"`
}

func Default() Config {
	return Config{
		Version:     VersionOne,
		DefaultBase: "main",
	}
}

func LoadInto(v *viper.Viper, cfg *Config) error {
	if err := v.ReadInConfig(); err != nil {
		var cfgNotFound viper.ConfigFileNotFoundError
		if errors.As(err, &cfgNotFound) {
			return os.ErrNotExist
		}
		var pathErr *os.PathError
		if errors.As(err, &pathErr) && errors.Is(pathErr.Err, os.ErrNotExist) {
			return os.ErrNotExist
		}
		return Error{Message: fmt.Sprintf("read config: %v", err)}
	}

	if err := v.Unmarshal(cfg); err != nil {
		return Error{Message: fmt.Sprintf("decode config: %v", err)}
	}

	if cfg.Version == 0 {
		cfg.Version = VersionOne
	}
	if cfg.DefaultBase == "" {
		cfg.DefaultBase = "main"
	}

	return nil
}

func Initialize(repoRoot string) (bool, error) {
	if repoRoot == "" {
		return false, Error{Message: "repository root is required"}
	}

	cfgPath := filepath.Join(repoRoot, FileName)
	metaDir := filepath.Join(repoRoot, DirName)

	if err := os.MkdirAll(metaDir, 0o755); err != nil {
		return false, Error{Message: fmt.Sprintf("create metadata directory: %v", err)}
	}

	if _, err := os.Stat(cfgPath); err == nil {
		return false, nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return false, Error{Message: fmt.Sprintf("stat config file: %v", err)}
	}

	data, err := yaml.Marshal(Default())
	if err != nil {
		return false, Error{Message: fmt.Sprintf("marshal config: %v", err)}
	}

	if err := os.WriteFile(cfgPath, data, 0o644); err != nil {
		return false, Error{Message: fmt.Sprintf("write config file: %v", err)}
	}

	return true, nil
}
