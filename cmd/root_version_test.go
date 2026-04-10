package cmd

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"runtime/debug"
	"strings"
	"testing"

	"github.com/lutefd/weaver/internal/config"
	"github.com/spf13/cobra"
)

func TestUsageErrorAndMarkUsage(t *testing.T) {
	t.Parallel()

	if markUsage(nil) != nil {
		t.Fatal("markUsage(nil) != nil")
	}
	err := markUsage(errors.New("bad"))
	var usageErr usageError
	if !errors.As(err, &usageErr) || usageErr.Error() != "bad" {
		t.Fatalf("markUsage() = %v", err)
	}
}

func TestLoadConfigDefaultsWhenMissing(t *testing.T) {
	t.Parallel()

	repoRoot := t.TempDir()
	opts = cliOptions{}
	t.Cleanup(func() { opts = cliOptions{} })

	cfg, err := loadConfig(repoRoot)
	if err != nil {
		t.Fatalf("loadConfig() error = %v", err)
	}
	if cfg.DefaultBase != "main" {
		t.Fatalf("DefaultBase = %q", cfg.DefaultBase)
	}
}

func TestLoadConfigFromExplicitPath(t *testing.T) {
	t.Parallel()

	repoRoot := t.TempDir()
	cfgPath := filepath.Join(repoRoot, "custom.yaml")
	if err := os.WriteFile(cfgPath, []byte("version: 1\ndefault_base: develop\n"), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	opts = cliOptions{ConfigPath: cfgPath}
	t.Cleanup(func() { opts = cliOptions{} })

	cfg, err := loadConfig(repoRoot)
	if err != nil {
		t.Fatalf("loadConfig() error = %v", err)
	}
	if cfg.DefaultBase != "develop" {
		t.Fatalf("DefaultBase = %q", cfg.DefaultBase)
	}
}

func TestBootstrapAppAndCurrentBranchName(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd() error = %v", err)
	}
	repoRoot := t.TempDir()
	runGit(t, repoRoot, "init", "-b", "main")
	if err := os.WriteFile(filepath.Join(repoRoot, config.FileName), []byte("version: 1\ndefault_base: main\n"), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	if err := os.Chdir(repoRoot); err != nil {
		t.Fatalf("Chdir() error = %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(wd); app = nil; opts = cliOptions{} })

	cmd := &cobra.Command{Use: "status"}
	cmd.SetOut(io.Discard)
	if err := bootstrapApp(context.Background(), cmd); err != nil {
		t.Fatalf("bootstrapApp() error = %v", err)
	}
	if AppContext() == nil || AppContext().Config.DefaultBase != "main" {
		t.Fatalf("AppContext() = %#v", AppContext())
	}
}

func TestExecute(t *testing.T) {
	prev := rootCmd
	rootCmd = &cobra.Command{
		Use: "weaver",
		RunE: func(_ *cobra.Command, _ []string) error {
			return nil
		},
	}
	t.Cleanup(func() { rootCmd = prev })

	if err := Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
}

func TestResolvedVersion(t *testing.T) {
	prev := version
	version = "1.2.3"
	t.Cleanup(func() { version = prev })

	if got := resolvedVersion(); got != "1.2.3" {
		t.Fatalf("resolvedVersion() = %q", got)
	}
}

func TestResolvedVersionFallsBackToBuildInfoOrDev(t *testing.T) {
	prev := version
	version = "dev"
	t.Cleanup(func() { version = prev })

	want := "dev"
	if info, ok := debug.ReadBuildInfo(); ok && info.Main.Version != "" && info.Main.Version != "(devel)" {
		want = info.Main.Version
	}

	if got := resolvedVersion(); got != want {
		t.Fatalf("resolvedVersion() = %q, want %q", got, want)
	}
}

func TestVersionCommandWritesResolvedVersion(t *testing.T) {
	prev := version
	version = "9.9.9"
	t.Cleanup(func() { version = prev })

	var out bytes.Buffer
	versionCmd.SetOut(&out)
	versionCmd.Run(versionCmd, nil)

	if got := strings.TrimSpace(out.String()); got != "9.9.9" {
		t.Fatalf("version command output = %q, want %q", got, "9.9.9")
	}
}

func TestCurrentBranchNameErrors(t *testing.T) {
	t.Parallel()

	t.Run("runner error", func(t *testing.T) {
		repoRoot := t.TempDir()
		setTestApp(t, repoRoot, &commandRecordingRunner{
			repoRoot: repoRoot,
			errs: map[string]error{
				"branch --show-current": errors.New("boom"),
			},
		})

		_, err := currentBranchName(context.Background())
		if err == nil || err.Error() != "resolve current branch: boom" {
			t.Fatalf("currentBranchName() error = %v, want wrapped runner error", err)
		}
	})

	t.Run("empty branch", func(t *testing.T) {
		repoRoot := t.TempDir()
		setTestApp(t, repoRoot, &commandRecordingRunner{repoRoot: repoRoot})

		_, err := currentBranchName(context.Background())
		if err == nil || err.Error() != "resolve current branch: empty branch name" {
			t.Fatalf("currentBranchName() error = %v, want empty branch error", err)
		}
	})
}

func TestBootstrapAppDoctorAllowsConfigError(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd() error = %v", err)
	}

	repoRoot := t.TempDir()
	runGit(t, repoRoot, "init", "-b", "main")
	if err := os.WriteFile(filepath.Join(repoRoot, config.FileName), []byte("version: [\n"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	if err := os.Chdir(repoRoot); err != nil {
		t.Fatalf("Chdir() error = %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(wd)
		app = nil
		opts = cliOptions{}
	})

	cmd := &cobra.Command{Use: "doctor"}
	if err := bootstrapApp(nil, cmd); err != nil {
		t.Fatalf("bootstrapApp() error = %v", err)
	}
	if AppContext() == nil || AppContext().ConfigErr == nil {
		t.Fatalf("AppContext() = %#v, want retained config error", AppContext())
	}
	if AppContext().Config.DefaultBase != "main" {
		t.Fatalf("DefaultBase = %q, want main", AppContext().Config.DefaultBase)
	}
}

func TestBootstrapAppReturnsConfigErrorForNonDoctor(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd() error = %v", err)
	}

	repoRoot := t.TempDir()
	runGit(t, repoRoot, "init", "-b", "main")
	if err := os.WriteFile(filepath.Join(repoRoot, config.FileName), []byte("version: [\n"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	if err := os.Chdir(repoRoot); err != nil {
		t.Fatalf("Chdir() error = %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(wd)
		app = nil
		opts = cliOptions{}
	})

	err = bootstrapApp(context.Background(), &cobra.Command{Use: "status"})
	var cfgErr config.Error
	if !errors.As(err, &cfgErr) {
		t.Fatalf("bootstrapApp() error = %v, want config.Error", err)
	}
}

func TestBootstrapAppOutsideRepo(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd() error = %v", err)
	}

	nonRepo := t.TempDir()
	if err := os.Chdir(nonRepo); err != nil {
		t.Fatalf("Chdir() error = %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(wd)
		app = nil
		opts = cliOptions{}
	})

	err = bootstrapApp(context.Background(), &cobra.Command{Use: "status"})
	if err == nil || !strings.Contains(err.Error(), "discover repository root") {
		t.Fatalf("bootstrapApp() error = %v, want discover repository root failure", err)
	}
}
