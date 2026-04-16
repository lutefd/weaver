package cmd

import (
	"context"
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	gitrunner "github.com/lutefd/weaver/internal/git"
	weaverintegration "github.com/lutefd/weaver/internal/integration"
	"github.com/lutefd/weaver/internal/ui"
)

type trackedIntegrationBranchBrowserModel struct {
	ctx           context.Context
	theme         ui.Theme
	runner        gitrunner.Runner
	store         *weaverintegration.BranchStore
	entries       []trackedIntegrationBranchEntry
	selected      int
	confirmDelete bool
	flashTone     ui.Tone
	flashMessage  string
	quitting      bool
}

type trackedIntegrationBranchRefreshMsg struct {
	entries []trackedIntegrationBranchEntry
	err     error
}

type trackedIntegrationBranchDeleteMsg struct {
	name   string
	result trackedIntegrationBranchDeleteResult
	err    error
}

func runTrackedIntegrationBranchBrowser(ctx context.Context, term ui.Terminal, runner gitrunner.Runner, store *weaverintegration.BranchStore, entries []trackedIntegrationBranchEntry) error {
	model := newTrackedIntegrationBranchBrowserModel(ctx, term, runner, store, entries)
	program := tea.NewProgram(
		model,
		tea.WithContext(ctx),
		tea.WithInput(term.In()),
		tea.WithOutput(term.Out()),
		tea.WithAltScreen(),
	)
	_, err := program.Run()
	if err != nil && err != tea.ErrProgramKilled {
		return err
	}
	return nil
}

func newTrackedIntegrationBranchBrowserModel(ctx context.Context, term ui.Terminal, runner gitrunner.Runner, store *weaverintegration.BranchStore, entries []trackedIntegrationBranchEntry) trackedIntegrationBranchBrowserModel {
	return trackedIntegrationBranchBrowserModel{
		ctx:      ctx,
		theme:    ui.NewTheme(term),
		runner:   runner,
		store:    store,
		entries:  append([]trackedIntegrationBranchEntry(nil), entries...),
		selected: clampTrackedIntegrationBranchIndex(0, len(entries)),
	}
}

func (m trackedIntegrationBranchBrowserModel) Init() tea.Cmd {
	return nil
}

func (m trackedIntegrationBranchBrowserModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if m.confirmDelete {
			return m.updateConfirm(msg)
		}
		return m.updateBrowse(msg)
	case trackedIntegrationBranchRefreshMsg:
		if msg.err != nil {
			m.flashTone = ui.ToneDanger
			m.flashMessage = msg.err.Error()
			return m, nil
		}
		m.entries = append([]trackedIntegrationBranchEntry(nil), msg.entries...)
		m.selected = clampTrackedIntegrationBranchIndex(m.selected, len(m.entries))
		m.flashTone = ui.ToneInfo
		m.flashMessage = "refreshed integration branches"
		return m, nil
	case trackedIntegrationBranchDeleteMsg:
		if msg.err != nil {
			m.flashTone = ui.ToneDanger
			m.flashMessage = msg.err.Error()
			return m, nil
		}

		nextEntries := make([]trackedIntegrationBranchEntry, 0, len(m.entries))
		for _, entry := range m.entries {
			if entry.Name == msg.name {
				continue
			}
			nextEntries = append(nextEntries, entry)
		}
		m.entries = nextEntries
		m.selected = clampTrackedIntegrationBranchIndex(m.selected, len(m.entries))
		m.flashTone = ui.ToneSuccess
		if msg.result.DeletedBranch {
			m.flashMessage = fmt.Sprintf("deleted %s", msg.name)
		} else {
			m.flashMessage = fmt.Sprintf("removed tracked entry for %s", msg.name)
		}
		return m, nil
	}

	return m, nil
}

func (m trackedIntegrationBranchBrowserModel) updateBrowse(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "ctrl+c":
		m.quitting = true
		return m, tea.Quit
	case "up", "k":
		if m.selected > 0 {
			m.selected--
		}
	case "down", "j":
		if m.selected < len(m.entries)-1 {
			m.selected++
		}
	case "g", "home":
		m.selected = 0
	case "G", "end":
		if len(m.entries) > 0 {
			m.selected = len(m.entries) - 1
		}
	case "r":
		return m, refreshTrackedIntegrationBranchEntriesCmd(m.ctx, m.runner, m.store)
	case "d", "x", "backspace":
		if len(m.entries) > 0 {
			m.confirmDelete = true
			m.flashTone = ui.ToneWarn
			m.flashMessage = fmt.Sprintf("delete %s? press y to confirm", m.entries[m.selected].Name)
		}
	}
	return m, nil
}

func (m trackedIntegrationBranchBrowserModel) updateConfirm(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "ctrl+c":
		m.quitting = true
		return m, tea.Quit
	case "esc", "n":
		m.confirmDelete = false
		m.flashTone = ui.ToneInfo
		m.flashMessage = "delete cancelled"
		return m, nil
	case "y":
		if len(m.entries) == 0 {
			m.confirmDelete = false
			return m, nil
		}
		name := m.entries[m.selected].Name
		m.confirmDelete = false
		m.flashTone = ui.ToneWarn
		m.flashMessage = fmt.Sprintf("deleting %s", name)
		return m, deleteTrackedIntegrationBranchCmd(m.ctx, m.runner, m.store, name)
	}
	return m, nil
}

func (m trackedIntegrationBranchBrowserModel) View() string {
	if m.quitting {
		return ""
	}

	bodyParts := []string{
		m.renderHeader(),
		m.renderList(),
		m.renderDetails(),
	}
	if m.flashMessage != "" {
		bodyParts = append(bodyParts, m.theme.Bullet(m.flashTone, m.flashMessage, ""))
	}
	bodyParts = append(bodyParts, m.renderHelp())

	return m.theme.Card(
		"Integration Branches",
		"Tracked branches created by compose --create",
		lipgloss.JoinVertical(lipgloss.Left, bodyParts...),
	)
}

func (m trackedIntegrationBranchBrowserModel) renderHeader() string {
	count := fmt.Sprintf("%d tracked", len(m.entries))
	selected := "none selected"
	if len(m.entries) > 0 {
		selected = fmt.Sprintf("%d/%d", m.selected+1, len(m.entries))
	}
	return lipgloss.JoinHorizontal(
		lipgloss.Center,
		m.theme.Badge(ui.ToneInfo, count),
		"  ",
		m.theme.Badge(ui.ToneMuted, selected),
	)
}

func (m trackedIntegrationBranchBrowserModel) renderList() string {
	if len(m.entries) == 0 {
		return m.theme.Muted("No tracked integration branches yet. Compose with --create to add one.")
	}

	lines := make([]string, 0, len(m.entries))
	for idx, entry := range m.entries {
		cursor := "  "
		if idx == m.selected {
			cursor = m.theme.Connector(">")
		}

		statusTone := ui.ToneWarn
		switch entry.Status() {
		case "present", "integrated":
			statusTone = ui.ToneSuccess
		case "partial":
			statusTone = ui.ToneInfo
		}
		line := lipgloss.JoinHorizontal(
			lipgloss.Center,
			cursor,
			" ",
			m.theme.Branch(entry.Name),
			"  ",
			m.theme.Badge(statusTone, entry.Status()),
			"  ",
			m.theme.Muted("base"),
			" ",
			m.theme.BaseBranch(entry.Record.Base),
		)
		lines = append(lines, line)
	}

	return lipgloss.JoinVertical(lipgloss.Left, lines...)
}

func (m trackedIntegrationBranchBrowserModel) renderDetails() string {
	if len(m.entries) == 0 {
		return ""
	}

	entry := m.entries[m.selected]
	details := []ui.KeyValue{
		{Label: "branch", Value: entry.Name},
		{Label: "status", Value: entry.Status()},
		{Label: "base", Value: entry.Record.Base},
		{Label: "branches", Value: formatTrackedBranchSlice(entry.Record.Branches)},
	}
	if included := entry.includedSkipped(); len(included) > 0 {
		details = append(details, ui.KeyValue{Label: "integrated", Value: strings.Join(included, ", ")})
	}
	if pending := entry.pendingSkipped(); len(pending) > 0 {
		details = append(details, ui.KeyValue{Label: "skipped", Value: strings.Join(pending, ", ")})
	}
	if entry.Record.Integration != "" {
		details = append(details, ui.KeyValue{Label: "integration", Value: entry.Record.Integration})
	}

	return m.theme.Card("Details", "Selected integration branch", m.theme.KeyValues(details))
}

func (m trackedIntegrationBranchBrowserModel) renderHelp() string {
	if m.confirmDelete {
		return m.theme.Muted("y confirm delete  n cancel  q quit")
	}
	return m.theme.Muted("up/down move  d delete  r refresh  q quit")
}

func refreshTrackedIntegrationBranchEntriesCmd(ctx context.Context, runner gitrunner.Runner, store *weaverintegration.BranchStore) tea.Cmd {
	return func() tea.Msg {
		entries, err := loadTrackedIntegrationBranchEntries(ctx, runner, store)
		return trackedIntegrationBranchRefreshMsg{
			entries: entries,
			err:     err,
		}
	}
}

func deleteTrackedIntegrationBranchCmd(ctx context.Context, runner gitrunner.Runner, store *weaverintegration.BranchStore, name string) tea.Cmd {
	return func() tea.Msg {
		result, err := deleteTrackedIntegrationBranch(ctx, runner, store, name)
		return trackedIntegrationBranchDeleteMsg{
			name:   name,
			result: result,
			err:    err,
		}
	}
}

func clampTrackedIntegrationBranchIndex(idx int, size int) int {
	if size == 0 {
		return 0
	}
	if idx < 0 {
		return 0
	}
	if idx >= size {
		return size - 1
	}
	return idx
}
