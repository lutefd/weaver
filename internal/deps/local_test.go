package deps

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"strings"
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

func TestLocalSourceReplaceAndMap(t *testing.T) {
	t.Parallel()

	repoRoot := t.TempDir()
	source := NewLocalSource(repoRoot)
	dependencies := map[string]string{"feature-b": "feature-a"}
	if err := source.Replace(dependencies); err != nil {
		t.Fatalf("Replace() error = %v", err)
	}

	dependencies["feature-c"] = "feature-b"
	got, err := source.Map(context.Background())
	if err != nil {
		t.Fatalf("Map() error = %v", err)
	}
	want := map[string]string{"feature-b": "feature-a"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Map() = %#v, want %#v", got, want)
	}
}

func TestLocalSourceLoadDecodeError(t *testing.T) {
	t.Parallel()

	repoRoot := t.TempDir()
	source := NewLocalSource(repoRoot)
	if err := os.MkdirAll(filepath.Dir(source.path()), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(source.path(), []byte("dependencies: ["), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	_, err := source.Load(context.Background())
	if err == nil || !strings.Contains(err.Error(), "decode deps file") {
		t.Fatalf("Load() error = %v, want decode deps file", err)
	}
}

func TestLocalSourceMapAndParseErrors(t *testing.T) {
	t.Parallel()

	repoRoot := t.TempDir()
	source := NewLocalSource(repoRoot)
	if err := os.MkdirAll(filepath.Dir(source.path()), 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(source.path(), []byte("version: 1\ndependencies:\n"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	values, err := source.Map(context.Background())
	if err != nil {
		t.Fatalf("Map() error = %v", err)
	}
	if len(values) != 0 {
		t.Fatalf("Map() = %#v, want empty map", values)
	}

	if err := source.Replace(nil); err != nil {
		t.Fatalf("Replace(nil) error = %v", err)
	}
	if err := source.Remove(context.Background(), "missing"); err != nil {
		t.Fatalf("Remove(missing) error = %v", err)
	}
}
