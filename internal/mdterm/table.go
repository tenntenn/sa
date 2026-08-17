package mdterm

import "strings"

// align is how a column's cells sit inside the column's width.
type align int

const (
	alignLeft align = iota
	alignCenter
	alignRight
)

// isTableStart reports whether a table begins at lines[i].
//
// A table is only a table when the delimiter row under the header agrees with
// it about how many columns there are. That is the GFM rule, and it is what
// keeps a paragraph that happens to contain a pipe from being drawn as a
// mangled table.
func isTableStart(lines []string, i int) bool {
	if i+1 >= len(lines) || !strings.Contains(lines[i], "|") {
		return false
	}
	aligns := parseAligns(lines[i+1])
	return aligns != nil && len(aligns) == len(splitRow(lines[i]))
}

// table renders a pipe table, padded so the columns line up.
//
// Nothing here is wrapped, even past Width. A table that has been folded at
// some column is no longer a table, and a reader can scroll sideways; they
// cannot unscramble rows.
func table(lines []string, opts Options) (int, []string) {
	aligns := parseAligns(lines[1])
	rows := [][]string{splitRow(lines[0])}
	n := 2
	for ; n < len(lines); n++ {
		l := lines[n]
		if isBlank(l) || !strings.Contains(l, "|") || isFence(l) || headingLevel(l) > 0 {
			break
		}
		rows = append(rows, splitRow(l))
	}

	// Every row is rendered before anything is measured: the width of a
	// column is the width of what will actually be printed in it, which is
	// not the width of the Markdown that produced it.
	cols := len(aligns)
	widths := make([]int, cols)
	rendered := make([][]string, len(rows))
	for r, row := range rows {
		cells := make([]string, cols)
		for c := 0; c < cols; c++ {
			if c < len(row) {
				cells[c] = inline(row[c], opts)
			}
			if w := displayWidth(cells[c]); w > widths[c] {
				widths[c] = w
			}
		}
		rendered[r] = cells
	}
	for c, w := range widths {
		if w < 1 {
			widths[c] = 1 // an entirely empty column still needs a column
		}
	}

	out := make([]string, 0, len(rendered)+1)
	out = append(out, tableRow(rendered[0], widths, aligns))
	rules := make([]string, cols)
	for c, w := range widths {
		rules[c] = strings.Repeat("-", w)
	}
	out = append(out, strings.Join(rules, "-+-"))
	for _, cells := range rendered[1:] {
		out = append(out, tableRow(cells, widths, aligns))
	}
	return n, out
}

func tableRow(cells []string, widths []int, aligns []align) string {
	padded := make([]string, len(cells))
	for c, cell := range cells {
		padded[c] = pad(cell, widths[c], aligns[c])
	}
	return strings.TrimRight(strings.Join(padded, " | "), " ")
}

func pad(s string, width int, a align) string {
	gap := width - displayWidth(s)
	if gap <= 0 {
		return s
	}
	switch a {
	case alignRight:
		return strings.Repeat(" ", gap) + s
	case alignCenter:
		left := gap / 2
		return strings.Repeat(" ", left) + s + strings.Repeat(" ", gap-left)
	}
	return s + strings.Repeat(" ", gap)
}

// parseAligns reads the delimiter row, or returns nil when the line is not one.
func parseAligns(l string) []align {
	if !strings.Contains(l, "-") {
		return nil
	}
	cells := splitRow(l)
	aligns := make([]align, 0, len(cells))
	for _, cell := range cells {
		body := cell
		left := strings.HasPrefix(body, ":")
		right := strings.HasSuffix(body, ":")
		body = strings.TrimPrefix(body, ":")
		body = strings.TrimSuffix(body, ":")
		if body == "" || strings.Trim(body, "-") != "" {
			return nil
		}
		switch {
		case left && right:
			aligns = append(aligns, alignCenter)
		case right:
			aligns = append(aligns, alignRight)
		default:
			aligns = append(aligns, alignLeft)
		}
	}
	if len(aligns) == 0 {
		return nil
	}
	return aligns
}

// splitRow cuts one table row into its cells. An escaped pipe belongs to the
// cell it is written in; the escape itself is resolved later, when the cell is
// rendered as inline text.
func splitRow(l string) []string {
	s := strings.TrimSpace(l)
	s = strings.TrimPrefix(s, "|")
	if strings.HasSuffix(s, "|") && !strings.HasSuffix(s, "\\|") {
		s = s[:len(s)-1]
	}
	var cells []string
	var cur strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] == '\\' && i+1 < len(s) {
			cur.WriteByte(s[i])
			cur.WriteByte(s[i+1])
			i++
			continue
		}
		if s[i] == '|' {
			cells = append(cells, strings.TrimSpace(cur.String()))
			cur.Reset()
			continue
		}
		cur.WriteByte(s[i])
	}
	return append(cells, strings.TrimSpace(cur.String()))
}
