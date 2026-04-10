package ui

import (
	"bytes"
	"os"
	"testing"
)

func TestNewTerminalDefaults(t *testing.T) {
	t.Parallel()

	term := NewTerminal(bytes.NewBuffer(nil), bytes.NewBuffer(nil))
	if term.Width() != defaultWidth {
		t.Fatalf("Width() = %d, want %d", term.Width(), defaultWidth)
	}
	if term.Styled() {
		t.Fatal("Styled() = true, want false")
	}
	if term.Interactive() {
		t.Fatal("Interactive() = true, want false")
	}
	if term.In() == nil || term.Out() == nil {
		t.Fatal("In()/Out() should preserve streams")
	}
}

func TestTerminalWidthFallbackAndANSISupport(t *testing.T) {
	t.Parallel()

	if got := (Terminal{}).Width(); got != defaultWidth {
		t.Fatalf("Width() = %d, want %d", got, defaultWidth)
	}

	termEnv := os.Getenv("TERM")
	noColorEnv := os.Getenv("NO_COLOR")
	t.Cleanup(func() {
		if termEnv == "" {
			os.Unsetenv("TERM")
		} else {
			os.Setenv("TERM", termEnv)
		}
		if noColorEnv == "" {
			os.Unsetenv("NO_COLOR")
		} else {
			os.Setenv("NO_COLOR", noColorEnv)
		}
	})

	os.Setenv("TERM", "dumb")
	os.Unsetenv("NO_COLOR")
	if supportsANSI() {
		t.Fatal("supportsANSI() = true, want false for TERM=dumb")
	}

	os.Setenv("TERM", "xterm-256color")
	os.Setenv("NO_COLOR", "1")
	if supportsANSI() {
		t.Fatal("supportsANSI() = true, want false for NO_COLOR")
	}

	os.Setenv("TERM", "xterm-256color")
	os.Unsetenv("NO_COLOR")
	if !supportsANSI() {
		t.Fatal("supportsANSI() = false, want true")
	}
}
