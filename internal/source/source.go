// Package source recovers the new side of a changed file.
//
// A unified diff only carries the changed hunks, so the working tree file is
// the better source whenever it is still there; otherwise the new side is
// rebuilt from the diff, which is complete only for added files.
package source

import (
	"os"
	"path/filepath"

	"github.com/tenntenn/sa/internal/diff"
	"github.com/tenntenn/sa/internal/model"
)

// Kind tells where the content came from.
type Kind string

const (
	// FromWorktree means the file was read from disk.
	FromWorktree Kind = "worktree"
	// FromDiff means the content was rebuilt out of the diff.
	FromDiff Kind = "reconstructed"
)

// Result is the recovered new side of a file.
type Result struct {
	Content string
	Kind    Kind
	// Path is the working tree file the content was read from, empty for
	// rebuilt content.
	Path string
	// Complete reports whether Content is the whole file.
	Complete bool
}

// NewSide returns the content of a file after the change. baseDir is the
// directory the diff paths are relative to.
func NewSide(baseDir string, f *model.File) Result {
	if path := AbsPath(baseDir, f.Path()); path != "" {
		if st, err := os.Stat(path); err == nil && st.Mode().IsRegular() {
			if b, err := os.ReadFile(path); err == nil {
				return Result{Content: string(b), Kind: FromWorktree, Path: path, Complete: true}
			}
		}
	}
	content, complete := diff.Reconstruct(f)
	return Result{Content: content, Kind: FromDiff, Complete: complete}
}

// AbsPath resolves a diff path against the directory the diff was sent from.
func AbsPath(baseDir, rel string) string {
	if rel == "" {
		return ""
	}
	if filepath.IsAbs(rel) {
		return filepath.Clean(rel)
	}
	if baseDir == "" {
		return ""
	}
	return filepath.Clean(filepath.Join(baseDir, filepath.FromSlash(rel)))
}

// maxClimb bounds how far Root walks up from the directory a diff was sent
// from. No project is forty levels deep, and a runaway loop on a broken path
// would be worse than a wrong guess.
const maxClimb = 40

// Root works out which directory the paths of a diff are relative to.
//
// sa is told the directory the diff was sent from, but that is not
// necessarily the directory the paths hang off: diffs are usually written
// relative to the root of a checkout, and the command may well have been run
// three directories inside it. So the diff is asked instead of a version
// control system: for every ancestor of dir, count how many of the files the
// diff names are actually there, and keep the one that explains the most of
// them - the deepest, when several explain as many.
//
// This knows nothing about git, jj or anything else, which is the point: it
// works the same for a checkout, a worktree, an unpacked tarball, or a patch
// mailed from a machine you have never seen. When nothing matches - the files
// are gone, or were never here - dir is the answer.
func Root(dir string, files []*model.File) string {
	if dir == "" {
		return ""
	}
	dir = filepath.Clean(dir)
	paths := candidatePaths(files)
	if len(paths) == 0 {
		return dir
	}

	best, bestScore := dir, 0
	current := dir
	for range maxClimb {
		if score := explains(current, paths); score > bestScore {
			best, bestScore = current, score
		}
		parent := filepath.Dir(current)
		if parent == current {
			break
		}
		current = parent
	}
	return best
}

// candidatePaths are the relative paths of a diff worth looking for.
func candidatePaths(files []*model.File) []string {
	paths := make([]string, 0, len(files))
	for _, f := range files {
		p := f.Path()
		if p == "" || filepath.IsAbs(p) {
			continue
		}
		paths = append(paths, filepath.FromSlash(p))
		if len(paths) == 16 {
			// A handful of files says as much as a thousand.
			break
		}
	}
	return paths
}

// explains counts how many of the paths exist under dir.
func explains(dir string, paths []string) int {
	found := 0
	for _, p := range paths {
		if _, err := os.Lstat(filepath.Join(dir, p)); err == nil {
			found++
		}
	}
	return found
}
