package diff

import (
	"fmt"
	"strings"

	"github.com/tenntenn/sa/internal/model"
)

// GapMarker is inserted where the diff does not tell us what the file looks
// like. Unified diffs only carry the changed hunks and their context, so a
// modified file can only be reconstructed partially.
const GapMarker = "<!-- sa: %d line(s) not included in the diff -->"

// Reconstruct rebuilds the new side of a file from its hunks.
//
// complete reports whether the result is the whole file. It is true for added
// files (the diff contains every line) and for diffs whose hunks happen to
// cover the file from line 1 without gaps.
func Reconstruct(f *model.File) (content string, complete bool) {
	var b strings.Builder
	complete = true
	next := 1 // the next new-side line number we expect to write

	for _, h := range f.Hunks {
		if h.NewStart > next {
			complete = false
			fmt.Fprintf(&b, "\n"+GapMarker+"\n\n", h.NewStart-next)
		}
		for _, l := range h.Lines {
			if l.Kind == model.LineDelete {
				continue
			}
			b.WriteString(l.Content)
			b.WriteString("\n")
			if l.NewNumber > 0 {
				next = l.NewNumber + 1
			}
		}
	}
	if f.Status == model.StatusAdded && len(f.Hunks) > 0 {
		// An added file is fully described by its hunks.
		complete = true
	}
	return b.String(), complete
}

// Snippet returns the lines of a file between start and end on the given
// side, each keeping its diff marker. It is what a comment stores so that it
// still says something outside the browser.
func Snippet(f *model.File, side string, start, end int) string {
	var b strings.Builder
	for _, h := range f.Hunks {
		for _, l := range h.Lines {
			num := l.NewNumber
			if side == "old" {
				num = l.OldNumber
			}
			if num < start || num > end {
				continue
			}
			if side == "old" && l.Kind == model.LineAdd {
				continue
			}
			if side != "old" && l.Kind == model.LineDelete {
				continue
			}
			b.WriteString(marker(l.Kind))
			b.WriteString(l.Content)
			b.WriteString("\n")
		}
	}
	return strings.TrimRight(b.String(), "\n")
}

func marker(kind model.LineKind) string {
	switch kind {
	case model.LineAdd:
		return "+"
	case model.LineDelete:
		return "-"
	default:
		return " "
	}
}
