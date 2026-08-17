package mdterm

import "strings"

// parseBlocks turns the body of a document into blocks, each already rendered
// to its own lines. The caller puts the blank line between them, so a block
// never has to know what came before it.
func parseBlocks(lines []string, opts Options) [][]string {
	var blocks [][]string
	for i := 0; i < len(lines); {
		if isBlank(lines[i]) {
			i++
			continue
		}
		var n int
		var block []string
		switch {
		case isFence(lines[i]):
			n, block = codeBlock(lines[i:], opts)
		case isThematicBreak(lines[i]):
			n, block = 1, []string{strings.Repeat("-", opts.Width)}
		case headingLevel(lines[i]) > 0:
			n, block = 1, heading(lines[i], opts)
		case isTableStart(lines, i):
			n, block = table(lines[i:], opts)
		case quoteDepth(lines[i]) > 0:
			n, block = quote(lines[i:], opts)
		case isListItem(lines[i]):
			n, block = list(lines[i:], opts)
		default:
			n, block = paragraph(lines[i:], opts)
		}
		if n < 1 {
			n = 1 // no block may stand still; that would never terminate
		}
		i += n
		if len(block) > 0 {
			blocks = append(blocks, block)
		}
	}
	return blocks
}

// startsBlock reports whether lines[i] begins a block of its own, which is how
// a paragraph or a list knows it has ended without waiting for a blank line.
func startsBlock(lines []string, i int) bool {
	l := lines[i]
	switch {
	case isFence(l), isThematicBreak(l), headingLevel(l) > 0, isTableStart(lines, i):
		return true
	case quoteDepth(l) > 0, isListItem(l):
		return true
	}
	return false
}

func isBlank(l string) bool {
	return strings.TrimSpace(l) == ""
}

// paragraph gathers the run of text lines at the front of lines and wraps it.
//
// A newline inside a paragraph closes up to a single space, matching the
// browser side, which calls marked with breaks off. Two spaces at the end of a
// line are the one way to ask for a line break, and that ends up as a break
// here too, each piece wrapped on its own.
func paragraph(lines []string, opts Options) (int, []string) {
	var segments []string
	var cur string
	n := 0
	for ; n < len(lines); n++ {
		l := lines[n]
		if isBlank(l) {
			break
		}
		if n > 0 && startsBlock(lines, n) {
			break
		}
		hard := len(l)-len(strings.TrimRight(l, " ")) >= 2
		text := strings.TrimSpace(l)
		if cur == "" {
			cur = text
		} else {
			cur += " " + text
		}
		if hard {
			segments = append(segments, cur)
			cur = ""
		}
	}
	if cur != "" {
		segments = append(segments, cur)
	}
	var out []string
	for _, seg := range segments {
		out = append(out, wrap(inline(seg, opts), opts.Width)...)
	}
	return n, out
}

// headingLevel is the number of leading #s of an ATX heading, or 0 when the
// line is not one. A # needs a space after it: #hashtag is prose.
func headingLevel(l string) int {
	s := strings.TrimLeft(l, " \t")
	n := 0
	for n < len(s) && s[n] == '#' {
		n++
	}
	if n == 0 || n > 6 {
		return 0
	}
	if n < len(s) && !isSpaceByte(s[n]) {
		return 0
	}
	return n
}

// heading renders one heading. The top two levels are underlined rather than
// left wearing their #s, because a rule under a line is what a terminal has
// instead of a type scale; deeper levels keep the #s, which say the level
// exactly and cost one line instead of two.
func heading(l string, opts Options) []string {
	level := headingLevel(l)
	s := strings.TrimLeft(l, " \t")
	text := trimClosingHashes(strings.TrimLeft(s[level:], " \t"))
	if text == "" {
		return nil
	}
	rendered := inline(text, opts)
	if opts.Color {
		rendered = boldOn + rendered + boldOff
	}
	switch level {
	case 1:
		return []string{rendered, strings.Repeat("=", displayWidth(rendered))}
	case 2:
		return []string{rendered, strings.Repeat("-", displayWidth(rendered))}
	}
	return []string{strings.Repeat("#", level) + " " + rendered}
}

// trimClosingHashes drops the optional run of #s that closes an ATX heading.
func trimClosingHashes(text string) string {
	t := strings.TrimRight(text, " \t")
	i := len(t)
	for i > 0 && t[i-1] == '#' {
		i--
	}
	switch {
	case i == len(t):
		return t // nothing closing it
	case i == 0:
		return "" // the line was all #s
	case isSpaceByte(t[i-1]):
		return strings.TrimRight(t[:i], " \t")
	}
	return t
}

// isThematicBreak reports whether the line is a horizontal rule: three or more
// of -, * or _, spaces allowed between them and nothing else.
func isThematicBreak(l string) bool {
	s := strings.TrimSpace(l)
	if len(s) < 3 {
		return false
	}
	c := s[0]
	if c != '-' && c != '*' && c != '_' {
		return false
	}
	n := 0
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case c:
			n++
		case ' ', '\t':
		default:
			return false
		}
	}
	return n >= 3
}

// quoteDepth is how many levels of > open the line.
func quoteDepth(l string) int {
	d, _ := splitQuote(l)
	return d
}

func splitQuote(l string) (depth int, text string) {
	i := 0
	for i < len(l) {
		for i < len(l) && isSpaceByte(l[i]) {
			i++
		}
		if i < len(l) && l[i] == '>' {
			depth++
			i++
			continue
		}
		break
	}
	return depth, l[i:]
}

// quote renders a run of quoted lines, each behind a │ per level.
//
// The bar is per line and per level, wrapped continuations included: the point
// of a quote in a terminal is that every line of it is visibly not yours.
func quote(lines []string, opts Options) (int, []string) {
	var out []string
	n := 0
	for ; n < len(lines); n++ {
		l := lines[n]
		if isBlank(l) {
			break
		}
		depth, text := splitQuote(l)
		if depth == 0 {
			break
		}
		prefix := strings.Repeat("│ ", depth)
		body := wrap(inline(strings.TrimSpace(text), opts), opts.Width-displayWidth(prefix))
		for _, line := range body {
			out = append(out, strings.TrimRight(prefix+line, " "))
		}
	}
	return n, out
}

// listItem is one entry of a list, already stripped of its Markdown marker.
type listItem struct {
	depth  int    // how many levels in, one level per two spaces or one tab
	marker string // what the terminal shows instead: "• ", "1. ", "☐ ", "☑ "
	text   string
}

func isListItem(l string) bool {
	_, ok := parseListItem(l)
	return ok
}

// parseListItem reads the marker of a list item.
//
// The bullet does not change with depth. A rule that says "• at every level"
// can be read off the output and tested; a rotating set of glyphs only tells
// the reader something they can already see from the indentation.
func parseListItem(l string) (listItem, bool) {
	indent := spacesAndTabs(l)
	spaces, depth := 0, 0
	for i := 0; i < indent; i++ {
		if l[i] == '\t' {
			depth++
			continue
		}
		spaces++
	}
	depth += spaces / 2
	rest := l[indent:]
	if len(rest) >= 2 && (rest[0] == '-' || rest[0] == '*' || rest[0] == '+') && isSpaceByte(rest[1]) {
		text := strings.TrimLeft(rest[2:], " \t")
		if marker, body, ok := taskMarker(text); ok {
			return listItem{depth: depth, marker: marker, text: body}, true
		}
		return listItem{depth: depth, marker: "• ", text: text}, true
	}
	digits := 0
	for digits < len(rest) && isDigitByte(rest[digits]) {
		digits++
	}
	if digits > 0 && digits+1 < len(rest) && (rest[digits] == '.' || rest[digits] == ')') && isSpaceByte(rest[digits+1]) {
		text := strings.TrimLeft(rest[digits+2:], " \t")
		return listItem{depth: depth, marker: rest[:digits] + ". ", text: text}, true
	}
	return listItem{}, false
}

// spacesAndTabs is the length of the indentation at the front of l.
func spacesAndTabs(l string) int {
	i := 0
	for i < len(l) && isSpaceByte(l[i]) {
		i++
	}
	return i
}

// taskMarker turns the checkbox of a task list item into the box a terminal
// can draw.
func taskMarker(text string) (marker, body string, ok bool) {
	if len(text) < 3 || text[0] != '[' || text[2] != ']' {
		return "", "", false
	}
	if len(text) > 3 && !isSpaceByte(text[3]) {
		return "", "", false
	}
	switch text[1] {
	case ' ':
		marker = "☐ "
	case 'x', 'X':
		marker = "☑ "
	default:
		return "", "", false
	}
	return marker, strings.TrimLeft(text[3:], " \t"), true
}

// list renders a run of list items. Continuation lines of an item fold into it
// the way they do in a paragraph, and a wrapped item lines up under its own
// text rather than under its marker.
func list(lines []string, opts Options) (int, []string) {
	var items []listItem
	n := 0
	for ; n < len(lines); n++ {
		l := lines[n]
		if isBlank(l) || isThematicBreak(l) || isFence(l) || headingLevel(l) > 0 || quoteDepth(l) > 0 {
			break
		}
		if item, ok := parseListItem(l); ok {
			items = append(items, item)
			continue
		}
		if len(items) == 0 {
			break
		}
		items[len(items)-1].text += " " + strings.TrimSpace(l)
	}
	var out []string
	for _, item := range items {
		indent := strings.Repeat("  ", item.depth)
		hang := indent + strings.Repeat(" ", displayWidth(item.marker))
		limit := opts.Width - displayWidth(hang)
		for k, line := range wrap(inline(item.text, opts), limit) {
			prefix := hang
			if k == 0 {
				prefix = indent + item.marker
			}
			out = append(out, strings.TrimRight(prefix+line, " "))
		}
	}
	return n, out
}

// isFence reports whether the line opens or closes a fenced code block.
func isFence(l string) bool {
	_, _, ok := fenceOf(l)
	return ok
}

func fenceOf(l string) (char byte, count int, ok bool) {
	s := strings.TrimLeft(l, " \t")
	if len(s) < 3 || (s[0] != '`' && s[0] != '~') {
		return 0, 0, false
	}
	c := s[0]
	n := 0
	for n < len(s) && s[n] == c {
		n++
	}
	if n < 3 {
		return 0, 0, false
	}
	return c, n, true
}

// codeBlock renders a fenced block.
//
// The contents are copied out untouched and never wrapped: code that has been
// folded at some column is code that no longer says what it said. The fence
// and its language tag are dropped, and the block is indented instead - that
// is what marks it as code on a terminal.
func codeBlock(lines []string, opts Options) (int, []string) {
	char, count, _ := fenceOf(lines[0])
	var out []string
	n := 1
	for ; n < len(lines); n++ {
		if closesFence(lines[n], char, count) {
			n++
			break
		}
		out = append(out, codeLine(lines[n], opts))
	}
	return n, out
}

// closesFence reports whether the line is a closing fence: the same character,
// at least as long as the opening run, with nothing else on the line.
func closesFence(l string, char byte, count int) bool {
	c, n, ok := fenceOf(l)
	if !ok || c != char || n < count {
		return false
	}
	return strings.Trim(strings.TrimSpace(l), string(char)) == ""
}

func codeLine(l string, opts Options) string {
	if l == "" {
		return "" // nothing to indent, and no colour to give it
	}
	if opts.Color {
		return "  " + codeOn + l + codeOff
	}
	return "  " + l
}
