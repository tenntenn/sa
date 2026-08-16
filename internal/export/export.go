// Package export writes a review as a single self-contained HTML page.
//
// The page carries the same UI as the sbnn server, but with the diff frozen
// into it: no server, no mo, no network. It is what you hand to someone who
// should look at a change without running sbnn, and it is what makes a review
// publishable as an artifact.
package export

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"sort"
	"strings"
	"time"

	"github.com/tenntenn/sbnn/internal/model"
	"github.com/tenntenn/sbnn/internal/source"
)

// PayloadVersion is the schema version of the embedded data.
const PayloadVersion = 1

// Preview is the Markdown of one file, frozen at export time.
type Preview struct {
	Content  string `json:"content"`
	Source   string `json:"source"`
	Complete bool   `json:"complete"`
	Path     string `json:"path,omitempty"`
}

// Payload is the data the exported page reads out of window.__SBNN_DATA__.
type Payload struct {
	Version     int                `json:"version"`
	SaVersion   string             `json:"saVersion,omitempty"`
	GeneratedAt time.Time          `json:"generatedAt"`
	Group       string             `json:"group"`
	Diffs       []*model.Diff      `json:"diffs"`
	Comments    []*model.Comment   `json:"comments"`
	Previews    map[string]Preview `json:"previews"`
}

// Build freezes a group into a payload. Markdown files are resolved the same
// way the live preview does: the working tree file when it is still there,
// the new side rebuilt from the diff otherwise.
func Build(g *model.Group, saVersion string, now time.Time) *Payload {
	p := &Payload{
		Version:     PayloadVersion,
		SaVersion:   saVersion,
		GeneratedAt: now,
		Group:       g.Name,
		Diffs:       make([]*model.Diff, 0, len(g.Diffs)),
		Comments:    g.Comments,
		Previews:    map[string]Preview{},
	}
	if p.Comments == nil {
		p.Comments = []*model.Comment{}
	}
	for _, d := range g.Diffs {
		// The raw diff text is already represented by the parsed files;
		// dropping it keeps the page roughly half the size.
		frozen := *d
		frozen.Raw = ""
		p.Diffs = append(p.Diffs, &frozen)

		for _, f := range d.Files {
			if !f.IsMarkdown || f.IsBinary || f.Status == model.StatusDeleted {
				continue
			}
			got := source.NewSide(d.BaseDir, f)
			if strings.TrimSpace(got.Content) == "" {
				continue
			}
			p.Previews[d.ID+":"+f.ID] = Preview{
				Content:  got.Content,
				Source:   string(got.Kind),
				Complete: got.Complete,
				Path:     got.Path,
			}
		}
	}
	return p
}

// Options tunes the generated page.
type Options struct {
	// Title is the document title; a default is derived from the group.
	Title string
	// Fragment writes only the page body, for embedding into a host page
	// that supplies its own <html> and <head> (a Claude Artifact, a static
	// site, a mail...).
	Fragment bool
}

// Render writes the page. assets is the built UI (the dist tree embedded in
// the sbnn binary).
func Render(payload *Payload, assets fs.FS, opts Options) (string, error) {
	css, js, err := readAssets(assets)
	if err != nil {
		return "", err
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("cannot serialise the review: %w", err)
	}
	title := opts.Title
	if title == "" {
		title = "sbnn review: " + payload.Group
	}

	var b strings.Builder
	if !opts.Fragment {
		b.WriteString("<!doctype html>\n<html lang=\"en\">\n<head>\n")
		b.WriteString("<meta charset=\"utf-8\">\n")
		b.WriteString("<meta name=\"viewport\" content=\"width=device-width, initial-scale=1\">\n")
		fmt.Fprintf(&b, "<title>%s</title>\n", escapeHTML(title))
		fmt.Fprintf(&b, "<style>\n%s\n</style>\n", css)
		b.WriteString("</head>\n<body>\n")
	} else {
		fmt.Fprintf(&b, "<style>\n%s\n</style>\n", css)
	}

	b.WriteString("<div id=\"root\"></div>\n")
	fmt.Fprintf(&b, "<script>window.__SBNN_DATA__ = %s;</script>\n", escapeJSONForScript(data))
	fmt.Fprintf(&b, "<script type=\"module\">\n%s\n</script>\n", js)

	if !opts.Fragment {
		b.WriteString("</body>\n</html>\n")
	}
	return b.String(), nil
}

// readAssets collects the stylesheet and the script Vite produced.
func readAssets(assets fs.FS) (css, js string, err error) {
	entries, err := fs.ReadDir(assets, "assets")
	if err != nil {
		return "", "", fmt.Errorf("the sbnn UI is not built into this binary: %w", err)
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		if !e.IsDir() {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)

	var cssParts, jsParts []string
	for _, name := range names {
		b, err := fs.ReadFile(assets, "assets/"+name)
		if err != nil {
			return "", "", err
		}
		switch {
		case strings.HasSuffix(name, ".css"):
			cssParts = append(cssParts, string(b))
		case strings.HasSuffix(name, ".js"):
			jsParts = append(jsParts, string(b))
		}
	}
	if len(jsParts) == 0 {
		return "", "", fmt.Errorf("no script found in the embedded UI")
	}
	return strings.Join(cssParts, "\n"), strings.Join(jsParts, "\n"), nil
}

// escapeJSONForScript makes JSON safe to inline in a <script> element.
// encoding/json already escapes <, > and & for us; U+2028 and U+2029 are
// valid JSON but terminate a JavaScript string.
func escapeJSONForScript(b []byte) string {
	s := string(b)
	s = strings.ReplaceAll(s, "\u2028", `\u2028`)
	s = strings.ReplaceAll(s, "\u2029", `\u2029`)
	return s
}

func escapeHTML(s string) string {
	r := strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", `"`, "&quot;")
	return r.Replace(s)
}
