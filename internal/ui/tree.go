package ui

import (
	"fmt"
	"strings"

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
	roots := dag.Roots()
	for idx, root := range roots {
		lines = append(lines, renderSubtree(dag, root, "", idx == len(roots)-1)...)
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
