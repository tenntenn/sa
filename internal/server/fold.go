package server

import (
	"path"
	"strings"

	"github.com/tenntenn/sa/internal/diff"
	"github.com/tenntenn/sa/internal/model"
	"github.com/tenntenn/sa/internal/source"
)

// foldFiles decides which files of a diff open shut, and says why for each
// one.
//
// There are exactly two reasons, and neither of them is sa's opinion:
//
//   - the sender asked for it, by naming the file in --collapse. Whoever
//     produced the diff is the one who knows what their generated files are
//     called, so that knowledge comes in with the diff instead of being
//     guessed at here;
//   - the file says so itself, in a "DO NOT EDIT" or "@generated" line put
//     there by whatever wrote it.
//
// Nothing is folded on a hunch - not by size, not by directory, not by
// extension. A review is the wrong place to be clever: a file folded for a
// bad reason is a file nobody read.
func foldFiles(d *model.Diff, patterns []string) {
	for _, f := range d.Files {
		if pattern, ok := matchAny(patterns, f.Path()); ok {
			f.Folded = true
			f.FoldReason = "the sender asked for it (--collapse " + pattern + ")"
			continue
		}
		if marker := declaredGenerated(d, f); marker != "" {
			f.Folded = true
			f.FoldReason = "the file says so: " + shorten(marker, 60)
			continue
		}
		// A previous round may have folded it; nothing says so now.
		f.Folded, f.FoldReason = false, ""
	}
}

// declaredGenerated returns the line by which a file declares itself
// generated.
//
// The working tree file is the whole file, so it answers properly. Failing
// that - a patch from elsewhere, a file already deleted - only what the diff
// shows can be read, and a unified diff reaches the top of a file just when
// its first hunk starts there. When neither can answer, the file is left
// alone: a declaration that cannot be found is not a declaration that is
// absent, and folding on that difference would be guessing.
func declaredGenerated(d *model.Diff, f *model.File) string {
	if f.IsBinary {
		return ""
	}
	if got := source.NewSide(d.BaseDir, f); got.Kind == source.FromWorktree {
		return diff.GeneratedMarker(got.Content)
	}
	return diff.GeneratedMarker(diff.VisibleTop(f))
}

// matchAny reports which pattern a path matches, if any.
//
// A pattern with a slash in it is matched against the whole path, one
// without against the file name at any depth - the shape people already know
// from .gitignore. "**" stands for any run of directories.
func matchAny(patterns []string, p string) (string, bool) {
	for _, pattern := range patterns {
		pattern = strings.TrimSpace(pattern)
		if pattern == "" {
			continue
		}
		if matchPath(pattern, p) {
			return pattern, true
		}
	}
	return "", false
}

func matchPath(pattern, p string) bool {
	if strings.Contains(pattern, "**") {
		return matchDoubleStar(pattern, p)
	}
	if !strings.Contains(pattern, "/") {
		if ok, _ := path.Match(pattern, path.Base(p)); ok {
			return true
		}
		return false
	}
	ok, _ := path.Match(pattern, p)
	return ok
}

// matchDoubleStar handles the one pattern path.Match cannot: "**" for any
// number of directories.
func matchDoubleStar(pattern, p string) bool {
	prefix, suffix, _ := strings.Cut(pattern, "**")
	if !strings.HasPrefix(p, prefix) {
		return false
	}
	rest := strings.TrimPrefix(p, prefix)
	suffix = strings.TrimPrefix(suffix, "/")
	if suffix == "" {
		return true
	}
	// The suffix may match at any depth below the prefix.
	for {
		if ok, _ := path.Match(suffix, rest); ok {
			return true
		}
		_, after, found := strings.Cut(rest, "/")
		if !found {
			return false
		}
		rest = after
	}
}

func shorten(s string, n int) string {
	s = strings.TrimSpace(s)
	if len(s) <= n {
		return s
	}
	return s[:n-1] + "…"
}
