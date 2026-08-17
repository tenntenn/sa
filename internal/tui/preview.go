package tui

import (
	"strings"

	"github.com/tenntenn/sbnn/internal/mdterm"
	"github.com/tenntenn/sbnn/internal/model"
	"github.com/tenntenn/sbnn/internal/source"
)

// A preview shows the file, where the diff shows the change to it. Reviewing
// Markdown is reading it: a table is a table rather than a row of pipes, and
// the paragraph a comment is about is the paragraph, not the three lines of it
// that happened to change.
//
// The Markdown itself comes from internal/source, which prefers the working
// tree and falls back to rebuilding the file out of the diff. That fallback is
// why a preview says when it is partial: a unified diff carries the hunks that
// changed and nothing else, so a file that is not on disk can only be shown in
// the pieces the diff brought along.

// partialNotice is the last line of a preview built from a diff that did not
// carry the whole file. It is added after the Markdown is drawn, not before:
// run through the renderer it would be folded to the width of the pane, and a
// notice split across two lines is a notice nobody can search for.
const partialNotice = "(this preview is partial: the diff does not carry the whole file)"

// Content returns the Markdown of f after the change, and whether that is the
// whole file.
//
// The path comes out of the diff, which sbnn did not write and does not vouch
// for, so it is resolved by internal/source: one that leaves baseDir is not
// read from disk at all. An empty baseDir reads nothing and rebuilds from the
// diff, which is what a session with no directory behind it does.
func Content(baseDir string, f *model.File) (text string, complete bool) {
	if f == nil {
		return "", false
	}
	got := source.NewSide(baseDir, f)
	return got.Content, got.Complete
}

// Rendered returns the Markdown of f formatted for the terminal, as lines.
//
// The lines carry no escape sequence: colour in this package is the palette's
// to add, and a frame is plain text until it gets there.
func Rendered(baseDir string, f *model.File, width int) []string {
	if !previewable(f) {
		return []string{}
	}
	text, complete := Content(baseDir, f)
	return withPartialNotice(mdterm.Lines(text, mdterm.Options{Width: width, Color: false}), complete)
}

// Raw returns the Markdown of f as it is, one element per line. Nothing is
// expanded and nothing is folded: this is the view for reading what was
// written, markup and all.
func Raw(baseDir string, f *model.File) []string {
	if !previewable(f) {
		return []string{}
	}
	text, complete := Content(baseDir, f)
	lines := strings.Split(text, "\n")
	// The content ends with a newline, and Split turns that into an empty
	// element rather than the end of the file. Only that one comes off: a
	// blank line the author wrote is a line.
	if n := len(lines); n > 0 && lines[n-1] == "" {
		lines = lines[:n-1]
	}
	return withPartialNotice(lines, complete)
}

// withPartialNotice adds the notice when the preview is missing part of the
// file.
func withPartialNotice(lines []string, complete bool) []string {
	if complete {
		return lines
	}
	return append(lines, partialNotice)
}

// previewKey is what a cached preview was drawn from. The width is in it
// because the renderer folds to it: the same file at two widths is two
// different sets of lines.
type previewKey struct {
	file  string
	view  View
	width int
}

// PaneBody returns the lines the right pane shows for the current view,
// reading the file at most once per (file, view, width).
//
// Without the cache the file would be read on every key press, twice: once to
// find out how far the pane can scroll and once to draw it.
func (s *State) PaneBody(width int) []string {
	if s.View == ViewDiff {
		return nil // the diff pane draws itself, out of the diff it was given
	}
	f := s.Current()
	if f == nil {
		return nil
	}
	key := previewKey{file: f.ID, view: s.View, width: width}
	if body, ok := s.preview[key]; ok {
		return body
	}
	var body []string
	switch s.View {
	case ViewMarkdown:
		body = Rendered(s.BaseDir, f, width)
	case ViewRaw:
		body = Raw(s.BaseDir, f)
	}
	if s.preview == nil {
		s.preview = make(map[previewKey][]string)
	}
	s.preview[key] = body
	return body
}
