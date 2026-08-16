package cmd

import (
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/tenntenn/sbnn/internal/client"
	"github.com/tenntenn/sbnn/internal/diff"
	"github.com/tenntenn/sbnn/internal/export"
	"github.com/tenntenn/sbnn/internal/model"
	"github.com/tenntenn/sbnn/version"
	"github.com/tenntenn/sbnn/web"
)

var (
	exportFragment bool
	exportTitle    string
)

var exportCmd = &cobra.Command{
	Use:   "export [file]",
	Short: "Write a review as a single self-contained HTML page",
	Long: `Write a review as one self-contained HTML page.

The page carries the same UI as the sbnn server with the diff frozen into it:
no server, no mo, no network. Comments can still be written; they are kept in
the browser and "Copy prompt" produces the same text as ` + "`sbnn comments`" + `.

  $ git diff | sbnn export review.html    # straight from stdin, no server needed
  $ sbnn export -t api review.html        # the "api" group of a running server
  $ git diff | sbnn export                # to stdout

  # body only, to embed in a page that brings its own <html> (an artifact,
  # a static site, a mail):
  $ git diff | sbnn export --fragment review.body.html`,
	Args:         cobra.MaximumNArgs(1),
	RunE:         runExport,
	SilenceUsage: true,
}

func init() {
	f := exportCmd.Flags()
	f.StringVarP(&target, "target", "t", "", "Group name (default \"default\", or $SBNN_TARGET)")
	f.IntVarP(&port, "port", "p", DefaultPort, "Port of the sbnn server to read the group from")
	f.StringVarP(&bind, "bind", "b", "localhost", "Bind address of the sbnn server")
	f.StringVar(&title, "title", "", "Title of the diff read from stdin")
	f.StringVar(&exportTitle, "page-title", "", "Title of the generated page")
	f.BoolVar(&exportFragment, "fragment", false, "Write only the page body, for embedding")
}

func runExport(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()
	group, err := groupName(target)
	if err != nil {
		return err
	}

	content, err := readStdin()
	if err != nil {
		return err
	}

	var g *model.Group
	switch {
	case content != "":
		// A piped diff is exported on its own: no server is started, and a
		// running one is left alone.
		files := diff.Parse(content)
		if len(files) == 0 {
			return fmt.Errorf("no file diff found in the input")
		}
		name := title
		if name == "" {
			name = "diff 1"
		}
		g = &model.Group{
			Name: group,
			Diffs: []*model.Diff{{
				ID:        "d1",
				Title:     name,
				BaseDir:   workingDir(),
				CreatedAt: time.Now(),
				Raw:       content,
				Files:     files,
			}},
		}
	default:
		c := client.New(addr(), 10*time.Second)
		if _, err := c.Status(ctx); err != nil {
			return fmt.Errorf("no diff on stdin and no sbnn server on %s", c.Addr)
		}
		g, err = c.Group(ctx, group)
		if err != nil {
			return err
		}
		if len(g.Diffs) == 0 {
			return fmt.Errorf("group %q has no diff to export", group)
		}
	}

	payload := export.Build(g, version.Version, time.Now())
	page, err := export.Render(payload, web.FS(), export.Options{
		Title:    exportTitle,
		Fragment: exportFragment,
	})
	if err != nil {
		return err
	}

	if len(args) == 0 {
		_, err := os.Stdout.WriteString(page)
		return err
	}
	if err := os.WriteFile(args[0], []byte(page), 0o644); err != nil {
		return err
	}
	files := 0
	for _, d := range g.Diffs {
		files += len(d.Files)
	}
	fmt.Fprintf(os.Stderr, "sbnn: wrote %s (%d file(s), %d preview(s), %d KiB)\n",
		args[0], files, len(payload.Previews), len(page)/1024)
	return nil
}
