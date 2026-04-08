package cmd

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lutefd/weaver/internal/config"
	gitrunner "github.com/lutefd/weaver/internal/git"
	"github.com/lutefd/weaver/internal/group"
	"github.com/spf13/cobra"
)

func TestStackCommandRejectsSelfDependency(t *testing.T) {
	repoRoot := t.TempDir()
	setTestApp(t, repoRoot, &staticRunner{repoRoot: repoRoot})

	stackCmd.Flags().Set("on", "feature-a")
	t.Cleanup(func() {
		stackCmd.Flags().Set("on", "")
	})

	err := stackCmd.RunE(stackCmd, []string{"feature-a"})
	var usageErr usageError
	if !errors.As(err, &usageErr) {
		t.Fatalf("RunE() error = %v, want usageError", err)
	}
}

func TestGroupCommands(t *testing.T) {
	repoRoot := t.TempDir()
	setTestApp(t, repoRoot, &staticRunner{repoRoot: repoRoot})

	var out bytes.Buffer
	groupCreateCmd.SetOut(&out)
	if err := groupCreateCmd.RunE(groupCreateCmd, []string{"sprint-42", "feature-a", "feature-b"}); err != nil {
		t.Fatalf("group create error = %v", err)
	}

	out.Reset()
	groupListCmd.SetOut(&out)
	if err := groupListCmd.RunE(groupListCmd, nil); err != nil {
		t.Fatalf("group list error = %v", err)
	}
	if got := strings.TrimSpace(out.String()); got != "sprint-42: feature-a, feature-b" {
		t.Fatalf("group list output = %q", got)
	}
}

func TestResolveComposeBranches(t *testing.T) {
	repoRoot := t.TempDir()
	setTestApp(t, repoRoot, &staticRunner{repoRoot: repoRoot})

	if err := group.NewStore(repoRoot).Create("sprint-42", []string{"feature-a", "feature-b"}); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	cmd := cloneComposeCommand()
	cmd.Flags().Set("group", "sprint-42")
	branches, err := resolveComposeBranches(repoRoot, nil, cmd)
	if err != nil {
		t.Fatalf("resolveComposeBranches() error = %v", err)
	}
	if got := strings.Join(branches, ","); got != "feature-a,feature-b" {
		t.Fatalf("resolveComposeBranches() = %q", got)
	}

	cmd = cloneComposeCommand()
	cmd.Flags().Set("all", "true")
	if _, err := os.Stat(filepath.Join(repoRoot, ".git", "weaver", "deps.yaml")); !os.IsNotExist(err) {
		t.Fatalf("deps file should not exist yet")
	}
	_, err = resolveComposeBranches(repoRoot, nil, cmd)
	if err == nil {
		t.Fatal("resolveComposeBranches() error = nil, want error")
	}
}

func TestExportAndImportCommands(t *testing.T) {
	repoRoot := t.TempDir()
	setTestApp(t, repoRoot, &staticRunner{repoRoot: repoRoot})

	stackCmd.Flags().Set("on", "feature-a")
	t.Cleanup(func() {
		stackCmd.Flags().Set("on", "")
	})
	if err := stackCmd.RunE(stackCmd, []string{"feature-b"}); err != nil {
		t.Fatalf("stack error = %v", err)
	}
	if err := group.NewStore(repoRoot).Create("sprint-42", []string{"feature-a", "feature-b"}); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	var exported bytes.Buffer
	exportCmd.SetOut(&exported)
	if err := exportCmd.RunE(exportCmd, nil); err != nil {
		t.Fatalf("export error = %v", err)
	}

	importRepo := t.TempDir()
	setTestApp(t, importRepo, &staticRunner{repoRoot: importRepo})
	exportPath := filepath.Join(t.TempDir(), "state.json")
	if err := os.WriteFile(exportPath, exported.Bytes(), 0o644); err != nil {
		t.Fatalf("write export file: %v", err)
	}

	if err := importCmd.RunE(importCmd, []string{exportPath}); err != nil {
		t.Fatalf("import error = %v", err)
	}

	data, err := os.ReadFile(filepath.Join(importRepo, ".git", "weaver", "deps.yaml"))
	if err != nil {
		t.Fatalf("read imported deps: %v", err)
	}
	if !strings.Contains(string(data), "feature-b: feature-a") {
		t.Fatalf("imported deps missing branch mapping: %s", data)
	}
}

func setTestApp(t *testing.T, repoRoot string, runner gitrunner.Runner) {
	t.Helper()

	prevApp := app
	prevOpts := opts
	app = &App{
		Options: cliOptions{},
		Runner:  runner,
		Config:  &config.Config{Version: config.VersionOne, DefaultBase: "main"},
	}
	opts = cliOptions{}

	t.Cleanup(func() {
		app = prevApp
		opts = prevOpts
	})

	if err := os.MkdirAll(filepath.Join(repoRoot, ".git"), 0o755); err != nil {
		t.Fatalf("mkdir .git: %v", err)
	}
}

func cloneComposeCommand() *cobra.Command {
	cmd := &cobra.Command{}
	cmd.Flags().String("group", "", "group")
	cmd.Flags().Bool("all", false, "all")
	cmd.Flags().Bool("dry-run", false, "dry-run")
	return cmd
}

type staticRunner struct {
	repoRoot string
}

func (r *staticRunner) Run(_ context.Context, _ ...string) (gitrunner.Result, error) {
	return gitrunner.Result{}, nil
}

func (r *staticRunner) RepoRoot() string {
	return r.repoRoot
}
