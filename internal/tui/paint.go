package tui

import (
	"os"
	"strings"
)

// Colour is written out as SGR by hand, rather than through a styling library.
// A library that colours a terminal wants to know what the terminal can do,
// and asks it - it writes a query to stdout and waits for the answer. sbnn tui
// cannot afford that: stdout carries the diff someone piped in, and a terminal
// that never answers costs a five second pause on every run of every
// subcommand, because the query is sent from a package init.
//
// The codes below are the ones every terminal since the seventies has had, so
// there is nothing worth asking about. The colours are the basic ANSI ones on
// purpose: the reader's own terminal theme decides what green and red look
// like, the way git does it.
const (
	sgrReset   = "\x1b[0m"
	sgrBold    = "\x1b[1m"
	sgrFaint   = "\x1b[2m"
	sgrReverse = "\x1b[7m"
	sgrGreen   = "\x1b[32m"
	sgrRed     = "\x1b[31m"
)

// palette paints a frame. It holds one bit, because the only question is
// whether to colour at all: which colours to use is not the terminal's to
// decide.
type palette struct{ colour bool }

func newPalette(colour bool) palette {
	return palette{colour: colour}
}

// colourEnabled reports whether to colour this session. The frames go to
// /dev/tty, so there is no pipe to detect and nothing to ask the terminal:
// what is left is what the reader asked for.
func colourEnabled() bool {
	if os.Getenv("NO_COLOR") != "" {
		return false
	}
	term := os.Getenv("TERM")
	if term == "" || strings.HasPrefix(term, "dumb") {
		return false
	}
	return true
}

// Paint draws the frame and colours it, returning one string per row. It
// changes not one character of what Frame drew: every line comes back with the
// same text in it, with escape sequences around the parts that get a colour.
func (p palette) Paint(s *State, width, height int) []string {
	lines := Frame(s, width, height)
	if len(lines) == 0 {
		return nil
	}
	listWidth, _ := paneWidths(width)
	// Green and red mean added and deleted, and in a preview they would mean
	// neither: a bullet opens with the same "-" a deleted line does, and a
	// list of pros and cons would come out coloured like a diff of itself.
	// The pane is showing the file, so the marker is just a character.
	colourDiff := s.View == ViewDiff
	out := make([]string, len(lines))
	for i, line := range lines {
		switch {
		case i == 0:
			out[i] = p.wrap(sgrBold, line)
		case i == len(lines)-1:
			out[i] = p.wrap(sgrFaint, line)
		default:
			out[i] = p.styleBody(line, listWidth, colourDiff)
		}
	}
	return out
}

// wrap puts sgr in front of text and turns it off again after. An empty line
// gets nothing: there is no point colouring a run of no cells, and a bare
// reset at the end of a blank row only makes the output harder to read.
func (p palette) wrap(sgr, text string) string {
	if !p.colour || sgr == "" || text == "" {
		return text
	}
	return sgr + text + sgrReset
}

// styleBody colours one body line without changing a character of it: the
// file column, the rule, and the right column are styled where Frame put them.
// The columns are found by counting cells, which is where Frame put them too.
// colourDiff says whether the right column is a diff; when it is not, it is
// the text of a file and none of its lines are added or deleted.
func (p palette) styleBody(line string, listWidth int, colourDiff bool) string {
	right := func(text string) string {
		if !colourDiff {
			return text
		}
		return p.wrap(diffStyle(text), text)
	}
	if listWidth <= 0 {
		return right(line)
	}
	if cellWidth(line) <= listWidth {
		return p.wrap(fileStyle(line), line)
	}
	list, rest := splitCells(line, listWidth)
	sep, diff := rest, ""
	if strings.HasPrefix(rest, separator) {
		sep, diff = separator, rest[len(separator):]
	}
	return p.wrap(fileStyle(list), list) + sep + right(diff)
}

// fileStyle picks out the selected file, which FileListLines marks with "> ".
func fileStyle(list string) string {
	if strings.HasPrefix(list, "> ") {
		return sgrReverse
	}
	return ""
}

// diffStyle colours a diff line by the marker DiffLines drew in front of its
// content.
func diffStyle(diff string) string {
	switch diffMarker(diff) {
	case '+':
		return sgrGreen
	case '-':
		return sgrRed
	default:
		return ""
	}
}

// diffMarker is the first character of a diff line past its number gutter.
func diffMarker(diff string) byte {
	for i := 0; i < len(diff); i++ {
		if c := diff[i]; c != ' ' && (c < '0' || c > '9') {
			return c
		}
	}
	return 0
}
