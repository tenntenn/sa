// Package mdterm renders Markdown as text a terminal can print unchanged.
//
// sbnn shows the same Markdown in two places. The browser side gets it through
// web/src/markdown.ts, which hands the body to marked with GFM on and soft
// breaks off, and shows the YAML front matter in a frame of its own. A
// terminal has no HTML to lean on, so this package does that job with its own
// rules: block by block into lines, wrapped to a width, and decorated with
// ANSI escapes only when the caller asks for them.
//
// The Markdown arrives inside a diff, so it is not trusted input. Nothing here
// starts a process, opens a file or reaches the network, and no input is
// allowed to panic: markup that does not parse is printed as the text it is.
// For a review that is the right answer anyway - the reader came to see what
// was written, and swallowing a line they wrote is worse than showing markup
// unrendered.
package mdterm

import "strings"

// defaultWidth is the terminal width assumed when the caller names none. Eighty
// columns is the width every terminal has at least, so it is the one that never
// makes the output worse than the caller asked for.
const defaultWidth = 80

// Options is the configuration of a conversion.
type Options struct {
	// Width is the display width to wrap at, counted in half-width columns.
	// Zero or less means defaultWidth.
	Width int
	// Color decorates the output with ANSI escapes when true. When false the
	// output contains no escape sequence at all, which is what makes it safe
	// to write to a file or a pipe.
	Color bool
}

// Render converts Markdown into text ready for a terminal.
//
// The result ends with exactly one newline and never begins with a blank line.
// The empty input is the only input that renders as the empty string.
func Render(src string, opts Options) string {
	if src == "" {
		return ""
	}
	if opts.Width <= 0 {
		opts.Width = defaultWidth
	}
	src = strings.ReplaceAll(src, "\r\n", "\n")
	src = strings.ReplaceAll(src, "\r", "\n")

	var blocks [][]string
	front, body := splitFrontMatter(src)
	if front != "" {
		blocks = append(blocks, strings.Split(front, "\n"))
	}
	blocks = append(blocks, parseBlocks(strings.Split(body, "\n"), opts)...)

	var out []string
	for _, block := range blocks {
		if allBlank(block) {
			continue // an empty block would leave a second blank line behind
		}
		if len(out) > 0 {
			out = append(out, "")
		}
		out = append(out, block...)
	}
	// A block can still open or close with a blank line of its own - an empty
	// row of a table, a fenced block that starts with one - and at the edges of
	// the document those read as padding nobody asked for.
	for len(out) > 0 && out[0] == "" {
		out = out[1:]
	}
	for len(out) > 0 && out[len(out)-1] == "" {
		out = out[:len(out)-1]
	}
	return strings.Join(out, "\n") + "\n"
}

func allBlank(block []string) bool {
	for _, l := range block {
		if !isBlank(l) {
			return false
		}
	}
	return true
}

// Lines is Render split into lines, with no empty line left at the end.
//
// A caller drawing a scrolling pane wants the lines, not the text, and it must
// not have to strip the final newline itself to avoid an extra blank row at
// the bottom of the screen.
func Lines(src string, opts Options) []string {
	out := Render(src, opts)
	if out == "" {
		return nil
	}
	return strings.Split(strings.TrimSuffix(out, "\n"), "\n")
}

// splitFrontMatter peels off the YAML metadata block at the top of a document.
//
// The block is shown as it was written, with no wrapping and no markup applied,
// the way the exported page frames it: front matter is data about the document
// rather than part of it, and a rendered version of it would be a different
// document's worth of text.
func splitFrontMatter(src string) (front, body string) {
	if !strings.HasPrefix(src, "---\n") {
		return "", src
	}
	lines := strings.Split(src, "\n")
	for i := 1; i < len(lines); i++ {
		if lines[i] == "---" {
			return strings.Join(lines[1:i], "\n"), strings.Join(lines[i+1:], "\n")
		}
	}
	return "", src // never closed, so there is no front matter here
}
