package resolver

import (
	"context"
	"fmt"

	"github.com/lutefd/weaver/internal/deps"
	"github.com/lutefd/weaver/internal/stack"
)

type Resolver struct {
	source deps.Source
}

func New(source deps.Source) *Resolver {
	return &Resolver{source: source}
}

func (r *Resolver) Resolve(ctx context.Context) (*stack.DAG, error) {
	dependencies, err := r.source.Load(ctx)
	if err != nil {
		return nil, fmt.Errorf("load dependencies: %w", err)
	}

	dag, err := stack.NewDAG(dependencies)
	if err != nil {
		return nil, fmt.Errorf("build dependency graph: %w", err)
	}

	return dag, nil
}
