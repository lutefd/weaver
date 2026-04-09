package ui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

type Tone string

const (
	ToneInfo    Tone = "info"
	ToneSuccess Tone = "success"
	ToneWarn    Tone = "warn"
	ToneDanger  Tone = "danger"
	ToneMuted   Tone = "muted"
)

type KeyValue struct {
	Label string
	Value string
}

type Theme struct {
	width     int
	card      lipgloss.Style
	title     lipgloss.Style
	subtitle  lipgloss.Style
	label     lipgloss.Style
	value     lipgloss.Style
	muted     lipgloss.Style
	branch    lipgloss.Style
	base      lipgloss.Style
	connector lipgloss.Style
	command   lipgloss.Style
}

const (
	cardBorderWidth   = 2
	cardHorizontalPad = 2
)

func NewTheme(term Terminal) Theme {
	width := term.Width()
	if width > 108 {
		width = 108
	}
	if width < 56 {
		width = 56
	}

	border := lipgloss.AdaptiveColor{Light: "#C7D0DD", Dark: "#3B495F"}
	title := lipgloss.AdaptiveColor{Light: "#123047", Dark: "#E6EEF7"}
	subtitle := lipgloss.AdaptiveColor{Light: "#516074", Dark: "#91A0B6"}
	label := lipgloss.AdaptiveColor{Light: "#0A7C86", Dark: "#5FD7E2"}
	text := lipgloss.AdaptiveColor{Light: "#243142", Dark: "#D5DFEC"}
	muted := lipgloss.AdaptiveColor{Light: "#6A7A90", Dark: "#8A97AA"}
	command := lipgloss.AdaptiveColor{Light: "#5B2F14", Dark: "#FFD3B3"}
	base := lipgloss.AdaptiveColor{Light: "#0A7C86", Dark: "#6BE4EF"}
	branch := lipgloss.AdaptiveColor{Light: "#233142", Dark: "#EAF2FA"}

	return Theme{
		width: width,
		card: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(border).
			Padding(0, 1).
			Width(width),
		title: lipgloss.NewStyle().
			Bold(true).
			Foreground(title),
		subtitle: lipgloss.NewStyle().
			Foreground(subtitle),
		label: lipgloss.NewStyle().
			Bold(true).
			Foreground(label),
		value: lipgloss.NewStyle().
			Foreground(text),
		muted: lipgloss.NewStyle().
			Foreground(muted),
		branch: lipgloss.NewStyle().
			Bold(true).
			Foreground(branch),
		base: lipgloss.NewStyle().
			Bold(true).
			Foreground(base),
		connector: lipgloss.NewStyle().
			Foreground(muted),
		command: lipgloss.NewStyle().
			Foreground(command),
	}
}

func (t Theme) Badge(tone Tone, text string) string {
	background := lipgloss.AdaptiveColor{Light: "#DCE4EE", Dark: "#394A5E"}
	foreground := lipgloss.AdaptiveColor{Light: "#243142", Dark: "#E6EEF7"}

	switch tone {
	case ToneInfo:
		background = lipgloss.AdaptiveColor{Light: "#D2F2F4", Dark: "#0B4E55"}
		foreground = lipgloss.AdaptiveColor{Light: "#0B4E55", Dark: "#D7FCFF"}
	case ToneSuccess:
		background = lipgloss.AdaptiveColor{Light: "#D9F4E2", Dark: "#144D32"}
		foreground = lipgloss.AdaptiveColor{Light: "#0F4A2D", Dark: "#DEFFE9"}
	case ToneWarn:
		background = lipgloss.AdaptiveColor{Light: "#FFF0C7", Dark: "#5C4306"}
		foreground = lipgloss.AdaptiveColor{Light: "#6B4B00", Dark: "#FFF6DB"}
	case ToneDanger:
		background = lipgloss.AdaptiveColor{Light: "#FFD9D2", Dark: "#6D2618"}
		foreground = lipgloss.AdaptiveColor{Light: "#6D2618", Dark: "#FFE8E1"}
	case ToneMuted:
		background = lipgloss.AdaptiveColor{Light: "#E7EDF4", Dark: "#304054"}
		foreground = lipgloss.AdaptiveColor{Light: "#55657B", Dark: "#C7D2E1"}
	}

	return lipgloss.NewStyle().
		Bold(true).
		Padding(0, 1).
		Background(background).
		Foreground(foreground).
		Render(strings.ToUpper(text))
}

func (t Theme) Card(title, subtitle, body string) string {
	parts := []string{t.title.Render(title)}
	if subtitle != "" {
		parts = append(parts, t.subtitle.Render(subtitle))
	}
	if body != "" {
		parts = append(parts, body)
	}
	return t.card.Render(lipgloss.JoinVertical(lipgloss.Left, parts...))
}

func (t Theme) ContentWidth() int {
	width := t.width - cardBorderWidth - cardHorizontalPad
	if width < 20 {
		return 20
	}
	return width
}

func (t Theme) KeyValues(items []KeyValue) string {
	if len(items) == 0 {
		return ""
	}

	labelWidth := 0
	for _, item := range items {
		if w := lipgloss.Width(item.Label); w > labelWidth {
			labelWidth = w
		}
	}

	lines := make([]string, 0, len(items))
	for _, item := range items {
		line := lipgloss.JoinHorizontal(
			lipgloss.Top,
			t.label.Width(labelWidth).Render(item.Label),
			"  ",
			t.value.Render(item.Value),
		)
		lines = append(lines, line)
	}
	return lipgloss.JoinVertical(lipgloss.Left, lines...)
}

func (t Theme) List(items []string) string {
	lines := make([]string, 0, len(items))
	for _, item := range items {
		lines = append(lines, lipgloss.JoinHorizontal(lipgloss.Top, t.connector.Render("•"), " ", t.value.Render(item)))
	}
	return lipgloss.JoinVertical(lipgloss.Left, lines...)
}

func (t Theme) Bullet(tone Tone, title, detail string) string {
	line := lipgloss.JoinHorizontal(lipgloss.Top, t.Badge(tone, string(tone)), " ", t.value.Render(title))
	if detail == "" {
		return line
	}
	return lipgloss.JoinVertical(
		lipgloss.Left,
		line,
		lipgloss.JoinHorizontal(lipgloss.Top, "   ", t.muted.Render("fix"), "  ", t.value.Render(detail)),
	)
}

func (t Theme) Command(text string) string {
	return t.command.MaxWidth(t.ContentWidth()).Render(text)
}

func (t Theme) Branch(name string) string {
	return t.branch.Render(name)
}

func (t Theme) BaseBranch(name string) string {
	return t.base.Render(name)
}

func (t Theme) Connector(text string) string {
	return t.connector.Render(text)
}

func (t Theme) Muted(text string) string {
	return t.muted.Render(text)
}

func (t Theme) Value(text string) string {
	return t.value.Render(text)
}
