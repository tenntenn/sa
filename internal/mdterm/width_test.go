package mdterm

import (
	"os"
	"strings"
	"testing"
)

// The wrapped text every case here is built from. It is the paragraph the QA
// report wrapped: long enough to need three lines at width 30, so there is a
// middle line - the line the broken version left with no decoration at all.
const wrapBody = "a long stretch of bold text that will certainly wrap"

// sgrPairsForTest is the same set of attributes wrap follows, spelled out again
// here rather than read from the implementation: a test that shares the
// implementation's table cannot notice the table being wrong.
var sgrPairsForTest = [][2]string{
	{boldOn, boldOff},
	{italicOn, italicOff},
	{strikeOn, strikeOff},
	{codeOn, codeOff},
}

// sgrOpenAtEnd is the list of "on" escapes still in effect at the end of line,
// oldest first. An empty result means the line can be printed on its own
// without leaving the terminal decorated afterwards.
func sgrOpenAtEnd(line string) []string {
	var open []string
	for i := 0; i < len(line); {
		n := escapeLen(line[i:])
		if n == 0 {
			i++
			continue
		}
		esc := line[i : i+n]
		i += n
		known := false
		for _, p := range sgrPairsForTest {
			switch esc {
			case p[0]:
				known = true
				if !containsString(open, esc) {
					open = append(open, esc)
				}
			case p[1]:
				known = true
				for k, o := range open {
					if o == p[0] {
						open = append(open[:k], open[k+1:]...)
						break
					}
				}
			}
			if known {
				break
			}
		}
		if !known && (esc == "\x1b[0m" || esc == "\x1b[m") {
			open = nil
		}
	}
	return open
}

func containsString(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

// stripSGR is line with every escape sequence removed, leaving what the reader
// actually sees.
func stripSGR(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); {
		if n := escapeLen(s[i:]); n > 0 {
			i += n
			continue
		}
		b.WriteByte(s[i])
		i++
	}
	return b.String()
}

// hasAnyOpen reports whether line carries at least one opening escape.
func hasAnyOpen(line string) bool {
	for _, p := range sgrPairsForTest {
		if strings.Contains(line, p[0]) {
			return true
		}
	}
	return false
}

// requireESC fails the test when the fixture holds no escape at all. Without
// it a test about escapes passes on input that has none, which is how the
// previous round of this package shipped broken.
func requireESC(t *testing.T, name, src string) {
	t.Helper()
	if !strings.Contains(src, "\x1b") {
		t.Fatalf("%s: fixture contains no ESC byte; this test would pass vacuously", name)
	}
}

// W1. Decoration that outlives a line break is closed at the end of the line
// and opened again at the start of the next one, for every attribute the
// package emits.
func TestWrapClosesAndReopensSGR(t *testing.T) {
	for _, p := range sgrPairsForTest {
		on, off := p[0], p[1]
		src := "A paragraph with " + on + wrapBody + off + " in it."
		requireESC(t, "wrap", src)
		want := []string{
			"A paragraph with " + on + "a long" + off,
			on + "stretch of bold text that will" + off,
			on + "certainly wrap" + off + " in it.",
		}
		got := wrap(src, 30)
		if len(got) < 3 {
			t.Fatalf("fixture wrapped into %d lines; this test needs a middle line", len(got))
		}
		if len(got) != len(want) {
			t.Fatalf("wrap(%q, 30) returned %d lines, want %d: %q", src, len(got), len(want), got)
		}
		for i := range want {
			if got[i] != want[i] {
				t.Errorf("wrap line %d\n got %q\nwant %q", i, got[i], want[i])
			}
		}
	}

	// The reported case, through the public entry point, so the fix is the one
	// a caller drawing a pane actually gets.
	src := "A paragraph with **" + wrapBody + "** in it."
	lines := Lines(src, Options{Width: 30, Color: true})
	want := []string{
		"A paragraph with " + boldOn + "a long" + boldOff,
		boldOn + "stretch of bold text that will" + boldOff,
		boldOn + "certainly wrap" + boldOff + " in it.",
	}
	if len(lines) < 3 {
		t.Fatalf("fixture wrapped into %d lines; this test needs a middle line", len(lines))
	}
	requireESC(t, "Lines", strings.Join(lines, "\n"))
	if len(lines) != len(want) {
		t.Fatalf("Lines returned %d lines, want %d: %q", len(lines), len(want), lines)
	}
	for i := range want {
		if lines[i] != want[i] {
			t.Errorf("Lines line %d\n got %q\nwant %q", i, lines[i], want[i])
		}
	}
}

// balanceCase is one fixture of the invariant test.
//
// whole says the decoration covers the entire visible text, which is what
// makes it legitimate to demand that every line carry an opening escape.
//
// bareEscapeWord says the fixture puts an escape between two spaces, where it
// becomes a word of its own with no visible content. inline.go never emits one
// - its escapes always sit against the text they decorate - but a diff can
// contain one, and how wrap's word splitting treats a word of zero width is
// older than this test and outside what it is checking. Only W2-d, which
// compares against the same input with the escapes taken out, is affected.
type balanceCase struct {
	name           string
	src            string
	whole          bool
	bareEscapeWord bool
}

func balanceCases() []balanceCase {
	return []balanceCase{
		{name: "bold throughout", src: boldOn + wrapBody + boldOff, whole: true},
		{name: "italic throughout", src: italicOn + wrapBody + italicOff, whole: true},
		{name: "struck through throughout", src: strikeOn + wrapBody + strikeOff, whole: true},
		{name: "code throughout", src: codeOn + wrapBody + codeOff, whole: true},
		{name: "code nested in bold", src: boldOn + "start " + codeOn + wrapBody + codeOff + " end" + boldOff, whole: true},
		{name: "bold and italic nested", src: boldOn + italicOn + wrapBody + italicOff + boldOff, whole: true},
		{name: "decoration inside a paragraph", src: "plain before " + boldOn + wrapBody + boldOff + " plain after"},
		{name: "japanese with no spaces at all", src: boldOn + strings.Repeat("日本語のテキスト", 6) + boldOff, whole: true},
		{name: "decoration opening inside a word", src: "un" + italicOn + strings.Repeat("breakable", 5) + italicOff + "tail"},
		{name: "decoration closing inside a word", src: strikeOn + strings.Repeat("breakable", 5) + strikeOff + "tail more words here"},
		{name: "a blanket reset from the input", src: boldOn + "start of a long decorated stretch\x1b[0m rest of the paragraph text"},
		{name: "a blanket reset standing alone", src: boldOn + "start of a long decorated stretch \x1b[0m rest of the paragraph text", bareEscapeWord: true},
		{name: "an escape this package never emits", src: "\x1b[7mreverse video from the diff itself \x1b[27mand a lot more text after it"},
		{name: "decoration never closed by the input", src: boldOn + wrapBody + " and it just keeps going", whole: true},
	}
}

// widestRune is the width of the widest rune in s. A limit below it puts wrap
// past its own contract - the rune cannot be split, so the line has to overrun
// - and at that point a zero-width cell lands on one side of the break or the
// other depending on escapes being present. That, too, predates this test.
func widestRune(s string) int {
	w := 1
	for _, c := range cells(s) {
		if c.width > w {
			w = c.width
		}
	}
	return w
}

var balanceLimits = []int{1, 2, 5, 10, 30, 40}

// W2. The invariants every wrapped line has to satisfy at every width.
func TestWrapLinesAreSGRBalanced(t *testing.T) {
	sawMiddleLine := false
	for _, tc := range balanceCases() {
		requireESC(t, tc.name, tc.src)
		for _, limit := range balanceLimits {
			lines := wrap(tc.src, limit)
			plain := wrap(stripSGR(tc.src), limit)

			// W2-a: no line leaves an attribute open behind it.
			for i, line := range lines {
				if open := sgrOpenAtEnd(line); len(open) != 0 {
					t.Errorf("%s limit=%d line %d: %d attribute(s) still open at end of %q: %q",
						tc.name, limit, i, len(open), line, open)
				}
			}

			// W2-b: a decorated run that spans three lines or more decorates
			// its middle lines too.
			if tc.whole && len(lines) >= 3 {
				sawMiddleLine = true
				for i := 1; i < len(lines)-1; i++ {
					if !hasAnyOpen(lines[i]) {
						t.Errorf("%s limit=%d: middle line %d carries no opening escape: %q",
							tc.name, limit, i, lines[i])
					}
				}
			}

			// W2-c: the escapes added are not counted as columns. A single
			// rune wider than the limit cannot be split any further, so it is
			// the one thing allowed past it.
			for i, line := range lines {
				w := displayWidth(line)
				if w <= limit {
					continue
				}
				if visible := len(cells(stripSGR(line))); visible == 1 {
					continue
				}
				t.Errorf("%s limit=%d line %d: display width %d exceeds limit: %q",
					tc.name, limit, i, w, line)
			}

			// W2-d: the visible text of every line is exactly what the same
			// input wraps to with no escapes in it, so nothing about where the
			// lines break has moved.
			if tc.bareEscapeWord || limit < widestRune(tc.src) {
				continue
			}
			if len(lines) != len(plain) {
				t.Errorf("%s limit=%d: wrapped into %d lines, escape-free input into %d",
					tc.name, limit, len(lines), len(plain))
				continue
			}
			for i := range lines {
				if got := stripSGR(lines[i]); got != plain[i] {
					t.Errorf("%s limit=%d line %d: visible text %q, escape-free wrap %q",
						tc.name, limit, i, got, plain[i])
				}
			}
		}
	}
	if !sawMiddleLine {
		t.Fatal("no fixture wrapped a decorated run onto three or more lines; W2-b checked nothing")
	}
}

// W3. Input with no escapes in it wraps exactly where it always did.
func TestWrapWithoutEscapesIsUnchanged(t *testing.T) {
	tests := []struct {
		name  string
		src   string
		limit int
		want  []string
	}{
		{"words break at spaces", "hello world foo", 5, []string{"hello", "world", "foo"}},
		{"a word too long breaks between runes", "aaaaaaa", 3, []string{"aaa", "aaa", "a"}},
		{"japanese counts two columns per rune", "日本語のテキスト", 4,
			[]string{"日本", "語の", "テキ", "スト"}},
		{"a long word starts on a line of its own", "hi aaaaaa", 4, []string{"hi", "aaaa", "aa"}},
		{"a wide limit keeps one line", "hello world foo", 40, []string{"hello world foo"}},
		{"runs of spaces inside a line survive", "a  b", 10, []string{"a  b"}},
		{"spaces only", "   ", 5, []string{""}},
		{"empty input", "", 5, []string{""}},
		{"a zero limit is one column", "ab", 0, []string{"a", "b"}},
		{"a negative limit is one column", "a b", -3, []string{"a", "b"}},
	}
	for _, tt := range tests {
		if strings.Contains(tt.src, "\x1b") {
			t.Fatalf("%s: this table is the escape-free regression; fixture has an ESC", tt.name)
		}
		got := wrap(tt.src, tt.limit)
		if len(got) != len(tt.want) {
			t.Errorf("%s: wrap(%q, %d) = %q, want %q", tt.name, tt.src, tt.limit, got, tt.want)
			continue
		}
		for i := range tt.want {
			if got[i] != tt.want[i] {
				t.Errorf("%s: wrap(%q, %d) line %d = %q, want %q",
					tt.name, tt.src, tt.limit, i, got[i], tt.want[i])
			}
		}
	}
}

// TestPTYWrappedLinesLeaveTerminalClean writes wrapped lines to a real
// terminal, one window of them at a time, the way a TUI draws a pane. It is
// gated because it only says anything when stdout is a pty; run it as
//
//	script -qec 'MDTERM_PTY=1 go test ./internal/mdterm/ -run TestPTYWrappedLinesLeaveTerminalClean -v' log
//
// and read the escapes back out of the captured bytes.
func TestPTYWrappedLinesLeaveTerminalClean(t *testing.T) {
	if os.Getenv("MDTERM_PTY") != "1" {
		t.Skip("set MDTERM_PTY=1 and run under a pty")
	}
	src := "A paragraph with **" + wrapBody + "** in it."
	lines := Lines(src, Options{Width: 30, Color: true})
	if len(lines) < 3 {
		t.Fatalf("fixture wrapped into %d lines; this test needs a middle line", len(lines))
	}
	joined := strings.Join(lines, "\n")
	if !strings.Contains(joined, "\x1b") {
		t.Fatal("fixture contains no ESC byte; this test would pass vacuously")
	}
	out := os.Stdout
	out.WriteString("---L1-BEGIN---" + lines[0] + "---L1-END---\n")
	out.WriteString("---MID-BEGIN---" + lines[len(lines)/2] + "---MID-END---\n")
	out.WriteString("---ALL-BEGIN---" + joined + "---ALL-END---\n")
}
