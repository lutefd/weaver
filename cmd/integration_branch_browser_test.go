package cmd

import (
	"bytes"
	"context"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	gitrunner "github.com/lutefd/weaver/internal/git"
	weaverintegration "github.com/lutefd/weaver/internal/integration"
	"github.com/lutefd/weaver/internal/ui"
)

func TestTrackedIntegrationBranchBrowserNavigationAndView(t *testing.T) {
	t.Parallel()

	term := ui.NewTerminal(bytes.NewBuffer(nil), &bytes.Buffer{})
	model := newTrackedIntegrationBranchBrowserModel(context.Background(), term, &commandRecordingRunner{}, weaverintegration.NewBranchStore(t.TempDir()), []trackedIntegrationBranchEntry{
		{Name: "release-1", Exists: true, Record: weaverintegration.BranchRecord{Base: "main", Branches: []string{"feature-a"}}},
		{Name: "release-2", Exists: false, Record: weaverintegration.BranchRecord{Base: "integration", Branches: []string{"feature-b"}, Skipped: []string{"feature-c"}, Integration: "staging"}},
	})

	next, _ := model.Update(tea.KeyMsg{Type: tea.KeyDown})
	model = next.(trackedIntegrationBranchBrowserModel)
	if model.selected != 1 {
		t.Fatalf("selected = %d, want 1", model.selected)
	}

	next, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("g")})
	model = next.(trackedIntegrationBranchBrowserModel)
	if model.selected != 0 {
		t.Fatalf("selected after g = %d, want 0", model.selected)
	}

	next, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("G")})
	model = next.(trackedIntegrationBranchBrowserModel)
	if model.selected != 1 {
		t.Fatalf("selected after G = %d, want 1", model.selected)
	}

	next, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("d")})
	model = next.(trackedIntegrationBranchBrowserModel)
	if !model.confirmDelete {
		t.Fatal("confirmDelete = false, want true")
	}
	if !strings.Contains(model.flashMessage, "delete release-2") {
		t.Fatalf("flashMessage = %q", model.flashMessage)
	}

	view := model.View()
	for _, want := range []string{"Integration Branches", "release-2", "Selected integration branch", "composed", "pending", "feature-c", "y confirm delete"} {
		if !strings.Contains(view, want) {
			t.Fatalf("View() missing %q in %q", want, view)
		}
	}

	next, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("n")})
	model = next.(trackedIntegrationBranchBrowserModel)
	if model.confirmDelete {
		t.Fatal("confirmDelete = true, want false")
	}
	if model.flashMessage != "delete cancelled" {
		t.Fatalf("flashMessage = %q, want delete cancelled", model.flashMessage)
	}
}

func TestTrackedIntegrationBranchBrowserRendersMergedLaterState(t *testing.T) {
	t.Parallel()

	term := ui.NewTerminal(bytes.NewBuffer(nil), &bytes.Buffer{})
	model := newTrackedIntegrationBranchBrowserModel(context.Background(), term, &commandRecordingRunner{}, weaverintegration.NewBranchStore(t.TempDir()), []trackedIntegrationBranchEntry{
		{
			Name:            "release-1",
			Exists:          true,
			IncludedSkipped: []string{"feature-c"},
			Record:          weaverintegration.BranchRecord{Base: "main", Branches: []string{"feature-a", "feature-b"}, Skipped: []string{"feature-c"}},
		},
	})

	view := model.View()
	for _, want := range []string{"COMPLETE", "composed", "feature-a, feature-b", "merged later", "feature-c"} {
		if !strings.Contains(view, want) {
			t.Fatalf("View() missing %q in %q", want, view)
		}
	}
}

func TestTrackedIntegrationBranchBrowserDeleteFlow(t *testing.T) {
	t.Parallel()

	repoRoot := t.TempDir()
	store := weaverintegration.NewBranchStore(repoRoot)
	if err := store.Track("release-1", weaverintegration.BranchRecord{
		Base:     "main",
		Branches: []string{"feature-a"},
	}); err != nil {
		t.Fatalf("Track() error = %v", err)
	}

	term := ui.NewTerminal(bytes.NewBuffer(nil), &bytes.Buffer{})
	runner := &commandRecordingRunner{
		repoRoot: repoRoot,
		results: map[string]gitrunner.Result{
			"show-ref --verify --quiet refs/heads/release-1": {},
			"branch --show-current":                          {Stdout: "feature-b"},
		},
	}
	model := newTrackedIntegrationBranchBrowserModel(context.Background(), term, runner, store, []trackedIntegrationBranchEntry{
		{Name: "release-1", Exists: true, Record: weaverintegration.BranchRecord{Base: "main", Branches: []string{"feature-a"}}},
	})

	next, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("d")})
	model = next.(trackedIntegrationBranchBrowserModel)
	next, cmd := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("y")})
	model = next.(trackedIntegrationBranchBrowserModel)
	if cmd == nil {
		t.Fatal("delete cmd = nil, want cmd")
	}
	if model.confirmDelete {
		t.Fatal("confirmDelete = true, want false after confirmation")
	}

	msg := cmd()
	deleteMsg, ok := msg.(trackedIntegrationBranchDeleteMsg)
	if !ok {
		t.Fatalf("cmd() = %#v, want trackedIntegrationBranchDeleteMsg", msg)
	}
	if deleteMsg.err != nil {
		t.Fatalf("delete cmd err = %v", deleteMsg.err)
	}
	if !deleteMsg.result.DeletedBranch {
		t.Fatal("DeletedBranch = false, want true")
	}

	next, _ = model.Update(deleteMsg)
	model = next.(trackedIntegrationBranchBrowserModel)
	if len(model.entries) != 0 {
		t.Fatalf("entries = %#v, want empty", model.entries)
	}
	if model.flashMessage != "deleted release-1" {
		t.Fatalf("flashMessage = %q", model.flashMessage)
	}
	if _, ok, err := store.Get("release-1"); err != nil {
		t.Fatalf("Get() error = %v", err)
	} else if ok {
		t.Fatal("Get() ok = true, want false")
	}
}

func TestTrackedIntegrationBranchBrowserRefreshAndQuit(t *testing.T) {
	t.Parallel()

	repoRoot := t.TempDir()
	store := weaverintegration.NewBranchStore(repoRoot)
	if err := store.Track("release-1", weaverintegration.BranchRecord{
		Base:     "main",
		Branches: []string{"feature-a"},
	}); err != nil {
		t.Fatalf("Track(release-1) error = %v", err)
	}

	term := ui.NewTerminal(bytes.NewBuffer(nil), &bytes.Buffer{})
	runner := &commandRecordingRunner{
		repoRoot: repoRoot,
		results: map[string]gitrunner.Result{
			"show-ref --verify --quiet refs/heads/release-1": {},
			"show-ref --verify --quiet refs/heads/release-2": {},
		},
	}
	model := newTrackedIntegrationBranchBrowserModel(context.Background(), term, runner, store, []trackedIntegrationBranchEntry{
		{Name: "release-1", Exists: true, Record: weaverintegration.BranchRecord{Base: "main", Branches: []string{"feature-a"}}},
	})

	if err := store.Track("release-2", weaverintegration.BranchRecord{
		Base:     "integration",
		Branches: []string{"feature-b"},
	}); err != nil {
		t.Fatalf("Track(release-2) error = %v", err)
	}

	next, cmd := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("r")})
	model = next.(trackedIntegrationBranchBrowserModel)
	if cmd == nil {
		t.Fatal("refresh cmd = nil, want cmd")
	}

	msg := cmd()
	refreshMsg, ok := msg.(trackedIntegrationBranchRefreshMsg)
	if !ok {
		t.Fatalf("cmd() = %#v, want trackedIntegrationBranchRefreshMsg", msg)
	}
	if refreshMsg.err != nil {
		t.Fatalf("refresh cmd err = %v", refreshMsg.err)
	}

	next, _ = model.Update(refreshMsg)
	model = next.(trackedIntegrationBranchBrowserModel)
	if len(model.entries) != 2 {
		t.Fatalf("entries = %#v, want 2 entries", model.entries)
	}
	if model.flashMessage != "refreshed integration branches" {
		t.Fatalf("flashMessage = %q", model.flashMessage)
	}

	next, quitCmd := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})
	model = next.(trackedIntegrationBranchBrowserModel)
	if !model.quitting {
		t.Fatal("quitting = false, want true")
	}
	if quitCmd == nil || quitCmd() != tea.Quit() {
		t.Fatalf("quit cmd = %#v, want tea.Quit()", quitCmd)
	}
}

func TestTrackedIntegrationBranchBrowserErrorAndHelperPaths(t *testing.T) {
	t.Parallel()

	term := ui.NewTerminal(bytes.NewBuffer(nil), &bytes.Buffer{})
	model := newTrackedIntegrationBranchBrowserModel(context.Background(), term, &commandRecordingRunner{}, weaverintegration.NewBranchStore(t.TempDir()), nil)
	if model.Init() != nil {
		t.Fatal("Init() != nil, want nil")
	}
	if got := model.View(); !strings.Contains(got, "No tracked integration branches yet") || !strings.Contains(got, "up/down move") {
		t.Fatalf("View() = %q", got)
	}

	next, _ := model.Update(trackedIntegrationBranchRefreshMsg{err: context.Canceled})
	model = next.(trackedIntegrationBranchBrowserModel)
	if model.flashTone != ui.ToneDanger || !strings.Contains(model.flashMessage, "context canceled") {
		t.Fatalf("refresh error state = %#v", model)
	}

	next, _ = model.Update(trackedIntegrationBranchDeleteMsg{name: "release-1", err: context.DeadlineExceeded})
	model = next.(trackedIntegrationBranchBrowserModel)
	if model.flashTone != ui.ToneDanger || !strings.Contains(model.flashMessage, "context deadline exceeded") {
		t.Fatalf("delete error state = %#v", model)
	}

	model.confirmDelete = true
	if got := model.renderHelp(); !strings.Contains(got, "y confirm delete") {
		t.Fatalf("renderHelp(confirm) = %q", got)
	}
	next, cmd := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("y")})
	model = next.(trackedIntegrationBranchBrowserModel)
	if model.confirmDelete {
		t.Fatal("confirmDelete = true, want false with empty y flow")
	}
	if cmd != nil {
		t.Fatalf("empty confirm cmd = %#v, want nil", cmd)
	}

	model.confirmDelete = true
	next, quitCmd := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})
	model = next.(trackedIntegrationBranchBrowserModel)
	if !model.quitting {
		t.Fatal("quitting = false, want true")
	}
	if quitCmd == nil || quitCmd() != tea.Quit() {
		t.Fatalf("confirm quit cmd = %#v, want tea.Quit()", quitCmd)
	}

	if got := clampTrackedIntegrationBranchIndex(-1, 2); got != 0 {
		t.Fatalf("clampTrackedIntegrationBranchIndex(-1, 2) = %d", got)
	}
	if got := clampTrackedIntegrationBranchIndex(3, 2); got != 1 {
		t.Fatalf("clampTrackedIntegrationBranchIndex(3, 2) = %d", got)
	}
	if got := clampTrackedIntegrationBranchIndex(1, 0); got != 0 {
		t.Fatalf("clampTrackedIntegrationBranchIndex(1, 0) = %d", got)
	}
}

func TestRunTrackedIntegrationBranchBrowser(t *testing.T) {
	t.Parallel()

	var out bytes.Buffer
	term := ui.NewTerminal(bytes.NewBufferString("q"), &out)
	err := runTrackedIntegrationBranchBrowser(
		context.Background(),
		term,
		&commandRecordingRunner{},
		weaverintegration.NewBranchStore(t.TempDir()),
		[]trackedIntegrationBranchEntry{{Name: "release-1", Exists: true, Record: weaverintegration.BranchRecord{Base: "main", Branches: []string{"feature-a"}}}},
	)
	if err != nil {
		t.Fatalf("runTrackedIntegrationBranchBrowser() error = %v", err)
	}
}
