package cmd

import (
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/tenntenn/sa/internal/client"
	"github.com/tenntenn/sa/internal/server"
)

var (
	commentsFormat   string
	commentsResolved bool
	commentsClear    bool
	commentsJSON     bool
)

var commentsCmd = &cobra.Command{
	Use:   "comments",
	Short: "Print the review comments left in the browser",
	Long: `Print the review comments of a group.

The comments live in the sa server, so an agent can read them back after a
human has written them:

  $ sa comments                    # ready to paste into an agent
  $ sa comments --format json      # machine readable
  $ sa comments -t api             # comments of the "api" group
  $ sa comments --clear            # drop them before the next round`,
	Args:         cobra.NoArgs,
	RunE:         runComments,
	SilenceUsage: true,
}

func init() {
	f := commentsCmd.Flags()
	f.StringVarP(&target, "target", "t", server.DefaultGroup, "Group name")
	f.IntVarP(&port, "port", "p", DefaultPort, "Server port")
	f.StringVarP(&bind, "bind", "b", "localhost", "Bind address")
	f.StringVar(&commentsFormat, "format", "prompt", "Output format: prompt, markdown or json")
	f.BoolVar(&commentsResolved, "include-resolved", false, "Include comments marked as resolved")
	f.BoolVar(&commentsClear, "clear", false, "Remove the comments instead of printing them")
	f.BoolVar(&commentsJSON, "json", false, "Shorthand for --format json")
}

func runComments(cmd *cobra.Command, _ []string) error {
	ctx := cmd.Context()
	group, err := server.ValidateGroupName(target)
	if err != nil {
		return err
	}
	c := client.New(addr(), 5*time.Second)
	if _, err := c.Status(ctx); err != nil {
		return fmt.Errorf("no sa server found on %s", c.Addr)
	}

	if commentsClear {
		removed, err := c.ClearComments(ctx, group, false)
		if err != nil {
			return err
		}
		fmt.Fprintf(os.Stderr, "sa: removed %d comment(s) from group %q\n", removed, group)
		return nil
	}

	format := commentsFormat
	if commentsJSON {
		format = "json"
	}
	switch format {
	case "json":
		comments, err := c.Comments(ctx, group)
		if err != nil {
			return err
		}
		if !commentsResolved {
			open := comments[:0]
			for _, cm := range comments {
				if !cm.Resolved {
					open = append(open, cm)
				}
			}
			comments = open
		}
		return jsonEncoder(os.Stdout).Encode(comments)
	case "prompt", "markdown":
		text, err := c.Prompt(ctx, group, commentsResolved, format == "prompt")
		if err != nil {
			return err
		}
		fmt.Print(text)
		return nil
	default:
		return fmt.Errorf("unknown format %q: use prompt, markdown or json", format)
	}
}
