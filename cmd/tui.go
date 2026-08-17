package cmd

import (
	"errors"
	"os"

	"github.com/spf13/cobra"

	"github.com/tenntenn/sbnn/internal/diff"
	"github.com/tenntenn/sbnn/internal/tui"
)

var (
	tuiDump   bool
	tuiWidth  int
	tuiHeight int
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
	if tuiDump {
		return tui.Dump(os.Stdout, files, tuiWidth, tuiHeight)
	}
	return tui.Run(files)
}
