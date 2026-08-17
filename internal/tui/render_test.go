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

// frameSizes are the shapes a frame gets asked for: wide, tall, too narrow for
// two panes, and short enough that there is nothing but a header and a footer.
var frameSizes = []struct{ width, height int }{
	{100, 30},
	{60, 40},
	{20, 10}, // too narrow for the file list
	{100, 2}, // nothing but the header and the footer
}

func TestFrameHasNoANSI(t *testing.T) {
	// The diffs carry escapes and wide characters, because a check for
	// escapes over text that has none checks nothing at all.
	for _, src := range []struct {
		name, diff string
	}{
		{"ascii", twoFiles},
		{"wide", wideFiles},
		{"escaped", escapedFiles},
	} {
		t.Run(src.name, func(t *testing.T) {
			// The fixture has to carry what the frame is meant to strip.
			if src.name == "escaped" && !strings.Contains(src.diff, "\x1b") {
				t.Fatalf("the fixture carries no escape: there is nothing to drop")
			}
			s := newTestState(t, src.diff)
			for _, size := range frameSizes {
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
					if n := cellWidth(line); n > size.width {
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
		})
	}
}

// A diff carrying an escape sequence must not repaint the terminal of whoever
// reads it, so the frame drops it.
func TestFrameDropsEscapesFromTheDiff(t *testing.T) {
	s := newTestState(t, escapedFiles)
	// The escapes have to survive the parse, or this test passes on a diff
	// that never had any.
	found := false
	for _, f := range s.Files {
		for _, h := range f.Hunks {
			for _, l := range h.Lines {
				if strings.Contains(l.Content, "\x1b") {
					found = true
				}
			}
		}
	}
	if !found {
		t.Fatalf("no parsed line carries an escape: the fixture cannot show anything")
	}
	for i, line := range Frame(s, 100, 30) {
		if strings.Contains(line, "\x1b") {
			t.Errorf("line %d = %q, want the escape gone", i, line)
		}
	}
}

// A column of a terminal is not a rune. fit promises the result is drawable in
// the width it was given, counted the way the terminal counts.
func TestFitCountsCells(t *testing.T) {
	for _, tt := range []struct {
		name, text string
		width      int
		want       string
	}{
		{"short ascii is left alone", "hello", 10, "hello"},
		{"exact ascii is left alone", "hello", 5, "hello"},
		{"long ascii is cut", "hello world", 8, "hello w…"},
		{"wide text that fits is left alone", "あいう", 6, "あいう"},
		{"wide text is cut on a character", "あいうえお", 6, "あい…"},
		{"a cut that would halve a character stops short", "あいうえお", 5, "あい…"},
		{"one cell is the ellipsis alone", "あいうえお", 1, "…"},
		{"mixed text is measured, not counted", "ab あいう", 6, "ab あ…"},
		{"a tab is expanded before measuring", "\tあ", 8, "    あ"},
		{"no width draws nothing", "あいう", 0, ""},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if got := fit(tt.text, tt.width); got != tt.want {
				t.Errorf("fit(%q, %d) = %q, want %q", tt.text, tt.width, got, tt.want)
			}
		})
	}

	// Whatever the text, the result draws inside the width it was given.
	for _, text := range []string{
		"plain ascii text", "あいうえおかきくけこ", "mixed あい mixed あい",
		"えe漢a字b", "\t\tたぶ", "…", "",
	} {
		for width := 1; width <= 24; width++ {
			got := fit(text, width)
			if n := cellWidth(got); n > width {
				t.Errorf("fit(%q, %d) = %q, which is %d cells wide", text, width, got, n)
			}
		}
	}
}

// pad promises the opposite of fit's promise: exactly the width, never more
// and never less, so that what comes after it starts in the same column on
// every row.
func TestPadCountsCells(t *testing.T) {
	if got, want := pad("あい", 6), "あい  "; got != want {
		t.Errorf("pad(%q, 6) = %q, want %q", "あい", got, want)
	}
	if got, want := pad("ab", 5), "ab   "; got != want {
		t.Errorf("pad(%q, 5) = %q, want %q", "ab", got, want)
	}
	for _, text := range []string{
		"plain ascii text", "あいうえおかきくけこ", "mixed あい mixed あい",
		"えe漢a字b", "\tたぶ", "", "> docs/日本語の設計メモ.md  +1 -1  M",
	} {
		for width := 1; width <= 40; width++ {
			got := pad(text, width)
			if n := cellWidth(got); n != width {
				t.Errorf("pad(%q, %d) = %q, which is %d cells wide", text, width, got, n)
			}
		}
	}
}

func TestSplitCellsCutsOnCharacters(t *testing.T) {
	for _, tt := range []struct {
		name, text string
		cells      int
		head, tail string
	}{
		{"ascii", "hello world", 5, "hello", " world"},
		{"wide on a boundary", "あいうえ", 4, "あい", "うえ"},
		{"wide across a character", "あいうえ", 3, "あ", "いうえ"},
		{"shorter than asked for", "ab", 10, "ab", ""},
		{"nothing asked for", "あい", 0, "", "あい"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			head, tail := splitCells(tt.text, tt.cells)
			if head != tt.head || tail != tt.tail {
				t.Errorf("splitCells(%q, %d) = %q, %q, want %q, %q",
					tt.text, tt.cells, head, tail, tt.head, tt.tail)
			}
			if head+tail != tt.text {
				t.Errorf("the halves do not add up to %q: %q + %q", tt.text, head, tail)
			}
			if n := cellWidth(head); n > tt.cells {
				t.Errorf("head %q is %d cells, more than the %d asked for", head, n, tt.cells)
			}
		})
	}
}

// The panes are drawn side by side, so the rule between them has to stand in
// one column on every row. Counting runes puts it somewhere else on every row
// that holds a wide character.
func TestFrameFitsCellWidth(t *testing.T) {
	s := newTestState(t, wideFiles)
	for _, size := range frameSizes {
		lines := Frame(s, size.width, size.height)
		if len(lines) != size.height {
			t.Fatalf("Frame(%d, %d) returned %d lines, want %d",
				size.width, size.height, len(lines), size.height)
		}
		for i, line := range lines {
			if n := cellWidth(line); n > size.width {
				t.Errorf("Frame(%d, %d) line %d is %d cells wide, want at most %d: %q",
					size.width, size.height, i, n, size.width, line)
			}
		}

		listWidth, _ := paneWidths(size.width)
		if listWidth <= 0 || size.height <= 2 {
			// Too narrow for two panes, or too short to have a body: there is
			// no rule on a frame that is all header and footer.
			continue
		}
		rules := 0
		for i, line := range lines {
			at := strings.Index(line, separator)
			if at < 0 {
				continue
			}
			rules++
			if col := cellWidth(line[:at]); col != listWidth {
				t.Errorf("Frame(%d, %d) line %d puts the rule in column %d, want %d: %q",
					size.width, size.height, i, col, listWidth, line)
			}
		}
		if rules == 0 {
			t.Errorf("Frame(%d, %d) drew no rule: there is no alignment to check",
				size.width, size.height)
		}
	}
}
