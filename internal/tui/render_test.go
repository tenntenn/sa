package tui

import (
	"strings"
	"testing"

	"github.com/tenntenn/sbnn/internal/model"
)

func TestFileListLinesMarksCursor(t *testing.T) {
	s := newTestState(t, twoFiles)
	const height = 5

	lines := FileListLines(s, 30, height)
	if len(lines) != height {
		t.Fatalf("got %d lines, want %d", len(lines), height)
	}
	if !strings.HasPrefix(lines[0], "> ") {
		t.Errorf("selected line = %q, want it to start with %q", lines[0], "> ")
	}
	if !strings.HasPrefix(lines[1], "  ") || strings.HasPrefix(lines[1], "> ") {
		t.Errorf("unselected line = %q, want it to start with two spaces", lines[1])
	}
	if !strings.Contains(lines[0], "foo.txt") {
		t.Errorf("line = %q, want the path in it", lines[0])
	}
	if !strings.Contains(lines[0], "+1") || !strings.Contains(lines[0], "-1") {
		t.Errorf("line = %q, want the counts in it", lines[0])
	}
	// The files are two and the pane is five lines: the rest is blank.
	for i, line := range lines[2:] {
		if line != "" {
			t.Errorf("padding line %d = %q, want it empty", i+2, line)
		}
	}

	s.Key("j")
	lines = FileListLines(s, 30, height)
	if !strings.HasPrefix(lines[1], "> ") {
		t.Errorf("after moving, line 1 = %q, want it selected", lines[1])
	}
	if strings.HasPrefix(lines[0], "> ") {
		t.Errorf("after moving, line 0 = %q, want it unselected", lines[0])
	}

	// A list longer than the pane scrolls to keep the cursor on screen.
	long := FileListLines(s, 30, 1)
	if len(long) != 1 {
		t.Fatalf("got %d lines, want 1", len(long))
	}
	if !strings.HasPrefix(long[0], "> ") {
		t.Errorf("one line pane = %q, want the selected file in it", long[0])
	}
}

func TestDiffLinesKeepsMarkers(t *testing.T) {
	s := newTestState(t, twoFiles)
	lines := DiffLines(s.Files[0], 0, 60, 10)
	if len(lines) != 10 {
		t.Fatalf("got %d lines, want 10", len(lines))
	}
	if !strings.HasPrefix(strings.TrimLeft(lines[0], " "), "@@") {
		t.Fatalf("first line = %q, want the hunk header", lines[0])
	}

	// The marker sits right in front of the content, behind the number
	// gutter, so "+new" and "-old" read as they do in the diff itself.
	want := []struct {
		marker  byte
		content string
	}{
		{' ', "keep"},
		{'-', "old"},
		{'+', "new"},
		{' ', "tail"},
	}
	for i, w := range want {
		line := lines[i+1]
		if len(line) <= gutterWidth+1 {
			t.Fatalf("line %d = %q, too short to carry a marker", i+1, line)
		}
		if got := line[gutterWidth+1]; got != w.marker {
			t.Errorf("line %d = %q, marker %q, want %q", i+1, line, got, w.marker)
		}
		if got, want := line[gutterWidth+1:], string(w.marker)+w.content; got != want {
			t.Errorf("line %d = %q, want %q", i+1, got, want)
		}
	}

	if got := DiffLines(nil, 0, 60, 10); len(got) != 0 {
		t.Errorf("DiffLines(nil) = %q, want no lines", got)
	}
	file := &model.File{NewPath: "logo.png", IsBinary: true}
	binary := DiffLines(file, 0, 60, 10)
	if len(binary) != 1 || binary[0] != "binary file" {
		t.Errorf("binary file = %q, want one %q line", binary, "binary file")
	}
	// The pane is one line shorter than the screen even so: the frame fills
	// what the diff does not.
	if got := Frame(NewState([]*model.File{file}), 60, 10); len(got) != 10 {
		t.Errorf("Frame with a binary file returned %d lines, want 10", len(got))
	}
}

func TestFrameHasNoANSI(t *testing.T) {
	s := newTestState(t, twoFiles)
	for _, size := range []struct{ width, height int }{
		{100, 30},
		{60, 40},
		{20, 10}, // too narrow for the file list
		{100, 2}, // nothing but the header and the footer
	} {
		lines := Frame(s, size.width, size.height)
		if len(lines) != size.height {
			t.Errorf("Frame(%d, %d) returned %d lines, want %d",
				size.width, size.height, len(lines), size.height)
		}
		for i, line := range lines {
			if strings.Contains(line, "\x1b") {
				t.Errorf("Frame(%d, %d) line %d = %q, want no escape in it",
					size.width, size.height, i, line)
			}
			if n := len([]rune(line)); n > size.width {
				t.Errorf("Frame(%d, %d) line %d is %d cells wide: %q",
					size.width, size.height, i, n, line)
			}
		}
		if !strings.Contains(lines[0], "sbnn tui") {
			t.Errorf("header = %q, want %q in it", lines[0], "sbnn tui")
		}
		if !strings.Contains(lines[len(lines)-1], "q: quit") {
			t.Errorf("footer = %q, want %q in it", lines[len(lines)-1], "q: quit")
		}
	}
}

// A diff carrying an escape sequence must not repaint the terminal of whoever
// reads it, so the frame drops it.
func TestFrameDropsEscapesFromTheDiff(t *testing.T) {
	const src = "--- a/paint.txt\n+++ b/paint.txt\n@@ -1,1 +1,1 @@\n-plain\n+\x1b[31mred\x1b[0m\n"
	s := newTestState(t, src)
	for i, line := range Frame(s, 100, 30) {
		if strings.Contains(line, "\x1b") {
			t.Errorf("line %d = %q, want the escape gone", i, line)
		}
	}
}
