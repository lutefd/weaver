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

func TestLocalSourceSetInitializesDependenciesAndReadDefaults(t *testing.T) {
	t.Parallel()

	repoRoot := t.TempDir()
	source := NewLocalSource(repoRoot)
	if err := os.MkdirAll(filepath.Dir(source.path()), 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(source.path(), []byte("version: 0\ndependencies:\n"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	file, err := source.read()
	if err != nil {
		t.Fatalf("read() error = %v", err)
	}
	if file.Version != 1 {
		t.Fatalf("read().Version = %d, want 1", file.Version)
	}
	if file.Dependencies == nil {
		t.Fatal("read().Dependencies = nil, want empty map")
	}

	if err := source.Set(context.Background(), "feature-c", "feature-b"); err != nil {
		t.Fatalf("Set() error = %v", err)
	}

	got, err := source.Map(context.Background())
	if err != nil {
		t.Fatalf("Map() error = %v", err)
	}
	if !reflect.DeepEqual(got, map[string]string{"feature-c": "feature-b"}) {
		t.Fatalf("Map() = %#v, want feature-c dependency", got)
	}
}

func TestLocalSourceReadAndWriteErrors(t *testing.T) {
	t.Parallel()

	source := NewLocalSource(t.TempDir())
	if err := os.MkdirAll(source.path(), 0o755); err != nil {
		t.Fatalf("MkdirAll(path) error = %v", err)
	}
	if _, err := source.Load(context.Background()); err == nil || !strings.Contains(err.Error(), "read deps file") {
		t.Fatalf("Load() error = %v, want read deps file error", err)
	}

	repoRoot := filepath.Join(t.TempDir(), "repo-root-file")
	if err := os.WriteFile(repoRoot, []byte("x"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	if err := NewLocalSource(repoRoot).Replace(map[string]string{"feature-b": "feature-a"}); err == nil || !strings.Contains(err.Error(), "create deps directory") {
		t.Fatalf("Replace() error = %v, want create deps directory error", err)
	}

	writeSource := NewLocalSource(t.TempDir())
	if err := os.MkdirAll(writeSource.dir(), 0o755); err != nil {
		t.Fatalf("MkdirAll(dir) error = %v", err)
	}
	if err := os.MkdirAll(writeSource.path(), 0o755); err != nil {
		t.Fatalf("MkdirAll(path) error = %v", err)
	}
	if err := writeSource.Replace(map[string]string{"feature-b": "feature-a"}); err == nil || !strings.Contains(err.Error(), "write deps file") {
		t.Fatalf("Replace() error = %v, want write deps file error", err)
	}
}

func TestLocalSourceOperationReadErrors(t *testing.T) {
	t.Parallel()

	source := NewLocalSource(t.TempDir())
	if err := os.MkdirAll(source.path(), 0o755); err != nil {
		t.Fatalf("MkdirAll(path) error = %v", err)
	}

	if err := source.Set(context.Background(), "feature-b", "feature-a"); err == nil || !strings.Contains(err.Error(), "read deps file") {
		t.Fatalf("Set() error = %v, want read deps file error", err)
	}
	if err := source.Remove(context.Background(), "feature-b"); err == nil || !strings.Contains(err.Error(), "read deps file") {
		t.Fatalf("Remove() error = %v, want read deps file error", err)
	}
	if _, err := source.Map(context.Background()); err == nil || !strings.Contains(err.Error(), "read deps file") {
		t.Fatalf("Map() error = %v, want read deps file error", err)
	}
}
