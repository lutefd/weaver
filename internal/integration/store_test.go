package integration

import (
	"path/filepath"
	"reflect"
	"testing"
)

func TestStoreSaveGetRemove(t *testing.T) {
	t.Parallel()

	store := NewStore(t.TempDir())
	if err := store.Save("integration", Recipe{
		Base:     "main",
		Branches: []string{"feature-a", "feature-b", "feature-a"},
	}); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	got, ok, err := store.Get("integration")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if !ok {
		t.Fatal("Get() ok = false, want true")
	}

	want := Recipe{
		Base:     "main",
		Branches: []string{"feature-a", "feature-b"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Get() = %#v, want %#v", got, want)
	}

	if err := store.Remove("integration"); err != nil {
		t.Fatalf("Remove() error = %v", err)
	}
	if _, ok, err := store.Get("integration"); err != nil {
		t.Fatalf("Get() after remove error = %v", err)
	} else if ok {
		t.Fatal("Get() after remove ok = true, want false")
	}
}

func TestStoreReplaceAndNames(t *testing.T) {
	t.Parallel()

	store := NewStore(t.TempDir())
	if err := store.Replace(map[string]Recipe{
		"integration": {
			Base:     "main",
			Branches: []string{"feature-a", "feature-b"},
		},
		"release": {
			Base:     "release",
			Branches: []string{"hotfix-a"},
		},
	}); err != nil {
		t.Fatalf("Replace() error = %v", err)
	}

	names, err := store.Names()
	if err != nil {
		t.Fatalf("Names() error = %v", err)
	}
	wantNames := []string{"integration", "release"}
	if !reflect.DeepEqual(names, wantNames) {
		t.Fatalf("Names() = %#v, want %#v", names, wantNames)
	}

	got, err := store.List()
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	want := map[string]Recipe{
		"integration": {
			Base:     "main",
			Branches: []string{"feature-a", "feature-b"},
		},
		"release": {
			Base:     "release",
			Branches: []string{"hotfix-a"},
		},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("List() = %#v, want %#v", got, want)
	}
}

func TestStorePath(t *testing.T) {
	t.Parallel()

	repoRoot := t.TempDir()
	store := NewStore(repoRoot)
	want := filepath.Join(repoRoot, ".git", "weaver", "integrations.yaml")
	if got := store.path(); got != want {
		t.Fatalf("path() = %q, want %q", got, want)
	}
}

func TestValidateRecipe(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		recipe     Recipe
		recipeName string
		wantErr    bool
	}{
		{name: "missing name", recipeName: "", recipe: Recipe{Base: "main", Branches: []string{"feature-a"}}, wantErr: true},
		{name: "missing base", recipeName: "integration", recipe: Recipe{Branches: []string{"feature-a"}}, wantErr: true},
		{name: "missing branches", recipeName: "integration", recipe: Recipe{Base: "main"}, wantErr: true},
		{name: "valid", recipeName: "integration", recipe: Recipe{Base: "main", Branches: []string{"feature-a"}}, wantErr: false},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := validateRecipe(tt.recipeName, tt.recipe)
			if (err != nil) != tt.wantErr {
				t.Fatalf("validateRecipe() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
