package tui

import (
	"fmt"
	"strings"

	"github.com/rivo/uniseg"

	"github.com/tenntenn/sbnn/internal/model"
)

// Everything in this file is plain text. A frame carries no escape sequence of
// its own, and the ones a diff may contain are dropped, so a frame can be
// written to a pipe, diffed, or grepped. Colour is added later, in the view.

const (
	// gutterWidth is the width of the line number column of the diff pane.
	// Four digits cover the files people read; a longer number widens the
	// column rather than being cut.
	gutterWidth = 4
	// tabWidth is how wide a tab is drawn. Columns are counted in cells, so
	// the tabs a diff carries are expanded once, here.
	tabWidth = 4
	// separator is the vertical rule between the two panes.
	separator = " | "
	// minSplitWidth is the narrowest screen that still gets two panes.
	// Below it the file list is dropped and the diff takes everything.
	minSplitWidth = 30
	// listMaxWidth and listMinWidth bound the file list pane.
	listMaxWidth = 40
	listMinWidth = 20
	// ellipsis marks a line that was cut. It takes one cell.
	ellipsis = "…"
)

// cellWidth is how many columns of a terminal s takes up. A column is not a
// byte and not a rune: a kanji takes two, a combining accent takes none, and
// an emoji joined by a zero-width joiner takes two however many runes it is
// spelled with. Every width in this package is counted here, and nowhere else,
// so that the panes line up whatever the diff is written in.
func cellWidth(s string) int {
	return uniseg.StringWidth(s)
}

// splitCells cuts s after exactly cells columns and returns the two halves.
// The cut lands on a grapheme cluster boundary, so a two-cell character is
// never halved: when one straddles the mark, it goes to tail and head comes
// back one cell short. A string narrower than cells is returned whole.
func splitCells(s string, cells int) (head, tail string) {
	if cells <= 0 {
		return "", s
	}
	if cellWidth(s) <= cells {
		return s, ""
	}
	used, end, state := 0, 0, -1
	for rest := s; rest != ""; {
		cluster, remainder, boundaries, next := uniseg.StepString(rest, state)
		w := boundaries >> uniseg.ShiftWidth
		if used+w > cells {
			break
		}
		used += w
		end += len(cluster)
		rest, state = remainder, next
	}
	return s[:end], s[end:]
}

// FileListLines draws the file list, one line per file, exactly height lines
// long. A list too long to fit is scrolled so that the selected file is on
// screen; a list too short is padded with blank lines.
func FileListLines(s *State, width, height int) []string {
	if s == nil || width <= 0 || height <= 0 {
		return nil
	}
	lines := make([]string, 0, len(s.Files))
	for i, f := range s.Files {
		marker := "  "
		if i == s.Cursor {
			marker = "> "
		}
		lines = append(lines, fit(fmt.Sprintf("%s%s  +%d -%d  %s",
			marker, f.Path(), f.Additions, f.Deletions, f.Status), width))
	}
	return window(lines, s.Cursor, height)
}

// DiffLines draws the diff of f from line top, exactly height lines long.
// Each line keeps the +, - or space of a unified diff right before its
// content, behind a column of new-side line numbers.
func DiffLines(f *model.File, top, width, height int) []string {
	if f == nil || width <= 0 || height <= 0 {
		return nil
	}
	if f.IsBinary {
		return []string{fit("binary file", width)}
	}
	return PaneLines(diffBody(f), top, width, height)
}

// PaneLines draws body from line top, exactly height lines long. A line wider
// than the pane is cut with an ellipsis; a body shorter than the pane is
// padded with blank lines, so the rows below it are the pane's own and not
// whatever the previous frame left there.
func PaneLines(body []string, top, width, height int) []string {
	if width <= 0 || height <= 0 {
		return nil
	}
	top = clamp(top, 0, len(body))
	out := make([]string, 0, height)
	for i := top; i < len(body) && len(out) < height; i++ {
		out = append(out, fit(body[i], width))
	}
	for len(out) < height {
		out = append(out, "")
	}
	return out
}

// diffBody is every line of the diff of f: each hunk header followed by the
// lines of that hunk.
func diffBody(f *model.File) []string {
	lines := make([]string, 0, diffLineCount(f))
	for _, h := range f.Hunks {
		lines = append(lines, fmt.Sprintf("%s %s", blankGutter(), h.Header))
		for _, l := range h.Lines {
			lines = append(lines, fmt.Sprintf("%s %s%s", lineNumber(l.NewNumber), lineMarker(l.Kind), l.Content))
		}
	}
	return lines
}

// diffLineCount is how many lines the diff of f takes, without drawing them.
func diffLineCount(f *model.File) int {
	if f == nil {
		return 0
	}
	if f.IsBinary {
		return 1
	}
	n := 0
	for _, h := range f.Hunks {
		n += 1 + len(h.Lines)
	}
	return n
}

// lineNumber draws the new-side number of a line. A deleted line has none, so
// its gutter is left blank.
func lineNumber(n int) string {
	if n == 0 {
		return blankGutter()
	}
	return fmt.Sprintf("%*d", gutterWidth, n)
}

func blankGutter() string {
	return strings.Repeat(" ", gutterWidth)
}

// lineMarker is the one character a unified diff puts in front of a line.
func lineMarker(kind model.LineKind) string {
	switch kind {
	case model.LineAdd:
		return "+"
	case model.LineDelete:
		return "-"
	default:
		return " "
	}
}

// Header is the top line of the frame: what this is, and what it holds.
func Header(s *State) string {
	adds, dels := 0, 0
	for _, f := range s.Files {
		adds += f.Additions
		dels += f.Deletions
	}
	return fmt.Sprintf("sbnn tui  %d %s  +%d -%d  [%s]",
		len(s.Files), plural(len(s.Files), "file"), adds, dels, viewName(s.View))
}

// viewName is what the header calls a view.
func viewName(v View) string {
	switch v {
	case ViewMarkdown:
		return "markdown"
	case ViewRaw:
		return "raw"
	default:
		return "diff"
	}
}

// Footer is the bottom line of the frame: the keys, and which pane they move.
// The way out comes first, because a narrow screen cuts the end of the line
// and nobody should lose the key that lets them leave.
func Footer(s *State) string {
	pane := "files"
	if s.Focus == PaneDiff {
		pane = "diff"
	}
	// The preview key is listed whether or not this file has one. A row of
	// keys that changes as the cursor moves is a row nobody can read.
	return fmt.Sprintf("q: quit  [%s]  j/k: move  tab: switch pane  p: preview  g/G: first/last  ctrl+d/ctrl+u: page", pane)
}

// Frame draws the whole screen: the header, the two panes side by side, and
// the footer. It returns exactly height lines, none of them wider than width.
func Frame(s *State, width, height int) []string {
	if s == nil || width <= 0 || height <= 0 {
		return nil
	}
	out := make([]string, 0, height)
	out = append(out, fit(Header(s), width))
	if height == 1 {
		return out
	}
	if body := height - 2; body > 0 {
		listWidth, diffWidth := paneWidths(width)
		var list []string
		if listWidth > 0 {
			list = FileListLines(s, listWidth, body)
		}
		// The right pane is one pane with three things it can show, rather
		// than three panes with one of them on: a screen is not wide enough
		// to read a diff and the file it changed side by side.
		var right []string
		if s.View == ViewDiff {
			right = DiffLines(s.Current(), s.Top, diffWidth, body)
		} else {
			right = PaneLines(s.PaneBody(diffWidth), s.Top, diffWidth, body)
		}
		for i := 0; i < body; i++ {
			out = append(out, bodyRow(at(list, i), at(right, i), listWidth))
		}
	}
	return append(out, fit(Footer(s), width))
}

// paneWidths splits the screen between the two panes. The file list takes
// three tenths, within bounds; a screen too narrow for both shows the diff
// alone, because a diff cut to nothing says nothing.
func paneWidths(width int) (list, diff int) {
	if width < minSplitWidth {
		return 0, width
	}
	list = clamp(width*3/10, listMinWidth, listMaxWidth)
	diff = width - list - len(separator)
	if diff < 1 {
		diff = 1
	}
	return list, diff
}

// bodyRow puts one line of each pane side by side.
func bodyRow(list, diff string, listWidth int) string {
	if listWidth <= 0 {
		return strings.TrimRight(diff, " ")
	}
	return strings.TrimRight(pad(list, listWidth)+separator+diff, " ")
}

// window returns exactly height lines of lines, scrolled far enough for the
// line at cursor to be among them, and padded when there are too few.
func window(lines []string, cursor, height int) []string {
	start := 0
	if len(lines) > height {
		if cursor >= height {
			start = cursor - height + 1
		}
		if last := len(lines) - height; start > last {
			start = last
		}
	}
	out := make([]string, 0, height)
	for i := start; i < len(lines) && len(out) < height; i++ {
		out = append(out, lines[i])
	}
	for len(out) < height {
		out = append(out, "")
	}
	return out
}

// fit makes text drawable in width cells: control bytes go, tabs become
// spaces, and what is still too long is cut with an ellipsis. The result is
// never wider than width, counted in cells.
func fit(text string, width int) string {
	if width <= 0 {
		return ""
	}
	text = sanitize(text)
	if cellWidth(text) <= width {
		return text
	}
	if width == 1 {
		return ellipsis
	}
	// The ellipsis takes the last cell, so the text gets width-1. Cutting on a
	// cluster boundary can leave a cell spare in front of a wide character;
	// that is a blank column, not a broken one.
	head, _ := splitCells(text, width-1)
	return head + ellipsis
}

// pad widens text to exactly width cells, and cuts it when it is wider.
func pad(text string, width int) string {
	text = fit(text, width)
	if n := width - cellWidth(text); n > 0 {
		return text + strings.Repeat(" ", n)
	}
	return text
}

// sanitize expands tabs and drops control bytes. A frame is plain text: an
// escape sequence read out of a diff would otherwise repaint the terminal of
// whoever is reading it.
func sanitize(text string) string {
	if !strings.ContainsFunc(text, needsSanitizing) {
		return text
	}
	var b strings.Builder
	b.Grow(len(text))
	for _, r := range text {
		switch {
		case r == '\t':
			b.WriteString(strings.Repeat(" ", tabWidth))
		case isControl(r):
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

func needsSanitizing(r rune) bool {
	return r == '\t' || isControl(r)
}

func isControl(r rune) bool {
	return r < 0x20 || r == 0x7f
}

// at is lines[i], or the empty line when the pane ran out of them.
func at(lines []string, i int) string {
	if i < 0 || i >= len(lines) {
		return ""
	}
	return lines[i]
}

func plural(n int, word string) string {
	if n == 1 {
		return word
	}
	return word + "s"
}
