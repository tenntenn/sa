package mdterm

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

// cell is one indivisible piece of rendered text: either a printable rune
// with the columns it occupies, or an ANSI escape sequence, which occupies
// none. Wrapping works on cells rather than bytes so that a line never
// breaks in the middle of a rune or in the middle of an escape sequence -
// half an escape sequence printed on its own line is garbage on screen.
type cell struct {
	text  string
	width int
}

// cells splits s into the pieces a terminal draws.
//
// Invalid UTF-8 is not rejected: the Markdown arrived in a diff, so it may be
// anything at all. DecodeRuneInString turns a bad byte into one replacement
// rune, which is exactly what a terminal shows for it.
func cells(s string) []cell {
	out := make([]cell, 0, len(s))
	for i := 0; i < len(s); {
		if n := escapeLen(s[i:]); n > 0 {
			out = append(out, cell{text: s[i : i+n]})
			i += n
			continue
		}
		r, size := utf8.DecodeRuneInString(s[i:])
		out = append(out, cell{text: s[i : i+size], width: runeWidth(r)})
		i += size
	}
	return out
}

// escapeLen is the length of the CSI escape sequence at the start of s, or 0
// when s does not start with one. A sequence that never terminates is not one,
// and its bytes are then measured as the control characters they are.
func escapeLen(s string) int {
	if !strings.HasPrefix(s, "\x1b[") {
		return 0
	}
	for i := 2; i < len(s); i++ {
		if s[i] >= 0x40 && s[i] <= 0x7e {
			return i + 1
		}
	}
	return 0
}

// displayWidth is the number of terminal columns s occupies.
func displayWidth(s string) int {
	n := 0
	for _, c := range cells(s) {
		n += c.width
	}
	return n
}

func cellsWidth(cs []cell) int {
	n := 0
	for _, c := range cs {
		n += c.width
	}
	return n
}

func cellsText(cs []cell) string {
	var b strings.Builder
	for _, c := range cs {
		b.WriteString(c.text)
	}
	return b.String()
}

// wideRanges are the runes drawn two columns wide.
//
// The full East Asian Width table is not in the standard library and this
// package takes no dependencies, so this approximation is the definition sbnn
// uses: the CJK, Hangul, fullwidth and common emoji blocks. It is deliberately
// a fixed list rather than a guess refined over time - column arithmetic that
// changes underfoot makes table alignment untestable.
var wideRanges = [...][2]rune{
	{0x1100, 0x115F},
	{0x2E80, 0x303E},
	{0x3041, 0x33FF},
	{0x3400, 0x4DBF},
	{0x4E00, 0x9FFF},
	{0xA000, 0xA4CF},
	{0xAC00, 0xD7A3},
	{0xF900, 0xFAFF},
	{0xFE10, 0xFE19},
	{0xFE30, 0xFE6F},
	{0xFF00, 0xFF60},
	{0xFFE0, 0xFFE6},
	{0x1F300, 0x1F64F},
	{0x1F900, 0x1F9FF},
	{0x20000, 0x2FFFD},
	{0x30000, 0x3FFFD},
}

// runeWidth is how many columns one rune takes: none for a combining mark or
// a control character, two inside wideRanges, one otherwise.
func runeWidth(r rune) int {
	if r < 0x20 || r == 0x7f {
		return 0
	}
	if unicode.Is(unicode.Mn, r) {
		return 0
	}
	for _, rg := range wideRanges {
		if r >= rg[0] && r <= rg[1] {
			return 2
		}
	}
	return 1
}

// sgrPairs are the attributes wrap follows across a line break, each "on"
// escape beside the escape that turns just it off again. The strings are the
// ones inline.go emits and are not redefined here: an escape wrap invents is an
// escape no other part of the package knows how to close.
var sgrPairs = [...]struct{ on, off string }{
	{boldOn, boldOff},
	{italicOn, italicOff},
	{strikeOn, strikeOff},
	{codeOn, codeOff},
}

// sgrOff is the escape that closes the attribute opened by on.
func sgrOff(on string) string {
	for _, p := range sgrPairs {
		if p.on == on {
			return p.off
		}
	}
	return ""
}

// isSGRReset reports whether esc is the blanket reset, which drops every
// attribute at once. Input arrives from a diff, so it can carry a reset this
// package would never emit itself, and what follows one is undecorated.
func isSGRReset(esc string) bool {
	if escapeLen(esc) != len(esc) || !strings.HasSuffix(esc, "m") {
		return false
	}
	// escapeLen only accepts a two-byte CSI introducer, so the parameters are
	// everything between it and the final byte.
	for _, param := range strings.Split(esc[2:len(esc)-1], ";") {
		if param != "" && param != "0" {
			return false
		}
	}
	return true
}

// trackSGR folds one cell into open, the list of attributes in effect, kept in
// the order they were opened so they can be closed in the opposite one.
//
// Any other escape - a colour, a cursor move, something from the input - is
// left alone: wrap passes it through untouched and does not try to reopen on
// the next line something it does not know how to close.
func trackSGR(open []string, text string) []string {
	if len(text) == 0 || escapeLen(text) != len(text) {
		return open
	}
	for _, p := range sgrPairs {
		switch text {
		case p.on:
			for _, o := range open {
				if o == text {
					return open // already open; opening twice would close once too few
				}
			}
			return append(open, text)
		case p.off:
			for i, o := range open {
				if o == p.on {
					return append(open[:i:i], open[i+1:]...)
				}
			}
			return open
		}
	}
	if isSGRReset(text) {
		return nil
	}
	return open
}

// wrap breaks s into lines of at most limit columns.
//
// Spaces are where a line prefers to break, and the space itself disappears
// with the break. A word too long to fit anywhere - a URL, a run of Japanese,
// which has no spaces at all - is broken between runes instead, because the
// alternative is a line that runs off the screen.
//
// Decoration never crosses a break: a line that ends inside a bold run closes
// the bold at its end, and the next line opens it again at its start, so every
// line returned is balanced on its own. A TUI draws a window of these lines,
// not all of them, and both halves of that matter - printing only the middle of
// a wrapped bold sentence has to come out bold, and printing only its first
// line must not leave the terminal bold for whatever is drawn next.
//
// The escapes added this way are not counted: they occupy no columns, so lines
// break in exactly the same places as they would without them.
func wrap(s string, limit int) []string {
	if limit < 1 {
		limit = 1
	}
	cs := cells(s)
	var lines []string
	var line strings.Builder
	lineWidth := 0
	var open []string // attributes the line so far has left open, oldest first
	write := func(cs []cell) {
		for _, c := range cs {
			line.WriteString(c.text)
			lineWidth += c.width
			open = trackSGR(open, c.text)
		}
	}
	// closeOpen and reopen write escapes directly rather than through write:
	// they must not disturb the attributes being tracked, and they add no
	// columns to the line.
	closeOpen := func() {
		for i := len(open) - 1; i >= 0; i-- {
			line.WriteString(sgrOff(open[i]))
		}
	}
	reopen := func() {
		for _, on := range open {
			line.WriteString(on)
		}
	}
	flush := func() {
		closeOpen()
		lines = append(lines, line.String())
		line.Reset()
		lineWidth = 0
		reopen()
	}
	for i := 0; i < len(cs); {
		gapStart := i
		for i < len(cs) && isSpaceCell(cs[i]) {
			i++
		}
		gap := cs[gapStart:i]
		wordStart := i
		for i < len(cs) && !isSpaceCell(cs[i]) {
			i++
		}
		word := cs[wordStart:i]
		if len(word) == 0 {
			break // trailing spaces have nothing left to hold on to
		}
		wordWidth := cellsWidth(word)
		gapWidth := cellsWidth(gap)
		if lineWidth > 0 && lineWidth+gapWidth+wordWidth <= limit {
			write(gap)
			write(word)
			continue
		}
		if lineWidth > 0 {
			flush()
		}
		if wordWidth <= limit {
			write(word)
			continue
		}
		for k := range word {
			if lineWidth > 0 && lineWidth+word[k].width > limit {
				flush()
			}
			write(word[k : k+1])
		}
	}
	if line.Len() > 0 || len(lines) == 0 {
		closeOpen()
		lines = append(lines, line.String())
	}
	return lines
}

func isSpaceCell(c cell) bool {
	return c.text == " " || c.text == "\t"
}
