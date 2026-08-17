package tui

// decodeKeys turns the bytes of one read into the key names State understands.
// One read can hold several keys, and one key can span several bytes, so the
// two are not the same count; a key the terminal spells in a way we do not
// know is dropped rather than guessed at.
//
// The names are the ones State.Key already switches on, which is what lets the
// terminal and the state stay strangers to each other.
func decodeKeys(b []byte) []string {
	var keys []string
	for i := 0; i < len(b); {
		switch c := b[i]; {
		case c == 0x1b:
			name, n := decodeEscape(b[i:])
			if name != "" {
				keys = append(keys, name)
			}
			i += n
		case c == 0x03:
			keys = append(keys, "ctrl+c")
			i++
		case c == 0x04:
			keys = append(keys, "ctrl+d")
			i++
		case c == 0x15:
			keys = append(keys, "ctrl+u")
			i++
		case c == 0x09:
			keys = append(keys, "tab")
			i++
		case c >= 0x20 && c < 0x7f:
			keys = append(keys, string(rune(c)))
			i++
		default:
			// Another control byte, or part of a character outside ASCII. No
			// key here is bound to either, so it is read and let go.
			i++
		}
	}
	return keys
}

// decodeEscape reads one sequence starting at the escape byte and returns its
// key name, empty when the sequence means nothing here, along with how many
// bytes it took. It never returns zero, so the caller always moves on.
func decodeEscape(b []byte) (name string, n int) {
	if len(b) < 2 {
		// Nothing came after it: the reader pressed Escape.
		return "esc", 1
	}
	switch b[1] {
	case '[':
		return decodeCSI(b)
	case 'O':
		// SS3, which is how a terminal in application-cursor mode spells the
		// arrow keys.
		if len(b) < 3 {
			return "", len(b)
		}
		switch b[2] {
		case 'A':
			return "up", 3
		case 'B':
			return "down", 3
		}
		return "", 3
	}
	// An escape with an ordinary byte behind it is Alt+that key on most
	// terminals. Nothing here is bound to one, so the escape is taken at face
	// value and the byte after it is decoded on its own.
	return "esc", 1
}

// decodeCSI reads a control sequence: parameter bytes, then intermediate
// bytes, then the one byte that says what it was.
func decodeCSI(b []byte) (name string, n int) {
	i := 2
	for i < len(b) && b[i] >= 0x30 && b[i] <= 0x3f {
		i++
	}
	for i < len(b) && b[i] >= 0x20 && b[i] <= 0x2f {
		i++
	}
	if i >= len(b) {
		// The sequence was cut in half by the end of the buffer. Dropping the
		// tail is better than reading it as the keys it happens to spell.
		return "", len(b)
	}
	final := b[i]
	i++
	switch final {
	case 'A':
		return "up", i
	case 'B':
		return "down", i
	}
	// Left, right, page keys, mouse reports: read and dropped, so that they
	// cannot arrive as the letters they are written with.
	return "", i
}
