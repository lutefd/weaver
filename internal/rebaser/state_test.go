package rebaser

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/lutefd/weaver/internal/config"
)

func TestStateStoreRoundTrip(t *testing.T) {
	t.Parallel()

	repoRoot := t.TempDir()
	store := NewStateStore(repoRoot)
	now := time.Date(2026, 4, 7, 14, 30, 0, 0, time.UTC)

	want := &State{
		StartedAt:      now,
		OriginalBranch: "feature-c",
		BaseBranch:     "main",
		AllBranches:    []string{"feature-a", "feature-b", "feature-c"},
		Completed:      []string{"feature-a"},
		Current:        "feature-b",
		CurrentOnto:    "feature-a",
	}
	if err := store.Save(want); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	got, err := store.Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	want.Version = config.VersionOne
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Load() = %#v, want %#v", got, want)
	}

	if !store.HasPending() {
		t.Fatal("HasPending() = false, want true")
	}
}

func TestStateStoreClear(t *testing.T) {
	t.Parallel()

	store := NewStateStore(t.TempDir())
	if err := store.Save(&State{OriginalBranch: "main"}); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	if err := store.Clear(); err != nil {
		t.Fatalf("Clear() error = %v", err)
	}

	if store.HasPending() {
		t.Fatal("HasPending() = true, want false")
	}
}

func TestStateStorePath(t *testing.T) {
	t.Parallel()

	repoRoot := t.TempDir()
	store := NewStateStore(repoRoot)
	want := filepath.Join(repoRoot, ".git", "weaver", "rebase-state.yaml")
	if got := store.path(); got != want {
		t.Fatalf("path() = %q, want %q", got, want)
	}

	if err := store.Save(&State{}); err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	if _, err := os.Stat(want); err != nil {
		t.Fatalf("state file missing: %v", err)
	}
}

func TestStateStoreErrorsAndDefaults(t *testing.T) {
	t.Parallel()

	store := NewStateStore(t.TempDir())
	if err := store.Clear(); err != nil {
		t.Fatalf("Clear() missing file error = %v", err)
	}

	if err := os.MkdirAll(filepath.Dir(store.path()), 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(store.path(), []byte("original_branch: topic\n"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	state, err := store.Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if state.Version != config.VersionOne {
		t.Fatalf("Load().Version = %d, want %d", state.Version, config.VersionOne)
	}

	if err := os.WriteFile(store.path(), []byte("version: ["), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	if _, err := store.Load(); err == nil || !strings.Contains(err.Error(), "decode rebase state") {
		t.Fatalf("Load() error = %v, want decode rebase state", err)
	}

	repoRoot := filepath.Join(t.TempDir(), "not-a-directory")
	if err := os.WriteFile(repoRoot, []byte("x"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	if err := NewStateStore(repoRoot).Save(&State{}); err == nil || !strings.Contains(err.Error(), "create rebase state directory") {
		t.Fatalf("Save() error = %v, want create rebase state directory", err)
	}

	if err := os.Remove(store.path()); err != nil {
		t.Fatalf("Remove() error = %v", err)
	}
	if err := os.MkdirAll(store.path(), 0o755); err != nil {
		t.Fatalf("MkdirAll(path) error = %v", err)
	}
	if err := store.Save(&State{}); err == nil || !strings.Contains(err.Error(), "write rebase state") {
		t.Fatalf("Save() error = %v, want write rebase state", err)
	}

	if err := os.WriteFile(filepath.Join(store.path(), "child"), []byte("x"), 0o644); err != nil {
		t.Fatalf("WriteFile(child) error = %v", err)
	}
	if err := store.Clear(); err == nil || !strings.Contains(err.Error(), "remove rebase state") {
		t.Fatalf("Clear() error = %v, want remove rebase state", err)
	}
}
