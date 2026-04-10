package config

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
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

func TestConfigErrorString(t *testing.T) {
	t.Parallel()

	if got := (Error{Message: "boom"}).Error(); got != "boom" {
		t.Fatalf("Error() = %q, want boom", got)
	}
}

func TestInitializeRejectsEmptyRepoRoot(t *testing.T) {
	t.Parallel()

	created, err := Initialize("")
	if created {
		t.Fatal("Initialize(\"\") created = true, want false")
	}
	var cfgErr Error
	if !errors.As(err, &cfgErr) {
		t.Fatalf("Initialize(\"\") error = %v, want config.Error", err)
	}
}

func TestInitializeReturnsFalseWhenConfigExists(t *testing.T) {
	t.Parallel()

	repoRoot := t.TempDir()
	if err := os.MkdirAll(filepath.Join(repoRoot, ".git", "weaver"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repoRoot, FileName), []byte("version: 1\n"), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	created, err := Initialize(repoRoot)
	if err != nil {
		t.Fatalf("Initialize() error = %v", err)
	}
	if created {
		t.Fatal("Initialize() created = true, want false")
	}
}

func TestLoadIntoDecodeError(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	cfgPath := filepath.Join(dir, FileName)
	if err := os.WriteFile(cfgPath, []byte("default_base:\n  - nope\n"), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	v := viper.New()
	v.SetConfigFile(cfgPath)
	cfg := Config{}
	err := LoadInto(v, &cfg)
	var cfgErr Error
	if !errors.As(err, &cfgErr) || !strings.Contains(err.Error(), "decode config") {
		t.Fatalf("LoadInto() error = %v, want decode config error", err)
	}
}

func TestLoadIntoReadErrorAndInitializeMetadataDirError(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config-dir")
	if err := os.MkdirAll(cfgPath, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}

	v := viper.New()
	v.SetConfigFile(cfgPath)
	cfg := Config{}
	err := LoadInto(v, &cfg)
	var cfgErr Error
	if !errors.As(err, &cfgErr) || !strings.Contains(err.Error(), "read config") {
		t.Fatalf("LoadInto() error = %v, want read config error", err)
	}

	repoRoot := filepath.Join(t.TempDir(), "repo-root-file")
	if err := os.WriteFile(repoRoot, []byte("x"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	created, err := Initialize(repoRoot)
	if created {
		t.Fatal("Initialize() created = true, want false")
	}
	if !errors.As(err, &cfgErr) || !strings.Contains(err.Error(), "create metadata directory") {
		t.Fatalf("Initialize() error = %v, want metadata directory error", err)
	}
}

func TestLoadIntoVersionDefaultAndInitializeWriteError(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	cfgPath := filepath.Join(dir, FileName)
	if err := os.WriteFile(cfgPath, []byte("version: 0\ndefault_base: \"\"\n"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	v := viper.New()
	v.SetConfigFile(cfgPath)
	cfg := Config{}
	if err := LoadInto(v, &cfg); err != nil {
		t.Fatalf("LoadInto() error = %v", err)
	}
	if cfg.Version != VersionOne || cfg.DefaultBase != "main" {
		t.Fatalf("LoadInto() = %#v, want version %d and default base main", cfg, VersionOne)
	}

	repoRoot := t.TempDir()
	if err := os.MkdirAll(filepath.Join(repoRoot, ".git", "weaver"), 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.Chmod(repoRoot, 0o555); err != nil {
		t.Fatalf("Chmod() error = %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(repoRoot, 0o755) })

	created, err := Initialize(repoRoot)
	if created {
		t.Fatal("Initialize() created = true, want false")
	}
	var cfgErr Error
	if !errors.As(err, &cfgErr) || !strings.Contains(err.Error(), "write config file") {
		t.Fatalf("Initialize() error = %v, want write config file error", err)
	}
}
