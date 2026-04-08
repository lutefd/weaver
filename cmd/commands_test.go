package cmd

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
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

func TestResolveBranchSelection(t *testing.T) {
	repoRoot := t.TempDir()
	setTestApp(t, repoRoot, &staticRunner{repoRoot: repoRoot})

	if err := group.NewStore(repoRoot).Create("sprint-42", []string{"feature-a", "feature-b"}); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	cmd := cloneComposeCommand()
	cmd.Flags().Set("group", "sprint-42")
	branches, err := resolveBranchSelection(repoRoot, nil, cmd)
	if err != nil {
		t.Fatalf("resolveBranchSelection() error = %v", err)
	}
	if got := strings.Join(branches, ","); got != "feature-a,feature-b" {
		t.Fatalf("resolveBranchSelection() = %q", got)
	}

	cmd = cloneComposeCommand()
	cmd.Flags().Set("all", "true")
	if _, err := os.Stat(filepath.Join(repoRoot, ".git", "weaver", "deps.yaml")); !os.IsNotExist(err) {
		t.Fatalf("deps file should not exist yet")
	}
	_, err = resolveBranchSelection(repoRoot, nil, cmd)
	if err == nil {
		t.Fatal("resolveBranchSelection() error = nil, want error")
	}
}

func TestResolveBranchSelectionExplicitArgs(t *testing.T) {
	repoRoot := t.TempDir()
	setTestApp(t, repoRoot, &staticRunner{repoRoot: repoRoot})

	cmd := cloneUpdateCommand()
	branches, err := resolveBranchSelection(repoRoot, []string{"main", "feature-a"}, cmd)
	if err != nil {
		t.Fatalf("resolveBranchSelection() error = %v", err)
	}
	if got := strings.Join(branches, ","); got != "main,feature-a" {
		t.Fatalf("resolveBranchSelection() = %q", got)
	}
}

func TestResolveComposeOptions(t *testing.T) {
	cmd := cloneComposeCommand()
	cmd.Flags().Set("dry-run", "true")
	cmd.Flags().Set("create", "integration")

	opts, err := resolveComposeOptions(cmd, "main")
	if err != nil {
		t.Fatalf("resolveComposeOptions() error = %v", err)
	}
	if !opts.DryRun || opts.CreateBranch != "integration" || opts.UpdateBranch != "" {
		t.Fatalf("resolveComposeOptions() = %#v, want dry-run create opts", opts)
	}

	cmd = cloneComposeCommand()
	cmd.Flags().Set("update", "integration")
	opts, err = resolveComposeOptions(cmd, "main")
	if err != nil {
		t.Fatalf("resolveComposeOptions() error = %v", err)
	}
	if opts.UpdateBranch != "integration" || opts.CreateBranch != "" {
		t.Fatalf("resolveComposeOptions() = %#v, want update opts", opts)
	}

	cmd = cloneComposeCommand()
	cmd.Flags().Set("update", "integration")
	cmd.Flags().Set("create", "integration-preview")
	_, err = resolveComposeOptions(cmd, "main")
	var usageErr usageError
	if !errors.As(err, &usageErr) {
		t.Fatalf("resolveComposeOptions() error = %v, want usageError", err)
	}

	cmd = cloneComposeCommand()
	cmd.Flags().Set("create", "main")
	_, err = resolveComposeOptions(cmd, "main")
	if !errors.As(err, &usageErr) {
		t.Fatalf("resolveComposeOptions() error = %v, want usageError", err)
	}

	cmd = cloneComposeCommand()
	cmd.Flags().Set("update", "main")
	_, err = resolveComposeOptions(cmd, "main")
	if !errors.As(err, &usageErr) {
		t.Fatalf("resolveComposeOptions() error = %v, want usageError", err)
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

func TestDoctorCommand(t *testing.T) {
	repoRoot := t.TempDir()
	runGit(t, repoRoot, "init", "-b", "main")
	runGit(t, repoRoot, "config", "user.name", "Command Test")
	runGit(t, repoRoot, "config", "user.email", "command@example.com")
	if _, err := config.Initialize(repoRoot); err != nil {
		t.Fatalf("Initialize() error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(repoRoot, "README.md"), []byte("base\n"), 0o644); err != nil {
		t.Fatalf("write README: %v", err)
	}
	runGit(t, repoRoot, "add", "README.md", config.FileName)
	runGit(t, repoRoot, "commit", "-m", "init")

	setTestApp(t, repoRoot, gitrunner.NewCLIRunner(repoRoot, nil))

	var out bytes.Buffer
	doctorCmd.SetOut(&out)
	if err := doctorCmd.RunE(doctorCmd, nil); err != nil {
		t.Fatalf("doctor error = %v", err)
	}
	if !strings.Contains(out.String(), "summary:") {
		t.Fatalf("doctor output = %q, want summary", out.String())
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
	cmd.Flags().String("base", "", "base")
	cmd.Flags().String("create", "", "create")
	cmd.Flags().String("update", "", "update")
	cmd.Flags().Bool("dry-run", false, "dry-run")
	return cmd
}

func cloneUpdateCommand() *cobra.Command {
	cmd := &cobra.Command{}
	cmd.Flags().String("group", "", "group")
	cmd.Flags().Bool("all", false, "all")
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

func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()

	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=Command Test",
		"GIT_AUTHOR_EMAIL=command@example.com",
		"GIT_COMMITTER_NAME=Command Test",
		"GIT_COMMITTER_EMAIL=command@example.com",
	)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v failed: %v\n%s", args, err, output)
	}
}
