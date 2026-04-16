package cmd

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/lutefd/weaver/internal/composer"
	"github.com/lutefd/weaver/internal/config"
	"github.com/lutefd/weaver/internal/deps"
	"github.com/lutefd/weaver/internal/doctor"
	gitrunner "github.com/lutefd/weaver/internal/git"
	"github.com/lutefd/weaver/internal/group"
	weaverintegration "github.com/lutefd/weaver/internal/integration"
	"github.com/lutefd/weaver/internal/merger"
	"github.com/lutefd/weaver/internal/stack"
	"github.com/lutefd/weaver/internal/ui"
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

func TestIntegrationCommands(t *testing.T) {
	repoRoot := t.TempDir()
	setTestApp(t, repoRoot, &staticRunner{repoRoot: repoRoot})

	var out bytes.Buffer
	integrationSaveCmd.SetOut(&out)
	integrationSaveCmd.Flags().Set("base", "main")
	t.Cleanup(func() {
		integrationSaveCmd.Flags().Set("base", "")
	})
	if err := integrationSaveCmd.RunE(integrationSaveCmd, []string{"integration", "feature-a", "feature-b"}); err != nil {
		t.Fatalf("integration save error = %v", err)
	}

	out.Reset()
	integrationShowCmd.SetOut(&out)
	if err := integrationShowCmd.RunE(integrationShowCmd, []string{"integration"}); err != nil {
		t.Fatalf("integration show error = %v", err)
	}
	if got := out.String(); !strings.Contains(got, "base: main") || !strings.Contains(got, "feature-a -> feature-b") {
		t.Fatalf("integration show output = %q", got)
	}
}

func TestIntegrationBranchCommands(t *testing.T) {
	repoRoot := t.TempDir()
	runner := &commandRecordingRunner{
		repoRoot: repoRoot,
		results: map[string]gitrunner.Result{
			"branch --show-current":                          {Stdout: "feature-b"},
			"show-ref --verify --quiet refs/heads/release-1": {ExitCode: 1},
		},
		errs: map[string]error{
			"show-ref --verify --quiet refs/heads/release-1": errors.New("exit status 1"),
		},
	}
	setTestApp(t, repoRoot, runner)

	store := weaverintegration.NewBranchStore(repoRoot)
	if err := store.Track("release-1", weaverintegration.BranchRecord{
		Base:        "main",
		Branches:    []string{"feature-a", "feature-b"},
		Skipped:     []string{"feature-c"},
		Integration: "staging",
	}); err != nil {
		t.Fatalf("Track() error = %v", err)
	}

	var out bytes.Buffer
	integrationBranchListCmd.SetOut(&out)
	if err := integrationBranchListCmd.RunE(integrationBranchListCmd, nil); err != nil {
		t.Fatalf("integration branch list error = %v", err)
	}
	if got := out.String(); !strings.Contains(got, "release-1: status=missing") || !strings.Contains(got, "integration=staging") {
		t.Fatalf("integration branch list output = %q", got)
	}

	out.Reset()
	integrationBranchDeleteCmd.SetOut(&out)
	if err := integrationBranchDeleteCmd.RunE(integrationBranchDeleteCmd, []string{"release-1"}); err != nil {
		t.Fatalf("integration branch delete error = %v", err)
	}
	if got := out.String(); !strings.Contains(got, "removed tracked integration branch release-1") {
		t.Fatalf("integration branch delete output = %q", got)
	}
	if _, ok, err := store.Get("release-1"); err != nil {
		t.Fatalf("Get() error = %v", err)
	} else if ok {
		t.Fatal("Get() ok = true, want false")
	}
}

func TestIntegrationBranchDeleteCommandDeletesGitBranch(t *testing.T) {
	repoRoot := t.TempDir()
	runner := &commandRecordingRunner{
		repoRoot: repoRoot,
		results: map[string]gitrunner.Result{
			"branch --show-current":                          {Stdout: "feature-b"},
			"show-ref --verify --quiet refs/heads/release-1": {},
		},
	}
	setTestApp(t, repoRoot, runner)

	store := weaverintegration.NewBranchStore(repoRoot)
	if err := store.Track("release-1", weaverintegration.BranchRecord{
		Base:     "main",
		Branches: []string{"feature-a"},
	}); err != nil {
		t.Fatalf("Track() error = %v", err)
	}

	var out bytes.Buffer
	integrationBranchDeleteCmd.SetOut(&out)
	if err := integrationBranchDeleteCmd.RunE(integrationBranchDeleteCmd, []string{"release-1"}); err != nil {
		t.Fatalf("integration branch delete error = %v", err)
	}
	if got := out.String(); !strings.Contains(got, "deleted integration branch release-1") {
		t.Fatalf("integration branch delete output = %q", got)
	}

	wantCalls := []string{
		"show-ref --verify --quiet refs/heads/release-1",
		"branch --show-current",
		"branch -D release-1",
	}
	if got := runner.calls; strings.Join(got, "\n") != strings.Join(wantCalls, "\n") {
		t.Fatalf("calls = %#v, want %#v", got, wantCalls)
	}
}

func TestIntegrationBranchDeleteCommandRejectsCurrentBranch(t *testing.T) {
	repoRoot := t.TempDir()
	runner := &commandRecordingRunner{
		repoRoot: repoRoot,
		results: map[string]gitrunner.Result{
			"branch --show-current":                          {Stdout: "release-1"},
			"show-ref --verify --quiet refs/heads/release-1": {},
		},
	}
	setTestApp(t, repoRoot, runner)

	store := weaverintegration.NewBranchStore(repoRoot)
	if err := store.Track("release-1", weaverintegration.BranchRecord{
		Base:     "main",
		Branches: []string{"feature-a"},
	}); err != nil {
		t.Fatalf("Track() error = %v", err)
	}

	err := integrationBranchDeleteCmd.RunE(integrationBranchDeleteCmd, []string{"release-1"})
	if err == nil || !strings.Contains(err.Error(), `cannot delete current branch "release-1"`) {
		t.Fatalf("integration branch delete error = %v, want current branch error", err)
	}
	if _, ok, err := store.Get("release-1"); err != nil {
		t.Fatalf("Get() error = %v", err)
	} else if !ok {
		t.Fatal("Get() ok = false, want true")
	}
}

func TestIntegrationDoctorCommand(t *testing.T) {
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
	runGit(t, repoRoot, "checkout", "-b", "feature-a")
	if err := os.WriteFile(filepath.Join(repoRoot, "feature-a.txt"), []byte("feature-a\n"), 0o644); err != nil {
		t.Fatalf("write feature-a: %v", err)
	}
	runGit(t, repoRoot, "add", "feature-a.txt")
	runGit(t, repoRoot, "commit", "-m", "feature-a")
	runGit(t, repoRoot, "checkout", "main")

	if err := weaverintegration.NewStore(repoRoot).Save("integration", weaverintegration.Recipe{
		Base:     "main",
		Branches: []string{"feature-a"},
	}); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	setTestApp(t, repoRoot, gitrunner.NewCLIRunner(repoRoot, nil))

	var out bytes.Buffer
	integrationDoctorCmd.SetOut(&out)
	if err := integrationDoctorCmd.RunE(integrationDoctorCmd, []string{"integration"}); err != nil {
		t.Fatalf("integration doctor error = %v", err)
	}
	if !strings.Contains(out.String(), "summary:") {
		t.Fatalf("integration doctor output = %q, want summary", out.String())
	}
}

func TestIntegrationDoctorCommandWarnsOnDriftWithoutFailing(t *testing.T) {
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
	runGit(t, repoRoot, "checkout", "-b", "feature-a")
	if err := os.WriteFile(filepath.Join(repoRoot, "feature-a.txt"), []byte("feature-a\n"), 0o644); err != nil {
		t.Fatalf("write feature-a: %v", err)
	}
	runGit(t, repoRoot, "add", "feature-a.txt")
	runGit(t, repoRoot, "commit", "-m", "feature-a")
	runGit(t, repoRoot, "checkout", "main")
	for i := 0; i < 10; i++ {
		filename := filepath.Join(repoRoot, "main.txt")
		if err := os.WriteFile(filename, []byte(strings.Repeat("m", i+1)), 0o644); err != nil {
			t.Fatalf("write main.txt: %v", err)
		}
		runGit(t, repoRoot, "add", "main.txt")
		runGit(t, repoRoot, "commit", "-m", "main-update")
	}

	if err := weaverintegration.NewStore(repoRoot).Save("integration", weaverintegration.Recipe{
		Base:     "main",
		Branches: []string{"feature-a"},
	}); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	setTestApp(t, repoRoot, gitrunner.NewCLIRunner(repoRoot, nil))

	var out bytes.Buffer
	integrationDoctorCmd.SetOut(&out)
	if err := integrationDoctorCmd.RunE(integrationDoctorCmd, []string{"integration"}); err != nil {
		t.Fatalf("integration doctor error = %v, want drift warning only", err)
	}
	if !strings.Contains(out.String(), `WARN branch "feature-a" is 10 commit(s) behind expected parent "main" (1 ahead)`) {
		t.Fatalf("integration doctor output = %q, want drift warning", out.String())
	}
}

func TestSyncCommandMergeMode(t *testing.T) {
	repoRoot := t.TempDir()
	runner := &commandRecordingRunner{
		repoRoot: repoRoot,
		results: map[string]gitrunner.Result{
			"branch --show-current": {Stdout: "feature-c"},
		},
	}
	setTestApp(t, repoRoot, runner)
	if err := deps.NewLocalSource(repoRoot).Replace(map[string]string{
		"feature-b": "feature-a",
		"feature-c": "feature-b",
	}); err != nil {
		t.Fatalf("Replace() error = %v", err)
	}

	var out bytes.Buffer
	syncCmd.SetOut(&out)
	if err := syncCmd.Flags().Set("merge", "true"); err != nil {
		t.Fatalf("Set(merge) error = %v", err)
	}
	t.Cleanup(func() {
		syncCmd.Flags().Set("merge", "false")
	})

	if err := syncCmd.RunE(syncCmd, []string{"feature-c"}); err != nil {
		t.Fatalf("sync error = %v", err)
	}
	if !strings.Contains(out.String(), "synced feature-c") {
		t.Fatalf("sync output = %q, want synced feature-c", out.String())
	}

	wantCalls := []string{
		"branch --show-current",
		"checkout feature-a",
		"merge --no-edit main",
		"checkout feature-b",
		"merge --no-edit feature-a",
		"checkout feature-c",
		"merge --no-edit feature-b",
		"checkout feature-c",
	}
	if got := runner.calls; strings.Join(got, "\n") != strings.Join(wantCalls, "\n") {
		t.Fatalf("calls = %#v, want %#v", got, wantCalls)
	}
}

func TestContinueCommandResumesMergeSync(t *testing.T) {
	repoRoot := t.TempDir()
	runner := &commandRecordingRunner{repoRoot: repoRoot}
	setTestApp(t, repoRoot, runner)
	if err := merger.NewStateStore(repoRoot).Save(&merger.State{
		OriginalBranch: "feature-c",
		BaseBranch:     "main",
		AllBranches:    []string{"feature-a"},
		Current:        "feature-a",
		CurrentOnto:    "main",
	}); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	var out bytes.Buffer
	continueCmd.SetOut(&out)
	if err := continueCmd.RunE(continueCmd, nil); err != nil {
		t.Fatalf("continue error = %v", err)
	}
	if !strings.Contains(out.String(), "continued stack sync") {
		t.Fatalf("continue output = %q, want continued stack sync", out.String())
	}

	wantCalls := []string{"merge --continue", "checkout feature-c"}
	if got := runner.calls; strings.Join(got, "\n") != strings.Join(wantCalls, "\n") {
		t.Fatalf("calls = %#v, want %#v", got, wantCalls)
	}
}

func TestEstimateComposeOps(t *testing.T) {
	repoRoot := t.TempDir()
	setTestApp(t, repoRoot, &staticRunner{repoRoot: repoRoot})

	dag, err := stack.NewDAG([]stack.Dependency{
		{Branch: "feature-b", Parent: "feature-a"},
		{Branch: "feature-c", Parent: "feature-b"},
	})
	if err != nil {
		t.Fatalf("NewDAG() error = %v", err)
	}

	got := estimateComposeOps(dag, []string{"feature-c"}, "main", composer.ComposeOpts{})
	if got != 6 {
		t.Fatalf("estimateComposeOps() = %d, want 6", got)
	}
}

func TestEstimateComposeOpsWithSkippedBranchAndCreate(t *testing.T) {
	repoRoot := t.TempDir()
	setTestApp(t, repoRoot, &staticRunner{repoRoot: repoRoot})

	dag, err := stack.NewDAG([]stack.Dependency{
		{Branch: "feature-b", Parent: "feature-a"},
		{Branch: "feature-c", Parent: "feature-b"},
	})
	if err != nil {
		t.Fatalf("NewDAG() error = %v", err)
	}

	got := estimateComposeOps(dag, []string{"feature-c"}, "main", composer.ComposeOpts{
		CreateBranch: "integration",
		SkipBranches: []string{"feature-b"},
	})
	if got != 6 {
		t.Fatalf("estimateComposeOps() = %d, want 6", got)
	}
}

func TestAbortCommandAbortsMergeSync(t *testing.T) {
	repoRoot := t.TempDir()
	runner := &commandRecordingRunner{repoRoot: repoRoot}
	setTestApp(t, repoRoot, runner)
	if err := merger.NewStateStore(repoRoot).Save(&merger.State{
		OriginalBranch: "feature-c",
	}); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	var out bytes.Buffer
	abortCmd.SetOut(&out)
	if err := abortCmd.RunE(abortCmd, nil); err != nil {
		t.Fatalf("abort error = %v", err)
	}
	if !strings.Contains(out.String(), "aborted stack sync") {
		t.Fatalf("abort output = %q, want aborted stack sync", out.String())
	}

	wantCalls := []string{"merge --abort", "checkout feature-c"}
	if got := runner.calls; strings.Join(got, "\n") != strings.Join(wantCalls, "\n") {
		t.Fatalf("calls = %#v, want %#v", got, wantCalls)
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
	selection, err := resolveBranchSelection(repoRoot, nil, cmd)
	if err != nil {
		t.Fatalf("resolveBranchSelection() error = %v", err)
	}
	if got := strings.Join(selection.Branches, ","); got != "feature-a,feature-b" {
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
	selection, err := resolveBranchSelection(repoRoot, []string{"main", "feature-a"}, cmd)
	if err != nil {
		t.Fatalf("resolveBranchSelection() error = %v", err)
	}
	if got := strings.Join(selection.Branches, ","); got != "main,feature-a" {
		t.Fatalf("resolveBranchSelection() = %q", got)
	}
}

func TestResolveBranchSelectionIntegration(t *testing.T) {
	repoRoot := t.TempDir()
	setTestApp(t, repoRoot, &staticRunner{repoRoot: repoRoot})

	if err := weaverintegration.NewStore(repoRoot).Save("integration", weaverintegration.Recipe{
		Base:     "main",
		Branches: []string{"feature-a", "feature-b"},
	}); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	cmd := cloneComposeCommand()
	cmd.Flags().Set("integration", "integration")
	selection, err := resolveBranchSelection(repoRoot, nil, cmd)
	if err != nil {
		t.Fatalf("resolveBranchSelection() error = %v", err)
	}
	if selection.IntegrationName != "integration" {
		t.Fatalf("IntegrationName = %q, want integration", selection.IntegrationName)
	}
	if selection.Base != "main" {
		t.Fatalf("Base = %q, want main", selection.Base)
	}
	if got := strings.Join(selection.Branches, ","); got != "feature-a,feature-b" {
		t.Fatalf("Branches = %q, want feature-a,feature-b", got)
	}
}

func TestResolveBranchSelectionFlagError(t *testing.T) {
	repoRoot := t.TempDir()
	setTestApp(t, repoRoot, &staticRunner{repoRoot: repoRoot})

	_, err := resolveBranchSelection(repoRoot, nil, &cobra.Command{})
	if err == nil || !strings.Contains(err.Error(), "flag accessed but not defined") {
		t.Fatalf("resolveBranchSelection() error = %v, want flag error", err)
	}
}

func TestResolveBranchSelectionModeAdditionalCases(t *testing.T) {
	repoRoot := t.TempDir()
	setTestApp(t, repoRoot, &staticRunner{repoRoot: repoRoot})

	if err := group.NewStore(repoRoot).Create("empty", nil); err != nil {
		t.Fatalf("Create(empty) error = %v", err)
	}
	if err := deps.NewLocalSource(repoRoot).Replace(map[string]string{
		"feature-b": "feature-a",
		"feature-a": "main",
	}); err != nil {
		t.Fatalf("Replace() error = %v", err)
	}

	_, err := resolveBranchSelectionMode(repoRoot, []string{"feature-a"}, "sprint-42", "", false)
	var usageErr usageError
	if !errors.As(err, &usageErr) {
		t.Fatalf("resolveBranchSelectionMode() error = %v, want usageError", err)
	}

	_, err = resolveBranchSelectionMode(repoRoot, nil, "missing", "", false)
	if err == nil || !strings.Contains(err.Error(), `group "missing" does not exist`) {
		t.Fatalf("resolveBranchSelectionMode() error = %v, want missing group error", err)
	}

	_, err = resolveBranchSelectionMode(repoRoot, nil, "empty", "", false)
	if err == nil || !strings.Contains(err.Error(), `group "empty" is empty`) {
		t.Fatalf("resolveBranchSelectionMode() error = %v, want empty group error", err)
	}

	_, err = resolveBranchSelectionMode(repoRoot, nil, "", "missing", false)
	if err == nil || !strings.Contains(err.Error(), `integration "missing" does not exist`) {
		t.Fatalf("resolveBranchSelectionMode() error = %v, want missing integration error", err)
	}

	selection, err := resolveBranchSelectionMode(repoRoot, nil, "", "", true)
	if err != nil {
		t.Fatalf("resolveBranchSelectionMode() error = %v", err)
	}
	if got := strings.Join(selection.Branches, ","); got != "feature-a,feature-b" {
		t.Fatalf("resolveBranchSelectionMode() branches = %q, want feature-a,feature-b", got)
	}
}

func TestResolveBranchSelectionAdditionalFlagErrors(t *testing.T) {
	repoRoot := t.TempDir()
	setTestApp(t, repoRoot, &staticRunner{repoRoot: repoRoot})

	cmd := &cobra.Command{}
	cmd.Flags().String("group", "", "group")
	cmd.Flags().String("all", "", "all")
	_, err := resolveBranchSelection(repoRoot, nil, cmd)
	if err == nil || !strings.Contains(err.Error(), "trying to get bool value") {
		t.Fatalf("resolveBranchSelection() error = %v, want bool flag error", err)
	}

	cmd = &cobra.Command{}
	cmd.Flags().String("group", "", "group")
	cmd.Flags().Bool("all", false, "all")
	cmd.Flags().Int("integration", 0, "integration")
	_, err = resolveBranchSelection(repoRoot, nil, cmd)
	if err == nil || !strings.Contains(err.Error(), "trying to get string value") {
		t.Fatalf("resolveBranchSelection() error = %v, want integration string flag error", err)
	}

	_, err = resolveBranchSelectionMode(repoRoot, nil, "", "", false)
	var usageErr usageError
	if !errors.As(err, &usageErr) {
		t.Fatalf("resolveBranchSelectionMode() error = %v, want usageError", err)
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

func TestRecordComposeIntegrationBranch(t *testing.T) {
	repoRoot := t.TempDir()
	setTestApp(t, repoRoot, &staticRunner{repoRoot: repoRoot})

	selection := branchSelection{IntegrationName: "staging"}
	result := &composer.ComposeResult{
		BaseBranch:    "main",
		Order:         []string{"feature-a", "feature-b"},
		Skipped:       []string{"feature-c"},
		CreatedBranch: "release-1",
	}

	if err := recordComposeIntegrationBranch(selection, result); err != nil {
		t.Fatalf("recordComposeIntegrationBranch() error = %v", err)
	}

	got, ok, err := weaverintegration.NewBranchStore(repoRoot).Get("release-1")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if !ok {
		t.Fatal("Get() ok = false, want true")
	}
	want := weaverintegration.BranchRecord{
		Base:        "main",
		Branches:    []string{"feature-a", "feature-b"},
		Skipped:     []string{"feature-c"},
		Integration: "staging",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("tracked branch = %#v, want %#v", got, want)
	}
}

func TestRecordComposeIntegrationBranchTracksUpdatedBranches(t *testing.T) {
	repoRoot := t.TempDir()
	setTestApp(t, repoRoot, &staticRunner{repoRoot: repoRoot})

	store := weaverintegration.NewBranchStore(repoRoot)
	if err := store.Track("release-1", weaverintegration.BranchRecord{
		Base:     "main",
		Branches: []string{"feature-a"},
	}); err != nil {
		t.Fatalf("Track() error = %v", err)
	}

	if err := recordComposeIntegrationBranch(branchSelection{}, &composer.ComposeResult{
		BaseBranch:    "integration",
		Order:         []string{"feature-a", "feature-b"},
		UpdatedBranch: "release-1",
	}); err != nil {
		t.Fatalf("recordComposeIntegrationBranch(update tracked) error = %v", err)
	}
	got, ok, err := store.Get("release-1")
	if err != nil {
		t.Fatalf("Get(release-1) error = %v", err)
	}
	if !ok || got.Base != "integration" || strings.Join(got.Branches, ",") != "feature-a,feature-b" {
		t.Fatalf("Get(release-1) = %#v, ok=%v", got, ok)
	}

	if err := recordComposeIntegrationBranch(branchSelection{}, &composer.ComposeResult{
		BaseBranch:    "main",
		Order:         []string{"feature-c"},
		UpdatedBranch: "release-2",
	}); err != nil {
		t.Fatalf("recordComposeIntegrationBranch(update new) error = %v", err)
	}
	got, ok, err = store.Get("release-2")
	if err != nil {
		t.Fatalf("Get(release-2) error = %v", err)
	}
	if !ok {
		t.Fatal("Get(release-2) ok = false, want true")
	}
	if got.Base != "main" || strings.Join(got.Branches, ",") != "feature-c" {
		t.Fatalf("Get(release-2) = %#v, want tracked update branch", got)
	}

	if err := recordComposeIntegrationBranch(branchSelection{}, &composer.ComposeResult{
		BaseBranch:    "main",
		Order:         []string{"feature-a"},
		CreatedBranch: "dry-run",
		DryRun:        true,
	}); err != nil {
		t.Fatalf("recordComposeIntegrationBranch(dry-run) error = %v", err)
	}
	if _, ok, err := store.Get("dry-run"); err != nil {
		t.Fatalf("Get(dry-run) error = %v", err)
	} else if ok {
		t.Fatal("Get(dry-run) ok = true, want false")
	}
}

func TestRecordComposeIntegrationBranchErrors(t *testing.T) {
	repoRoot := t.TempDir()
	setTestApp(t, repoRoot, &staticRunner{repoRoot: repoRoot})

	if err := os.MkdirAll(filepath.Join(repoRoot, ".git", "weaver", "integration-branches.yaml"), 0o755); err != nil {
		t.Fatalf("MkdirAll(path) error = %v", err)
	}

	err := recordComposeIntegrationBranch(branchSelection{}, &composer.ComposeResult{
		BaseBranch:    "main",
		Order:         []string{"feature-a"},
		CreatedBranch: "release-1",
	})
	if err == nil || !strings.Contains(err.Error(), `track integration branch "release-1"`) {
		t.Fatalf("recordComposeIntegrationBranch(create) error = %v", err)
	}

	err = recordComposeIntegrationBranch(branchSelection{}, &composer.ComposeResult{
		BaseBranch:    "main",
		Order:         []string{"feature-a"},
		UpdatedBranch: "release-1",
	})
	if err == nil || !strings.Contains(err.Error(), `track integration branch "release-1"`) {
		t.Fatalf("recordComposeIntegrationBranch(update) error = %v", err)
	}
}

func TestFormatSkipped(t *testing.T) {
	if got := formatSkipped(nil); got != "" {
		t.Fatalf("formatSkipped(nil) = %q, want empty", got)
	}
	if got := formatSkipped([]string{"feature-a", "feature-b"}); got != " (skipped: feature-a, feature-b)" {
		t.Fatalf("formatSkipped(...) = %q", got)
	}
}

func TestFormatComposeConflictError(t *testing.T) {
	err := formatComposeConflictError(composer.ConflictError{Branch: "feature-a", Files: []string{"a.go", "b.go"}})
	if got := err.Error(); !strings.Contains(got, "feature-a") || !strings.Contains(got, "a.go, b.go") {
		t.Fatalf("formatComposeConflictError() = %q", got)
	}

	err = formatComposeConflictError(composer.ConflictError{Branch: "feature-b"})
	if got := err.Error(); got != "compose failed while merging feature-b" {
		t.Fatalf("formatComposeConflictError() = %q", got)
	}
}

func TestRenderManualMergeSummary(t *testing.T) {
	var out bytes.Buffer
	renderManualMergeSummary(&out, &composer.ComposeResult{
		CreatedBranch: "integration",
		Skipped:       []string{"feature-a"},
	})
	if got := out.String(); !strings.Contains(got, "manual merge required onto integration: feature-a") {
		t.Fatalf("renderManualMergeSummary() = %q", got)
	}

	out.Reset()
	renderManualMergeSummary(&out, &composer.ComposeResult{
		UpdatedBranch: "integration",
		Skipped:       []string{"feature-b"},
	})
	if got := out.String(); !strings.Contains(got, "manual merge required onto integration: feature-b") {
		t.Fatalf("renderManualMergeSummary() updated = %q", got)
	}

	out.Reset()
	renderManualMergeSummary(&out, &composer.ComposeResult{Skipped: []string{"feature-c"}})
	if got := out.String(); !strings.Contains(got, "manual merge still required for skipped branches: feature-c") {
		t.Fatalf("renderManualMergeSummary() ephemeral = %q", got)
	}

	out.Reset()
	renderManualMergeSummary(&out, nil)
	if out.Len() != 0 {
		t.Fatalf("renderManualMergeSummary(nil) = %q", out.String())
	}
}

func TestAppendUniqueBranch(t *testing.T) {
	if got := appendUniqueBranch([]string{"feature-a"}, "feature-a"); strings.Join(got, ",") != "feature-a" {
		t.Fatalf("appendUniqueBranch duplicate = %#v", got)
	}
	if got := appendUniqueBranch([]string{"feature-a"}, "feature-b"); strings.Join(got, ",") != "feature-a,feature-b" {
		t.Fatalf("appendUniqueBranch new = %#v", got)
	}
}

func TestFormatTrackedBranchSlice(t *testing.T) {
	if got := formatTrackedBranchSlice(nil); got != "(none)" {
		t.Fatalf("formatTrackedBranchSlice(nil) = %q", got)
	}
	if got := formatTrackedBranchSlice([]string{"feature-a", "feature-b"}); got != "feature-a, feature-b" {
		t.Fatalf("formatTrackedBranchSlice(...) = %q", got)
	}
}

func TestTrackedIntegrationBranchEntryStatus(t *testing.T) {
	tests := []struct {
		name  string
		entry trackedIntegrationBranchEntry
		want  string
	}{
		{
			name:  "missing",
			entry: trackedIntegrationBranchEntry{Name: "release-1"},
			want:  "missing",
		},
		{
			name: "present",
			entry: trackedIntegrationBranchEntry{
				Name:   "release-1",
				Exists: true,
				Record: weaverintegration.BranchRecord{Base: "main", Branches: []string{"feature-a"}},
			},
			want: "present",
		},
		{
			name: "pending fallback",
			entry: trackedIntegrationBranchEntry{
				Name:   "release-1",
				Exists: true,
				Record: weaverintegration.BranchRecord{Base: "main", Branches: []string{"feature-a"}, Skipped: []string{"feature-b"}},
			},
			want: "pending",
		},
		{
			name: "partial",
			entry: trackedIntegrationBranchEntry{
				Name:            "release-1",
				Exists:          true,
				IncludedSkipped: []string{"feature-b"},
				PendingSkipped:  []string{"feature-c"},
			},
			want: "partial",
		},
		{
			name: "integrated",
			entry: trackedIntegrationBranchEntry{
				Name:            "release-1",
				Exists:          true,
				IncludedSkipped: []string{"feature-b"},
			},
			want: "integrated",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.entry.Status(); got != tt.want {
				t.Fatalf("Status() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestWriteTrackedIntegrationBranchList(t *testing.T) {
	term := ui.NewTerminal(bytes.NewBuffer(nil), &bytes.Buffer{})

	var out bytes.Buffer
	if err := writeTrackedIntegrationBranchList(&out, term, false, nil); err != nil {
		t.Fatalf("writeTrackedIntegrationBranchList(empty) error = %v", err)
	}
	if got := out.String(); !strings.Contains(got, "no integration branches") {
		t.Fatalf("writeTrackedIntegrationBranchList(empty) = %q", got)
	}

	out.Reset()
	entries := []trackedIntegrationBranchEntry{{
		Name:            "release-1",
		Exists:          true,
		IncludedSkipped: []string{"feature-b"},
		PendingSkipped:  []string{"feature-c"},
		Record:          weaverintegration.BranchRecord{Base: "main", Branches: []string{"feature-a"}, Skipped: []string{"feature-b", "feature-c"}, Integration: "staging"},
	}}
	if err := writeTrackedIntegrationBranchList(&out, term, false, entries); err != nil {
		t.Fatalf("writeTrackedIntegrationBranchList(plain) error = %v", err)
	}
	if got := out.String(); !strings.Contains(got, "release-1: status=partial base=main branches=feature-a integrated=feature-b skipped=feature-c integration=staging") {
		t.Fatalf("writeTrackedIntegrationBranchList(plain) = %q", got)
	}

	out.Reset()
	if err := writeTrackedIntegrationBranchList(&out, term, true, entries); err != nil {
		t.Fatalf("writeTrackedIntegrationBranchList(styled) error = %v", err)
	}
	if got := out.String(); !strings.Contains(got, "Integration Branches") || !strings.Contains(got, "release-1") {
		t.Fatalf("writeTrackedIntegrationBranchList(styled) = %q", got)
	}
}

func TestResolveTrackedIntegrationBranchProgress(t *testing.T) {
	runner := &commandRecordingRunner{
		results: map[string]gitrunner.Result{
			"show-ref --verify --quiet refs/heads/feature-b": {},
			"show-ref --verify --quiet refs/heads/feature-c": {},
			"show-ref --verify --quiet refs/heads/feature-d": {ExitCode: 1},
			"merge-base --is-ancestor feature-b release-1":   {},
			"merge-base --is-ancestor feature-c release-1":   {ExitCode: 1},
		},
		errs: map[string]error{
			"show-ref --verify --quiet refs/heads/feature-d": errors.New("exit status 1"),
			"merge-base --is-ancestor feature-c release-1":   errors.New("exit status 1"),
		},
	}

	included, pending, err := resolveTrackedIntegrationBranchProgress(context.Background(), runner, "release-1", weaverintegration.BranchRecord{
		Skipped: []string{"feature-b", "feature-c", "feature-d"},
	}, true)
	if err != nil {
		t.Fatalf("resolveTrackedIntegrationBranchProgress() error = %v", err)
	}
	if !reflect.DeepEqual(included, []string{"feature-b"}) {
		t.Fatalf("included = %#v, want feature-b", included)
	}
	if !reflect.DeepEqual(pending, []string{"feature-c", "feature-d"}) {
		t.Fatalf("pending = %#v, want feature-c, feature-d", pending)
	}

	included, pending, err = resolveTrackedIntegrationBranchProgress(context.Background(), runner, "release-1", weaverintegration.BranchRecord{
		Skipped: []string{"feature-b"},
	}, false)
	if err != nil {
		t.Fatalf("resolveTrackedIntegrationBranchProgress(missing branch) error = %v", err)
	}
	if included != nil {
		t.Fatalf("included for missing branch = %#v, want nil", included)
	}
	if !reflect.DeepEqual(pending, []string{"feature-b"}) {
		t.Fatalf("pending for missing branch = %#v, want feature-b", pending)
	}
}

func TestResolveTrackedIntegrationBranchProgressErrors(t *testing.T) {
	runner := &rawCommandRunner{
		outcomes: map[string]runnerOutcome{
			"show-ref --verify --quiet refs/heads/feature-b": {
				result: gitrunner.Result{},
				err:    errors.New("boom"),
			},
		},
	}
	_, _, err := resolveTrackedIntegrationBranchProgress(context.Background(), runner, "release-1", weaverintegration.BranchRecord{
		Skipped: []string{"feature-b"},
	}, true)
	if err == nil || !strings.Contains(err.Error(), `check whether skipped branch "feature-b" is integrated into "release-1"`) {
		t.Fatalf("resolveTrackedIntegrationBranchProgress() error = %v", err)
	}

	runner = &rawCommandRunner{
		outcomes: map[string]runnerOutcome{
			"show-ref --verify --quiet refs/heads/feature-b": {
				result: gitrunner.Result{},
			},
			"merge-base --is-ancestor feature-b release-1": {
				result: gitrunner.Result{},
				err:    errors.New("boom"),
			},
		},
	}
	_, _, err = resolveTrackedIntegrationBranchProgress(context.Background(), runner, "release-1", weaverintegration.BranchRecord{
		Skipped: []string{"feature-b"},
	}, true)
	if err == nil || !strings.Contains(err.Error(), `check whether skipped branch "feature-b" is integrated into "release-1"`) {
		t.Fatalf("resolveTrackedIntegrationBranchProgress(merge-base) error = %v", err)
	}
}

func TestWriteTrackedIntegrationBranchDeleteResult(t *testing.T) {
	term := ui.NewTerminal(bytes.NewBuffer(nil), &bytes.Buffer{})

	var out bytes.Buffer
	if err := writeTrackedIntegrationBranchDeleteResult(&out, term, false, "release-1", trackedIntegrationBranchDeleteResult{DeletedBranch: true}); err != nil {
		t.Fatalf("writeTrackedIntegrationBranchDeleteResult(plain deleted) error = %v", err)
	}
	if got := out.String(); !strings.Contains(got, "deleted integration branch release-1") {
		t.Fatalf("writeTrackedIntegrationBranchDeleteResult(plain deleted) = %q", got)
	}

	out.Reset()
	if err := writeTrackedIntegrationBranchDeleteResult(&out, term, false, "release-1", trackedIntegrationBranchDeleteResult{}); err != nil {
		t.Fatalf("writeTrackedIntegrationBranchDeleteResult(plain missing) error = %v", err)
	}
	if got := out.String(); !strings.Contains(got, "removed tracked integration branch release-1") {
		t.Fatalf("writeTrackedIntegrationBranchDeleteResult(plain missing) = %q", got)
	}

	out.Reset()
	if err := writeTrackedIntegrationBranchDeleteResult(&out, term, true, "release-1", trackedIntegrationBranchDeleteResult{DeletedBranch: true}); err != nil {
		t.Fatalf("writeTrackedIntegrationBranchDeleteResult(styled deleted) error = %v", err)
	}
	if got := out.String(); !strings.Contains(got, "Integration Branch Deleted") || !strings.Contains(got, "release-1") {
		t.Fatalf("writeTrackedIntegrationBranchDeleteResult(styled deleted) = %q", got)
	}

	out.Reset()
	if err := writeTrackedIntegrationBranchDeleteResult(&out, term, true, "release-1", trackedIntegrationBranchDeleteResult{}); err != nil {
		t.Fatalf("writeTrackedIntegrationBranchDeleteResult(styled missing) error = %v", err)
	}
	if got := out.String(); !strings.Contains(got, "already missing locally") {
		t.Fatalf("writeTrackedIntegrationBranchDeleteResult(styled missing) = %q", got)
	}
}

func TestPromptSkipOnComposeConflict(t *testing.T) {
	cmd := &cobra.Command{}
	cmd.SetIn(bytes.NewBufferString("skip\n"))
	var out bytes.Buffer
	cmd.SetOut(&out)

	skip, err := promptSkipOnComposeConflict(cmd, composer.ConflictError{
		Branch: "feature-b",
		Files:  []string{"app/service.go"},
		Err:    errors.New("exit status 1"),
	})
	if err != nil {
		t.Fatalf("promptSkipOnComposeConflict() error = %v", err)
	}
	if !skip {
		t.Fatal("promptSkipOnComposeConflict() = false, want true")
	}
	if !strings.Contains(out.String(), "app/service.go") {
		t.Fatalf("prompt output = %q, want conflicted file", out.String())
	}
}

func TestPromptSkipOnComposeConflictRetriesAndAborts(t *testing.T) {
	cmd := &cobra.Command{}
	cmd.SetIn(bytes.NewBufferString("later\nabort\n"))
	var out bytes.Buffer
	cmd.SetOut(&out)

	skip, err := promptSkipOnComposeConflict(cmd, composer.ConflictError{
		Branch: "feature-b",
		Err:    errors.New("exit status 1"),
	})
	if err != nil {
		t.Fatalf("promptSkipOnComposeConflict() error = %v", err)
	}
	if skip {
		t.Fatal("promptSkipOnComposeConflict() = true, want false")
	}
	if !strings.Contains(out.String(), "please answer skip or abort") {
		t.Fatalf("prompt output = %q, want retry hint", out.String())
	}
}

func TestRunComposeWithRecoverySkipsAndRetries(t *testing.T) {
	repoRoot := t.TempDir()
	runner := &interactiveComposeRunner{
		results: map[string]gitrunner.Result{
			"branch --show-current":            {Stdout: "topic"},
			"diff --name-only --diff-filter=U": {Stdout: "app/service.go"},
		},
	}
	setTestApp(t, repoRoot, runner)

	dag, err := stack.NewDAG([]stack.Dependency{
		{Branch: "feature-b", Parent: "feature-a"},
		{Branch: "feature-c", Parent: "feature-b"},
	})
	if err != nil {
		t.Fatalf("NewDAG() error = %v", err)
	}

	cmd := &cobra.Command{}
	cmd.SetIn(bytes.NewBufferString("skip\n"))
	var out bytes.Buffer
	cmd.SetOut(&out)

	result, err := runComposeWithRecovery(context.Background(), cmd, dag, []string{"feature-c"}, "main", composer.ComposeOpts{CreateBranch: "integration"})
	if err != nil {
		t.Fatalf("runComposeWithRecovery() error = %v", err)
	}
	if got := strings.Join(result.Skipped, ","); got != "feature-b" {
		t.Fatalf("Skipped = %q, want feature-b", got)
	}
	if !strings.Contains(out.String(), "skipping feature-b and retrying compose") {
		t.Fatalf("output = %q, want retry message", out.String())
	}
}

func TestRunComposeWithRecoveryAbort(t *testing.T) {
	repoRoot := t.TempDir()
	runner := &interactiveComposeRunner{
		results: map[string]gitrunner.Result{
			"branch --show-current":            {Stdout: "topic"},
			"diff --name-only --diff-filter=U": {Stdout: "app/service.go"},
		},
	}
	setTestApp(t, repoRoot, runner)

	dag, err := stack.NewDAG([]stack.Dependency{
		{Branch: "feature-b", Parent: "feature-a"},
		{Branch: "feature-c", Parent: "feature-b"},
	})
	if err != nil {
		t.Fatalf("NewDAG() error = %v", err)
	}

	cmd := &cobra.Command{}
	cmd.SetIn(bytes.NewBufferString("abort\n"))
	var out bytes.Buffer
	cmd.SetOut(&out)

	_, err = runComposeWithRecovery(context.Background(), cmd, dag, []string{"feature-c"}, "main", composer.ComposeOpts{UpdateBranch: "integration"})
	if err == nil {
		t.Fatal("runComposeWithRecovery() error = nil, want conflict")
	}
	if !strings.Contains(err.Error(), "compose failed while merging feature-b") {
		t.Fatalf("error = %v, want conflict branch", err)
	}
}

func TestLoadTrackedIntegrationBranchEntries(t *testing.T) {
	repoRoot := t.TempDir()
	store := weaverintegration.NewBranchStore(repoRoot)
	if err := store.Track("release-1", weaverintegration.BranchRecord{
		Base:     "main",
		Branches: []string{"feature-a"},
		Skipped:  []string{"feature-b", "feature-c"},
	}); err != nil {
		t.Fatalf("Track() error = %v", err)
	}

	entries, err := loadTrackedIntegrationBranchEntries(context.Background(), &commandRecordingRunner{
		repoRoot: repoRoot,
		results: map[string]gitrunner.Result{
			"show-ref --verify --quiet refs/heads/release-1": {},
			"show-ref --verify --quiet refs/heads/feature-b": {},
			"show-ref --verify --quiet refs/heads/feature-c": {},
			"merge-base --is-ancestor feature-b release-1":   {},
			"merge-base --is-ancestor feature-c release-1":   {ExitCode: 1},
		},
		errs: map[string]error{
			"merge-base --is-ancestor feature-c release-1": errors.New("exit status 1"),
		},
	}, store)
	if err != nil {
		t.Fatalf("loadTrackedIntegrationBranchEntries() error = %v", err)
	}
	if len(entries) != 1 || entries[0].Name != "release-1" || !entries[0].Exists {
		t.Fatalf("entries = %#v", entries)
	}
	if got := entries[0].Status(); got != "partial" {
		t.Fatalf("entries[0].Status() = %q, want partial", got)
	}
	if !reflect.DeepEqual(entries[0].IncludedSkipped, []string{"feature-b"}) {
		t.Fatalf("IncludedSkipped = %#v, want feature-b", entries[0].IncludedSkipped)
	}
	if !reflect.DeepEqual(entries[0].PendingSkipped, []string{"feature-c"}) {
		t.Fatalf("PendingSkipped = %#v, want feature-c", entries[0].PendingSkipped)
	}

	emptyEntries, err := loadTrackedIntegrationBranchEntries(context.Background(), &commandRecordingRunner{repoRoot: repoRoot}, weaverintegration.NewBranchStore(t.TempDir()))
	if err != nil {
		t.Fatalf("loadTrackedIntegrationBranchEntries(empty) error = %v", err)
	}
	if len(emptyEntries) != 0 {
		t.Fatalf("empty entries = %#v, want none", emptyEntries)
	}
}

func TestLoadTrackedIntegrationBranchEntriesErrors(t *testing.T) {
	repoRoot := t.TempDir()
	store := weaverintegration.NewBranchStore(repoRoot)
	if err := store.Track("release-1", weaverintegration.BranchRecord{
		Base:     "main",
		Branches: []string{"feature-a"},
		Skipped:  []string{"feature-b"},
	}); err != nil {
		t.Fatalf("Track() error = %v", err)
	}

	_, err := loadTrackedIntegrationBranchEntries(context.Background(), &rawCommandRunner{
		repoRoot: repoRoot,
		outcomes: map[string]runnerOutcome{
			"show-ref --verify --quiet refs/heads/release-1": {err: errors.New("boom")},
		},
	}, store)
	if err == nil || !strings.Contains(err.Error(), `check integration branch "release-1"`) {
		t.Fatalf("loadTrackedIntegrationBranchEntries() error = %v", err)
	}

	_, err = loadTrackedIntegrationBranchEntries(context.Background(), &rawCommandRunner{
		repoRoot: repoRoot,
		outcomes: map[string]runnerOutcome{
			"show-ref --verify --quiet refs/heads/release-1": {},
			"show-ref --verify --quiet refs/heads/feature-b": {err: errors.New("boom")},
		},
	}, store)
	if err == nil || !strings.Contains(err.Error(), `check whether skipped branch "feature-b" is integrated into "release-1"`) {
		t.Fatalf("loadTrackedIntegrationBranchEntries(skipped branch) error = %v", err)
	}

	brokenRepoRoot := t.TempDir()
	brokenStore := weaverintegration.NewBranchStore(brokenRepoRoot)
	brokenPath := filepath.Join(brokenRepoRoot, ".git", "weaver", "integration-branches.yaml")
	if err := os.MkdirAll(filepath.Dir(brokenPath), 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(brokenPath, []byte("branches: ["), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	_, err = loadTrackedIntegrationBranchEntries(context.Background(), &commandRecordingRunner{repoRoot: repoRoot}, brokenStore)
	if err == nil || !strings.Contains(err.Error(), "decode integration branches file") {
		t.Fatalf("loadTrackedIntegrationBranchEntries(broken store) error = %v", err)
	}
}

func TestDeleteTrackedIntegrationBranchAdditionalCases(t *testing.T) {
	repoRoot := t.TempDir()
	store := weaverintegration.NewBranchStore(repoRoot)

	if _, err := deleteTrackedIntegrationBranch(context.Background(), &commandRecordingRunner{repoRoot: repoRoot}, store, "missing"); err == nil || !strings.Contains(err.Error(), `integration branch "missing" does not exist`) {
		t.Fatalf("deleteTrackedIntegrationBranch(missing) error = %v", err)
	}

	if err := store.Track("release-1", weaverintegration.BranchRecord{
		Base:     "main",
		Branches: []string{"feature-a"},
	}); err != nil {
		t.Fatalf("Track() error = %v", err)
	}

	_, err := deleteTrackedIntegrationBranch(context.Background(), &rawCommandRunner{
		repoRoot: repoRoot,
		outcomes: map[string]runnerOutcome{
			"show-ref --verify --quiet refs/heads/release-1": {err: errors.New("boom")},
		},
	}, store, "release-1")
	if err == nil || !strings.Contains(err.Error(), `check integration branch "release-1"`) {
		t.Fatalf("deleteTrackedIntegrationBranch(show-ref) error = %v", err)
	}

	_, err = deleteTrackedIntegrationBranch(context.Background(), &commandRecordingRunner{
		repoRoot: repoRoot,
		results: map[string]gitrunner.Result{
			"show-ref --verify --quiet refs/heads/release-1": {},
		},
		errs: map[string]error{
			"branch --show-current": errors.New("boom"),
		},
	}, store, "release-1")
	if err == nil || !strings.Contains(err.Error(), "boom") {
		t.Fatalf("deleteTrackedIntegrationBranch(current branch) error = %v", err)
	}

	_, err = deleteTrackedIntegrationBranch(context.Background(), &commandRecordingRunner{
		repoRoot: repoRoot,
		results: map[string]gitrunner.Result{
			"show-ref --verify --quiet refs/heads/release-1": {},
			"branch --show-current":                          {Stdout: "feature-b"},
		},
		errs: map[string]error{
			"branch -D release-1": errors.New("boom"),
		},
	}, store, "release-1")
	if err == nil || !strings.Contains(err.Error(), "boom") {
		t.Fatalf("deleteTrackedIntegrationBranch(delete) error = %v", err)
	}

	brokenRepoRoot := t.TempDir()
	brokenStore := weaverintegration.NewBranchStore(brokenRepoRoot)
	brokenPath := filepath.Join(brokenRepoRoot, ".git", "weaver", "integration-branches.yaml")
	if err := os.MkdirAll(filepath.Dir(brokenPath), 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.MkdirAll(brokenPath, 0o755); err != nil {
		t.Fatalf("MkdirAll(path) error = %v", err)
	}
	_, err = deleteTrackedIntegrationBranch(context.Background(), &commandRecordingRunner{repoRoot: repoRoot}, brokenStore, "release-1")
	if err == nil || !strings.Contains(err.Error(), "read integration branches file") {
		t.Fatalf("deleteTrackedIntegrationBranch(store read) error = %v", err)
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
	if err := weaverintegration.NewStore(repoRoot).Save("integration", weaverintegration.Recipe{
		Base:     "main",
		Branches: []string{"feature-a", "feature-b"},
	}); err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	if err := weaverintegration.NewBranchStore(repoRoot).Track("release-1", weaverintegration.BranchRecord{
		Base:        "main",
		Branches:    []string{"feature-a"},
		Skipped:     []string{"feature-b"},
		Integration: "integration",
	}); err != nil {
		t.Fatalf("Track() integration branch error = %v", err)
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

	integrationData, err := os.ReadFile(filepath.Join(importRepo, ".git", "weaver", "integrations.yaml"))
	if err != nil {
		t.Fatalf("read imported integrations: %v", err)
	}
	if !strings.Contains(string(integrationData), "base: main") || !strings.Contains(string(integrationData), "- feature-a") {
		t.Fatalf("imported integrations missing recipe: %s", integrationData)
	}

	integrationBranchData, err := os.ReadFile(filepath.Join(importRepo, ".git", "weaver", "integration-branches.yaml"))
	if err != nil {
		t.Fatalf("read imported integration branches: %v", err)
	}
	if !strings.Contains(string(integrationBranchData), "release-1:") || !strings.Contains(string(integrationBranchData), "integration: integration") {
		t.Fatalf("imported integration branches missing tracked branch: %s", integrationBranchData)
	}
}

func TestIntegrationExportAndImportCommands(t *testing.T) {
	repoRoot := t.TempDir()
	setTestApp(t, repoRoot, &staticRunner{repoRoot: repoRoot})

	if err := weaverintegration.NewStore(repoRoot).Save("integration", weaverintegration.Recipe{
		Base:     "main",
		Branches: []string{"feature-a", "feature-b"},
	}); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	var exported bytes.Buffer
	integrationExportCmd.SetOut(&exported)
	integrationExportCmd.Flags().Set("json", "true")
	t.Cleanup(func() {
		integrationExportCmd.Flags().Set("json", "false")
	})
	if err := integrationExportCmd.RunE(integrationExportCmd, []string{"integration"}); err != nil {
		t.Fatalf("integration export error = %v", err)
	}

	importRepo := t.TempDir()
	setTestApp(t, importRepo, &staticRunner{repoRoot: importRepo})
	exportPath := filepath.Join(t.TempDir(), "integration.json")
	if err := os.WriteFile(exportPath, exported.Bytes(), 0o644); err != nil {
		t.Fatalf("write integration export file: %v", err)
	}

	if err := integrationImportCmd.RunE(integrationImportCmd, []string{exportPath}); err != nil {
		t.Fatalf("integration import error = %v", err)
	}

	got, ok, err := weaverintegration.NewStore(importRepo).Get("integration")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if !ok {
		t.Fatal("Get() ok = false, want true")
	}
	if got.Base != "main" || strings.Join(got.Branches, ",") != "feature-a,feature-b" {
		t.Fatalf("imported integration = %#v", got)
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

func TestRenderDoctorReport(t *testing.T) {
	var out bytes.Buffer
	renderDoctorReport(&out, &doctor.Report{
		Checks: []doctor.Check{{Level: doctor.LevelWarn, Message: "drift", Hint: "fix it"}},
		Summary: doctor.Summary{
			Warn: 1,
		},
	})
	if got := out.String(); !strings.Contains(got, "WARN drift") || !strings.Contains(got, "fix: fix it") {
		t.Fatalf("renderDoctorReport() = %q", got)
	}
}

func TestExitCode(t *testing.T) {
	var cfgErr config.Error
	if got := ExitCode(nil); got != 0 {
		t.Fatalf("ExitCode(nil) = %d", got)
	}
	if got := ExitCode(markUsage(errors.New("bad"))); got != 2 {
		t.Fatalf("ExitCode(usage) = %d", got)
	}
	if got := ExitCode(cfgErr); got != 3 {
		t.Fatalf("ExitCode(config) = %d", got)
	}
	if got := ExitCode(errors.New("boom")); got != 1 {
		t.Fatalf("ExitCode(default) = %d", got)
	}
}

func TestCurrentBranchName(t *testing.T) {
	repoRoot := t.TempDir()
	runner := &commandRecordingRunner{
		repoRoot: repoRoot,
		results: map[string]gitrunner.Result{
			"branch --show-current": {Stdout: "feature-a"},
		},
	}
	setTestApp(t, repoRoot, runner)

	got, err := currentBranchName(context.Background())
	if err != nil {
		t.Fatalf("currentBranchName() error = %v", err)
	}
	if got != "feature-a" {
		t.Fatalf("currentBranchName() = %q", got)
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
	cmd.Flags().String("integration", "", "integration")
	cmd.Flags().Bool("all", false, "all")
	cmd.Flags().String("base", "", "base")
	cmd.Flags().String("create", "", "create")
	cmd.Flags().String("update", "", "update")
	cmd.Flags().StringSlice("skip", nil, "skip")
	cmd.Flags().Bool("dry-run", false, "dry-run")
	return cmd
}

func cloneUpdateCommand() *cobra.Command {
	cmd := &cobra.Command{}
	cmd.Flags().String("group", "", "group")
	cmd.Flags().String("integration", "", "integration")
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

type interactiveComposeRunner struct {
	repoRoot string
	results  map[string]gitrunner.Result
	calls    []string
}

func (r *interactiveComposeRunner) Run(_ context.Context, args ...string) (gitrunner.Result, error) {
	key := strings.Join(args, " ")
	r.calls = append(r.calls, key)
	if result, ok := r.results[key]; ok {
		return result, nil
	}

	switch key {
	case "merge --no-ff --no-edit feature-b":
		return gitrunner.Result{ExitCode: 1}, errors.New("exit status 1")
	case "branch integration HEAD", "branch -f integration HEAD":
		return gitrunner.Result{}, nil
	}

	return gitrunner.Result{}, nil
}

func (r *interactiveComposeRunner) RepoRoot() string {
	return r.repoRoot
}

type commandRecordingRunner struct {
	repoRoot string
	results  map[string]gitrunner.Result
	errs     map[string]error
	calls    []string
}

type runnerOutcome struct {
	result gitrunner.Result
	err    error
}

type rawCommandRunner struct {
	repoRoot string
	outcomes map[string]runnerOutcome
	calls    []string
}

func (r *commandRecordingRunner) Run(_ context.Context, args ...string) (gitrunner.Result, error) {
	key := strings.Join(args, " ")
	r.calls = append(r.calls, key)

	if result, ok := r.results[key]; ok {
		if err, hasErr := r.errs[key]; hasErr {
			if result.ExitCode == 0 {
				result.ExitCode = 1
			}
			return result, err
		}
		return result, nil
	}

	if err, ok := r.errs[key]; ok {
		return gitrunner.Result{ExitCode: 1}, err
	}

	return gitrunner.Result{}, nil
}

func (r *commandRecordingRunner) RepoRoot() string {
	return r.repoRoot
}

func (r *rawCommandRunner) Run(_ context.Context, args ...string) (gitrunner.Result, error) {
	key := strings.Join(args, " ")
	r.calls = append(r.calls, key)
	if outcome, ok := r.outcomes[key]; ok {
		return outcome.result, outcome.err
	}
	return gitrunner.Result{}, nil
}

func (r *rawCommandRunner) RepoRoot() string {
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
