package server

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/tenntenn/sbnn/internal/mo"
	"github.com/tenntenn/sbnn/internal/model"
	"github.com/tenntenn/sbnn/internal/source"
)

// errNotPreviewable is returned for files mo cannot show.
var errNotPreviewable = errors.New("no Markdown preview for this file")

// PreviewSource tells where the previewed Markdown came from.
type PreviewSource string

const (
	// SourceWorktree means the file was read from the working tree, so the
	// preview shows the whole file.
	SourceWorktree PreviewSource = "worktree"
	// SourceReconstructed means the Markdown was rebuilt from the diff
	// itself, which is complete only for added files.
	SourceReconstructed PreviewSource = "reconstructed"
)

// PreviewResponse is the payload of the preview endpoint.
type PreviewResponse struct {
	// URL is the frameable URL of the mo page, served through sbnn's preview
	// proxy. It is empty when the proxy could not be started.
	URL string `json:"url"`
	// MoURL is the URL of the same page on the mo server itself, for
	// opening the preview in its own window.
	MoURL string `json:"moUrl"`
	// Path is the file mo was pointed at.
	Path string `json:"path"`
	// Source says whether the preview shows the working tree file or
	// Markdown rebuilt from the diff.
	Source PreviewSource `json:"source"`
	// Complete reports whether the previewed Markdown is the whole file.
	Complete bool `json:"complete"`
}

// FileContentResponse is the payload of the file content endpoint: the new
// side of a file, for a client that renders Markdown itself.
type FileContentResponse struct {
	Path     string        `json:"path"`
	Source   PreviewSource `json:"source"`
	Complete bool          `json:"complete"`
	Content  string        `json:"content"`
}

// content returns the Markdown of a file without involving mo. A phone has
// no room for mo's own chrome inside the preview pane, so the browser
// renders the Markdown itself there and keeps mo for its own window.
func (p *previewer) content(d *model.Diff, f *model.File) (*FileContentResponse, error) {
	if err := previewable(f); err != nil {
		return nil, err
	}
	got := source.NewSide(d.BaseDir, f)
	if strings.TrimSpace(got.Content) == "" {
		return nil, fmt.Errorf("%w: nothing to preview for %s", errNotPreviewable, f.Path())
	}
	kind := SourceReconstructed
	if got.Kind == source.FromWorktree {
		kind = SourceWorktree
	}
	return &FileContentResponse{
		Path:     got.Path,
		Source:   kind,
		Complete: got.Complete,
		Content:  got.Content,
	}, nil
}

// previewable rejects the files there is no Markdown preview for.
func previewable(f *model.File) error {
	switch {
	case !f.IsMarkdown:
		return fmt.Errorf("%w: %s is not Markdown", errNotPreviewable, f.Path())
	case f.IsBinary:
		return fmt.Errorf("%w: %s is binary", errNotPreviewable, f.Path())
	case f.Status == model.StatusDeleted:
		return fmt.Errorf("%w: %s was deleted", errNotPreviewable, f.Path())
	}
	return nil
}

// previewer turns a file of a diff into a mo preview.
type previewer struct {
	mo       *mo.Runner
	proxy    *moProxy
	cacheDir string
}

// preview hands the Markdown of f to mo and returns the URLs of the
// resulting page.
func (p *previewer) preview(ctx context.Context, group string, d *model.Diff, f *model.File) (*PreviewResponse, error) {
	if err := previewable(f); err != nil {
		return nil, err
	}

	path, source, complete, err := p.resolve(group, d, f)
	if err != nil {
		return nil, err
	}
	res, err := p.mo.Open(ctx, moGroupName(group), path)
	if err != nil {
		return nil, err
	}
	moURL := res.URLFor(path)
	out := &PreviewResponse{
		MoURL:    moURL,
		Path:     path,
		Source:   source,
		Complete: complete,
	}
	if p.proxy != nil {
		out.URL = p.proxy.rewrite(moURL)
	}
	return out, nil
}

// moGroupName keeps sbnn's previews in their own mo group so that they never
// mix with the files the user opened in mo directly.
func moGroupName(group string) string {
	return "sbnn-" + group
}

// resolve returns the path of the Markdown handed to mo. The working tree
// file wins because it is the complete document; when it is missing, the new
// side is rebuilt from the diff and written to the cache directory so that mo
// has a file to open.
func (p *previewer) resolve(group string, d *model.Diff, f *model.File) (path string, src PreviewSource, complete bool, err error) {
	rel := f.Path()
	got := source.NewSide(d.BaseDir, f)
	if got.Kind == source.FromWorktree {
		return got.Path, SourceWorktree, got.Complete, nil
	}
	if strings.TrimSpace(got.Content) == "" {
		return "", "", false, fmt.Errorf("%w: nothing to preview for %s", errNotPreviewable, rel)
	}
	if p.cacheDir == "" {
		return "", "", false, fmt.Errorf("no cache directory to write the reconstructed preview of %s", rel)
	}
	dst := filepath.Join(p.cacheDir, "preview", safeSegment(group), safeSegment(d.ID), safeRelPath(rel))
	if err := os.MkdirAll(filepath.Dir(dst), 0o700); err != nil {
		return "", "", false, err
	}
	// Rewriting an unchanged file would make mo reload the preview for
	// nothing, so only write when the content actually differs.
	if old, err := os.ReadFile(dst); err != nil || string(old) != got.Content {
		if err := os.WriteFile(dst, []byte(got.Content), 0o600); err != nil {
			return "", "", false, err
		}
	}
	return dst, SourceReconstructed, got.Complete, nil
}

// safeRelPath makes a diff path usable inside the cache directory.
func safeRelPath(rel string) string {
	cleaned := filepath.Clean(filepath.FromSlash(rel))
	parts := strings.Split(cleaned, string(filepath.Separator))
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		switch part {
		case "", ".", "..":
			continue
		}
		if filepath.VolumeName(part) != "" {
			continue
		}
		out = append(out, safeSegment(part))
	}
	if len(out) == 0 {
		return "preview.md"
	}
	return filepath.Join(out...)
}

// safeSegment strips characters that must not end up in a path component.
func safeSegment(s string) string {
	return strings.Map(func(r rune) rune {
		switch {
		case r == filepath.Separator, r == '/', r == '\\', r == ':':
			return '_'
		case r < 0x20:
			return '_'
		}
		return r
	}, s)
}
