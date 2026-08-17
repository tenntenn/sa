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

// View is what the right pane shows.
type View int

const (
	// ViewDiff is the unified diff of the selected file: the default, and
	// the only view a file that is not Markdown has.
	ViewDiff View = iota
	// ViewMarkdown is the Markdown of the selected file, drawn for a
	// terminal.
	ViewMarkdown
	// ViewRaw is the Markdown of the selected file as it was written.
	ViewRaw
)

// nextView is the view the preview key moves on to. The three of them make a
// ring, so the key that leaves the diff is also the key that comes back to it.
func nextView(v View) View {
	switch v {
	case ViewDiff:
		return ViewMarkdown
	case ViewMarkdown:
		return ViewRaw
	default:
		return ViewDiff
	}
}

// State is what the reader is looking at. It holds no terminal and does no
// I/O of its own, so every movement can be tested by calling Key and reading
// the fields. The one thing it does read is the previewed file, and that is
// read through PaneBody, at most once per file, view and width.
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
	// View is what the right pane shows. The zero value is the diff, which
	// is what sbnn tui showed before there was anything else.
	View View
	// BaseDir is the directory the diff paths are relative to. Empty means
	// the working tree is not read at all and a preview is rebuilt from the
	// diff, which is what a session with nowhere to read from does.
	BaseDir string
	// preview holds the lines each preview drew, so that a key press does
	// not read the file again. It belongs to this session: a package-level
	// cache would outlive the diff it was built from.
	preview map[previewKey][]string
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

// Previewable reports whether the selected file has a Markdown preview.
//
// The rule is the one the server side previews by, kept the same on purpose:
// a file the browser offers a preview of and the terminal does not is a file
// the reader has to remember two answers about. A file that is not Markdown,
// a binary one, and one that was deleted have no new side worth drawing.
func (s *State) Previewable() bool {
	return previewable(s.Current())
}

// previewable is Previewable for a file that is not the selected one.
func previewable(f *model.File) bool {
	return f != nil && f.IsMarkdown && !f.IsBinary && f.Status != model.StatusDeleted
}

// SetView shows v in the right pane, starting at its first line. A view there
// is nothing to draw falls back to the diff rather than showing an empty pane,
// so asking for a preview of a Go file is answered with the diff of it.
func (s *State) SetView(v View) {
	if v != ViewDiff && !s.Previewable() {
		v = ViewDiff
	}
	s.View = v
	s.Top = 0
}

// Key applies one key press and reports whether the reader asked to leave.
// Keys that mean nothing here change nothing, so decodeKeys can hand over
// whatever it read without having to know what this switch covers.
func (s *State) Key(key string) (quit bool) {
	switch key {
	case "q", "ctrl+c", "esc":
		return true
	case "p":
		// A file with no preview keeps the view it has: the key is dead
		// rather than a way to scroll the diff back to its top.
		if s.Previewable() {
			s.SetView(nextView(s.View))
		}
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
	// The view belongs to the session, not to the file, so it is kept when
	// the next file can be drawn that way. When it cannot, the pane would
	// have nothing in it, and the diff is what there is to show instead.
	// Moving on to a Markdown file does not turn a preview on by itself:
	// the reader asked for a diff and is still reading diffs.
	if !s.Previewable() {
		s.View = ViewDiff
	}
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

// maxTop is the last scroll position that still shows a line of the right
// pane. It counts what that pane is actually drawing: a rendered Markdown
// file is longer than the diff of it whenever the diff carried a few changed
// lines of a long document, and shorter whenever it carried a hunk of a
// paragraph, so a limit taken from the diff would either cut the preview
// short or scroll it past its end.
func (s *State) maxTop() int {
	n := diffLineCount(s.Current())
	if s.View != ViewDiff {
		n = len(s.PaneBody(s.paneWidth()))
	}
	if t := n - s.bodyHeight(); t > 0 {
		return t
	}
	return 0
}

// paneWidth is how many cells the right pane gets on a screen this wide.
func (s *State) paneWidth() int {
	_, diff := paneWidths(s.Width)
	return diff
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
