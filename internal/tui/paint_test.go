package tui

import (
	"strings"
	"testing"
)

// knownSGR is every escape sequence the palette is allowed to write. Anything
// else in a painted frame came out of the diff, which is exactly what must
// never happen.
var knownSGR = []string{sgrReset, sgrBold, sgrFaint, sgrReverse, sgrGreen, sgrRed}

// stripSGR drops the palette's own escapes, so that a painted line can be
// compared with the plain one it was made from.
func stripSGR(line string) string {
	for _, sgr := range knownSGR {
		line = strings.ReplaceAll(line, sgr, "")
	}
	return line
}

func TestPaintAddsColourAndKeepsText(t *testing.T) {
	const width, height = 80, 14
	s := newTestState(t, twoFiles)
	s.SetSize(width, height)
	p := newPalette(true)

	frame := Frame(s, width, height)
	painted := p.Paint(s, width, height)
	if len(painted) != len(frame) {
		t.Fatalf("Paint returned %d lines, Frame returned %d", len(painted), len(frame))
	}

	// Colour is put around the text, never through it: every line comes back
	// with the same characters in the same order.
	for i, line := range frame {
		if got := stripSGR(painted[i]); got != line {
			t.Errorf("painted line %d = %q, which is %q without colour, want %q",
				i, painted[i], got, line)
		}
	}

	if !strings.HasPrefix(painted[0], sgrBold) {
		t.Errorf("header = %q, want it bold", painted[0])
	}
	if last := painted[len(painted)-1]; !strings.HasPrefix(last, sgrFaint) {
		t.Errorf("footer = %q, want it faint", last)
	}

	body := strings.Join(painted[1:len(painted)-1], "\n")
	for _, want := range []struct {
		sgr, why string
	}{
		{sgrReverse, "the selected file"},
		{sgrGreen, "an added line"},
		{sgrRed, "a deleted line"},
	} {
		if !strings.Contains(body, want.sgr) {
			t.Errorf("the body carries no %q: %s is not coloured", want.sgr, want.why)
		}
	}

	// Reverse marks the selected file and nothing else, so it belongs to the
	// row the cursor is on.
	for i, line := range painted {
		if strings.Contains(line, sgrReverse) != strings.Contains(stripSGR(line), "> ") {
			t.Errorf("line %d = %q: reverse and the cursor mark disagree", i, line)
		}
	}

	// Every escape written is one of the palette's own.
	for i, line := range painted {
		if strings.Contains(stripSGR(line), "\x1b") {
			t.Errorf("painted line %d = %q carries an escape the palette did not write",
				i, painted[i])
		}
	}
}

// The colour is added last and can be left off entirely: without it a painted
// frame is the frame, byte for byte.
func TestPaintWithoutColourIsThePlainFrame(t *testing.T) {
	for _, src := range []string{twoFiles, wideFiles, escapedFiles} {
		s := newTestState(t, src)
		for _, size := range frameSizes {
			frame := Frame(s, size.width, size.height)
			painted := newPalette(false).Paint(s, size.width, size.height)
			if len(painted) != len(frame) {
				t.Fatalf("Paint returned %d lines, Frame returned %d", len(painted), len(frame))
			}
			for i := range frame {
				if painted[i] != frame[i] {
					t.Errorf("uncoloured line %d = %q, want %q", i, painted[i], frame[i])
				}
			}
		}
	}
}

// A diff may carry escape sequences of its own. Frame drops them; the palette
// must not be a way to smuggle them back in.
func TestPaintDoesNotLetTheDiffWriteEscapes(t *testing.T) {
	s := newTestState(t, escapedFiles)
	if !strings.Contains(escapedFiles, "\x1b") {
		t.Fatalf("the fixture carries no escape: there is nothing to smuggle")
	}
	for _, size := range frameSizes {
		for i, line := range newPalette(true).Paint(s, size.width, size.height) {
			if rest := stripSGR(line); strings.Contains(rest, "\x1b") {
				t.Errorf("Paint(%d, %d) line %d = %q carries %q from the diff",
					size.width, size.height, i, line, rest)
			}
		}
	}
}

// Colour is put on whole columns, so a wide character must not be cut in half
// by the escape that colours the column next to it.
func TestPaintSplitsPanesOnCells(t *testing.T) {
	const width, height = 100, 20
	s := newTestState(t, wideFiles)
	listWidth, _ := paneWidths(width)
	for i, line := range newPalette(true).Paint(s, width, height) {
		plain := stripSGR(line)
		if n := cellWidth(plain); n > width {
			t.Errorf("painted line %d is %d cells wide, want at most %d: %q", i, n, width, plain)
		}
		at := strings.Index(plain, separator)
		if at < 0 {
			continue
		}
		if col := cellWidth(plain[:at]); col != listWidth {
			t.Errorf("painted line %d puts the rule in column %d, want %d: %q",
				i, col, listWidth, plain)
		}
		// The rule itself is never coloured: it belongs to neither pane.
		if !strings.Contains(line, sgrReset+separator) && !strings.Contains(line, " "+separator[1:]) {
			t.Errorf("painted line %d = %q, want the rule outside both colours", i, line)
		}
	}
}

func TestColourEnabled(t *testing.T) {
	for _, tt := range []struct {
		name, noColor, term string
		want                bool
	}{
		{name: "a terminal", term: "xterm-256color", want: true},
		{name: "a plain terminal", term: "linux", want: true},
		{name: "NO_COLOR is honoured", noColor: "1", term: "xterm-256color", want: false},
		{name: "NO_COLOR of any value", noColor: "0", term: "xterm-256color", want: false},
		{name: "no TERM at all", term: "", want: false},
		{name: "a dumb terminal", term: "dumb", want: false},
		{name: "a dumb terminal with a suffix", term: "dumb-something", want: false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("NO_COLOR", tt.noColor)
			t.Setenv("TERM", tt.term)
			if got := colourEnabled(); got != tt.want {
				t.Errorf("colourEnabled() = %v with NO_COLOR=%q TERM=%q, want %v",
					got, tt.noColor, tt.term, tt.want)
			}
		})
	}
}
