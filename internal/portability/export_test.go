package portability

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/lutefd/weaver/internal/deps"
	"github.com/lutefd/weaver/internal/group"
	weaverintegration "github.com/lutefd/weaver/internal/integration"
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
	if err := weaverintegration.NewStore(repoRoot).Replace(map[string]weaverintegration.Recipe{
		"integration": {
			Base:     "main",
			Branches: []string{"feature-a", "feature-b"},
		},
	}); err != nil {
		t.Fatalf("Replace integrations error = %v", err)
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

	gotIntegrations, err := weaverintegration.NewStore(otherRepo).List()
	if err != nil {
		t.Fatalf("List() integrations error = %v", err)
	}
	wantIntegrations := map[string]weaverintegration.Recipe{
		"integration": {
			Base:     "main",
			Branches: []string{"feature-a", "feature-b"},
		},
	}
	if !reflect.DeepEqual(gotIntegrations, wantIntegrations) {
		t.Fatalf("integrations = %#v, want %#v", gotIntegrations, wantIntegrations)
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

func TestImportRejectsNilAndInvalidState(t *testing.T) {
	t.Parallel()

	manager := New(t.TempDir())
	if err := manager.Import(nil); err == nil || err.Error() != "import state is required" {
		t.Fatalf("Import(nil) error = %v, want required state error", err)
	}

	err := manager.Import(&State{
		Dependencies: map[string]string{
			"feature-a": "feature-b",
			"feature-b": "feature-a",
		},
	})
	if err == nil || !strings.Contains(err.Error(), "dependency cycle detected") {
		t.Fatalf("Import(cycle) error = %v, want cycle error", err)
	}
}

func TestDecodeInvalidJSON(t *testing.T) {
	t.Parallel()

	_, err := Decode(bytes.NewBufferString("{"))
	if err == nil || !strings.Contains(err.Error(), "decode export:") {
		t.Fatalf("Decode() error = %v, want wrapped decode error", err)
	}
}

func TestLoadFile(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "state.json")
	if err := os.WriteFile(path, []byte(`{"version":1,"dependencies":{"feature-b":"feature-a"}}`), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	state, err := LoadFile(path)
	if err != nil {
		t.Fatalf("LoadFile() error = %v", err)
	}
	if state.Version != 1 {
		t.Fatalf("Version = %d, want 1", state.Version)
	}
	if !reflect.DeepEqual(state.Dependencies, map[string]string{"feature-b": "feature-a"}) {
		t.Fatalf("Dependencies = %#v", state.Dependencies)
	}
}

func TestExportImportAndLoadFileErrors(t *testing.T) {
	t.Parallel()

	repoRoot := filepath.Join(t.TempDir(), "repo-root-file")
	if err := os.WriteFile(repoRoot, []byte("x"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	if _, err := New(repoRoot).Export(); err == nil {
		t.Fatal("Export() error = nil, want file-backed repo root error")
	}

	manager := New(t.TempDir())
	err := manager.Import(&State{
		Dependencies: map[string]string{"feature-b": "feature-a"},
		Groups:       map[string][]string{"sprint": {"feature-a"}},
		Integrations: map[string]weaverintegration.Recipe{
			"integration": {},
		},
	})
	if err == nil || !strings.Contains(err.Error(), `integration "integration" base is required`) {
		t.Fatalf("Import() error = %v, want integration validation error", err)
	}

	if _, err := LoadFile(filepath.Join(t.TempDir(), "missing.json")); err == nil {
		t.Fatal("LoadFile() error = nil, want open error")
	}
}

func TestExportErrorPropagation(t *testing.T) {
	t.Parallel()

	repoRoot := t.TempDir()
	if err := os.MkdirAll(filepath.Join(repoRoot, ".git", "weaver", "groups.yaml"), 0o755); err != nil {
		t.Fatalf("MkdirAll(group path) error = %v", err)
	}
	if _, err := New(repoRoot).Export(); err == nil || !strings.Contains(err.Error(), "read groups file") {
		t.Fatalf("Export() error = %v, want read groups file error", err)
	}

	repoRoot = t.TempDir()
	if err := os.MkdirAll(filepath.Join(repoRoot, ".git", "weaver", "integrations.yaml"), 0o755); err != nil {
		t.Fatalf("MkdirAll(integration path) error = %v", err)
	}
	if _, err := New(repoRoot).Export(); err == nil || !strings.Contains(err.Error(), "read integrations file") {
		t.Fatalf("Export() error = %v, want read integrations file error", err)
	}
}
