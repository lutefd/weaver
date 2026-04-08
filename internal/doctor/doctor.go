package doctor

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/lutefd/weaver/internal/config"
	"github.com/lutefd/weaver/internal/deps"
	gitrunner "github.com/lutefd/weaver/internal/git"
	"github.com/lutefd/weaver/internal/group"
	"github.com/lutefd/weaver/internal/rebaser"
	"github.com/lutefd/weaver/internal/resolver"
)

type Level string

const (
	LevelOK   Level = "ok"
	LevelWarn Level = "warn"
	LevelFail Level = "fail"
)

type Check struct {
	Level   Level  `json:"level"`
	Code    string `json:"code"`
	Message string `json:"message"`
	Hint    string `json:"hint,omitempty"`
}

type Summary struct {
	OK   int `json:"ok"`
	Warn int `json:"warn"`
	Fail int `json:"fail"`
}

type Report struct {
	Checks  []Check `json:"checks"`
	Summary Summary `json:"summary"`
}

func (r *Report) HasFailures() bool {
	return r.Summary.Fail > 0
}

func (r *Report) add(level Level, code string, format string, args ...any) {
	check := Check{
		Level:   level,
		Code:    code,
		Message: fmt.Sprintf(format, args...),
	}
	r.Checks = append(r.Checks, check)
	switch level {
	case LevelOK:
		r.Summary.OK++
	case LevelWarn:
		r.Summary.Warn++
	case LevelFail:
		r.Summary.Fail++
	}
}

func (r *Report) addHint(level Level, code string, hint string, format string, args ...any) {
	check := Check{
		Level:   level,
		Code:    code,
		Message: fmt.Sprintf(format, args...),
		Hint:    hint,
	}
	r.Checks = append(r.Checks, check)
	switch level {
	case LevelOK:
		r.Summary.OK++
	case LevelWarn:
		r.Summary.Warn++
	case LevelFail:
		r.Summary.Fail++
	}
}

type Checker struct {
	runner      gitrunner.Runner
	cfg         *config.Config
	cfgErr      error
	branchCache map[string]bool
}

func New(runner gitrunner.Runner, cfg *config.Config, cfgErr error) *Checker {
	if cfg == nil {
		defaultCfg := config.Default()
		cfg = &defaultCfg
	}

	return &Checker{
		runner:      runner,
		cfg:         cfg,
		cfgErr:      cfgErr,
		branchCache: map[string]bool{},
	}
}

func (c *Checker) Run(ctx context.Context) (*Report, error) {
	if ctx == nil {
		ctx = context.Background()
	}

	report := &Report{}
	trackedBranches := map[string]struct{}{}

	c.checkInitialization(report)
	c.checkConfig(report)
	c.checkBaseBranch(ctx, report)
	c.checkDependencies(ctx, report, trackedBranches)
	c.checkGroups(ctx, report, trackedBranches)
	c.checkRebaseState(ctx, report, trackedBranches)
	c.checkCurrentBranch(ctx, report)
	c.checkWorkingTree(ctx, report)
	c.checkGitOperations(ctx, report)

	return report, nil
}

func (c *Checker) checkInitialization(report *Report) {
	cfgPath := filepath.Join(c.runner.RepoRoot(), config.FileName)
	metaDir := filepath.Join(c.runner.RepoRoot(), config.DirName)

	missing := make([]string, 0, 2)
	if _, err := os.Stat(cfgPath); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			missing = append(missing, config.FileName)
		} else {
			report.addHint(LevelFail, "initialization", "check file permissions for Weaver metadata", "cannot stat %s: %v", config.FileName, err)
			return
		}
	}
	if _, err := os.Stat(metaDir); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			missing = append(missing, config.DirName)
		} else {
			report.addHint(LevelFail, "initialization", "check file permissions for Weaver metadata", "cannot stat %s: %v", config.DirName, err)
			return
		}
	}

	if len(missing) > 0 {
		report.addHint(LevelFail, "initialization", "run `weaver init` in this repository", "weaver is not fully initialized: missing %s", strings.Join(missing, ", "))
		return
	}

	report.add(LevelOK, "initialization", "weaver initialization files are present")
}

func (c *Checker) checkConfig(report *Report) {
	if c.cfgErr != nil {
		report.addHint(LevelFail, "config", "fix or recreate .weaver.yaml", "weaver config is invalid: %v", c.cfgErr)
		return
	}

	report.add(LevelOK, "config", "weaver config loaded with default base %q", c.cfg.DefaultBase)
}

func (c *Checker) checkBaseBranch(ctx context.Context, report *Report) {
	base := c.cfg.DefaultBase
	if base == "" {
		base = config.Default().DefaultBase
	}

	exists, err := c.branchExists(ctx, base)
	if err != nil {
		report.addHint(LevelFail, "base_branch", "check git branch state in this repository", "cannot verify base branch %q: %v", base, err)
		return
	}
	if !exists {
		currentBranch, currentErr := c.currentBranch(ctx)
		if currentErr == nil && currentBranch == base {
			report.add(LevelOK, "base_branch", "configured base branch is the current unborn branch: %s", base)
			return
		}
		report.addHint(LevelFail, "base_branch", fmt.Sprintf("create or rename the configured base branch %q", base), "configured base branch %q does not exist locally", base)
		return
	}

	report.add(LevelOK, "base_branch", "configured base branch exists locally: %s", base)
}

func (c *Checker) checkDependencies(ctx context.Context, report *Report, tracked map[string]struct{}) {
	source := deps.NewLocalSource(c.runner.RepoRoot())
	declared, err := source.Load(ctx)
	if err != nil {
		report.addHint(LevelFail, "dependencies", "fix or remove .git/weaver/deps.yaml", "cannot load dependency declarations: %v", err)
		return
	}

	if _, err := resolver.New(source).Resolve(ctx); err != nil {
		report.addHint(LevelFail, "dependencies", "fix invalid stack declarations in .git/weaver/deps.yaml", "dependency graph is invalid: %v", err)
	} else {
		report.add(LevelOK, "dependencies", "dependency graph is valid (%d declarations)", len(declared))
	}

	if len(declared) == 0 {
		return
	}

	seen := map[string]struct{}{}
	for _, dep := range declared {
		tracked[dep.Branch] = struct{}{}
		tracked[dep.Parent] = struct{}{}
		if _, ok := seen[dep.Branch]; !ok {
			c.checkBranchPresence(ctx, report, dep.Branch, "dependency branch", "dependency_branch")
			seen[dep.Branch] = struct{}{}
		}
		if _, ok := seen[dep.Parent]; !ok {
			c.checkBranchPresence(ctx, report, dep.Parent, "dependency parent", "dependency_parent")
			seen[dep.Parent] = struct{}{}
		}
	}
}

func (c *Checker) checkGroups(ctx context.Context, report *Report, tracked map[string]struct{}) {
	store := group.NewStore(c.runner.RepoRoot())
	groups, err := store.List()
	if err != nil {
		report.addHint(LevelFail, "groups", "fix or remove .git/weaver/groups.yaml", "cannot load compose groups: %v", err)
		return
	}

	report.add(LevelOK, "groups", "compose groups file is valid (%d groups)", len(groups))

	names := make([]string, 0, len(groups))
	for name := range groups {
		names = append(names, name)
	}
	sort.Strings(names)

	for _, name := range names {
		for _, branch := range groups[name] {
			tracked[branch] = struct{}{}
			c.checkBranchPresence(ctx, report, branch, fmt.Sprintf("group branch in %q", name), "group_branch")
		}
	}
}

func (c *Checker) checkRebaseState(ctx context.Context, report *Report, tracked map[string]struct{}) {
	store := rebaser.NewStateStore(c.runner.RepoRoot())
	if !store.HasPending() {
		report.add(LevelOK, "rebase_state", "no pending stack sync state found")
		return
	}

	state, err := store.Load()
	if err != nil {
		report.addHint(LevelFail, "rebase_state", "remove or repair .git/weaver/rebase-state.yaml", "cannot load pending stack sync state: %v", err)
		return
	}

	report.add(LevelWarn, "rebase_state", "pending stack sync state detected for %q", state.Current)

	if state.OriginalBranch == "" {
		report.addHint(LevelFail, "rebase_state_branch", "remove or repair .git/weaver/rebase-state.yaml", "pending stack sync state is missing original_branch")
	} else {
		tracked[state.OriginalBranch] = struct{}{}
		c.checkBranchPresence(ctx, report, state.OriginalBranch, "rebase original branch", "rebase_state_branch")
	}

	if state.BaseBranch == "" {
		report.addHint(LevelFail, "rebase_state_branch", "remove or repair .git/weaver/rebase-state.yaml", "pending stack sync state is missing base_branch")
	} else {
		tracked[state.BaseBranch] = struct{}{}
		c.checkBranchPresence(ctx, report, state.BaseBranch, "rebase base branch", "rebase_state_branch")
	}

	if len(state.AllBranches) == 0 {
		report.addHint(LevelFail, "rebase_state_branch", "remove or repair .git/weaver/rebase-state.yaml", "pending stack sync state is missing all_branches")
	}

	for _, branch := range state.AllBranches {
		tracked[branch] = struct{}{}
		c.checkBranchPresence(ctx, report, branch, "rebase branch", "rebase_state_branch")
	}
}

func (c *Checker) checkCurrentBranch(ctx context.Context, report *Report) {
	currentBranch, err := c.currentBranch(ctx)
	if err != nil {
		report.addHint(LevelFail, "head", "check repository HEAD state", "cannot resolve current branch: %v", err)
		return
	}
	if currentBranch == "" {
		report.add(LevelWarn, "head", "repository is in detached HEAD state")
		return
	}

	report.add(LevelOK, "head", "current branch is %s", currentBranch)
}

func (c *Checker) checkWorkingTree(ctx context.Context, report *Report) {
	result, err := c.runner.Run(ctx, "status", "--porcelain", "--untracked-files=no")
	if err != nil {
		report.addHint(LevelFail, "working_tree", "run `git status` to inspect repository state", "cannot inspect working tree: %v", err)
		return
	}
	if result.Stdout == "" {
		report.add(LevelOK, "working_tree", "working tree is clean")
		return
	}

	lines := strings.Count(result.Stdout, "\n") + 1
	report.add(LevelWarn, "working_tree", "working tree has %d tracked change(s)", lines)
}

func (c *Checker) checkGitOperations(ctx context.Context, report *Report) {
	gitDir, err := c.gitDir(ctx)
	if err != nil {
		report.addHint(LevelFail, "git_operations", "check git repository metadata", "cannot resolve git directory: %v", err)
		return
	}

	operations := make([]string, 0)
	for _, marker := range []struct {
		path string
		name string
	}{
		{path: filepath.Join(gitDir, "rebase-merge"), name: "rebase"},
		{path: filepath.Join(gitDir, "rebase-apply"), name: "apply"},
		{path: filepath.Join(gitDir, "MERGE_HEAD"), name: "merge"},
		{path: filepath.Join(gitDir, "CHERRY_PICK_HEAD"), name: "cherry-pick"},
		{path: filepath.Join(gitDir, "REVERT_HEAD"), name: "revert"},
	} {
		if _, err := os.Stat(marker.path); err == nil {
			operations = append(operations, marker.name)
		} else if err != nil && !errors.Is(err, os.ErrNotExist) {
			report.addHint(LevelFail, "git_operations", "check permissions in the .git directory", "cannot inspect git operation marker %q: %v", marker.path, err)
			return
		}
	}

	if len(operations) == 0 {
		report.add(LevelOK, "git_operations", "no in-progress git operations detected")
		return
	}

	report.add(LevelWarn, "git_operations", "in-progress git operations detected: %s", strings.Join(operations, ", "))
}

func (c *Checker) checkBranchPresence(ctx context.Context, report *Report, branch string, label string, code string) {
	exists, err := c.branchExists(ctx, branch)
	if err != nil {
		report.addHint(LevelFail, code, "check git branch state in this repository", "cannot verify %s %q: %v", label, branch, err)
		return
	}
	if !exists {
		report.addHint(LevelFail, code, fmt.Sprintf("create or remove the missing branch %q from Weaver metadata", branch), "%s %q does not exist locally", label, branch)
		return
	}

	report.add(LevelOK, code, "%s exists locally: %s", label, branch)
}

func (c *Checker) branchExists(ctx context.Context, branch string) (bool, error) {
	if branch == "" {
		return false, nil
	}
	if exists, ok := c.branchCache[branch]; ok {
		return exists, nil
	}

	result, err := c.runner.Run(ctx, "show-ref", "--verify", "--quiet", "refs/heads/"+branch)
	if err != nil {
		if result.ExitCode != 0 {
			c.branchCache[branch] = false
			return false, nil
		}
		return false, err
	}

	c.branchCache[branch] = true
	return true, nil
}

func (c *Checker) gitDir(ctx context.Context) (string, error) {
	result, err := c.runner.Run(ctx, "rev-parse", "--git-dir")
	if err != nil {
		return "", err
	}
	if result.Stdout == "" {
		return "", fmt.Errorf("empty git dir")
	}
	if filepath.IsAbs(result.Stdout) {
		return result.Stdout, nil
	}
	return filepath.Join(c.runner.RepoRoot(), result.Stdout), nil
}

func (c *Checker) currentBranch(ctx context.Context) (string, error) {
	result, err := c.runner.Run(ctx, "branch", "--show-current")
	if err != nil {
		return "", err
	}
	return result.Stdout, nil
}
