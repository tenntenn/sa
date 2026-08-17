package tui

import (
	"fmt"
	"strings"
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

// wideFiles is a diff written in Japanese, where a character takes two cells
// and the count of runes is no longer the count of columns. The path is wide
// too, so both panes have to measure rather than count.
const wideFiles = `--- a/docs/日本語の設計メモ.md
+++ b/docs/日本語の設計メモ.md
@@ -1,4 +1,4 @@
 # 設計メモ
-これは古い説明文で、全角文字がたくさん並んでいる行である。
+これは新しい説明文で、全角文字がさらにたくさん並んでいる行である。
 末尾の行
--- a/README.md
+++ b/README.md
@@ -1,2 +1,2 @@
-old ascii line
+new ascii line
`

// escapedFiles is a diff carrying escape sequences of its own: colours, a
// cursor move, and a scroll region. Nothing drawn from it may reach the
// terminal as an escape, or reading a diff would let it repaint the screen of
// whoever is reading.
const escapedFiles = "--- a/paint.txt\n" +
	"+++ b/paint.txt\n" +
	"@@ -1,3 +1,3 @@\n" +
	" \x1b[2J plain\n" +
	"-\x1b[31mred\x1b[0m\n" +
	"+\x1b[1;32mgreen\x1b[0m\x1b[?1049h\n"

// longFirstDiff is a diff whose first file is far longer than any pane that
// will be asked to draw it, so that the diff pane can really be scrolled. The
// second file is short: what matters is moving between the two.
func longFirstDiff() string {
	var b strings.Builder
	b.WriteString("--- a/long.txt\n+++ b/long.txt\n@@ -1,60 +1,60 @@\n")
	for i := 1; i <= 60; i++ {
		fmt.Fprintf(&b, " line %d\n", i)
	}
	b.WriteString("--- a/short.txt\n+++ b/short.txt\n@@ -1,1 +1,1 @@\n-before\n+after\n")
	return b.String()
}

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

// A diff is read from its beginning. Scrolling one file and then choosing
// another must not drop the reader into the middle of the new one, so the
// scroll position belongs to the file and is reset with it.
func TestSelectFileResetsTop(t *testing.T) {
	// scrolled returns a state whose first file is scrolled away from its top
	// by the given key, having checked that the key really scrolled it: a
	// fixture that fits the pane would make everything below pass for nothing.
	scrolled := func(t *testing.T, key string) *State {
		t.Helper()
		s := newTestState(t, longFirstDiff())
		if len(s.Files) != 2 {
			t.Fatalf("the fixture parsed into %d files, want 2", len(s.Files))
		}
		if s.maxTop() <= 0 {
			t.Fatalf("the first file fits in %d lines: there is no scrolling to reset",
				s.bodyHeight())
		}
		s.Key("tab")
		if s.Focus != PaneDiff {
			t.Fatalf(`after "tab" focus = %v, want PaneDiff`, s.Focus)
		}
		s.Key(key)
		if s.Top <= 0 {
			t.Fatalf("top = %d after %q, want the diff scrolled off its first line", s.Top, key)
		}
		s.Key("tab")
		if s.Focus != PaneFiles {
			t.Fatalf(`after a second "tab" focus = %v, want PaneFiles`, s.Focus)
		}
		return s
	}

	t.Run("j after G", func(t *testing.T) {
		s := scrolled(t, "G")
		s.Key("j")
		if s.Cursor != 1 {
			t.Fatalf(`after "j" cursor = %d, want 1: no other file was selected`, s.Cursor)
		}
		if s.Top != 0 {
			t.Errorf("top = %d after selecting another file, want 0", s.Top)
		}
	})

	t.Run("j after ctrl+d", func(t *testing.T) {
		s := scrolled(t, "ctrl+d")
		s.Key("j")
		if s.Cursor != 1 {
			t.Fatalf(`after "j" cursor = %d, want 1`, s.Cursor)
		}
		if s.Top != 0 {
			t.Errorf("top = %d after selecting another file, want 0", s.Top)
		}
	})

	t.Run("G to the last file", func(t *testing.T) {
		s := scrolled(t, "G")
		s.Key("G")
		if want := len(s.Files) - 1; s.Cursor != want {
			t.Fatalf(`after "G" cursor = %d, want %d`, s.Cursor, want)
		}
		if s.Top != 0 {
			t.Errorf("top = %d after jumping to the last file, want 0", s.Top)
		}
	})

	// Choosing the file that is already chosen is not choosing another one:
	// the reader stays where they were reading.
	t.Run("the same file keeps its place", func(t *testing.T) {
		s := scrolled(t, "G")
		was := s.Top
		s.Key("g")
		if s.Cursor != 0 {
			t.Fatalf(`after "g" cursor = %d, want 0`, s.Cursor)
		}
		if s.Top != was {
			t.Errorf("top = %d after re-selecting the same file, want %d", s.Top, was)
		}
	})
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
