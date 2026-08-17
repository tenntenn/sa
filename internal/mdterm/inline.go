package mdterm

import "strings"

// The escapes this package emits, and nothing else. Each "on" code is paired
// with the code that turns just that attribute off again, never with the
// blanket reset \x1b[0m: a bold word inside a cyan line has to leave the line
// cyan when it ends.
const (
	boldOn    = "\x1b[1m"
	boldOff   = "\x1b[22m"
	italicOn  = "\x1b[3m"
	italicOff = "\x1b[23m"
	strikeOn  = "\x1b[9m"
	strikeOff = "\x1b[29m"
	codeOn    = "\x1b[36m"
	codeOff   = "\x1b[39m"
)

// maxNesting is how deep emphasis inside emphasis is followed. Two levels is
// more than review comments use, and a limit means malformed input cannot
// drive the recursion.
const maxNesting = 4

// inline renders the inline markup of one logical line of text.
//
// Markup that does not resolve - an unclosed **, a reference link, an HTML
// tag - is written out as it was typed. This is the rule the whole package
// follows: the text under review is the thing the reader came for, and
// dropping a character they wrote is worse than showing markup they did not
// want rendered.
func inline(s string, opts Options) string {
	return renderInline(s, opts, 0)
}

func renderInline(s string, opts Options, depth int) string {
	var b strings.Builder
	for i := 0; i < len(s); {
		switch c := s[i]; c {
		case '\\':
			// A backslash escape shows the next character as itself. Only
			// punctuation can be escaped, so a Windows path keeps its
			// separators instead of losing them one by one.
			if i+1 < len(s) && isASCIIPunct(s[i+1]) {
				b.WriteByte(s[i+1])
				i += 2
				continue
			}
			b.WriteByte(c)
			i++
		case '`':
			if n, out, ok := codeSpan(s[i:], opts); ok {
				b.WriteString(out)
				i += n
				continue
			}
			b.WriteByte(c)
			i++
		case '*', '_', '~':
			if n, out, ok := emphasis(s, i, opts, depth); ok {
				b.WriteString(out)
				i += n
				continue
			}
			b.WriteByte(c)
			i++
		case '!', '[':
			if n, out, ok := link(s, i, opts, depth); ok {
				b.WriteString(out)
				i += n
				continue
			}
			b.WriteByte(c)
			i++
		case '<':
			if n, out, ok := autolink(s[i:]); ok {
				b.WriteString(out)
				i += n
				continue
			}
			b.WriteByte(c)
			i++
		default:
			// Bytes of a multi-byte rune are all >= 0x80, so they never
			// collide with the cases above and can be copied one at a time.
			b.WriteByte(c)
			i++
		}
	}
	return b.String()
}

// codeSpan renders the code span starting at the backtick run at the front of
// s, and reports how many bytes of s it consumed.
//
// Nothing inside a code span is markup: it is quoted source, and an asterisk
// in quoted source is an asterisk. Without colour the backticks stay, because
// they are the only thing left that says "this is code".
func codeSpan(s string, opts Options) (n int, out string, ok bool) {
	open := 0
	for open < len(s) && s[open] == '`' {
		open++
	}
	rest := s[open:]
	for i := 0; i < len(rest); {
		if rest[i] != '`' {
			i++
			continue
		}
		j := i
		for j < len(rest) && rest[j] == '`' {
			j++
		}
		if j-i == open {
			content := rest[:i]
			if opts.Color {
				return open + i + open, codeOn + content + codeOff, true
			}
			fence := s[:open]
			return open + i + open, fence + content + fence, true
		}
		i = j
	}
	return 0, "", false
}

// emphasis renders the bold, italic or struck-through run that opens at s[i].
//
// Underscores only mark emphasis at the edge of a word. Identifiers and file
// names full of them arrive here constantly, and snake_case_names must survive
// the trip unchanged; asterisks carry no such restriction.
func emphasis(s string, i int, opts Options, depth int) (n int, out string, ok bool) {
	if depth >= maxNesting {
		return 0, "", false
	}
	c := s[i]
	run := 0
	for i+run < len(s) && s[i+run] == c {
		run++
	}
	switch {
	case c == '~':
		if run < 2 {
			return 0, "", false // a lone tilde is a tilde
		}
		run = 2
	case run > 3:
		run = 3
	}
	delim := s[i : i+run]
	if c == '_' && i > 0 && isWordByte(s[i-1]) {
		return 0, "", false
	}
	closeAt := findCloser(s, i+run, delim)
	if closeAt < 0 {
		return 0, "", false
	}
	content := s[i+run : closeAt]
	if content == "" || isSpaceByte(content[0]) || isSpaceByte(content[len(content)-1]) {
		return 0, "", false
	}
	end := closeAt + run
	if c == '_' && end < len(s) && isWordByte(s[end]) {
		return 0, "", false
	}
	inner := renderInline(content, opts, depth+1)
	if !opts.Color {
		return end - i, inner, true
	}
	on, off := emphasisCodes(c, run)
	return end - i, on + inner + off, true
}

func emphasisCodes(c byte, run int) (on, off string) {
	switch {
	case c == '~':
		return strikeOn, strikeOff
	case run >= 3:
		return boldOn + italicOn, italicOff + boldOff
	case run == 2:
		return boldOn, boldOff
	}
	return italicOn, italicOff
}

// findCloser is the index of the delimiter that closes the run opened before
// from, or -1 when the markup never closes. Escapes and code spans are stepped
// over so that a delimiter quoted inside them does not close anything.
func findCloser(s string, from int, delim string) int {
	for i := from; i < len(s); {
		switch s[i] {
		case '\\':
			i += 2
		case '`':
			if n, _, ok := codeSpan(s[i:], Options{}); ok {
				i += n
				continue
			}
			i++
		case delim[0]:
			j := i
			for j < len(s) && s[j] == delim[0] {
				j++
			}
			if j-i >= len(delim) && !isSpaceByte(s[i-1]) {
				return i
			}
			i = j
		default:
			i++
		}
	}
	return -1
}

// link renders a link or an image as text: there is nothing clickable to hide
// a URL behind here, so both parts are shown.
func link(s string, i int, opts Options, depth int) (n int, out string, ok bool) {
	image := s[i] == '!'
	open := i
	if image {
		if i+1 >= len(s) || s[i+1] != '[' {
			return 0, "", false
		}
		open = i + 1
	}
	end := closingBracket(s, open)
	if end < 0 || end+1 >= len(s) || s[end+1] != '(' {
		return 0, "", false // a reference link, or not a link at all
	}
	url, after, found := linkURL(s, end+2)
	if !found {
		return 0, "", false
	}
	label := renderInline(s[open+1:end], opts, depth+1)
	if !image {
		return after - i, label + " (" + url + ")", true
	}
	if label == "" {
		return after - i, "[image] (" + url + ")", true
	}
	return after - i, "[image: " + label + "] (" + url + ")", true
}

// closingBracket is the index of the ] that closes the [ at open, allowing
// brackets to nest inside the label.
func closingBracket(s string, open int) int {
	level := 0
	for i := open; i < len(s); {
		switch s[i] {
		case '\\':
			i += 2
		case '`':
			if n, _, ok := codeSpan(s[i:], Options{}); ok {
				i += n
				continue
			}
			i++
		case '[':
			level++
			i++
		case ']':
			level--
			if level == 0 {
				return i
			}
			i++
		default:
			i++
		}
	}
	return -1
}

// linkURL reads the destination of a link, starting just after its (, and
// returns the index just past the closing ). A title after the destination is
// dropped: it exists to be shown on hover, and there is no hovering here.
func linkURL(s string, from int) (url string, after int, ok bool) {
	level := 1
	i := from
	for ; i < len(s); i++ {
		switch s[i] {
		case '\\':
			i++
		case '(':
			level++
		case ')':
			level--
		}
		if level == 0 {
			break
		}
	}
	if i >= len(s) {
		return "", 0, false
	}
	url = strings.TrimSpace(s[from:i])
	if cut := strings.IndexAny(url, " \t"); cut >= 0 {
		url = url[:cut]
	}
	url = strings.TrimSuffix(strings.TrimPrefix(url, "<"), ">")
	return url, i + 1, true
}

// autolink renders <https://example.com> as the address itself.
//
// The requirement of a scheme is what keeps raw HTML out: <b> has no colon, so
// it is not a link, and it falls through to being printed as the text it is.
func autolink(s string) (n int, out string, ok bool) {
	end := strings.IndexByte(s, '>')
	if end < 0 {
		return 0, "", false
	}
	inner := s[1:end]
	if inner == "" || strings.ContainsAny(inner, " \t<") {
		return 0, "", false
	}
	colon := strings.IndexByte(inner, ':')
	if colon <= 0 {
		return 0, "", false
	}
	for k := 0; k < colon; k++ {
		c := inner[k]
		alpha := c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z'
		if alpha {
			continue
		}
		if k > 0 && (isDigitByte(c) || c == '+' || c == '.' || c == '-') {
			continue
		}
		return 0, "", false
	}
	return end + 1, inner, true
}

func isASCIIPunct(c byte) bool {
	return strings.IndexByte("!\"#$%&'()*+,-./:;<=>?@[\\]^_`{|}~", c) >= 0
}

func isSpaceByte(c byte) bool {
	return c == ' ' || c == '\t'
}

func isDigitByte(c byte) bool {
	return c >= '0' && c <= '9'
}

// isWordByte reports whether c can be part of a word. Every byte of a
// multi-byte rune counts as one: emphasis with underscores is not recognised
// inside a Japanese word either.
func isWordByte(c byte) bool {
	return isDigitByte(c) || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= 0x80
}
