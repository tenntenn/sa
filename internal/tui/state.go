package tui

import "github.com/tenntenn/sbnn/internal/model"

// Pane names the two halves of the screen.
type Pane int

const (
	// PaneFiles is the file list on the left.
	PaneFiles Pane = iota
	// PaneDiff is the diff of the selected file on the right.
	PaneDiff
)

// State is what the reader is looking at. It holds no terminal and does no
// I/O, so every movement can be tested by calling Key and reading the fields.
type State struct {
	Files []*model.File
	// Cursor is the selected entry of the file list.
	Cursor int
	// Top is the first drawn line of the diff pane. A different file starts
	// at its own beginning, so selecting one resets it.
	Top   int
	Focus Pane
	// Width and Height are the size of the terminal, in cells.
	Width, Height int
}

// NewState returns the state a session starts in: the first file selected,
// the file list focused, and a default size until the terminal reports one.
func NewState(files []*model.File) *State {
	return &State{
		Files:  files,
		Focus:  PaneFiles,
		Width:  DefaultWidth,
		Height: DefaultHeight,
	}
}

// SetSize records the size of the terminal. A shorter screen can leave the
// diff pane scrolled past its end, so the scroll position is pulled back.
func (s *State) SetSize(width, height int) {
	s.Width, s.Height = width, height
	s.Top = clamp(s.Top, 0, s.maxTop())
}

// Current returns the selected file, or nil when there is none.
func (s *State) Current() *model.File {
	if s.Cursor < 0 || s.Cursor >= len(s.Files) {
		return nil
	}
	return s.Files[s.Cursor]
}

// Key applies one key press and reports whether the reader asked to leave.
// Keys that mean nothing here change nothing. The keys are named the way
// bubbletea names them, which is what lets Update hand its own strings over
// without reading them.
func (s *State) Key(key string) (quit bool) {
	switch key {
	case "q", "ctrl+c", "esc":
		return true
	case "j", "down":
		s.move(1)
	case "k", "up":
		s.move(-1)
	case "tab":
		if s.Focus == PaneFiles {
			s.Focus = PaneDiff
		} else {
			s.Focus = PaneFiles
		}
	case "g":
		if s.Focus == PaneFiles {
			s.selectFile(0)
		} else {
			s.Top = 0
		}
	case "G":
		if s.Focus == PaneFiles {
			s.selectFile(len(s.Files) - 1)
		} else {
			s.Top = s.maxTop()
		}
	case "ctrl+d":
		s.scroll(s.page())
	case "ctrl+u":
		s.scroll(-s.page())
	}
	return false
}

// move steps the focused pane by delta lines.
func (s *State) move(delta int) {
	if s.Focus == PaneFiles {
		s.selectFile(s.Cursor + delta)
		return
	}
	s.scroll(delta)
}

// selectFile moves the file list cursor, keeping it on a file that exists.
func (s *State) selectFile(i int) {
	if len(s.Files) == 0 {
		s.Cursor, s.Top = 0, 0
		return
	}
	i = clamp(i, 0, len(s.Files)-1)
	if i == s.Cursor {
		return
	}
	s.Cursor = i
	s.Top = 0
}

// scroll moves the diff pane by delta lines, never past either end.
func (s *State) scroll(delta int) {
	s.Top = clamp(s.Top+delta, 0, s.maxTop())
}

// bodyHeight is how many lines the panes get: the frame spends one line on
// the header and one on the footer.
func (s *State) bodyHeight() int {
	if h := s.Height - 2; h > 0 {
		return h
	}
	return 1
}

// page is how far ctrl+d and ctrl+u jump. One line of the old screen stays
// visible, so the reader keeps a foothold.
func (s *State) page() int {
	if p := s.bodyHeight() - 1; p > 0 {
		return p
	}
	return 1
}

// maxTop is the last scroll position that still shows a line of the diff.
func (s *State) maxTop() int {
	if t := diffLineCount(s.Current()) - s.bodyHeight(); t > 0 {
		return t
	}
	return 0
}

func clamp(v, lo, hi int) int {
	if hi < lo {
		hi = lo
	}
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
