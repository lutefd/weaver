package stack

import (
	"fmt"
	"sort"
)

type DAG struct {
	nodes    map[string]struct{}
	parents  map[string]string
	children map[string][]string
}

func NewDAG(deps []Dependency) (*DAG, error) {
	dag := &DAG{
		nodes:    make(map[string]struct{}),
		parents:  make(map[string]string),
		children: make(map[string][]string),
	}

	for _, dep := range deps {
		if err := dag.Add(dep.Branch, dep.Parent); err != nil {
			return nil, err
		}
	}

	if _, err := dag.TopologicalSort(); err != nil {
		return nil, err
	}

	return dag, nil
}

func (d *DAG) Add(branch, parent string) error {
	if branch == "" {
		return fmt.Errorf("dependency branch is required")
	}
	if parent == "" {
		return fmt.Errorf("dependency parent is required")
	}
	if branch == parent {
		return fmt.Errorf("branch %q cannot depend on itself", branch)
	}

	if existing, ok := d.parents[branch]; ok && existing != parent {
		return fmt.Errorf("branch %q already depends on %q", branch, existing)
	}

	d.nodes[branch] = struct{}{}
	d.nodes[parent] = struct{}{}
	d.parents[branch] = parent
	d.children[parent] = appendUnique(d.children[parent], branch)

	return nil
}

func (d *DAG) Parent(branch string) (string, bool) {
	parent, ok := d.parents[branch]
	return parent, ok
}

func (d *DAG) Children(branch string) []string {
	children := append([]string(nil), d.children[branch]...)
	sort.Strings(children)
	return children
}

func (d *DAG) Branches() []string {
	branches := make([]string, 0, len(d.nodes))
	for branch := range d.nodes {
		branches = append(branches, branch)
	}
	sort.Strings(branches)
	return branches
}

func (d *DAG) Dependencies() []Dependency {
	branches := make([]string, 0, len(d.parents))
	for branch := range d.parents {
		branches = append(branches, branch)
	}
	sort.Strings(branches)

	deps := make([]Dependency, 0, len(branches))
	for _, branch := range branches {
		deps = append(deps, Dependency{Branch: branch, Parent: d.parents[branch]})
	}

	return deps
}

func (d *DAG) Roots() []string {
	roots := make([]string, 0)
	for branch := range d.nodes {
		if _, ok := d.parents[branch]; ok {
			continue
		}
		if len(d.children[branch]) == 0 {
			continue
		}
		roots = append(roots, branch)
	}
	sort.Strings(roots)
	return roots
}

func (d *DAG) Contains(branch string) bool {
	_, ok := d.nodes[branch]
	return ok
}

func (d *DAG) Ancestors(branch string) ([]string, error) {
	if branch == "" {
		return nil, fmt.Errorf("branch is required")
	}
	if !d.Contains(branch) {
		return []string{branch}, nil
	}

	visited := map[string]struct{}{}
	chain := []string{branch}
	current := branch

	for {
		parent, ok := d.parents[current]
		if !ok {
			break
		}
		if _, seen := visited[parent]; seen {
			return nil, fmt.Errorf("dependency cycle detected")
		}
		visited[parent] = struct{}{}
		chain = append(chain, parent)
		current = parent
	}

	reverse(chain)
	return chain, nil
}

func (d *DAG) TopologicalSort() ([]string, error) {
	inDegree := make(map[string]int, len(d.nodes))
	for branch := range d.nodes {
		inDegree[branch] = 0
	}
	for child := range d.parents {
		inDegree[child]++
	}

	queue := make([]string, 0)
	for branch, degree := range inDegree {
		if degree == 0 {
			queue = append(queue, branch)
		}
	}
	sort.Strings(queue)

	order := make([]string, 0, len(d.nodes))
	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		order = append(order, current)

		children := d.Children(current)
		for _, child := range children {
			inDegree[child]--
			if inDegree[child] == 0 {
				queue = append(queue, child)
				sort.Strings(queue)
			}
		}
	}

	if len(order) != len(d.nodes) {
		return nil, fmt.Errorf("dependency cycle detected")
	}

	return order, nil
}

func UpsertDependency(deps []Dependency, next Dependency) []Dependency {
	updated := make([]Dependency, 0, len(deps)+1)
	replaced := false
	for _, dep := range deps {
		if dep.Branch == next.Branch {
			if !replaced {
				updated = append(updated, next)
				replaced = true
			}
			continue
		}
		updated = append(updated, dep)
	}
	if !replaced {
		updated = append(updated, next)
	}
	return updated
}

func appendUnique(values []string, next string) []string {
	for _, value := range values {
		if value == next {
			return values
		}
	}
	return append(values, next)
}

func reverse(values []string) {
	for i, j := 0, len(values)-1; i < j; i, j = i+1, j-1 {
		values[i], values[j] = values[j], values[i]
	}
}
