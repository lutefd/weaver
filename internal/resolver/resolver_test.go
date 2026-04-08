package resolver

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/lutefd/weaver/internal/stack"
)

func TestResolverResolve(t *testing.T) {
	t.Parallel()

	dag, err := New(fakeSource{
		deps: []stack.Dependency{
			{Branch: "feature-b", Parent: "feature-a"},
			{Branch: "feature-c", Parent: "feature-b"},
		},
	}).Resolve(context.Background())
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}

	got, err := dag.TopologicalSort()
	if err != nil {
		t.Fatalf("TopologicalSort() error = %v", err)
	}

	want := []string{"feature-a", "feature-b", "feature-c"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("TopologicalSort() = %#v, want %#v", got, want)
	}
}

func TestResolverResolveError(t *testing.T) {
	t.Parallel()

	wantErr := errors.New("boom")
	_, err := New(fakeSource{err: wantErr}).Resolve(context.Background())
	if !errors.Is(err, wantErr) {
		t.Fatalf("Resolve() error = %v, want %v", err, wantErr)
	}
}

type fakeSource struct {
	deps []stack.Dependency
	err  error
}

func (f fakeSource) Load(context.Context) ([]stack.Dependency, error) {
	return f.deps, f.err
}

func (f fakeSource) Set(context.Context, string, string) error {
	return nil
}

func (f fakeSource) Remove(context.Context, string) error {
	return nil
}
