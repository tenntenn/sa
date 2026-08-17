package tui

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"syscall"

	xterm "github.com/charmbracelet/x/term"
)

// The loop below is the whole terminal driver: raw mode, the alternate screen,
// a read, a repaint. It is written out rather than taken from a library
// because the libraries that do this query the terminal from a package init,
// which stalls every subcommand of sbnn for five seconds on a terminal that
// does not answer. What is needed here is small enough to own.
const (
	enterAltScreen = "\x1b[?1049h"
	leaveAltScreen = "\x1b[?1049l"
	hideCursor     = "\x1b[?25l"
	showCursor     = "\x1b[?25h"
	// cursorHome starts a frame. It is written once per frame, and the frame
	// that follows overwrites the screen row by row: nothing is scrolled and
	// nothing is cleared wholesale, so the picture does not flicker.
	cursorHome = "\x1b[H"
	// clearLine wipes what the previous frame left past the end of a row.
	clearLine = "\x1b[K"
	// readBufferSize is one read of keys. An escape sequence is a handful of
	// bytes and a held-down key repeats; this holds a burst of either.
	readBufferSize = 64
)

// keyEvent is one read of the terminal: the keys it decoded to, or why the
// reading stopped.
type keyEvent struct {
	keys []string
	err  error
}

// runLoop shows s on tty until the reader quits, and gives the terminal back
// the way it found it. Nothing here touches stdin or stdout: stdin carried the
// diff in and stdout belongs to whoever piped it.
func runLoop(tty *os.File, s *State, p palette) error {
	fd := tty.Fd()
	saved, err := xterm.MakeRaw(fd)
	if err != nil {
		return fmt.Errorf("cannot put the terminal in raw mode: %w", err)
	}
	// The terminal is given back whatever happens next, including a panic:
	// leaving a reader with no echo and no line editing is worse than the
	// error that got us here.
	defer xterm.Restore(fd, saved)

	if _, err := io.WriteString(tty, enterAltScreen+hideCursor); err != nil {
		return err
	}
	defer io.WriteString(tty, showCursor+leaveAltScreen)

	resize := make(chan os.Signal, 1)
	notifyResize(resize)
	defer stopResize(resize)

	// A signal from outside means the same as pressing q: leave, and put the
	// terminal back first. Ctrl-C is not among them - in raw mode it arrives
	// as the byte 0x03 and is decoded as a key.
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(quit)

	// The read runs on its own goroutine so that a resize is not stuck behind
	// a key that nobody is going to press.
	done := make(chan struct{})
	defer close(done)
	events := make(chan keyEvent, 1)
	go readKeys(tty, events, done)

	fitToTerminal(tty, s)
	if err := paintFrame(tty, s, p); err != nil {
		return err
	}
	for {
		select {
		case <-quit:
			return nil
		case <-resize:
			fitToTerminal(tty, s)
		case ev := <-events:
			if ev.err != nil {
				if errors.Is(ev.err, io.EOF) {
					return nil
				}
				return ev.err
			}
			for _, key := range ev.keys {
				if s.Key(key) {
					return nil
				}
			}
		}
		if err := paintFrame(tty, s, p); err != nil {
			return err
		}
	}
}

// fitToTerminal asks the terminal how big it is. One that will not say gets
// the default size rather than an error: a frame of the wrong size is still
// readable, and there is nothing else to show instead.
func fitToTerminal(tty *os.File, s *State) {
	width, height, err := xterm.GetSize(tty.Fd())
	if err != nil || width <= 0 || height <= 0 {
		width, height = DefaultWidth, DefaultHeight
	}
	s.SetSize(width, height)
}

// paintFrame writes one frame: home, then every row, each wiped to its end.
// The whole frame goes out in one write, so a reader never catches the screen
// half drawn.
func paintFrame(w io.Writer, s *State, p palette) error {
	var b strings.Builder
	b.WriteString(cursorHome)
	for i, line := range p.Paint(s, s.Width, s.Height) {
		if i > 0 {
			// Raw mode turns off the newline-to-CRLF translation, so the
			// carriage return is ours to write.
			b.WriteString("\r\n")
		}
		b.WriteString(line)
		b.WriteString(clearLine)
	}
	_, err := io.WriteString(w, b.String())
	return err
}

// readKeys turns the terminal into a stream of key names until it closes or
// the loop it feeds goes away.
func readKeys(tty *os.File, out chan<- keyEvent, done <-chan struct{}) {
	buf := make([]byte, readBufferSize)
	for {
		n, err := tty.Read(buf)
		var ev keyEvent
		if n > 0 {
			ev.keys = decodeKeys(buf[:n])
		}
		ev.err = err
		select {
		case out <- ev:
		case <-done:
			return
		}
		if err != nil {
			return
		}
	}
}
