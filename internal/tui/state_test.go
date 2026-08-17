package tui

import (
	"testing"

	"github.com/tenntenn/sbnn/internal/diff"
)

// twoFiles is a diff of two files: one with a change on each side, one added.
const twoFiles = `--- a/foo.txt
+++ b/foo.txt
@@ -1,3 +1,3 @@
 keep
-old
+new
 tail
--- /dev/null
+++ b/bar.txt
@@ -0,0 +1,2 @@
+first
+second
`

func newTestState(t *testing.T, src string) *State {
	t.Helper()
	files := diff.Parse(src)
	if len(files) == 0 {
		t.Fatalf("the test diff parsed into no file")
	}
	return NewState(files)
}

func TestStateKeyMovesCursor(t *testing.T) {
	s := newTestState(t, twoFiles)
	if len(s.Files) != 2 {
		t.Fatalf("got %d files, want 2", len(s.Files))
	}
	if s.Cursor != 0 {
		t.Fatalf("cursor starts at %d, want 0", s.Cursor)
	}

	s.Key("j")
	if s.Cursor != 1 {
		t.Errorf(`after "j" cursor = %d, want 1`, s.Cursor)
	}
	// The last file is the last one: j does not walk off the end.
	s.Key("j")
	if s.Cursor != 1 {
		t.Errorf(`after a second "j" cursor = %d, want 1`, s.Cursor)
	}
	s.Key("down")
	if s.Cursor != 1 {
		t.Errorf(`after "down" cursor = %d, want 1`, s.Cursor)
	}

	s.Key("k")
	if s.Cursor != 0 {
		t.Errorf(`after "k" cursor = %d, want 0`, s.Cursor)
	}
	s.Key("k")
	if s.Cursor != 0 {
		t.Errorf(`after a second "k" cursor = %d, want 0`, s.Cursor)
	}

	// Scrolling belongs to the diff pane, so j moves Top once it has focus,
	// and the cursor stays where it was.
	s.Key("tab")
	if s.Focus != PaneDiff {
		t.Fatalf(`after "tab" focus = %v, want PaneDiff`, s.Focus)
	}
	s.Key("j")
	if s.Cursor != 0 {
		t.Errorf("cursor moved with the diff pane focused: %d", s.Cursor)
	}
	if s.Top != 0 {
		t.Errorf("top = %d, want 0: the diff is shorter than the pane", s.Top)
	}
}

func TestStateKeyQuits(t *testing.T) {
	for _, key := range []string{"q", "ctrl+c", "esc"} {
		s := newTestState(t, twoFiles)
		if quit := s.Key(key); !quit {
			t.Errorf("Key(%q) = false, want true", key)
		}
	}
	s := newTestState(t, twoFiles)
	for _, key := range []string{"j", "k", "tab", "g", "G", "ctrl+d", "ctrl+u", "x", ""} {
		if quit := s.Key(key); quit {
			t.Errorf("Key(%q) = true, want false", key)
		}
	}
}

func TestStateEmptyFilesDoNotPanic(t *testing.T) {
	s := NewState(nil)
	for _, key := range []string{"j", "k", "down", "up", "tab", "g", "G", "ctrl+d", "ctrl+u"} {
		if quit := s.Key(key); quit {
			t.Errorf("Key(%q) = true, want false", key)
		}
	}
	if s.Cursor != 0 || s.Top != 0 {
		t.Errorf("cursor/top = %d/%d, want 0/0", s.Cursor, s.Top)
	}
	if f := s.Current(); f != nil {
		t.Errorf("Current() = %+v, want nil", f)
	}
	if got := Frame(s, 100, 30); len(got) != 30 {
		t.Errorf("Frame returned %d lines, want 30", len(got))
	}
}
