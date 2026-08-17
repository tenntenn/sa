// Package tui reads a unified diff in the terminal.
//
// The diff arrives on stdin, so stdin is spoken for and cannot also carry key
// presses: the terminal is opened separately as /dev/tty and used for both the
// keys and the drawing. Nothing is written to stdout, which belongs to whoever
// piped the diff in. Without a controlling terminal there is nothing to drive,
// so Run refuses to start instead of reading the diff it is meant to show.
package tui

import (
	"fmt"
	"io"

	"github.com/tenntenn/sbnn/internal/model"
)

// DefaultWidth and DefaultHeight are the size of a dumped frame when no size
// is given. A terminal reports its own size; a pipe has none.
const (
	DefaultWidth  = 100
	DefaultHeight = 30
)

// Session is what a session starts with, beyond the diff itself. The zero
// value is the diff of the first file, read from nowhere: what sbnn tui did
// before there was anything else to show.
type Session struct {
	// BaseDir is the directory the diff paths are relative to.
	BaseDir string
	// Cursor is the file to start on.
	Cursor int
	// View is the mode to start in.
	View View
}

// start builds the state a session begins in.
func start(files []*model.File, o Session) *State {
	s := NewState(files)
	s.BaseDir = o.BaseDir
	if len(files) > 0 {
		s.Cursor = clamp(o.Cursor, 0, len(files)-1)
	}
	// SetView last, and through SetView: it is the cursor that decides
	// whether the view asked for is one this file has.
	s.SetView(o.View)
	return s
}

// Dump writes the first frame to w as plain text, one line per row, and opens
// no terminal at all. It is how the drawing is read where there is no TTY: a
// CI job, a test, a pipe into grep.
func Dump(w io.Writer, files []*model.File, width, height int, o Session) error {
	if width <= 0 {
		width = DefaultWidth
	}
	if height <= 0 {
		height = DefaultHeight
	}
	s := start(files, o)
	s.SetSize(width, height)
	for _, line := range Frame(s, width, height) {
		if _, err := fmt.Fprintln(w, line); err != nil {
			return err
		}
	}
	return nil
}

// Run shows the diff on the controlling terminal until the reader quits.
func Run(files []*model.File, o Session) error {
	tty, err := OpenTTY()
	if err != nil {
		return err
	}
	defer tty.Close()
	return runLoop(tty, start(files, o), newPalette(colourEnabled()))
}

// ParseView turns the name of a view into the view, and reports whether the
// name was one.
//
// The names are matched exactly. A flag is not prose: "Markdown" is a name
// the reader made up, and answering it with a guess is how a typo becomes a
// view nobody asked for.
func ParseView(name string) (View, bool) {
	switch name {
	case "diff":
		return ViewDiff, true
	case "markdown":
		return ViewMarkdown, true
	case "raw":
		return ViewRaw, true
	}
	return ViewDiff, false
}

// FindFile returns the index of the file whose path is path, and reports
// whether there was one. The match is exact: a prefix of a path is another
// file's name as often as it is this one's.
func FindFile(files []*model.File, path string) (int, bool) {
	for i, f := range files {
		if f.Path() == path {
			return i, true
		}
	}
	return 0, false
}
