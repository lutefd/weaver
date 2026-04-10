package merger

import (
	"os"
	"path/filepath"
	"reflect"
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
	want := filepath.Join(repoRoot, ".git", "weaver", "merge-state.yaml")
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
