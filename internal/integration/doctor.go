package integration

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/lutefd/weaver/internal/composer"
	gitparse "github.com/lutefd/weaver/internal/git"
	gitrunner "github.com/lutefd/weaver/internal/git"
	"github.com/lutefd/weaver/internal/stack"
)

const excessiveBehindThreshold = 10

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
	Integration string   `json:"integration"`
	Base        string   `json:"base"`
	Order       []string `json:"order,omitempty"`
	Checks      []Check  `json:"checks"`
	Summary     Summary  `json:"summary"`
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

type Analyzer struct {
	runner gitrunner.Runner
}

func NewAnalyzer(runner gitrunner.Runner) *Analyzer {
	return &Analyzer{runner: runner}
}

func (a *Analyzer) Analyze(ctx context.Context, dag *stack.DAG, name string, recipe Recipe) (*Report, error) {
	if err := validateRecipe(name, recipe); err != nil {
		return nil, err
	}
	if ctx == nil {
		ctx = context.Background()
	}

	report := &Report{
		Integration: name,
		Base:        recipe.Base,
	}

	baseExists, err := a.branchExists(ctx, recipe.Base)
	if err != nil {
		return nil, err
	}
	if !baseExists {
		report.addHint(LevelFail, "integration_base", manualMergeHint(name, ""), "integration base %q does not exist locally", recipe.Base)
		return report, nil
	}

	order, err := composer.ResolveOrder(dag, recipe.Branches, recipe.Base)
	if err != nil {
		report.addHint(LevelFail, "integration_order", manualMergeHint(name, ""), "cannot resolve compose order for integration %q: %v", name, err)
		return report, nil
	}
	report.Order = order

	orderSet := make(map[string]struct{}, len(order))
	for _, branch := range order {
		orderSet[branch] = struct{}{}
	}

	for _, branch := range order {
		exists, err := a.branchExists(ctx, branch)
		if err != nil {
			return nil, err
		}
		if !exists {
			report.addHint(LevelFail, "missing_branch", manualMergeHint(name, branch), "branch %q in integration %q does not exist locally", branch, name)
			continue
		}

		parent := recipe.Base
		if dagParent, ok := dag.Parent(branch); ok {
			if _, ok := orderSet[dagParent]; ok {
				parent = dagParent
			}
		}

		if err := a.checkBranch(ctx, report, dag, name, recipe.Base, branch, parent, order); err != nil {
			return nil, err
		}
	}

	return report, nil
}

func (a *Analyzer) checkBranch(ctx context.Context, report *Report, dag *stack.DAG, integrationName, base, branch, parent string, order []string) error {
	mergeBase, err := a.rev(ctx, "merge-base", branch, parent)
	if err != nil {
		return err
	}
	parentRev, err := a.rev(ctx, "rev-parse", parent)
	if err != nil {
		return err
	}

	if mergeBase == parentRev {
		report.add(LevelOK, "normalized", `branch %q is cleanly based on %q`, branch, parent)
	} else {
		ahead, behind, err := a.aheadBehind(ctx, branch, parent)
		if err != nil {
			return err
		}

		conflictFiles, conflictRisk, err := a.predictConflict(ctx, parent, branch)
		if err != nil {
			return err
		}
		switch {
		case conflictRisk:
			message := fmt.Sprintf(`branch %q is not normalized onto %q and is likely to conflict during compose`, branch, parent)
			if len(conflictFiles) > 0 {
				message += fmt.Sprintf(" (%s)", strings.Join(conflictFiles, ", "))
			}
			report.addHint(LevelFail, "conflict_risk", manualMergeHint(integrationName, branch), "%s", message)
		case behind >= excessiveBehindThreshold:
			report.addHint(LevelFail, "drift", manualMergeHint(integrationName, branch), `branch %q is %d commit(s) behind expected parent %q (%d ahead)`, branch, behind, parent, ahead)
		default:
			report.addHint(LevelWarn, "drift", manualMergeHint(integrationName, branch), `branch %q is %d commit(s) behind expected parent %q (%d ahead)`, branch, behind, parent, ahead)
		}
	}

	if mergeCommits, err := a.suspiciousMergeCommits(ctx, base, parent, branch, order); err != nil {
		return err
	} else if len(mergeCommits) > 0 {
		report.addHint(LevelWarn, "merge_commits", manualMergeHint(integrationName, branch), `branch %q contains merge commits from outside %q or the integration base (%s)`, branch, parent, strings.Join(shortRefs(mergeCommits), ", "))
	}

	foreignCommit, foreignBranch, err := a.findSharedForeignCommit(ctx, dag, branch, parent, base, order)
	if err != nil {
		return err
	}
	if foreignCommit != "" {
		report.addHint(LevelFail, "foreign_ancestry", manualMergeHint(integrationName, branch), `branch %q has foreign ancestry: shared commit %s also appears in %q`, branch, shortRef(foreignCommit), foreignBranch)
	}

	return nil
}

func (a *Analyzer) branchExists(ctx context.Context, branch string) (bool, error) {
	result, err := a.runner.Run(ctx, "show-ref", "--verify", "--quiet", "refs/heads/"+branch)
	if err != nil {
		if result.ExitCode != 0 {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func (a *Analyzer) rev(ctx context.Context, args ...string) (string, error) {
	result, err := a.runner.Run(ctx, args...)
	if err != nil {
		return "", err
	}
	return result.Stdout, nil
}

func (a *Analyzer) aheadBehind(ctx context.Context, branch, parent string) (int, int, error) {
	result, err := a.runner.Run(ctx, "rev-list", "--left-right", "--count", branch+"..."+parent)
	if err != nil {
		return 0, 0, err
	}
	return gitparse.ParseAheadBehind(result.Stdout)
}

func (a *Analyzer) predictConflict(ctx context.Context, parent, branch string) ([]string, bool, error) {
	result, err := a.runner.Run(ctx, "merge-tree", "--write-tree", "--messages", "--name-only", parent, branch)
	if err == nil {
		return nil, false, nil
	}
	if result.ExitCode == 0 {
		return nil, false, nil
	}

	files := make([]string, 0)
	for _, line := range strings.Split(result.Stdout, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if strings.Contains(line, "Auto-merging") || strings.HasPrefix(line, "CONFLICT") {
			continue
		}
		files = append(files, line)
	}
	sort.Strings(files)
	return files, true, nil
}

func (a *Analyzer) mergeCommits(ctx context.Context, parent, branch string) ([]string, error) {
	result, err := a.runner.Run(ctx, "rev-list", "--merges", parent+".."+branch)
	if err != nil {
		return nil, err
	}
	if result.Stdout == "" {
		return nil, nil
	}
	return strings.Fields(result.Stdout), nil
}

func (a *Analyzer) suspiciousMergeCommits(ctx context.Context, base, parent, branch string, order []string) ([]string, error) {
	mergeCommits, err := a.mergeCommits(ctx, parent, branch)
	if err != nil {
		return nil, err
	}

	suspicious := make([]string, 0, len(mergeCommits))
	for _, commit := range mergeCommits {
		parents, err := a.commitParents(ctx, commit)
		if err != nil {
			return nil, err
		}
		if len(parents) <= 1 {
			continue
		}
		safe, err := a.secondaryParentsAreSafe(ctx, parents[1:], base, parent, branch, order)
		if err != nil {
			return nil, err
		}
		if !safe {
			suspicious = append(suspicious, commit)
		}
	}

	return suspicious, nil
}

func (a *Analyzer) commitParents(ctx context.Context, commit string) ([]string, error) {
	result, err := a.runner.Run(ctx, "rev-list", "--parents", "-n", "1", commit)
	if err != nil {
		return nil, err
	}
	fields := strings.Fields(result.Stdout)
	if len(fields) <= 1 {
		return nil, nil
	}
	return fields[1:], nil
}

func (a *Analyzer) secondaryParentsAreSafe(ctx context.Context, parents []string, base, expectedParent, branch string, order []string) (bool, error) {
	safeRefs := make([]string, 0, len(order)+2)
	safeRefs = append(safeRefs, base, expectedParent)
	for _, candidate := range order {
		if candidate == branch {
			continue
		}
		safeRefs = append(safeRefs, candidate)
	}

	for _, parent := range parents {
		ok, err := a.isAncestorOfAny(ctx, parent, safeRefs)
		if err != nil {
			return false, err
		}
		if !ok {
			return false, nil
		}
	}

	return true, nil
}

func (a *Analyzer) isAncestorOfAny(ctx context.Context, commit string, refs []string) (bool, error) {
	for _, ref := range refs {
		if ref == "" {
			continue
		}
		result, err := a.runner.Run(ctx, "merge-base", "--is-ancestor", commit, ref)
		if err != nil {
			if result.ExitCode == 1 {
				continue
			}
			return false, err
		}
		return true, nil
	}
	return false, nil
}

func (a *Analyzer) findSharedForeignCommit(ctx context.Context, dag *stack.DAG, branch, parent, base string, order []string) (string, string, error) {
	commits, err := a.branchRange(ctx, parent, branch)
	if err != nil {
		return "", "", err
	}
	if len(commits) == 0 {
		return "", "", nil
	}

	branchFirstParent, err := a.firstParentSet(ctx, branch)
	if err != nil {
		return "", "", err
	}

	for _, other := range order {
		if other == branch {
			continue
		}
		if relatedInDAG(dag, branch, other) {
			continue
		}
		otherFirstParent, err := a.firstParentSet(ctx, other)
		if err != nil {
			return "", "", err
		}
		for _, commit := range commits {
			safe, err := a.isAncestorOfAny(ctx, commit, []string{base, parent})
			if err != nil {
				return "", "", err
			}
			if safe {
				continue
			}
			result, err := a.runner.Run(ctx, "merge-base", "--is-ancestor", commit, other)
			if err != nil {
				if result.ExitCode == 1 {
					continue
				}
				return "", "", err
			}
			_, branchOwns := branchFirstParent[commit]
			_, otherOwns := otherFirstParent[commit]
			// If the commit is on this branch's first-parent chain but not on the
			// other branch's first-parent chain, the other branch likely merged this
			// branch and should carry the foreign-ancestry warning instead.
			if branchOwns && !otherOwns {
				continue
			}
			// If neither branch carries the commit on its first-parent chain, both
			// branches received it through merges. That is better diagnosed via the
			// merge ancestry checks rather than blamed symmetrically here.
			if !branchOwns && !otherOwns {
				continue
			}
			return commit, other, nil
		}
	}

	return "", "", nil
}

func (a *Analyzer) branchRange(ctx context.Context, parent, branch string) ([]string, error) {
	result, err := a.runner.Run(ctx, "rev-list", "--reverse", parent+".."+branch)
	if err != nil {
		return nil, err
	}
	if result.Stdout == "" {
		return nil, nil
	}
	return strings.Fields(result.Stdout), nil
}

func (a *Analyzer) firstParentSet(ctx context.Context, branch string) (map[string]struct{}, error) {
	result, err := a.runner.Run(ctx, "rev-list", "--first-parent", branch)
	if err != nil {
		return nil, err
	}
	commits := strings.Fields(result.Stdout)
	set := make(map[string]struct{}, len(commits))
	for _, commit := range commits {
		set[commit] = struct{}{}
	}
	return set, nil
}

func relatedInDAG(dag *stack.DAG, left, right string) bool {
	if left == right {
		return true
	}
	leftAncestors, err := dag.Ancestors(left)
	if err == nil {
		for _, branch := range leftAncestors {
			if branch == right {
				return true
			}
		}
	}
	rightAncestors, err := dag.Ancestors(right)
	if err == nil {
		for _, branch := range rightAncestors {
			if branch == left {
				return true
			}
		}
	}
	return false
}

func shortRef(ref string) string {
	if len(ref) <= 12 {
		return ref
	}
	return ref[:12]
}

func shortRefs(refs []string) []string {
	short := make([]string, 0, len(refs))
	for _, ref := range refs {
		short = append(short, shortRef(ref))
	}
	return short
}

func manualMergeHint(integrationName, branch string) string {
	if branch == "" {
		return fmt.Sprintf("repair the saved integration %q, or temporarily remove the problematic branch and merge it manually onto the branch produced by `weaver compose --integration %s --create <branch>` or `--update <branch>`", integrationName, integrationName)
	}
	return fmt.Sprintf("remove %q from integration %q, repair or merge it manually first, then merge it onto the branch produced by `weaver compose --integration %s --create <branch>` or `--update <branch>` before adding it back", branch, integrationName, integrationName)
}
