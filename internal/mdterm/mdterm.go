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

// maxWidth is the largest width a caller can ask for.
//
// It is comfortably above the column count of any terminal that exists, so no
// real display is ever narrowed by it, and it is small enough that the widest
// single line this package can be made to build - a horizontal rule, which is
// Width characters of its own - stays a few kilobytes. Without the ceiling a
// Width read from a broken ioctl or a config file asks for that many bytes in
// one allocation, and math.MaxInt32 ends the process rather than the line.
const maxWidth = 4096

// Options is the configuration of a conversion.
type Options struct {
	// Width is the display width to wrap at, counted in half-width columns.
	// Zero or less means defaultWidth, and more than maxWidth means maxWidth.
	Width int
	// Color decorates the output with ANSI escapes when true. When false the
	// output contains no escape sequence at all, which is what makes it safe
	// to write to a file or a pipe. Either way the control characters carried
	// by the input are replaced with the visible pictures of themselves, so
	// the only escapes that can reach the terminal are the ones this package
	// wrote itself.
	Color bool
}

// Render converts Markdown into text ready for a terminal.
//
// The result ends with exactly one newline, never begins with a blank line and
// never holds two blank lines in a row. Input with nothing to draw in it - the
// empty string, whitespace, a fence with nothing between its ends - renders as
// the empty string, because a document of no content is not a document of one
// blank line.
func Render(src string, opts Options) string {
	if src == "" {
		return ""
	}
	if opts.Width <= 0 {
		opts.Width = defaultWidth
	}
	if opts.Width > maxWidth {
		opts.Width = maxWidth
	}
	src = strings.ReplaceAll(src, "\r\n", "\n")
	src = strings.ReplaceAll(src, "\r", "\n")
	src = defang(src)

	var blocks [][]string
	front, body := splitFrontMatter(src)
	if front != "" {
		blocks = append(blocks, strings.Split(front, "\n"))
	}
	blocks = append(blocks, parseBlocks(strings.Split(body, "\n"), opts)...)

	var out []string
	for _, block := range blocks {
		if len(block) == 0 {
			continue // a block that rendered to nothing separates nothing
		}
		if len(out) > 0 {
			out = append(out, "")
		}
		out = append(out, block...)
	}
	// A block can open or close with a blank line of its own - an empty row of
	// a table, a fenced block that starts with one - and beside the separator
	// the loop above just wrote, that reads as a gap twice the size of every
	// other gap in the document. Runs of them close up to one.
	//
	// Only the exactly empty line counts. The two spaces a blank line inside a
	// fence renders as are content, and content is not something this package
	// removes.
	kept := out[:0]
	for _, l := range out {
		if l == "" && len(kept) > 0 && kept[len(kept)-1] == "" {
			continue
		}
		kept = append(kept, l)
	}
	out = kept
	// At the two edges of the document a blank line is padding nobody asked
	// for, so it comes off entirely rather than closing up.
	for len(out) > 0 && out[0] == "" {
		out = out[1:]
	}
	for len(out) > 0 && out[len(out)-1] == "" {
		out = out[:len(out)-1]
	}
	if len(out) == 0 {
		return "" // nothing was drawn, so there is no line to end
	}
	return strings.Join(out, "\n") + "\n"
}

// defang replaces the control characters of the input with the visible
// pictures of themselves, leaving the tab and the newline alone.
//
// The Markdown arrives inside a diff, so a line of it can hold anything its
// author could type, \x1b[2J and \x1bc included. Passed through, those are not
// text the reader sees but commands the terminal obeys: the screen clears, or
// it resets, and the review is gone. Replacing them keeps the package's
// promise that Color:false emits no escape at all, and it holds with colour on
// too, so every escape in the output is one of the eight this package writes.
//
// They are replaced rather than dropped. U+241B is the reader being told an
// ESC was written there; nothing at all is the reader being told a lie.
func defang(r string) string {
	return strings.Map(func(c rune) rune {
		switch {
		case c == '\t' || c == '\n':
			return c
		case c < 0x20:
			return controlPicture + c // U+2400 upwards, one per C0 code
		case c == 0x7f:
			return 0x2421 // U+2421, the picture of DEL
		case c >= 0x80 && c <= 0x9f:
			// The C1 controls have no pictures of their own, and one of them
			// is the 8-bit CSI, which introduces a sequence just as \x1b[ does.
			return '�'
		}
		return c
	}, r)
}

// controlPicture is the base of the Control Pictures block: the picture of the
// C0 control with value n is at controlPicture + n.
const controlPicture = 0x2400

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
// document's worth of text. The one thing that does change is spacing: a run
// of blank lines inside it closes up to one, because Render holds that rule
// over the whole output rather than over part of it.
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
