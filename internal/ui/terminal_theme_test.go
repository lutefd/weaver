package ui

import (
	"bytes"
	"strings"
	"testing"
)

func TestTerminalHelpers(t *testing.T) {
	t.Parallel()

	term := Terminal{in: bytes.NewBuffer(nil), out: &bytes.Buffer{}, width: 77, styled: true, interactive: true}
	if term.Width() != 77 {
		t.Fatalf("Width() = %d", term.Width())
	}
	if term.Out() == nil || term.In() == nil {
		t.Fatal("Out()/In() returned nil")
	}
	if !term.Styled() || !term.Interactive() {
		t.Fatal("Styled()/Interactive() false, want true")
	}
}

func TestSupportsANSI(t *testing.T) {
	t.Setenv("TERM", "dumb")
	if supportsANSI() {
		t.Fatal("supportsANSI() = true for TERM=dumb")
	}
	t.Setenv("TERM", "xterm-256color")
	t.Setenv("NO_COLOR", "1")
	if supportsANSI() {
		t.Fatal("supportsANSI() = true for NO_COLOR")
	}
	t.Setenv("NO_COLOR", "")
	if !supportsANSI() {
		t.Fatal("supportsANSI() = false, want true")
	}
}

func TestThemeHelpers(t *testing.T) {
	t.Parallel()

	theme := NewTheme(Terminal{width: 80})
	if theme.ContentWidth() <= 0 {
		t.Fatalf("ContentWidth() = %d", theme.ContentWidth())
	}
	cases := []string{
		theme.Badge(ToneInfo, "info"),
		theme.Card("Title", "Subtitle", "Body"),
		theme.KeyValues([]KeyValue{{Label: "branch", Value: "feature-a"}}),
		theme.List([]string{"one", "two"}),
		theme.Bullet(ToneWarn, "title", "detail"),
		theme.Command("git status"),
		theme.Branch("feature-a"),
		theme.BaseBranch("main"),
		theme.Connector("->"),
		theme.Muted("text"),
		theme.Value("value"),
	}
	for _, got := range cases {
		if got == "" {
			t.Fatal("theme helper returned empty string")
		}
	}
	if !strings.Contains(theme.Card("Title", "Subtitle", "Body"), "Title") {
		t.Fatal("Card() missing title")
	}
}
