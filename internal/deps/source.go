package deps

import (
	"context"

	"github.com/lutefd/weaver/internal/stack"
)

type Source interface {
	Load(ctx context.Context) ([]stack.Dependency, error)
	Set(ctx context.Context, branch, parent string) error
	Remove(ctx context.Context, branch string) error
}
