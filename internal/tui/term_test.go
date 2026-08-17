package tui

import (
	"strings"
	"testing"
)

// A frame is written once, from the home position, one row per line. The home
// sequence marks where a frame begins, which is how anything reading the
// terminal - a person or a test - can tell one frame from the next.
func TestPaintFrameWritesOneFrame(t *testing.T) {
	const width, height = 80, 12
	s := newTestState(t, twoFiles)
	s.SetSize(width, height)

	var b strings.Builder
	if err := paintFrame(&b, s, newPalette(true)); err != nil {
		t.Fatalf("paintFrame: %v", err)
	}
	out := b.String()

	if n := strings.Count(out, cursorHome); n != 1 {
		t.Errorf("the frame carries %d home sequences, want exactly 1", n)
	}
	if !strings.HasPrefix(out, cursorHome) {
		t.Errorf("the frame does not start at home: %q", out[:min(len(out), 16)])
	}
	if n := strings.Count(out, clearLine); n != height {
		t.Errorf("%d rows were wiped to their end, want %d", n, height)
	}
	// Raw mode does not turn a newline into a carriage return, so every line
	// break carries its own. A bare newline would draw a staircase.
	if n := strings.Count(out, "\n"); n != height-1 {
		t.Errorf("the frame has %d line breaks, want %d", n, height-1)
	}
	if n := strings.Count(out, "\r\n"); n != height-1 {
		t.Errorf("%d of the line breaks carry a carriage return, want %d", n, height-1)
	}

	// The rows are the frame's rows, and nothing has been added between them.
	rows := strings.Split(strings.TrimPrefix(out, cursorHome), "\r\n")
	if len(rows) != height {
		t.Fatalf("the frame has %d rows, want %d", len(rows), height)
	}
	frame := Frame(s, width, height)
	for i, row := range rows {
		got := stripSGR(strings.TrimSuffix(row, clearLine))
		if got != frame[i] {
			t.Errorf("row %d = %q, want %q", i, got, frame[i])
		}
	}
}

// Without colour the frame is plain text with the drawing sequences around it,
// which is what a terminal that says it cannot colour should get.
func TestPaintFrameWithoutColour(t *testing.T) {
	const width, height = 60, 10
	s := newTestState(t, wideFiles)
	s.SetSize(width, height)

	var b strings.Builder
	if err := paintFrame(&b, s, newPalette(false)); err != nil {
		t.Fatalf("paintFrame: %v", err)
	}
	out := strings.TrimPrefix(b.String(), cursorHome)
	for i, row := range strings.Split(out, "\r\n") {
		row = strings.TrimSuffix(row, clearLine)
		if strings.Contains(row, "\x1b") {
			t.Errorf("row %d = %q, want no escape in it", i, row)
		}
		if n := cellWidth(row); n > width {
			t.Errorf("row %d is %d cells wide, want at most %d: %q", i, n, width, row)
		}
	}
}
