// Package model defines the data structures shared by the sa CLI, the sa
// server and the sa web UI.
package model

import "time"

// FileStatus represents how a file was changed in a diff.
type FileStatus string

const (
	StatusAdded    FileStatus = "added"
	StatusDeleted  FileStatus = "deleted"
	StatusModified FileStatus = "modified"
	StatusRenamed  FileStatus = "renamed"
	StatusCopied   FileStatus = "copied"
	StatusMode     FileStatus = "mode"
)

// LineKind represents the kind of a line inside a hunk.
type LineKind string

const (
	LineContext LineKind = "context"
	LineAdd     LineKind = "add"
	LineDelete  LineKind = "delete"
)

// ViewMode is the preferred way of rendering a file diff in the web UI.
type ViewMode string

const (
	// ViewUnified renders a single column. New (and deleted) files are always
	// rendered this way because there is no counterpart to put side by side.
	ViewUnified ViewMode = "unified"
	// ViewSplit renders old and new side by side.
	ViewSplit ViewMode = "split"
)

// Line is a single line inside a hunk.
type Line struct {
	Kind LineKind `json:"kind"`
	// OldNumber and NewNumber are 1-based line numbers, or 0 when the line
	// does not exist on that side.
	OldNumber int    `json:"oldNumber"`
	NewNumber int    `json:"newNumber"`
	Content   string `json:"content"`
	// NoNewline reports that "\ No newline at end of file" followed this line.
	NoNewline bool `json:"noNewline,omitempty"`
}

// Hunk is a single @@ section of a file diff.
type Hunk struct {
	Header   string `json:"header"`
	OldStart int    `json:"oldStart"`
	OldLines int    `json:"oldLines"`
	NewStart int    `json:"newStart"`
	NewLines int    `json:"newLines"`
	// Section is the optional function context after the closing @@.
	Section string `json:"section,omitempty"`
	Lines   []Line `json:"lines"`
}

// File is a single file entry of a diff.
type File struct {
	ID        string     `json:"id"`
	OldPath   string     `json:"oldPath"`
	NewPath   string     `json:"newPath"`
	Status    FileStatus `json:"status"`
	IsBinary  bool       `json:"isBinary"`
	OldMode   string     `json:"oldMode,omitempty"`
	NewMode   string     `json:"newMode,omitempty"`
	Index     string     `json:"index,omitempty"`
	Additions int        `json:"additions"`
	Deletions int        `json:"deletions"`
	// ViewMode is the rendering mode the UI should default to.
	ViewMode ViewMode `json:"viewMode"`
	// IsMarkdown reports whether the file can be previewed with mo.
	IsMarkdown bool    `json:"isMarkdown"`
	Hunks      []*Hunk `json:"hunks"`
}

// Path returns the path used to identify the file. Deleted files are
// identified by their old path, everything else by the new path.
func (f *File) Path() string {
	if f.NewPath != "" && f.NewPath != DevNull {
		return f.NewPath
	}
	return f.OldPath
}

// DevNull is the path git uses for a missing side of a diff.
const DevNull = "/dev/null"

// Diff is one chunk of unified diff text handed to sa through stdin.
type Diff struct {
	ID    string `json:"id"`
	Title string `json:"title"`
	// BaseDir is the directory the diff paths are relative to. It is the
	// working directory of the sa invocation that sent the diff.
	BaseDir   string    `json:"baseDir"`
	CreatedAt time.Time `json:"createdAt"`
	Raw       string    `json:"raw"`
	Files     []*File   `json:"files"`
}

// Stats returns the total number of added and deleted lines of the diff.
func (d *Diff) Stats() (additions, deletions int) {
	for _, f := range d.Files {
		additions += f.Additions
		deletions += f.Deletions
	}
	return additions, deletions
}

// FindFile returns the file with the given ID.
func (d *Diff) FindFile(id string) *File {
	for _, f := range d.Files {
		if f.ID == id {
			return f
		}
	}
	return nil
}

// Comment is a review comment attached to a range of lines of a file.
type Comment struct {
	ID     string `json:"id"`
	Group  string `json:"group"`
	DiffID string `json:"diffId"`
	FileID string `json:"fileId"`
	Path   string `json:"path"`
	// Side is "new" or "old" and tells which side of the diff the line
	// numbers belong to.
	Side string `json:"side"`
	// StartLine and EndLine are inclusive 1-based line numbers on Side.
	StartLine int    `json:"startLine"`
	EndLine   int    `json:"endLine"`
	Body      string `json:"body"`
	// Snippet is the reviewed code, kept so that the comment stays
	// meaningful once it is exported as a prompt.
	Snippet   string    `json:"snippet"`
	Resolved  bool      `json:"resolved"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// Group is a named collection of diffs and their review comments, served
// under its own URL path.
type Group struct {
	Name     string     `json:"name"`
	Diffs    []*Diff    `json:"diffs"`
	Comments []*Comment `json:"comments"`
}

// FindDiff returns the diff with the given ID.
func (g *Group) FindDiff(id string) *Diff {
	for _, d := range g.Diffs {
		if d.ID == id {
			return d
		}
	}
	return nil
}
