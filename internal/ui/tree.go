package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/lutefd/weaver/internal/stack"
)

func RenderChain(dag *stack.DAG, base, branch string) (string, error) {
	chain, err := dag.Ancestors(branch)
	if err != nil {
		return "", err
	}

	if base != "" && (len(chain) == 0 || chain[0] != base) {
		chain = append([]string{base}, chain...)
	}

	return strings.Join(chain, " -> "), nil
}

func RenderTree(dag *stack.DAG, base string) string {
	lines := []string{base}
	roots := rootsExcludingBase(dag, base)
	for idx, root := range roots {
		lines = append(lines, renderSubtree(dag, root, "", idx == len(roots)-1)...)
	}
	return strings.Join(lines, "\n")
}

func RenderStatusTree(dag *stack.DAG, base string, health map[string]stack.StackHealth) string {
	lines := []string{base}
	roots := rootsExcludingBase(dag, base)
	for idx, root := range roots {
		lines = append(lines, renderStatusSubtree(dag, root, "", idx == len(roots)-1, health)...)
	}
	return strings.Join(lines, "\n")
}

func RenderUpstreamStatusTree(dag *stack.DAG, base string, health map[string]stack.UpstreamHealth) string {
	lines := []string{base}
	roots := rootsExcludingBase(dag, base)
	for idx, root := range roots {
		lines = append(lines, renderUpstreamStatusSubtree(dag, root, "", idx == len(roots)-1, health)...)
	}
	return strings.Join(lines, "\n")
}

func RenderStyledChain(term Terminal, dag *stack.DAG, base, branch string) (string, error) {
	chain, err := dag.Ancestors(branch)
	if err != nil {
		return "", err
	}

	if base != "" && (len(chain) == 0 || chain[0] != base) {
		chain = append([]string{base}, chain...)
	}

	theme := NewTheme(term)
	segments := make([]string, 0, len(chain))
	for idx, item := range chain {
		if idx == 0 && item == base {
			segments = append(segments, theme.BaseBranch(item))
			continue
		}
		segments = append(segments, theme.Branch(item))
	}

	return strings.Join(segments, theme.Connector("  →  ")), nil
}

func RenderStyledTree(term Terminal, dag *stack.DAG, base string) string {
	theme := NewTheme(term)
	lines := []string{theme.BaseBranch(base)}
	roots := rootsExcludingBase(dag, base)
	for idx, root := range roots {
		lines = append(lines, renderStyledSubtree(theme, dag, root, "", idx == len(roots)-1)...)
	}
	return strings.Join(lines, "\n")
}

func RenderStyledStatusTree(term Terminal, dag *stack.DAG, base string, health map[string]stack.StackHealth) string {
	theme := NewTheme(term)
	lines := []string{theme.BaseBranch(base)}
	roots := rootsExcludingBase(dag, base)
	for idx, root := range roots {
		lines = append(lines, renderStyledStatusSubtree(theme, dag, root, "", idx == len(roots)-1, health)...)
	}
	return strings.Join(lines, "\n")
}

func RenderStyledUpstreamStatusTree(term Terminal, dag *stack.DAG, base string, health map[string]stack.UpstreamHealth) string {
	theme := NewTheme(term)
	lines := []string{theme.BaseBranch(base)}
	roots := rootsExcludingBase(dag, base)
	for idx, root := range roots {
		lines = append(lines, renderStyledUpstreamStatusSubtree(theme, dag, root, "", idx == len(roots)-1, health)...)
	}
	return strings.Join(lines, "\n")
}

func renderSubtree(dag *stack.DAG, branch, prefix string, last bool) []string {
	connector := "+-- "
	childPrefix := prefix + "|   "
	if last {
		connector = "`-- "
		childPrefix = prefix + "    "
	}

	lines := []string{fmt.Sprintf("%s%s%s", prefix, connector, branch)}
	children := dag.Children(branch)
	for idx, child := range children {
		lines = append(lines, renderSubtree(dag, child, childPrefix, idx == len(children)-1)...)
	}
	return lines
}

func renderStatusSubtree(dag *stack.DAG, branch, prefix string, last bool, health map[string]stack.StackHealth) []string {
	connector := "+-- "
	childPrefix := prefix + "|   "
	if last {
		connector = "`-- "
		childPrefix = prefix + "    "
	}

	line := fmt.Sprintf("%s%s%s", prefix, connector, branch)
	if status, ok := health[branch]; ok {
		line = fmt.Sprintf("%s  %s", line, formatHealth(status))
	}

	lines := []string{line}
	children := dag.Children(branch)
	for idx, child := range children {
		lines = append(lines, renderStatusSubtree(dag, child, childPrefix, idx == len(children)-1, health)...)
	}
	return lines
}

func renderUpstreamStatusSubtree(dag *stack.DAG, branch, prefix string, last bool, health map[string]stack.UpstreamHealth) []string {
	connector := "+-- "
	childPrefix := prefix + "|   "
	if last {
		connector = "`-- "
		childPrefix = prefix + "    "
	}

	line := fmt.Sprintf("%s%s%s", prefix, connector, branch)
	if status, ok := health[branch]; ok {
		line = fmt.Sprintf("%s  %s", line, formatUpstreamHealth(status))
	}

	lines := []string{line}
	children := dag.Children(branch)
	for idx, child := range children {
		lines = append(lines, renderUpstreamStatusSubtree(dag, child, childPrefix, idx == len(children)-1, health)...)
	}
	return lines
}

func renderStyledSubtree(theme Theme, dag *stack.DAG, branch, prefix string, last bool) []string {
	connector := "├─ "
	childPrefix := prefix + "│  "
	if last {
		connector = "└─ "
		childPrefix = prefix + "   "
	}

	line := theme.Connector(prefix) + theme.Connector(connector) + theme.Branch(branch)
	lines := []string{line}
	children := dag.Children(branch)
	for idx, child := range children {
		lines = append(lines, renderStyledSubtree(theme, dag, child, childPrefix, idx == len(children)-1)...)
	}
	return lines
}

func renderStyledStatusSubtree(theme Theme, dag *stack.DAG, branch, prefix string, last bool, health map[string]stack.StackHealth) []string {
	connector := "├─ "
	childPrefix := prefix + "│  "
	if last {
		connector = "└─ "
		childPrefix = prefix + "   "
	}

	line := theme.Connector(prefix) + theme.Connector(connector) + theme.Branch(branch)
	if status, ok := health[branch]; ok {
		line = lipgloss.JoinHorizontal(lipgloss.Center, line, "  ", healthBadges(theme, status))
	}

	lines := []string{line}
	children := dag.Children(branch)
	for idx, child := range children {
		lines = append(lines, renderStyledStatusSubtree(theme, dag, child, childPrefix, idx == len(children)-1, health)...)
	}
	return lines
}

func renderStyledUpstreamStatusSubtree(theme Theme, dag *stack.DAG, branch, prefix string, last bool, health map[string]stack.UpstreamHealth) []string {
	connector := "├─ "
	childPrefix := prefix + "│  "
	if last {
		connector = "└─ "
		childPrefix = prefix + "   "
	}

	line := theme.Connector(prefix) + theme.Connector(connector) + theme.Branch(branch)
	if status, ok := health[branch]; ok {
		line = lipgloss.JoinHorizontal(lipgloss.Center, line, "  ", upstreamHealthBadges(theme, status))
	}

	lines := []string{line}
	children := dag.Children(branch)
	for idx, child := range children {
		lines = append(lines, renderStyledUpstreamStatusSubtree(theme, dag, child, childPrefix, idx == len(children)-1, health)...)
	}
	return lines
}

func formatHealth(status stack.StackHealth) string {
	switch status.State {
	case stack.HealthClean:
		return string(status.State)
	case stack.HealthOutdated:
		if status.Behind > 0 {
			return fmt.Sprintf("needs sync (%d behind parent)", status.Behind)
		}
		return "needs sync"
	case stack.HealthConflictRisk:
		if status.Behind > 0 {
			return fmt.Sprintf("%s (%d behind parent)", status.State, status.Behind)
		}
		return string(status.State)
	default:
		return string(status.State)
	}
}

func healthBadges(theme Theme, status stack.StackHealth) string {
	parts := []string{primaryHealthBadge(theme, status.State)}
	if status.Behind > 0 && status.State != stack.HealthClean {
		parts = append(parts, theme.Badge(ToneMuted, fmt.Sprintf("%d behind parent", status.Behind)))
	}
	return lipgloss.JoinHorizontal(lipgloss.Center, parts...)
}

func primaryHealthBadge(theme Theme, state stack.StackHealthState) string {
	switch state {
	case stack.HealthClean:
		return theme.Badge(ToneSuccess, string(state))
	case stack.HealthOutdated:
		return theme.Badge(ToneWarn, "needs sync")
	case stack.HealthConflictRisk:
		return theme.Badge(ToneDanger, string(state))
	default:
		return theme.Badge(ToneMuted, string(state))
	}
}

func formatUpstreamHealth(status stack.UpstreamHealth) string {
	switch status.State {
	case stack.UpstreamCurrent:
		return string(status.State)
	case stack.UpstreamBehind:
		if status.Behind > 0 {
			return fmt.Sprintf("%s (%d behind)", status.State, status.Behind)
		}
		return string(status.State)
	case stack.UpstreamAhead:
		if status.Ahead > 0 {
			return fmt.Sprintf("%s (%d ahead)", status.State, status.Ahead)
		}
		return string(status.State)
	case stack.UpstreamDiverged:
		switch {
		case status.Ahead > 0 && status.Behind > 0:
			return fmt.Sprintf("%s (%d ahead, %d behind)", status.State, status.Ahead, status.Behind)
		case status.Ahead > 0:
			return fmt.Sprintf("%s (%d ahead)", status.State, status.Ahead)
		case status.Behind > 0:
			return fmt.Sprintf("%s (%d behind)", status.State, status.Behind)
		default:
			return string(status.State)
		}
	case stack.UpstreamMissing:
		return string(status.State)
	default:
		return string(status.State)
	}
}

func upstreamHealthBadges(theme Theme, status stack.UpstreamHealth) string {
	parts := []string{primaryUpstreamHealthBadge(theme, status.State)}
	switch status.State {
	case stack.UpstreamBehind:
		if status.Behind > 0 {
			parts = append(parts, theme.Badge(ToneMuted, fmt.Sprintf("%d behind", status.Behind)))
		}
	case stack.UpstreamAhead:
		if status.Ahead > 0 {
			parts = append(parts, theme.Badge(ToneMuted, fmt.Sprintf("%d ahead", status.Ahead)))
		}
	case stack.UpstreamDiverged:
		switch {
		case status.Ahead > 0 && status.Behind > 0:
			parts = append(parts, theme.Badge(ToneMuted, fmt.Sprintf("%d ahead / %d behind", status.Ahead, status.Behind)))
		case status.Ahead > 0:
			parts = append(parts, theme.Badge(ToneMuted, fmt.Sprintf("%d ahead", status.Ahead)))
		case status.Behind > 0:
			parts = append(parts, theme.Badge(ToneMuted, fmt.Sprintf("%d behind", status.Behind)))
		}
	}
	return lipgloss.JoinHorizontal(lipgloss.Center, parts...)
}

func primaryUpstreamHealthBadge(theme Theme, state stack.UpstreamHealthState) string {
	switch state {
	case stack.UpstreamCurrent:
		return theme.Badge(ToneSuccess, string(state))
	case stack.UpstreamBehind:
		return theme.Badge(ToneWarn, string(state))
	case stack.UpstreamAhead:
		return theme.Badge(ToneInfo, string(state))
	case stack.UpstreamDiverged:
		return theme.Badge(ToneDanger, string(state))
	case stack.UpstreamMissing:
		return theme.Badge(ToneMuted, string(state))
	default:
		return theme.Badge(ToneMuted, string(state))
	}
}

func rootsExcludingBase(dag *stack.DAG, base string) []string {
	roots := dag.Roots()
	filtered := make([]string, 0, len(roots))
	for _, root := range roots {
		if root == base {
			filtered = append(filtered, dag.Children(base)...)
			continue
		}
		filtered = append(filtered, root)
	}
	return filtered
}
