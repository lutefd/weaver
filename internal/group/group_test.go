package group

import (
	"path/filepath"
	"reflect"
	"testing"
)

func TestStoreCreateAddRemoveList(t *testing.T) {
	t.Parallel()

	store := NewStore(t.TempDir())
	if err := store.Create("sprint-42", []string{"feature-a", "feature-b"}); err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if err := store.Add("sprint-42", []string{"feature-c", "feature-b"}); err != nil {
		t.Fatalf("Add() error = %v", err)
	}

	got, ok, err := store.Get("sprint-42")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if !ok {
		t.Fatal("Get() ok = false, want true")
	}
	want := []string{"feature-a", "feature-b", "feature-c"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Get() = %#v, want %#v", got, want)
	}

	if err := store.Remove("sprint-42", []string{"feature-b"}); err != nil {
		t.Fatalf("Remove() error = %v", err)
	}
	got, ok, err = store.Get("sprint-42")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if !ok {
		t.Fatal("Get() ok = false, want true")
	}
	want = []string{"feature-a", "feature-c"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Get() after remove = %#v, want %#v", got, want)
	}
}

func TestStoreReplaceAndNames(t *testing.T) {
	t.Parallel()

	store := NewStore(t.TempDir())
	if err := store.Replace(map[string][]string{
		"hotfix":    {"fix-auth"},
		"sprint-42": {"feature-a", "feature-a", "feature-b"},
	}); err != nil {
		t.Fatalf("Replace() error = %v", err)
	}

	names, err := store.Names()
	if err != nil {
		t.Fatalf("Names() error = %v", err)
	}
	wantNames := []string{"hotfix", "sprint-42"}
	if !reflect.DeepEqual(names, wantNames) {
		t.Fatalf("Names() = %#v, want %#v", names, wantNames)
	}

	groups, err := store.List()
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	wantGroups := map[string][]string{
		"hotfix":    {"fix-auth"},
		"sprint-42": {"feature-a", "feature-b"},
	}
	if !reflect.DeepEqual(groups, wantGroups) {
		t.Fatalf("List() = %#v, want %#v", groups, wantGroups)
	}
}

func TestStorePath(t *testing.T) {
	t.Parallel()

	repoRoot := t.TempDir()
	store := NewStore(repoRoot)
	want := filepath.Join(repoRoot, ".git", "weaver", "groups.yaml")
	if got := store.path(); got != want {
		t.Fatalf("path() = %q, want %q", got, want)
	}
}
