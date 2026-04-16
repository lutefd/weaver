package integration

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/lutefd/weaver/internal/config"
)

func TestBranchStoreTrackGetRemove(t *testing.T) {
	t.Parallel()

	store := NewBranchStore(t.TempDir())
	if err := store.Track("release-1", BranchRecord{
		Base:        "main",
		Branches:    []string{"feature-a", "feature-b", "feature-a"},
		Skipped:     []string{"feature-c", "feature-c"},
		Integration: "staging",
	}); err != nil {
		t.Fatalf("Track() error = %v", err)
	}

	got, ok, err := store.Get("release-1")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if !ok {
		t.Fatal("Get() ok = false, want true")
	}

	want := BranchRecord{
		Base:        "main",
		Branches:    []string{"feature-a", "feature-b"},
		Skipped:     []string{"feature-c"},
		Integration: "staging",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Get() = %#v, want %#v", got, want)
	}

	if err := store.Remove("release-1"); err != nil {
		t.Fatalf("Remove() error = %v", err)
	}
	if _, ok, err := store.Get("release-1"); err != nil {
		t.Fatalf("Get() after remove error = %v", err)
	} else if ok {
		t.Fatal("Get() after remove ok = true, want false")
	}
}

func TestBranchStoreListNamesAndPath(t *testing.T) {
	t.Parallel()

	repoRoot := t.TempDir()
	store := NewBranchStore(repoRoot)
	if err := store.Track("release-2", BranchRecord{
		Base:     "main",
		Branches: []string{"feature-b"},
	}); err != nil {
		t.Fatalf("Track(release-2) error = %v", err)
	}
	if err := store.Track("release-1", BranchRecord{
		Base:     "integration",
		Branches: []string{"feature-a"},
	}); err != nil {
		t.Fatalf("Track(release-1) error = %v", err)
	}

	names, err := store.Names()
	if err != nil {
		t.Fatalf("Names() error = %v", err)
	}
	if want := []string{"release-1", "release-2"}; !reflect.DeepEqual(names, want) {
		t.Fatalf("Names() = %#v, want %#v", names, want)
	}

	got, err := store.List()
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	want := map[string]BranchRecord{
		"release-1": {Base: "integration", Branches: []string{"feature-a"}},
		"release-2": {Base: "main", Branches: []string{"feature-b"}},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("List() = %#v, want %#v", got, want)
	}

	if path := store.path(); path != filepath.Join(repoRoot, ".git", "weaver", "integration-branches.yaml") {
		t.Fatalf("path() = %q", path)
	}
}

func TestBranchStoreReplace(t *testing.T) {
	t.Parallel()

	store := NewBranchStore(t.TempDir())
	if err := store.Replace(map[string]BranchRecord{
		"release-2": {
			Base:     "main",
			Branches: []string{"feature-b"},
		},
		"release-1": {
			Base:        "integration",
			Branches:    []string{"feature-a"},
			Skipped:     []string{"feature-c"},
			Integration: "staging",
		},
	}); err != nil {
		t.Fatalf("Replace() error = %v", err)
	}

	got, err := store.List()
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	want := map[string]BranchRecord{
		"release-1": {
			Base:        "integration",
			Branches:    []string{"feature-a"},
			Skipped:     []string{"feature-c"},
			Integration: "staging",
		},
		"release-2": {
			Base:     "main",
			Branches: []string{"feature-b"},
		},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("List() after Replace = %#v, want %#v", got, want)
	}
}

func TestValidateBranchRecord(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		recordName string
		record     BranchRecord
		wantErr    bool
	}{
		{name: "missing name", record: BranchRecord{Base: "main", Branches: []string{"feature-a"}}, wantErr: true},
		{name: "missing base", recordName: "release", record: BranchRecord{Branches: []string{"feature-a"}}, wantErr: true},
		{name: "missing branches", recordName: "release", record: BranchRecord{Base: "main"}, wantErr: true},
		{name: "only skipped", recordName: "release", record: BranchRecord{Base: "main", Skipped: []string{"feature-a"}}, wantErr: false},
		{name: "valid", recordName: "release", record: BranchRecord{Base: "main", Branches: []string{"feature-a"}}, wantErr: false},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := validateBranchRecord(tt.recordName, tt.record)
			if (err != nil) != tt.wantErr {
				t.Fatalf("validateBranchRecord() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestBranchStoreDecodeAndWriteErrors(t *testing.T) {
	t.Parallel()

	store := NewBranchStore(t.TempDir())
	if err := store.Track("release", BranchRecord{Base: "main", Branches: []string{"feature-a"}}); err != nil {
		t.Fatalf("Track() error = %v", err)
	}
	if err := store.Remove("missing"); err == nil || !strings.Contains(err.Error(), `integration branch "missing" does not exist`) {
		t.Fatalf("Remove() error = %v, want missing branch error", err)
	}

	broken := NewBranchStore(t.TempDir())
	if err := os.MkdirAll(filepath.Dir(broken.path()), 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(broken.path(), []byte("branches: ["), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	if _, err := broken.List(); err == nil || !strings.Contains(err.Error(), "decode integration branches file") {
		t.Fatalf("List() error = %v, want decode error", err)
	}

	invalid := NewBranchStore(t.TempDir())
	if err := os.MkdirAll(filepath.Dir(invalid.path()), 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(invalid.path(), []byte("version: 1\nbranches:\n  broken:\n    base: main\n"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	if _, err := invalid.List(); err == nil || !strings.Contains(err.Error(), `invalid integration branch "broken"`) {
		t.Fatalf("List() error = %v, want invalid branch error", err)
	}

	repoRoot := filepath.Join(t.TempDir(), "repo-root-file")
	if err := os.WriteFile(repoRoot, []byte("x"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	if err := NewBranchStore(repoRoot).write(branchFile{
		Version: config.VersionOne,
		Branches: map[string]BranchRecord{
			"release": {Base: "main", Branches: []string{"feature-a"}},
		},
	}); err == nil || !strings.Contains(err.Error(), "create integration branches directory") {
		t.Fatalf("write() error = %v, want create directory error", err)
	}

	writeStore := NewBranchStore(t.TempDir())
	if err := os.MkdirAll(writeStore.dir(), 0o755); err != nil {
		t.Fatalf("MkdirAll(dir) error = %v", err)
	}
	if err := os.MkdirAll(writeStore.path(), 0o755); err != nil {
		t.Fatalf("MkdirAll(path) error = %v", err)
	}
	if err := writeStore.write(branchFile{
		Version: config.VersionOne,
		Branches: map[string]BranchRecord{
			"release": {Base: "main", Branches: []string{"feature-a"}},
		},
	}); err == nil || !strings.Contains(err.Error(), "write integration branches file") {
		t.Fatalf("write() error = %v, want write error", err)
	}
}

func TestBranchStoreTrackValidationAndReadErrors(t *testing.T) {
	t.Parallel()

	store := NewBranchStore(t.TempDir())
	if err := store.Track("", BranchRecord{Base: "main", Branches: []string{"feature-a"}}); err == nil || !strings.Contains(err.Error(), "integration branch name is required") {
		t.Fatalf("Track() error = %v, want validation error", err)
	}

	broken := NewBranchStore(t.TempDir())
	if err := os.MkdirAll(filepath.Dir(broken.path()), 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.MkdirAll(broken.path(), 0o755); err != nil {
		t.Fatalf("MkdirAll(path) error = %v", err)
	}
	if err := broken.Track("release", BranchRecord{Base: "main", Branches: []string{"feature-a"}}); err == nil || !strings.Contains(err.Error(), "read integration branches file") {
		t.Fatalf("Track() error = %v, want read error", err)
	}

	if _, err := broken.List(); err == nil || !strings.Contains(err.Error(), "read integration branches file") {
		t.Fatalf("List() error = %v, want read error", err)
	}

	if err := store.Replace(map[string]BranchRecord{"release": {}}); err == nil || !strings.Contains(err.Error(), `integration branch "release" base is required`) {
		t.Fatalf("Replace() error = %v, want validation error", err)
	}
}

func TestBranchStoreWriteDefaultsNilBranches(t *testing.T) {
	t.Parallel()

	store := NewBranchStore(t.TempDir())
	if err := store.write(branchFile{}); err != nil {
		t.Fatalf("write() error = %v", err)
	}

	data, err := os.ReadFile(store.path())
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if !strings.Contains(string(data), "version: 1") || !strings.Contains(string(data), "branches: {}") {
		t.Fatalf("written data = %q", string(data))
	}
}
