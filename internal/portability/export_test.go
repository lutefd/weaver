package portability

import (
	"bytes"
	"reflect"
	"testing"
	"time"

	"github.com/lutefd/weaver/internal/deps"
	"github.com/lutefd/weaver/internal/group"
)

func TestExportImportRoundTrip(t *testing.T) {
	t.Parallel()

	repoRoot := t.TempDir()
	if err := deps.NewLocalSource(repoRoot).Replace(map[string]string{
		"feature-b": "feature-a",
		"feature-c": "feature-b",
	}); err != nil {
		t.Fatalf("Replace deps error = %v", err)
	}
	if err := group.NewStore(repoRoot).Replace(map[string][]string{
		"sprint-42": {"feature-a", "feature-b"},
	}); err != nil {
		t.Fatalf("Replace groups error = %v", err)
	}

	manager := New(repoRoot)
	state, err := manager.Export()
	if err != nil {
		t.Fatalf("Export() error = %v", err)
	}

	var buf bytes.Buffer
	if err := Encode(&buf, state); err != nil {
		t.Fatalf("Encode() error = %v", err)
	}

	decoded, err := Decode(&buf)
	if err != nil {
		t.Fatalf("Decode() error = %v", err)
	}

	otherRepo := t.TempDir()
	if err := New(otherRepo).Import(decoded); err != nil {
		t.Fatalf("Import() error = %v", err)
	}

	gotDeps, err := deps.NewLocalSource(otherRepo).Map(nil)
	if err != nil {
		t.Fatalf("Map() error = %v", err)
	}
	wantDeps := map[string]string{
		"feature-b": "feature-a",
		"feature-c": "feature-b",
	}
	if !reflect.DeepEqual(gotDeps, wantDeps) {
		t.Fatalf("deps = %#v, want %#v", gotDeps, wantDeps)
	}

	gotGroups, err := group.NewStore(otherRepo).List()
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	wantGroups := map[string][]string{
		"sprint-42": {"feature-a", "feature-b"},
	}
	if !reflect.DeepEqual(gotGroups, wantGroups) {
		t.Fatalf("groups = %#v, want %#v", gotGroups, wantGroups)
	}
}

func TestDecode(t *testing.T) {
	t.Parallel()

	state, err := Decode(bytes.NewBufferString(`{"version":1,"exported_at":"2026-04-07T14:30:00Z","dependencies":{"feature-b":"feature-a"}}`))
	if err != nil {
		t.Fatalf("Decode() error = %v", err)
	}
	if state.Version != 1 {
		t.Fatalf("Version = %d, want 1", state.Version)
	}
	if !state.ExportedAt.Equal(time.Date(2026, 4, 7, 14, 30, 0, 0, time.UTC)) {
		t.Fatalf("ExportedAt = %v, want fixed time", state.ExportedAt)
	}
}
