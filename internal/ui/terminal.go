package ui

import (
	"io"
	"os"
	"strings"

	xterm "github.com/charmbracelet/x/term"
)

const defaultWidth = 96

type Terminal struct {
	in          io.Reader
	out         io.Writer
	width       int
	styled      bool
	interactive bool
}

func NewTerminal(in io.Reader, out io.Writer) Terminal {
	width := defaultWidth
	outFile, outOK := out.(*os.File)
	styled := false
	if outOK && xterm.IsTerminal(outFile.Fd()) && supportsANSI() {
		styled = true
		if measuredWidth, _, err := xterm.GetSize(outFile.Fd()); err == nil && measuredWidth > 0 {
			width = measuredWidth
		}
	}

	inFile, inOK := in.(*os.File)
	interactive := styled && inOK && xterm.IsTerminal(inFile.Fd())

	return Terminal{
		in:          in,
		out:         out,
		width:       width,
		styled:      styled,
		interactive: interactive,
	}
}

func (t Terminal) Width() int {
	if t.width > 0 {
		return t.width
	}
	return defaultWidth
}

func (t Terminal) Out() io.Writer {
	return t.out
}

func (t Terminal) In() io.Reader {
	return t.in
}

func (t Terminal) Styled() bool {
	return t.styled
}

func (t Terminal) Interactive() bool {
	return t.interactive
}

func supportsANSI() bool {
	if strings.EqualFold(os.Getenv("TERM"), "dumb") {
		return false
	}
	if os.Getenv("NO_COLOR") != "" {
		return false
	}
	return true
}
