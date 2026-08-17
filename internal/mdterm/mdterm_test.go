package mdterm

import (
	"slices"
	"strings"
	"testing"
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
		{"a blank line inside stays blank", "```\na\n\nb\n```", Options{}, "  a\n\n  b\n"},
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
// write into a file, a pipe or a test's golden data.
func TestPlainOutputHasNoEscapes(t *testing.T) {
	for _, width := range []int{1, 2, 10, 20, 80} {
		out := Render(kitchenSink, Options{Width: width})
		if strings.Contains(out, "\x1b") {
			t.Errorf("width=%d: plain output contains an escape sequence: %q", width, out)
		}
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
	}
	for _, src := range inputs {
		for _, color := range []bool{false, true} {
			for _, width := range []int{-1, 0, 1, 3, 20} {
				opts := Options{Width: width, Color: color}
				out := Render(src, opts)
				switch {
				case src == "" && out != "":
					t.Errorf("Render(%q) = %q, want the empty string", src, out)
				case src != "" && !strings.HasSuffix(out, "\n"):
					t.Errorf("Render(%q, width=%d) = %q, want it to end with a newline", src, width, out)
				case strings.HasSuffix(out, "\n\n"):
					t.Errorf("Render(%q, width=%d) = %q, want no blank line at the end", src, width, out)
				case out != "\n" && strings.HasPrefix(out, "\n"):
					t.Errorf("Render(%q, width=%d) = %q, want no blank line at the start", src, width, out)
				}
				if got, want := Lines(src, opts), strings.Split(strings.TrimSuffix(out, "\n"), "\n"); out != "" && !slices.Equal(got, want) {
					t.Errorf("Lines(%q) = %q, want %q", src, got, want)
				}
			}
		}
	}
}
