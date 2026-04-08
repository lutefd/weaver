package deps

import (
	"context"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/lutefd/weaver/internal/stack"
)

func TestLocalSourceSetLoadRemove(t *testing.T) {
	t.Parallel()

	repoRoot := t.TempDir()
	source := NewLocalSource(repoRoot)
	ctx := context.Background()

	if err := source.Set(ctx, "feature-b", "feature-a"); err != nil {
		t.Fatalf("Set() error = %v", err)
	}
	if err := source.Set(ctx, "feature-c", "feature-b"); err != nil {
		t.Fatalf("Set() error = %v", err)
	}

	got, err := source.Load(ctx)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	want := []stack.Dependency{
		{Branch: "feature-b", Parent: "feature-a"},
		{Branch: "feature-c", Parent: "feature-b"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Load() = %#v, want %#v", got, want)
	}

	if err := source.Remove(ctx, "feature-b"); err != nil {
		t.Fatalf("Remove() error = %v", err)
	}

	got, err = source.Load(ctx)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	want = []stack.Dependency{{Branch: "feature-c", Parent: "feature-b"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Load() after remove = %#v, want %#v", got, want)
	}
}

func TestLocalSourceMissingFile(t *testing.T) {
	t.Parallel()

	source := NewLocalSource(t.TempDir())
	got, err := source.Load(context.Background())
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("Load() = %#v, want empty", got)
	}
}

func TestLocalSourceWritesInsideGitMetadata(t *testing.T) {
	t.Parallel()

	repoRoot := t.TempDir()
	source := NewLocalSource(repoRoot)
	if err := source.Set(context.Background(), "feature-b", "feature-a"); err != nil {
		t.Fatalf("Set() error = %v", err)
	}

	if got, want := source.path(), filepath.Join(repoRoot, ".git", "weaver", "deps.yaml"); got != want {
		t.Fatalf("path() = %q, want %q", got, want)
	}
}
