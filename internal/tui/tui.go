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

// Dump writes the first frame to w as plain text, one line per row, and opens
// no terminal at all. It is how the drawing is read where there is no TTY: a
// CI job, a test, a pipe into grep.
func Dump(w io.Writer, files []*model.File, width, height int) error {
	if width <= 0 {
		width = DefaultWidth
	}
	if height <= 0 {
		height = DefaultHeight
	}
	s := NewState(files)
	s.SetSize(width, height)
	for _, line := range Frame(s, width, height) {
		if _, err := fmt.Fprintln(w, line); err != nil {
			return err
		}
	}
	return nil
}

// Run shows the diff on the controlling terminal until the reader quits.
func Run(files []*model.File) error {
	tty, err := OpenTTY()
	if err != nil {
		return err
	}
	defer tty.Close()
	return runLoop(tty, NewState(files), newPalette(colourEnabled()))
}
