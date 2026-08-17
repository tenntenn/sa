package cmd

import (
	"errors"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/tenntenn/sbnn/internal/diff"
	"github.com/tenntenn/sbnn/internal/tui"
)

var (
	tuiDump   bool
	tuiWidth  int
	tuiHeight int
	tuiView   string
	tuiFile   string
)

var tuiCmd = &cobra.Command{
	Use:   "tui",
	Short: "Read a unified diff in the terminal",
	Long: `Read a unified diff in the terminal, without a browser and without a server.

The file list is on the left, the diff of the selected file on the right.

  $ git diff | sbnn tui
  $ diff -u old.md new.md | sbnn tui

The diff comes in on stdin, so the keys are read from the controlling
terminal instead: without one there is nothing to drive, and sbnn tui says so
rather than starting.

A Markdown file has two more things the right pane can show: the file drawn
for the terminal, and the file as it was written. p moves between the three,
diff to markdown to raw and back. Files that are not Markdown have the diff
alone, and p does nothing on them.

  # start on a Markdown file, already rendered
  $ git diff | sbnn tui --view markdown --file docs/guide.md

  # the drawing as plain text, for a terminal that is not there
  $ git diff | sbnn tui --dump
  $ git diff | sbnn tui --dump --width 120 --height 40`,
	Args:         cobra.NoArgs,
	RunE:         runTUI,
	SilenceUsage: true,
}

func init() {
	// Registering the subcommand here keeps root.go out of it: a package may
	// have as many init functions as it has files, and rootCmd is built
	// before any of them runs.
	rootCmd.AddCommand(tuiCmd)

	f := tuiCmd.Flags()
	f.BoolVar(&tuiDump, "dump", false,
		"Write the first frame to stdout as plain text and exit, opening no terminal")
	f.IntVar(&tuiWidth, "width", tui.DefaultWidth, "Columns of the frame written by --dump")
	f.IntVar(&tuiHeight, "height", tui.DefaultHeight, "Rows of the frame written by --dump")
	f.StringVar(&tuiView, "view", "diff",
		"What the right pane starts on: diff, markdown or raw")
	f.StringVar(&tuiFile, "file", "",
		"Path of the file to start on, as the diff spells it; the first file by default")
}

func runTUI(*cobra.Command, []string) error {
	content, err := readStdin()
	if err != nil {
		return err
	}
	if content == "" {
		return errors.New("no diff on stdin")
	}
	files := diff.Parse(content)
	if len(files) == 0 {
		return errors.New("no file diff found in the input")
	}
	// Both flags are answered before the terminal is touched: a name that
	// was mistyped is worth saying so about, and saying it on the alternate
	// screen means saying it where nobody will see it.
	view, ok := tui.ParseView(tuiView)
	if !ok {
		return fmt.Errorf("invalid --view %q: want diff, markdown or raw", tuiView)
	}
	cursor := 0
	if tuiFile != "" {
		if cursor, ok = tui.FindFile(files, tuiFile); !ok {
			return fmt.Errorf("file not found in the diff: %s", tuiFile)
		}
	}
	// The diff paths are relative to where the diff was made, which is
	// where sbnn was run from. A directory that cannot be named is not a
	// reason to refuse the diff: the working tree simply goes unread and
	// the preview is rebuilt from the diff itself.
	baseDir, err := os.Getwd()
	if err != nil {
		baseDir = "."
	}
	o := tui.Session{BaseDir: baseDir, Cursor: cursor, View: view}
	if tuiDump {
		return tui.Dump(os.Stdout, files, tuiWidth, tuiHeight, o)
	}
	return tui.Run(files, o)
}
