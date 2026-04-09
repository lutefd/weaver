package ui

import (
	"context"
	"errors"
	"fmt"
	"math"
	"regexp"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/harmonica"
	"github.com/charmbracelet/lipgloss"
	gitrunner "github.com/lutefd/weaver/internal/git"
)

type TaskSpec struct {
	Title    string
	Subtitle string
	TotalOps int
}

type taskStepMsg struct {
	label     string
	completed int
	total     int
}

type taskDoneMsg struct{}
type taskShowMsg struct{}
type taskClearMsg struct{}
type taskQuitMsg struct{}

type activityModel struct {
	theme     Theme
	spec      TaskSpec
	spinner   spinner.Model
	progress  progress.Model
	spring    harmonica.Spring
	progressV float64
	velocity  float64
	target    float64
	lastStep  string
	stepCount int
	totalOps  int
	startedAt time.Time
	visible   bool
	clearing  bool
	completed bool
}

type taskResult[T any] struct {
	value T
	err   error
}

type pulseMsg time.Time

func RunTask[T any](ctx context.Context, terminal Terminal, runner gitrunner.Runner, spec TaskSpec, fn func(context.Context, gitrunner.Runner) (T, error)) (T, error) {
	var zero T
	if !terminal.Interactive() {
		return fn(ctx, runner)
	}

	baseRunner := quietRunner(runner)
	program := tea.NewProgram(
		newActivityModel(terminal, spec),
		tea.WithContext(ctx),
		tea.WithInput(nil),
		tea.WithOutput(terminal.Out()),
	)

	completedOps := 0
	observed := gitrunner.WithObserver(baseRunner, nil, func(event gitrunner.CommandEvent, _ gitrunner.Result, _ error) {
		completedOps++
		program.Send(taskStepMsg{
			label:     formatGitCommand(event.Args),
			completed: completedOps,
			total:     spec.TotalOps,
		})
	})

	uiErrCh := make(chan error, 1)
	go func() {
		_, err := program.Run()
		uiErrCh <- err
	}()

	workCh := make(chan taskResult[T], 1)
	go func() {
		value, err := fn(ctx, observed)
		workCh <- taskResult[T]{value: value, err: err}
		program.Send(taskDoneMsg{})
	}()

	result := <-workCh
	uiErr := <-uiErrCh
	if uiErr != nil && !errors.Is(uiErr, tea.ErrProgramKilled) && result.err == nil {
		return zero, uiErr
	}
	return result.value, result.err
}

func newActivityModel(terminal Terminal, spec TaskSpec) activityModel {
	theme := NewTheme(terminal)
	spin := spinner.New(spinner.WithSpinner(spinner.Dot))
	spin.Style = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "#0A7C86", Dark: "#6BE4EF"})

	barWidth := theme.ContentWidth() - 2
	if barWidth > 48 {
		barWidth = 48
	}
	if barWidth < 20 {
		barWidth = 20
	}

	bar := progress.New(
		progress.WithWidth(barWidth),
		progress.WithoutPercentage(),
		progress.WithGradient("#0A7C86", "#F2A541"),
	)

	return activityModel{
		theme:     theme,
		spec:      spec,
		spinner:   spin,
		progress:  bar,
		spring:    harmonica.NewSpring(harmonica.FPS(30), 7.5, 0.82),
		progressV: 0,
		target:    0.04,
		totalOps:  spec.TotalOps,
		startedAt: time.Now(),
	}
}

func (m activityModel) Init() tea.Cmd {
	return tea.Batch(m.spinner.Tick, pulseTick(), showTick())
}

func (m activityModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case taskShowMsg:
		if m.completed || m.visible {
			return m, nil
		}
		m.visible = true
		if m.target < 0.08 {
			m.target = 0.08
		}
		return m, nil
	case taskDoneMsg:
		m.completed = true
		m.target = 1
		m.progressV = 1
		if m.lastStep == "" {
			m.lastStep = "done"
		}
		if !m.visible {
			return m, tea.Quit
		}
		return m, completeTick()
	case taskClearMsg:
		m.clearing = true
		return m, quitTick()
	case taskQuitMsg:
		return m, tea.Quit
	case taskStepMsg:
		m.stepCount++
		m.lastStep = msg.label
		if msg.total > 0 {
			m.totalOps = msg.total
			m.target = clamp(float64(msg.completed)/float64(msg.total), 0, 1)
		} else {
			m.target = progressTargetForSteps(m.stepCount)
		}
		return m, nil
	case pulseMsg:
		if !m.completed && m.stepCount == 0 && time.Since(m.startedAt) > 1500*time.Millisecond && m.target < 0.10 {
			m.target = 0.12
		}
		if m.totalOps > 0 {
			m.progressV = clamp(m.target, 0, 1)
		} else {
			m.progressV, m.velocity = m.spring.Update(m.progressV, m.velocity, m.target)
			m.progressV = clamp(m.progressV, 0, 1)
		}
		return m, pulseTick()
	}

	var cmd tea.Cmd
	m.spinner, cmd = m.spinner.Update(msg)
	_, progressCmd := m.progress.Update(msg)
	return m, tea.Batch(cmd, progressCmd)
}

func (m activityModel) View() string {
	if !m.visible || m.clearing {
		return ""
	}

	lines := []string{
		lipgloss.JoinHorizontal(lipgloss.Center, m.spinner.View(), " ", m.theme.Muted("working")),
		m.progress.ViewAs(m.progressV),
	}

	meta := []string{
		m.theme.Badge(ToneInfo, stepSummary(m.stepCount, m.totalOps)),
		m.theme.Muted(humanizeDuration(time.Since(m.startedAt))),
	}
	lines = append(lines, lipgloss.JoinHorizontal(lipgloss.Center, meta[0], "  ", meta[1]))

	if m.lastStep != "" {
		lines = append(lines, m.theme.Command(m.lastStep))
	}

	return m.theme.Card(m.spec.Title, m.spec.Subtitle, lipgloss.JoinVertical(lipgloss.Left, lines...))
}

func pulseTick() tea.Cmd {
	return tea.Tick(90*time.Millisecond, func(t time.Time) tea.Msg {
		return pulseMsg(t)
	})
}

func showTick() tea.Cmd {
	return tea.Tick(160*time.Millisecond, func(time.Time) tea.Msg {
		return taskShowMsg{}
	})
}

func completeTick() tea.Cmd {
	return tea.Tick(140*time.Millisecond, func(time.Time) tea.Msg {
		return taskClearMsg{}
	})
}

func quitTick() tea.Cmd {
	return tea.Tick(20*time.Millisecond, func(time.Time) tea.Msg {
		return taskQuitMsg{}
	})
}

func quietRunner(runner gitrunner.Runner) gitrunner.Runner {
	if concrete, ok := runner.(*gitrunner.CLIRunner); ok {
		return concrete.WithStdout(nil)
	}
	return runner
}

func formatGitCommand(args []string) string {
	if len(args) == 0 {
		return "git"
	}

	switch args[0] {
	case "branch":
		if len(args) > 1 && args[1] == "--show-current" {
			return "reading current branch"
		}
	case "checkout", "switch":
		if len(args) > 1 {
			return "checking out " + cleanArg(args[len(args)-1])
		}
	case "fetch":
		return "fetching remotes"
	case "merge-base":
		if len(args) > 1 && args[1] == "--is-ancestor" {
			return "checking ancestry"
		}
		return "resolving merge base"
	case "merge":
		if len(args) > 1 && args[1] == "--ff-only" && len(args) > 2 {
			return "fast-forwarding from " + cleanArg(args[len(args)-1])
		}
		if len(args) > 1 && args[1] == "--continue" {
			return "continuing merge"
		}
		if len(args) > 1 && args[1] == "--abort" {
			return "aborting merge"
		}
		if len(args) > 1 {
			return "merging " + cleanArg(args[len(args)-1])
		}
	case "rebase":
		if len(args) > 1 && args[1] == "--continue" {
			return "continuing rebase"
		}
		if len(args) > 1 && args[1] == "--abort" {
			return "aborting rebase"
		}
		if len(args) > 1 {
			return "rebasing onto " + cleanArg(args[len(args)-1])
		}
	case "rev-parse":
		if len(args) > 1 {
			return "resolving " + cleanArg(args[len(args)-1])
		}
	case "for-each-ref":
		return "finding branch upstream"
	case "rev-list":
		if len(args) > 1 {
			switch args[1] {
			case "--left-right":
				return "measuring drift"
			case "--merges":
				return "scanning merge commits"
			case "--parents":
				return "inspecting merge parents"
			case "--reverse":
				return "building commit order"
			case "--first-parent":
				return "walking first-parent history"
			}
		}
		return "walking commit history"
	}

	parts := make([]string, 0, min(len(args), 3))
	for _, arg := range args {
		if strings.HasPrefix(arg, "-") {
			continue
		}
		parts = append(parts, cleanArg(arg))
		if len(parts) == 3 {
			break
		}
	}
	if len(parts) == 0 {
		return "git " + args[0]
	}
	return "git " + strings.Join(parts, " ")
}

func humanizeDuration(d time.Duration) string {
	if d < 500*time.Millisecond {
		return "starting"
	}
	if d < 10*time.Second {
		return fmt.Sprintf("%.1fs", d.Seconds())
	}
	d = d.Round(time.Second)
	return d.String()
}

var hexPattern = regexp.MustCompile(`^[0-9a-fA-F]+$`)

func cleanArg(arg string) string {
	arg = strings.TrimSpace(arg)
	if arg == "" {
		return arg
	}
	if strings.Contains(arg, "\t") {
		parts := strings.Split(arg, "\t")
		for i, part := range parts {
			parts[i] = cleanArg(part)
		}
		return strings.Join(parts, " ")
	}
	if strings.HasPrefix(arg, "refs/heads/") {
		return strings.TrimPrefix(arg, "refs/heads/")
	}
	if isHex(arg) && len(arg) > 8 {
		return arg[:8]
	}
	return arg
}

func isHex(s string) bool {
	return len(s) >= 7 && hexPattern.MatchString(s)
}

func stepSummary(stepCount, totalOps int) string {
	if totalOps > 0 {
		if stepCount < 0 {
			stepCount = 0
		}
		if stepCount > totalOps {
			stepCount = totalOps
		}
		return fmt.Sprintf("%d/%d ops", stepCount, totalOps)
	}
	if stepCount <= 0 {
		return "starting"
	}
	if stepCount == 1 {
		return "1 op"
	}
	return fmt.Sprintf("%d ops", stepCount)
}

func progressTargetForSteps(stepCount int) float64 {
	if stepCount <= 0 {
		return 0.06
	}
	target := 0.10 + 0.52*(1-math.Exp(-0.06*float64(stepCount)))
	if target > 0.62 {
		return 0.62
	}
	return target
}

func clamp(v, minV, maxV float64) float64 {
	if v < minV {
		return minV
	}
	if v > maxV {
		return maxV
	}
	return v
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
