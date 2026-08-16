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
