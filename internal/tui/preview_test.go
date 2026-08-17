package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tenntenn/sbnn/internal/diff"
	"github.com/tenntenn/sbnn/internal/model"
)

// markdownFiles is a diff of a Markdown file and a Go file. The Markdown is
// changed on both sides, so the diff of it has a line to colour green and a
// line to colour red, and it carries the things a preview has to get right: a
// heading, emphasis, inline code, bullets that open with the character a
// deleted line opens with, a fenced block that keeps those characters where
// the renderer would otherwise eat them, and an escape sequence somebody typed
// into the document. The Go file is second, so moving off the Markdown is one
// key press away.
var markdownFiles = "--- a/note.md\n" +
	"+++ b/note.md\n" +
	"@@ -1,11 +1,12 @@\n" +
	" # Title\n" +
	" \n" +
	"-plain old text\n" +
	"+text with **bold** and `code`\n" +
	" \n" +
	" - one\n" +
	" + plus\n" +
	" \n" +
	" ```\n" +
	" - one\n" +
	" + plus\n" +
	" ```\n" +
	"+esc \x1b[31mred\x1b[0m\n" +
	"--- a/main.go\n" +
	"+++ b/main.go\n" +
	"@@ -1,3 +1,3 @@\n" +
	" package main\n" +
	"-old\n" +
	"+new\n"

// addedMarkdown is a Markdown file the diff carries whole, because it is new:
// there is no part of it left out, so no preview of it is partial.
const addedMarkdown = "--- /dev/null\n" +
	"+++ b/note.md\n" +
	"@@ -0,0 +1,3 @@\n" +
	"+# Title\n" +
	"+\n" +
	"+text with **bold** and `code`\n"

// partialMarkdown is a hunk out of the middle of a Markdown file that is not
// in the working tree. Four lines of the document never arrived, so whatever
// is drawn from it is part of a file rather than a file.
const partialMarkdown = "--- a/doc.md\n" +
	"+++ b/doc.md\n" +
	"@@ -5,2 +5,2 @@\n" +
	" keep\n" +
	"-old\n" +
	"+new\n"

// longMarkdownDiff is one paragraph on one line: two lines of diff that draw
// into far more than two lines of preview once they are folded to a pane.
func longMarkdownDiff() string {
	var b strings.Builder
	b.WriteString("--- /dev/null\n+++ b/long.md\n@@ -0,0 +1,1 @@\n+")
	for i := 0; i < 40; i++ {
		b.WriteString("A paragraph long enough that it has to be folded many times over. ")
	}
	b.WriteString("\n")
	return b.String()
}

// changeTo is a diff of one changed line of the named file. It says nothing
// about the rest of the file, which is the point when the working tree is
// where the content is meant to come from.
func changeTo(path string) string {
	return "--- a/" + path + "\n+++ b/" + path + "\n@@ -1,1 +1,1 @@\n-old\n+new\n"
}

// onlyFile parses a diff of one file and returns it.
func onlyFile(t *testing.T, src string) *model.File {
	t.Helper()
	files := diff.Parse(src)
	if len(files) != 1 {
		t.Fatalf("the test diff parsed into %d files, want 1", len(files))
	}
	return files[0]
}

func TestStateKeyCyclesView(t *testing.T) {
	s := newTestState(t, markdownFiles)
	if !s.Previewable() {
		t.Fatalf("the first file of the fixture has no preview: there is nothing to cycle")
	}
	for _, want := range []View{ViewMarkdown, ViewRaw, ViewDiff} {
		// A view starts at its own first line, so a scrolled pane must come
		// back to the top. Putting it off the top first is what makes that
		// visible.
		s.Top = 3
		if quit := s.Key("p"); quit {
			t.Fatalf(`Key("p") = true, want false`)
		}
		if s.View != want {
			t.Errorf(`after "p" view = %v, want %v`, s.View, want)
		}
		if s.Top != 0 {
			t.Errorf(`after "p" top = %d, want 0`, s.Top)
		}
	}
}

func TestStateKeyIgnoresPreviewForNonMarkdown(t *testing.T) {
	s := newTestState(t, markdownFiles)
	s.Key("j")
	if got := s.Current().Path(); got != "main.go" {
		t.Fatalf("the cursor is on %q, want main.go: the fixture is not what this test reads", got)
	}
	if s.Previewable() {
		t.Fatalf("main.go reports a preview, so this test is not about a file without one")
	}
	s.Top = 2
	if quit := s.Key("p"); quit {
		t.Errorf(`Key("p") = true, want false`)
	}
	if s.View != ViewDiff {
		t.Errorf(`after "p" on a Go file view = %v, want ViewDiff`, s.View)
	}
	if s.Top != 2 {
		t.Errorf(`after "p" on a Go file top = %d, want 2: the key does nothing at all`, s.Top)
	}
}

func TestStateResetsViewOnFileChange(t *testing.T) {
	s := newTestState(t, markdownFiles)
	s.Key("p")
	if s.View != ViewMarkdown {
		t.Fatalf(`after "p" view = %v, want ViewMarkdown`, s.View)
	}

	// Off the Markdown file: the pane it was drawing has nothing to draw.
	s.Key("j")
	if got := s.Current().Path(); got != "main.go" {
		t.Fatalf("the cursor is on %q, want main.go", got)
	}
	if s.View != ViewDiff {
		t.Errorf("view = %v after moving to a Go file, want ViewDiff", s.View)
	}

	// Back onto the Markdown file. The reader is reading diffs now, and a
	// preview turning itself on is a screen they did not ask for.
	s.Key("k")
	if got := s.Current().Path(); got != "note.md" {
		t.Fatalf("the cursor is on %q, want note.md", got)
	}
	if s.View != ViewDiff {
		t.Errorf("view = %v after moving back to the Markdown file, want ViewDiff", s.View)
	}
}

func TestRenderedUsesMdterm(t *testing.T) {
	dir := t.TempDir()
	const src = "# Title\n\ntext with **bold** and `code`\n"
	if err := os.WriteFile(filepath.Join(dir, "note.md"), []byte(src), 0o600); err != nil {
		t.Fatalf("writing the fixture: %v", err)
	}
	f := onlyFile(t, changeTo("note.md"))

	got := Rendered(dir, f, 60)
	if len(got) < 2 {
		t.Fatalf("Rendered returned %d lines: %q", len(got), got)
	}
	// mdterm underlines a top-level heading rather than keeping the hashes,
	// which is the difference between rendering the Markdown and printing it.
	if got[0] != "Title" {
		t.Errorf("first line = %q, want %q", got[0], "Title")
	}
	if got[1] != strings.Repeat("=", len("Title")) {
		t.Errorf("second line = %q, want the heading underlined with %q",
			got[1], strings.Repeat("=", len("Title")))
	}
	body := strings.Join(got, "\n")
	if strings.Contains(body, "**bold**") {
		t.Errorf("the emphasis was left as markup: %q", body)
	}
	if !strings.Contains(body, "bold") {
		t.Errorf("the emphasised word is gone entirely: %q", body)
	}
}

func TestRawKeepsMarkdownAsWritten(t *testing.T) {
	dir := t.TempDir()
	const src = "# Title\n\ntext with **bold** and `code`\n"
	if err := os.WriteFile(filepath.Join(dir, "note.md"), []byte(src), 0o600); err != nil {
		t.Fatalf("writing the fixture: %v", err)
	}
	f := onlyFile(t, changeTo("note.md"))

	got := Raw(dir, f)
	want := []string{"# Title", "", "text with **bold** and `code`"}
	if len(got) != len(want) {
		t.Fatalf("Raw returned %d lines, want %d: %q", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("raw line %d = %q, want %q", i, got[i], want[i])
		}
	}
	for i, line := range got {
		if line != "" && strings.Trim(line, "=") == "" {
			t.Errorf("raw line %d = %q: the heading was underlined, so this is not the file as written", i, line)
		}
	}
}

func TestPartialPreviewSaysSo(t *testing.T) {
	partial := onlyFile(t, partialMarkdown)
	if _, complete := Content("", partial); complete {
		t.Fatalf("the fixture reconstructs whole, so there is no partial preview to test")
	}
	for _, tt := range []struct {
		name string
		got  []string
	}{
		{"Rendered", Rendered("", partial, 60)},
		{"Raw", Raw("", partial)},
	} {
		if len(tt.got) == 0 {
			t.Errorf("%s returned nothing", tt.name)
			continue
		}
		if last := tt.got[len(tt.got)-1]; last != partialNotice {
			t.Errorf("%s ends with %q, want %q", tt.name, last, partialNotice)
		}
	}

	// A file the diff created is a file the diff carries whole, and saying
	// otherwise about it would teach the reader to ignore the notice.
	added := onlyFile(t, addedMarkdown)
	if _, complete := Content("", added); !complete {
		t.Fatalf("the added fixture reconstructs partial: the negative case is not what it says")
	}
	for _, tt := range []struct {
		name string
		got  []string
	}{
		{"Rendered", Rendered("", added, 60)},
		{"Raw", Raw("", added)},
	} {
		for i, line := range tt.got {
			if line == partialNotice {
				t.Errorf("%s line %d of a whole file says the preview is partial", tt.name, i)
			}
		}
	}
}

func TestFrameHasNoANSIInPreview(t *testing.T) {
	// A check for escapes over text that has none checks nothing, and a
	// check over Markdown with no markup in it checks nothing either.
	for _, want := range []string{"# Title", "**bold**", "`code`", "\x1b[31m"} {
		if !strings.Contains(markdownFiles, want) {
			t.Fatalf("the fixture carries no %q: this test would pass on anything", want)
		}
	}

	for _, view := range []View{ViewMarkdown, ViewRaw} {
		for _, size := range frameSizes {
			s := newTestState(t, markdownFiles)
			s.SetSize(size.width, size.height)
			s.SetView(view)
			if s.View != view {
				t.Fatalf("SetView(%v) left the view at %v", view, s.View)
			}
			lines := Frame(s, size.width, size.height)
			if len(lines) != size.height {
				t.Errorf("Frame(%d, %d) in %v returned %d lines, want %d",
					size.width, size.height, view, len(lines), size.height)
			}
			for i, line := range lines {
				if strings.Contains(line, "\x1b") {
					t.Errorf("Frame(%d, %d) in %v line %d = %q, want no escape in it",
						size.width, size.height, view, i, line)
				}
			}
		}
	}

	// The preview really is what the pane drew: the raw view shows the
	// markup, so the escape above went through the pane rather than past it.
	s := newTestState(t, markdownFiles)
	s.SetSize(100, 30)
	s.SetView(ViewRaw)
	if body := strings.Join(Frame(s, 100, 30), "\n"); !strings.Contains(body, "**bold**") {
		t.Errorf("the raw view drew no markup, so the fixture never reached the pane:\n%s", body)
	}
}

func TestPreviewScrollsToItsOwnEnd(t *testing.T) {
	s := newTestState(t, longMarkdownDiff())
	s.SetSize(60, 12)
	s.SetView(ViewMarkdown)
	if s.View != ViewMarkdown {
		t.Fatalf("the fixture has no preview to scroll")
	}

	width := s.paneWidth()
	want := len(s.PaneBody(width)) - s.bodyHeight()
	if want <= 0 {
		t.Fatalf("the preview is %d lines in a pane of %d: there is no scrolling to do",
			len(s.PaneBody(width)), s.bodyHeight())
	}
	// The whole point is that the preview is not the length of the diff. If
	// the two agreed, a limit taken from either would pass this test.
	fromDiff := diffLineCount(s.Current()) - s.bodyHeight()
	if fromDiff < 0 {
		fromDiff = 0
	}
	if fromDiff == want {
		t.Fatalf("the diff and the preview are both %d lines past the pane: "+
			"this fixture cannot tell the two limits apart", want)
	}

	s.Key("tab")
	if s.Focus != PaneDiff {
		t.Fatalf(`after "tab" focus = %v, want PaneDiff`, s.Focus)
	}
	s.Key("G")
	if s.Top != want {
		t.Errorf(`after "G" top = %d, want %d (%d preview lines less a pane of %d); `+
			"the diff would have given %d", s.Top, want, len(s.PaneBody(width)), s.bodyHeight(), fromDiff)
	}
}

func TestPaintDoesNotColourPreviewLines(t *testing.T) {
	const width, height = 80, 16
	p := newPalette(true)

	s := newTestState(t, markdownFiles)
	s.SetSize(width, height)

	for _, view := range []View{ViewMarkdown, ViewRaw} {
		s.SetView(view)
		if s.View != view {
			t.Fatalf("SetView(%v) left the view at %v", view, s.View)
		}

		// Colour is only worth checking for over lines that would get it.
		// The document opens a bullet with "-" and a line with "+", and a
		// fenced block keeps both where the renderer cannot eat them.
		var plus, minus bool
		for _, line := range s.PaneBody(s.paneWidth()) {
			switch diffMarker(line) {
			case '+':
				plus = true
			case '-':
				minus = true
			}
		}
		if !plus || !minus {
			t.Fatalf("the %v preview has no line opening with + (%v) or - (%v): "+
				"there is nothing here that could be miscoloured", view, plus, minus)
		}

		painted := p.Paint(s, width, height)
		body := strings.Join(painted[1:len(painted)-1], "\n")
		if strings.Contains(body, sgrGreen) {
			t.Errorf("the %v preview carries %q: a line of the file was coloured as an addition",
				view, sgrGreen)
		}
		if strings.Contains(body, sgrRed) {
			t.Errorf("the %v preview carries %q: a line of the file was coloured as a deletion",
				view, sgrRed)
		}
	}

	// The same state showing the diff does get the colours, so the checks
	// above are about the view rather than about a palette that never paints.
	s.SetView(ViewDiff)
	body := strings.Join(p.Paint(s, width, height)[1:height-1], "\n")
	for _, sgr := range []string{sgrGreen, sgrRed} {
		if !strings.Contains(body, sgr) {
			t.Errorf("the diff of the fixture carries no %q, so the preview checks prove nothing", sgr)
		}
	}
}

func TestParseView(t *testing.T) {
	for _, tt := range []struct {
		name string
		want View
	}{
		{"diff", ViewDiff},
		{"markdown", ViewMarkdown},
		{"raw", ViewRaw},
	} {
		got, ok := ParseView(tt.name)
		if !ok {
			t.Errorf("ParseView(%q) = _, false, want it known", tt.name)
			continue
		}
		if got != tt.want {
			t.Errorf("ParseView(%q) = %v, want %v", tt.name, got, tt.want)
		}
	}
	for _, name := range []string{"", "Markdown", "nope", "DIFF", "md", " raw"} {
		if got, ok := ParseView(name); ok {
			t.Errorf("ParseView(%q) = %v, true, want it refused", name, got)
		}
	}
}

func TestFindFile(t *testing.T) {
	files := diff.Parse(markdownFiles)
	if len(files) != 2 {
		t.Fatalf("the fixture parsed into %d files, want 2", len(files))
	}
	for want, path := range []string{"note.md", "main.go"} {
		got, ok := FindFile(files, path)
		if !ok {
			t.Errorf("FindFile(%q) = _, false, want it found", path)
			continue
		}
		if got != want {
			t.Errorf("FindFile(%q) = %d, want %d", path, got, want)
		}
	}
	// A name that is part of a path is another file's name as often as it is
	// this one's, so nothing but the whole path counts.
	for _, path := range []string{"", "note", "note.md ", "docs/note.md", "main", "nope.md"} {
		if got, ok := FindFile(files, path); ok {
			t.Errorf("FindFile(%q) = %d, true, want it refused", path, got)
		}
	}
}
