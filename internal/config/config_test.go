package config

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/viper"
)

func TestInitialize(t *testing.T) {
	t.Parallel()

	repoRoot := t.TempDir()
	gitDir := filepath.Join(repoRoot, ".git")
	if err := os.MkdirAll(gitDir, 0o755); err != nil {
		t.Fatalf("mkdir .git: %v", err)
	}

	created, err := Initialize(repoRoot)
	if err != nil {
		t.Fatalf("Initialize() error = %v", err)
	}
	if !created {
		t.Fatalf("Initialize() created = false, want true")
	}

	if _, err := os.Stat(filepath.Join(repoRoot, FileName)); err != nil {
		t.Fatalf("config file missing: %v", err)
	}
	if _, err := os.Stat(filepath.Join(repoRoot, DirName)); err != nil {
		t.Fatalf("metadata dir missing: %v", err)
	}
}

func TestLoadIntoDefaultsMissingFields(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	cfgPath := filepath.Join(dir, FileName)
	if err := os.WriteFile(cfgPath, []byte("version: 1\n"), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	v := viper.New()
	v.SetConfigFile(cfgPath)

	cfg := Config{}
	if err := LoadInto(v, &cfg); err != nil {
		t.Fatalf("LoadInto() error = %v", err)
	}

	if cfg.DefaultBase != "main" {
		t.Fatalf("DefaultBase = %q, want main", cfg.DefaultBase)
	}
}

func TestLoadIntoMissingFile(t *testing.T) {
	t.Parallel()

	v := viper.New()
	v.SetConfigFile(filepath.Join(t.TempDir(), FileName))

	cfg := Config{}
	err := LoadInto(v, &cfg)
	if !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("LoadInto() error = %v, want %v", err, os.ErrNotExist)
	}
}
