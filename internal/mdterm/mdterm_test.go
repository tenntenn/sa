package mdterm

import (
	"fmt"
	"os"
	"runtime"
	"slices"
	"strings"
	"testing"
	"time"
)

// kitchenSink is one document with every construct this package knows about,
// used by the tests that make a claim about all output rather than about one
// rule.
const kitchenSink = "---\ntitle: Everything\n---\n" +
	"\n# Heading one\n" +
	"\n## Heading two\n" +
	"\n### Heading three\n" +
	"\nA paragraph with **bold**, *italic*, ~~struck~~ and `code` in it, long\n" +
	"enough that it has to be wrapped somewhere.\n" +
	"\n- a bullet with a [link](https://example.com)\n" +
	"  - nested\n" +
	"- [ ] unchecked\n" +
	"- [x] checked\n" +
	"\n1. first\n2. second\n" +
	"\n> quoted text\n>> deeper\n" +
	"\n```go\nfunc main() {\n\tprintln(\"a very long line that must not be wrapped at all\")\n}\n```\n" +
	"\n| name | n |\n| :--- | --: |\n| foo | 1 |\n| barbar | 22 |\n" +
	"\n---\n" +
	"\n![shot](a.png) and <https://example.org>\n"

// escapeSink is a document carrying the escapes a hostile diff would carry.
// kitchenSink has not one ESC byte in it, so on its own it lets every claim
// about escapes in the output pass without ever testing one; the tests that
// make such a claim read this fixture as well, and check first that it still
// holds an ESC.
//
// Each line is somewhere an escape could ride in from: prose, a heading, a
// fenced block that is copied out untouched, a list item, a table cell, a
// quote. \x1b[2J clears the screen and \x1bc resets the terminal - those two
// are the ones that destroy the review rather than colour it.
const escapeSink = "a \x1b[2J b\n" +
	"reset \x1bc here\n" +
	"\n# head \x1b[31m red\n" +
	"\ncolour \x1b[31mon\x1b[0m off\n" +
	"\ntitle \x1b]0;title\x07 set\n" +
	"\n```\n\x1b[2J\n\x1bc\n\x1b]0;title\x07\n```\n" +
	"\n- item \x1b[1m bold\n" +
	"\n> quoted \x1b[7m inverse\n" +
	"\n| a \x1b[31m | b |\n| --- | --- |\n| \x1bc | \x1b[0m |\n" +
	"\n8-bit CSI 31m and DEL \x7f and NUL \x00 end\n"

// escapeSinkMustBite fails the test that calls it when escapeSink has stopped
// containing the thing every escape test is about. A fixture that lost its ESC
// bytes would let all of them pass while checking nothing, which is exactly how
// this package shipped an unenforced invariant once already.
func escapeSinkMustBite(t *testing.T) {
	t.Helper()
	if !strings.Contains(escapeSink, "\x1b") {
		t.Fatal("fixture contains no ESC byte; this test would pass vacuously")
	}
	for _, want := range []string{"\x1b[2J", "\x1bc", "\x1b[31m", "\x1b[0m", "\x1b]0;title\x07"} {
		if !strings.Contains(escapeSink, want) {
			t.Fatalf("fixture no longer contains %q, which this test exists to catch", want)
		}
	}
}

// knownEscapes are the only sequences this package ever writes. Anything else
// in the output came from the input, which means it was not defanged.
var knownEscapes = []string{
	boldOn, boldOff, italicOn, italicOff, strikeOn, strikeOff, codeOn, codeOff,
}

// checkKnownEscapes reports every escape in out that this package did not write.
func checkKnownEscapes(t *testing.T, label, out string) {
	t.Helper()
	for i := 0; i < len(out); {
		if out[i] != 0x1b {
			i++
			continue
		}
		n := escapeLen(out[i:])
		if n == 0 {
			t.Errorf("%s: ESC that opens no known sequence at %d: %q", label, i, out[i:min(i+12, len(out))])
			return
		}
		if seq := out[i : i+n]; !slices.Contains(knownEscapes, seq) {
			t.Errorf("%s: output carries an escape this package never writes: %q", label, seq)
			return
		}
		i += n
	}
}

func check(t *testing.T, name, got, want string) {
	t.Helper()
	if got != want {
		t.Errorf("%s:\n got %q\nwant %q", name, got, want)
	}
}

// Headings. The top two levels trade their #s for a rule as wide as the text
// itself, which is why the width has to be counted in columns and not in bytes.
func TestHeadings(t *testing.T) {
	tests := []struct {
		name string
		src  string
		opts Options
		want string
	}{
		{"level one is underlined with =", "# Title", Options{}, "Title\n=====\n"},
		{"level two is underlined with -", "## Sub", Options{}, "Sub\n---\n"},
		{"level three keeps its hashes", "### Deep", Options{}, "### Deep\n"},
		{"level six keeps its hashes", "###### Six", Options{}, "###### Six\n"},
		{"the rule counts columns, not bytes", "# 日本語", Options{}, "日本語\n======\n"},
		{"a closing hash run is not text", "# Title #", Options{}, "Title\n=====\n"},
		{"seven hashes is a paragraph", "####### seven", Options{}, "####### seven\n"},
		{"a hash without a space is a paragraph", "#hashtag", Options{}, "#hashtag\n"},
		{"colour bolds the text, not the rule", "# Title", Options{Color: true}, boldOn + "Title" + boldOff + "\n=====\n"},
		{"colour on a level three heading", "### Deep", Options{Color: true}, "### " + boldOn + "Deep" + boldOff + "\n"},
		{"a heading is a block of its own", "# A\ntext", Options{}, "A\n=\n\ntext\n"},
	}
	for _, tt := range tests {
		check(t, tt.name, Render(tt.src, tt.opts), tt.want)
	}
}

// Inline markup. Without colour the markers come off, because the reader has
// nothing to gain from them; with colour they become the escapes that say the
// same thing to a terminal.
func TestInlineMarkup(t *testing.T) {
	tests := []struct {
		name string
		src  string
		opts Options
		want string
	}{
		{"bold", "**bold**", Options{}, "bold\n"},
		{"bold with underscores", "__bold__", Options{}, "bold\n"},
		{"italic", "*it*", Options{}, "it\n"},
		{"italic with underscores", "_it_", Options{}, "it\n"},
		{"struck through", "~~gone~~", Options{}, "gone\n"},
		{"code keeps its backticks in plain text", "`x = 1`", Options{}, "`x = 1`\n"},
		{"bold in colour", "**bold**", Options{Color: true}, boldOn + "bold" + boldOff + "\n"},
		{"italic in colour", "*it*", Options{Color: true}, italicOn + "it" + italicOff + "\n"},
		{"struck through in colour", "~~gone~~", Options{Color: true}, strikeOn + "gone" + strikeOff + "\n"},
		{"code loses its backticks in colour", "`x = 1`", Options{Color: true}, codeOn + "x = 1" + codeOff + "\n"},
		{"an identifier is not emphasis", "snake_case_name here", Options{}, "snake_case_name here\n"},
		{"a backslash shows the next character", `a \* b`, Options{}, "a * b\n"},
		{"one level of nesting", "**a `c` b**", Options{}, "a `c` b\n"},
		{"one level of nesting in colour", "**a `c` b**", Options{Color: true},
			boldOn + "a " + codeOn + "c" + codeOff + " b" + boldOff + "\n"},
		{"code holds no markup", "`**x**`", Options{}, "`**x**`\n"},
		{"code holds no markup in colour", "`**x**`", Options{Color: true}, codeOn + "**x**" + codeOff + "\n"},
		{"markup that never closes is text", "**oops", Options{}, "**oops\n"},
		{"a raw tag is text", "<b>tag</b>", Options{}, "<b>tag</b>\n"},
	}
	for _, tt := range tests {
		check(t, tt.name, Render(tt.src, tt.opts), tt.want)
	}
}

// Lists. The bullet is the same at every depth on purpose: the indentation
// already says how deep an item is.
func TestLists(t *testing.T) {
	tests := []struct {
		name string
		src  string
		opts Options
		want string
	}{
		{"bullets", "- one\n* two\n+ three", Options{}, "• one\n• two\n• three\n"},
		{"two spaces or one tab is one level", "- a\n  - b\n\t- c", Options{}, "• a\n  • b\n  • c\n"},
		{"numbers are kept as written", "3. three\n7. seven", Options{}, "3. three\n7. seven\n"},
		{"task boxes", "- [ ] todo\n- [x] done\n- [X] also", Options{}, "☐ todo\n☑ done\n☑ also\n"},
		{"items carry inline markup", "- **b** and `c`", Options{}, "• b and `c`\n"},
		{"a continuation line folds into the item", "- one\n  continued", Options{}, "• one continued\n"},
		{"a wrapped item lines up under its text", "- aaa bbb ccc", Options{Width: 12}, "• aaa bbb\n  ccc\n"},
		{"a wrapped numbered item hangs three columns", "1. aaa bbb ccc", Options{Width: 12}, "1. aaa bbb\n   ccc\n"},
		{"a list is one block", "text\n\n- one\n- two", Options{}, "text\n\n• one\n• two\n"},
	}
	for _, tt := range tests {
		check(t, tt.name, Render(tt.src, tt.opts), tt.want)
	}
}

// Code blocks. Only fenced blocks count, the contents are copied out byte for
// byte, and no line of them is ever folded.
func TestCodeBlocks(t *testing.T) {
	tests := []struct {
		name string
		src  string
		opts Options
		want string
	}{
		{"the contents are untouched", "```go\nfunc main() {\n\tx :=  1\n}\n```", Options{},
			"  func main() {\n  \tx :=  1\n  }\n"},
		{"tildes fence too", "~~~\nx\n~~~", Options{}, "  x\n"},
		{"an unclosed fence runs to the end", "```\nx", Options{}, "  x\n"},
		{"a blank line inside keeps the indent", "```\na\n\nb\n```", Options{}, "  a\n  \n  b\n"},
		{"a long line is not wrapped", "```\n" + strings.Repeat("x", 30) + "\n```", Options{Width: 10},
			"  " + strings.Repeat("x", 30) + "\n"},
		{"colour wraps each line inside the indent", "```\na\nb\n```", Options{Color: true},
			"  " + codeOn + "a" + codeOff + "\n  " + codeOn + "b" + codeOff + "\n"},
		{"four spaces is not a code block", "    x = 1", Options{}, "x = 1\n"},
	}
	for _, tt := range tests {
		check(t, tt.name, Render(tt.src, tt.opts), tt.want)
	}
}

// Quotes. Every line of a quote is marked, including the lines that only exist
// because the text was wrapped.
func TestQuotes(t *testing.T) {
	tests := []struct {
		name string
		src  string
		opts Options
		want string
	}{
		{"one level", "> quoted", Options{}, "│ quoted\n"},
		{"two levels", ">> deep", Options{}, "│ │ deep\n"},
		{"spaced levels count the same", "> > deep", Options{}, "│ │ deep\n"},
		{"consecutive lines are one block", "> a\n> b", Options{}, "│ a\n│ b\n"},
		{"quotes carry inline markup", "> **b**", Options{}, "│ b\n"},
		{"an empty quoted line keeps no trailing space", "> a\n>\n> b", Options{}, "│ a\n│\n│ b\n"},
		{"the prefix comes off the wrap width", "> aaa bbb ccc", Options{Width: 10}, "│ aaa bbb\n│ ccc\n"},
	}
	for _, tt := range tests {
		check(t, tt.name, Render(tt.src, tt.opts), tt.want)
	}
}

// Links. A terminal cannot hide a URL behind a word, so it shows both.
func TestLinks(t *testing.T) {
	tests := []struct {
		name string
		src  string
		opts Options
		want string
	}{
		{"a link shows its target", "[text](http://example.com)", Options{}, "text (http://example.com)\n"},
		{"an image says it is one", "![alt](img.png)", Options{}, "[image: alt] (img.png)\n"},
		{"an image with no alt text", "![](img.png)", Options{}, "[image] (img.png)\n"},
		{"an autolink is just the address", "<https://example.com>", Options{}, "https://example.com\n"},
		{"a title is dropped", `[a](u "title")`, Options{}, "a (u)\n"},
		{"a reference link is left alone", "[text][ref]", Options{}, "[text][ref]\n"},
		{"a reference definition is left alone", "[ref]: http://example.com", Options{}, "[ref]: http://example.com\n"},
		{"colour adds nothing to a link", "[text](u)", Options{Color: true}, "text (u)\n"},
	}
	for _, tt := range tests {
		check(t, tt.name, Render(tt.src, tt.opts), tt.want)
	}
}

// Tables. Column widths are measured in columns, which is the only way a table
// with Japanese in it lines up at all.
func TestTables(t *testing.T) {
	tests := []struct {
		name string
		src  string
		opts Options
		want string
	}{
		{
			"left and right alignment",
			"| name | n |\n| :--- | --: |\n| foo | 1 |\n| barbar | 22 |",
			Options{},
			"name   |  n\n" +
				"-------+---\n" +
				"foo    |  1\n" +
				"barbar | 22\n",
		},
		{
			"centre alignment",
			"| a | b |\n| :-: | --- |\n| longer | x |",
			Options{},
			"  a    | b\n" +
				"-------+--\n" +
				"longer | x\n",
		},
		{
			"wide characters take two columns",
			"| 名前 | n |\n| --- | --- |\n| あ | 1 |",
			Options{},
			"名前 | n\n" +
				"-----+--\n" +
				"あ   | 1\n",
		},
		{
			"a table is not wrapped",
			"| a | b |\n| --- | --- |\n| " + strings.Repeat("x", 30) + " | y |",
			Options{Width: 10},
			"a" + strings.Repeat(" ", 29) + " | b\n" +
				strings.Repeat("-", 30) + "-+-" + "-" + "\n" +
				strings.Repeat("x", 30) + " | y\n",
		},
		{
			"cells carry inline markup",
			"| **a** | `b` |\n| --- | --- |",
			Options{},
			"a | `b`\n" +
				"--+----\n",
		},
		{
			"a pipe is not a table without a delimiter row",
			"a | b",
			Options{},
			"a | b\n",
		},
	}
	for _, tt := range tests {
		check(t, tt.name, Render(tt.src, tt.opts), tt.want)
	}
}

// Front matter is data about the document, so it is shown as written and the
// body starts after it.
func TestFrontMatter(t *testing.T) {
	tests := []struct {
		name string
		src  string
		opts Options
		want string
	}{
		{
			"front matter is passed through untouched",
			"---\ntitle: Hello\ntags: [a, b]\n---\n\n# H\n",
			Options{},
			"title: Hello\ntags: [a, b]\n\nH\n=\n",
		},
		{
			"markup inside front matter is not markup",
			"---\nname: **not bold**\n---\ntext\n",
			Options{},
			"name: **not bold**\n\ntext\n",
		},
		{
			"a rule further down is a rule",
			"text\n\n---",
			Options{Width: 5},
			"text\n\n-----\n",
		},
		{
			"front matter that never closes is body",
			"---\ntitle: x",
			Options{Width: 20},
			strings.Repeat("-", 20) + "\n\ntitle: x\n",
		},
		{
			// Front matter is the one block that can arrive holding blank
			// lines of its own, so it is where the rule that no two of them
			// survive is worth stating outright. It applies here too.
			"a run of blank lines inside front matter closes up",
			"---\ntitle: x\n\n\n\nname: y\n---\n\nbody\n",
			Options{},
			"title: x\n\nname: y\n\nbody\n",
		},
	}
	for _, tt := range tests {
		check(t, tt.name, Render(tt.src, tt.opts), tt.want)
	}
}

// Wrapping. Spaces are where a line prefers to break; text without spaces
// breaks between runes, and never inside one.
func TestWrapping(t *testing.T) {
	tests := []struct {
		name string
		src  string
		opts Options
		want string
	}{
		{"english breaks at spaces", "aaa bbb ccc ddd eee fff", Options{Width: 20}, "aaa bbb ccc ddd eee\nfff\n"},
		{"japanese breaks between runes", "あいうえおかきくけこさしすせそ", Options{Width: 20}, "あいうえおかきくけこ\nさしすせそ\n"},
		{"a word longer than the width is split", "abcdefghijklmnopqrstuvwxy", Options{Width: 20}, "abcdefghijklmnopqrst\nuvwxy\n"},
		{"a soft break closes up to a space", "one\ntwo", Options{Width: 20}, "one two\n"},
		{"two trailing spaces break the line", "one  \ntwo", Options{Width: 20}, "one\ntwo\n"},
		{"a continuation line is not indented", "aaaa bbbb cccc", Options{Width: 10}, "aaaa bbbb\ncccc\n"},
		{"zero width means eighty columns", strings.Repeat("ab ", 30), Options{}, strings.TrimSpace(strings.Repeat("ab ", 27)) + "\nab ab ab\n"},
	}
	for _, tt := range tests {
		check(t, tt.name, Render(tt.src, tt.opts), tt.want)
	}
}

// Lines is Render, split. A caller that draws one line per screen row must get
// exactly the rows Render would have printed, with no empty one at the end.
func TestLinesMatchesRender(t *testing.T) {
	for _, color := range []bool{false, true} {
		for _, width := range []int{0, 20, 80} {
			opts := Options{Width: width, Color: color}
			got := Lines(kitchenSink, opts)
			want := strings.Split(strings.TrimSuffix(Render(kitchenSink, opts), "\n"), "\n")
			if !slices.Equal(got, want) {
				t.Errorf("Lines(width=%d, color=%v) is not Render split: got %d lines, want %d", width, color, len(got), len(want))
			}
			if n := len(got); n == 0 || got[n-1] == "" {
				t.Errorf("Lines(width=%d, color=%v) ends with an empty line: %q", width, color, got)
			}
			if got[0] == "" {
				t.Errorf("Lines(width=%d, color=%v) starts with an empty line", width, color)
			}
		}
		if n := len(Lines("", Options{Color: color})); n != 0 {
			t.Errorf("Lines(\"\") returned %d lines, want 0", n)
		}
	}
}

// Colour off means colour off: the output of a plain render has to be safe to
// write into a file, a pipe or a test's golden data. The input is not, so the
// fixture that carries escapes is rendered here too - without it the claim is
// only ever made about a document that had no escape to lose.
func TestPlainOutputHasNoEscapes(t *testing.T) {
	escapeSinkMustBite(t)
	for _, src := range []struct {
		name string
		text string
	}{
		{"kitchenSink", kitchenSink},
		{"escapeSink", escapeSink},
	} {
		for _, width := range []int{1, 2, 10, 20, 80} {
			out := Render(src.text, Options{Width: width})
			if strings.Contains(out, "\x1b") {
				t.Errorf("%s width=%d: plain output contains an escape sequence: %q", src.name, width, out)
			}
		}
	}
}

// Colour on is not permission to pass the input's escapes along. Every escape
// in the output has to be one of the eight this package writes, which is a
// claim a machine can check and a claim "it looked fine" cannot.
func TestColourOutputOnlyUsesKnownEscapes(t *testing.T) {
	escapeSinkMustBite(t)
	for _, src := range []struct {
		name string
		text string
	}{
		{"kitchenSink", kitchenSink},
		{"escapeSink", escapeSink},
	} {
		for _, width := range []int{1, 2, 10, 20, 80} {
			out := Render(src.text, Options{Width: width, Color: true})
			checkKnownEscapes(t, fmt.Sprintf("%s width=%d", src.name, width), out)
		}
	}
	// The escapes the input carried are shown as the pictures of themselves,
	// so nothing the author typed went missing on the way.
	out := Render(escapeSink, Options{Width: 80, Color: true})
	if !strings.Contains(out, "␛") {
		t.Errorf("colour output dropped the input's ESC instead of showing it: %q", out)
	}
}

// Markdown from a diff is not written to be parsed by this package. None of it
// may panic, and every non-empty input has to come back as one printable text.
func TestMalformedInputIsPrintedNotPanicked(t *testing.T) {
	inputs := []string{
		"",
		"\n",
		"\n\n\n",
		"   \n\t\n",
		"```",
		"```go\nunclosed",
		"~~~",
		"**unclosed",
		"__",
		"~~",
		"`",
		"``",
		"*",
		"***",
		"_",
		"[",
		"[x](",
		"![](",
		"[]()",
		"<",
		"<>",
		"<b",
		"|",
		"| a |",
		"| a |\n| - |",
		"|\n|-|\n|",
		"---",
		"- ",
		"-",
		"1.",
		"1. ",
		"> ",
		">",
		">>>>",
		"#",
		"######",
		"# \n",
		"- [ ]",
		"- [z] x",
		"\\",
		"a\\",
		"\xff\xfe **x**",
		"---\n",
		"---\n---\n",
		"\r\n\r\n",
		"a\r\nb",
		strings.Repeat("- a\n", 50),
		strings.Repeat("**", 50),
		strings.Repeat("[", 50) + strings.Repeat("]", 50),
		strings.Repeat("あ", 200),
		// A fence whose first or last line is blank: the row inside it must
		// survive, and the gap around it must not double.
		"```\n\na\n```\n",
		"```\na\n\n```\n",
		"p\n\n```\n\n\n```\n\nq\n",
		"para\n\n```\n\nx\n```\n",
		"# h\n\n```\n\ncode\n```\n",
		// Escapes riding in from the diff.
		"a \x1b[2J b\n",
		"a \x1bc b\n",
		"```\n\x1b]0;t\x07\n```\n",
		escapeSink,
		// Front matter is the one block that can hold blank lines of its own.
		"---\ntitle: x\n\n\n\nname: y\n---\n\nbody\n",
		"---\n\n\n\n---\n\nbody\n",
	}
	for _, src := range inputs {
		for _, color := range []bool{false, true} {
			for _, width := range []int{-1, 0, 1, 3, 20, 80} {
				opts := Options{Width: width, Color: color}
				out := Render(src, opts) // I1: getting here at all is the claim
				switch {
				case src == "" && out != "":
					t.Errorf("Render(%q) = %q, want the empty string", src, out)
				// I2. An input that draws nothing renders as nothing, and
				// only a non-empty result has an end to punctuate.
				case out != "" && !strings.HasSuffix(out, "\n"):
					t.Errorf("Render(%q, width=%d) = %q, want it to end with a newline", src, width, out)
				case strings.HasSuffix(out, "\n\n"):
					t.Errorf("Render(%q, width=%d) = %q, want no blank line at the end", src, width, out)
				case strings.HasPrefix(out, "\n"):
					t.Errorf("Render(%q, width=%d) = %q, want no blank line at the start", src, width, out)
				// I3. Two blank lines in a row are a gap the author did not
				// ask for, wherever in the document they turn up.
				case strings.Contains(out, "\n\n\n"):
					t.Errorf("Render(%q, width=%d) = %q, want no two blank lines in a row", src, width, out)
				}
				// I4. Lines is Render split, including when there is nothing
				// to split.
				got := Lines(src, opts)
				if out == "" {
					if len(got) != 0 {
						t.Errorf("Lines(%q, width=%d) = %q, want no lines", src, width, got)
					}
				} else if want := strings.Split(strings.TrimSuffix(out, "\n"), "\n"); !slices.Equal(got, want) {
					t.Errorf("Lines(%q) = %q, want %q", src, got, want)
				}
				// I5 and I6. What may appear in the output depends on Color,
				// and neither answer includes anything the input brought.
				if color {
					checkKnownEscapes(t, fmt.Sprintf("Render(%q, width=%d, color)", src, width), out)
				} else if strings.Contains(out, "\x1b") {
					t.Errorf("Render(%q, width=%d) = %q, want no escape at all", src, width, out)
				}
			}
		}
	}
}

// Fences keep the lines their author put in them. A blank line inside one is a
// row of the code, not the gap between two blocks, and the difference is what
// the indent on an otherwise empty line records.
func TestFenceKeepsBlankLines(t *testing.T) {
	tests := []struct {
		name string
		src  string
		want string
	}{
		{"a fence opening on a blank line", "```\n\na\n```\n", "  \n  a\n"},
		{"a fence closing on a blank line", "```\na\n\n```\n", "  a\n  \n"},
		{"blank rows between two paragraphs", "p\n\n```\n\n\n```\n\nq\n", "p\n\n  \n  \n\nq\n"},
		{"a paragraph then a fence opening blank", "para\n\n```\n\nx\n```\n", "para\n\n  \n  x\n"},
		{"a heading then a fence opening blank", "# h\n\n```\n\ncode\n```\n", "h\n=\n\n  \n  code\n"},
		{"newlines alone draw nothing", "\n\n\n", ""},
		{"whitespace alone draws nothing", "   \n\t\n", ""},
		{"an empty fence draws nothing", "```\n```\n", ""},
		{"front matter with nothing in it draws nothing", "---\n---\n", ""},
	}
	for _, tt := range tests {
		check(t, tt.name, Render(tt.src, Options{Width: 40}), tt.want)
	}
}

// Width has a ceiling, so a number that came from a broken ioctl or a config
// file cannot ask for a line the size of the machine's memory.
func TestWidthIsClamped(t *testing.T) {
	for _, width := range []int{4097, 100000000, 2147483647} {
		out := Render("---\n", Options{Width: width})
		line, _, _ := strings.Cut(out, "\n")
		if len(line) != maxWidth {
			t.Errorf("Render(rule, width=%d) drew a line of %d bytes, want %d", width, len(line), maxWidth)
		}
	}
}

// renderSink keeps the compiler from deciding a rendered document nobody reads
// need not be built.
var renderSink string

// longParagraph is n lines of prose with no blank line anywhere in it, which is
// how a release note or a design doc pasted into a review arrives. Each line is
// 26 bytes with its newline, so 72000 of them is the 1.8MB document the QA of
// this package timed at 12 seconds.
func longParagraph(n int) string {
	return strings.Repeat("lorem ipsum dolor sit ame\n", n)
}

// Joining a paragraph has to cost what the paragraph costs, not its square.
//
// Allocation is measured rather than time because it is the quantity that says
// which of the two shapes the code has: growing a string with += copies the
// whole run once per line, so four times the input allocates about sixteen
// times the bytes, while a Builder allocates about four.
func TestParagraphJoinIsLinear(t *testing.T) {
	measure := func(lines int) uint64 {
		src := longParagraph(lines)
		runtime.GC()
		var before, after runtime.MemStats
		runtime.ReadMemStats(&before)
		renderSink = Render(src, Options{Width: 80})
		runtime.ReadMemStats(&after)
		return after.TotalAlloc - before.TotalAlloc
	}
	small := measure(2000)
	large := measure(8000)
	if small == 0 {
		t.Fatal("rendering 2000 lines allocated nothing, so nothing was measured")
	}
	t.Logf("2000 lines: %d bytes, 8000 lines: %d bytes, ratio %.2f", small, large, float64(large)/float64(small))
	if large > small*8 {
		t.Errorf("four times the input allocated %.1f times the bytes (%d -> %d); linear is about 4 and quadratic about 16",
			float64(large)/float64(small), small, large)
	}
}

// The wall clock behind TestParagraphJoinIsLinear, off by default because the
// numbers a shared machine gives are not numbers to fail a build on.
func TestParagraphPerformance(t *testing.T) {
	if os.Getenv("MDTERM_PERF") != "1" {
		t.Skip("set MDTERM_PERF=1 to time the renderer")
	}
	cases := []struct {
		name  string
		lines int
		limit time.Duration
	}{
		{"1000 lines", 1000, 0},
		{"4000 lines", 4000, 0},
		{"16000 lines", 16000, 1000 * time.Millisecond},
		{"1.8MB", 72000, 3000 * time.Millisecond},
	}
	for _, c := range cases {
		src := longParagraph(c.lines)
		start := time.Now()
		renderSink = Render(src, Options{Width: 80})
		elapsed := time.Since(start)
		t.Logf("%s (%d bytes): %d ms", c.name, len(src), elapsed.Milliseconds())
		if c.limit > 0 && elapsed > c.limit {
			t.Errorf("%s took %d ms, want at most %d ms", c.name, elapsed.Milliseconds(), c.limit.Milliseconds())
		}
	}
}

// The one test that puts the output in front of a real terminal.
//
// Every other claim here is made about a Go string, and a Go string can be
// checked through a pipe - which is exactly how the escapes got through the
// last time: the check ran, the terminal was never involved, and the bytes that
// would have cleared the screen were never looked at. This writes the rendered
// fixture to stdout so a pty can be put in front of it and the bytes counted
// on the far side.
func TestPTYPlainOutputIsInert(t *testing.T) {
	if os.Getenv("MDTERM_PTY") != "1" {
		t.Skip("set MDTERM_PTY=1 and attach a pty to check the output on a real terminal")
	}
	escapeSinkMustBite(t)
	out := Render(escapeSink, Options{Width: 40})
	if out == "" {
		t.Fatal("the fixture rendered to nothing, so the terminal would be handed nothing")
	}
	fmt.Fprint(os.Stdout, "---MDTERM-BEGIN---")
	fmt.Fprint(os.Stdout, out)
	fmt.Fprint(os.Stdout, "---MDTERM-END---")
}
