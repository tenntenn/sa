package cmd

import (
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/tenntenn/sa/internal/client"
)

var (
	submitNote     string
	submitQuiet    bool
	submitExitCode bool
)

var submitCmd = &cobra.Command{
	Use:   "submit",
	Short: "Finish the review of a group, as the Submit button does",
	Long: `Finish a review from the command line.

Pressing "Submit review" in the browser is what ends a round: it wakes
anything waiting on it, starts the hooks, and writes the round into the log
of past reviews. This does the same thing without a browser, which is what a
reviewer who is not a person needs.

  $ git diff | sa --target api                    # the change to look at
  $ sa comment main.go:42 -m "..." --author me    # what you found
  $ sa submit --target api --note "one thing to fix"

A review with nothing in it is a review: submitting after finding nothing is
how the other side learns that, and it is the answer sa wait was waiting
for.

--exit-code says what was submitted without anyone reading the output: 1
when the review left something to address, 0 when it did not.`,
	Args:         cobra.NoArgs,
	RunE:         runSubmit,
	SilenceUsage: true,
}

func init() {
	f := submitCmd.Flags()
	f.StringVarP(&target, "target", "t", "", "Group name (default \"default\", or $SA_TARGET)")
	f.IntVarP(&port, "port", "p", DefaultPort, "Server port")
	f.StringVarP(&bind, "bind", "b", "localhost", "Bind address")
	f.StringVarP(&submitNote, "note", "m", "", "What the review says as a whole")
	f.BoolVar(&jsonOutput, "json", false, "Print the reviewed group as JSON")
	f.BoolVar(&submitExitCode, "exit-code", false,
		"Exit 1 when the review left something to address, 0 when it did not")
	f.BoolVarP(&submitQuiet, "quiet", "q", false, "Print nothing; implies --exit-code")
}

func runSubmit(cmd *cobra.Command, _ []string) error {
	ctx := cmd.Context()
	group, err := groupName(target)
	if err != nil {
		return err
	}
	c := client.New(addr(), 10*time.Second)
	if _, err := c.Status(ctx); err != nil {
		return fmt.Errorf("no sa server found on %s", c.Addr)
	}

	g, err := c.SubmitReview(ctx, group, submitNote)
	if err != nil {
		return err
	}

	open := 0
	for _, cm := range g.Comments {
		if !cm.Resolved {
			open++
		}
	}
	switch {
	case submitQuiet:
	case jsonOutput:
		if err := writeJSON(g); err != nil {
			return err
		}
	default:
		fmt.Fprintf(os.Stderr, "sa: reviewed %q, %d open comment(s)\n", group, open)
	}
	if submitQuiet || submitExitCode {
		return exitWithComments(g.Comments)
	}
	return nil
}
