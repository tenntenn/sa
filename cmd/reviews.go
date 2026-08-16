package cmd

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/tenntenn/sa/internal/client"
	"github.com/tenntenn/sa/internal/history"
	"github.com/tenntenn/sa/internal/paths"
)

var (
	reviewsSince  string
	reviewsLimit  int
	reviewsFormat string
	reviewsStats  bool
	reviewsAll    bool
	reviewsTop    int
)

var reviewsCmd = &cobra.Command{
	Use:   "reviews",
	Short: "List the reviews that were submitted, and what they say together",
	Long: `List the reviews that were submitted.

Every submitted review is written down: what was reviewed, what was said, how
long the change waited. A round of review is thrown away as soon as it is
over - comments cleared, group closed - and this is what stays, so that a
year of reviews can be read as one thing.

  $ sa reviews                     # newest last, one line each
  $ sa reviews --since 7d          # this week
  $ sa reviews -t api --limit 5
  $ sa reviews --stats             # what they say together
  $ sa reviews --format json       # every comment, for your own analysis

The log is a JSON object per line at ` + "`$XDG_STATE_HOME/sa/reviews.jsonl`" + `,
so jq and friends work on it directly. Nothing leaves the machine.`,
	Args:         cobra.NoArgs,
	RunE:         runReviews,
	SilenceUsage: true,
}

func init() {
	f := reviewsCmd.Flags()
	f.StringVarP(&target, "target", "t", "", "Only the reviews of this group")
	f.IntVarP(&port, "port", "p", DefaultPort, "Server port (used when sa is running)")
	f.StringVarP(&bind, "bind", "b", "localhost", "Bind address")
	f.StringVar(&reviewsSince, "since", "", "Only reviews after this: 7d, 36h, 2026-01-31")
	f.IntVar(&reviewsLimit, "limit", 0, "Keep only the newest n reviews")
	f.StringVar(&reviewsFormat, "format", "text", "Output format: text, json or jsonl")
	f.BoolVar(&reviewsStats, "stats", false, "Print what the reviews say together")
	f.BoolVar(&reviewsAll, "all", false, "Every group, ignoring --target and $SA_TARGET")
	f.IntVar(&reviewsTop, "top", 5, "How many entries each tally shows")
	f.BoolVar(&jsonOutput, "json", false, "Shorthand for --format json")
}

func runReviews(cmd *cobra.Command, _ []string) error {
	filter := history.Filter{Limit: reviewsLimit}
	if !reviewsAll {
		filter.Group = target
		if filter.Group == "" {
			filter.Group = os.Getenv(TargetEnv)
		}
	}
	if reviewsSince != "" {
		since, err := history.ParseSince(reviewsSince, time.Now())
		if err != nil {
			return err
		}
		filter.Since = since
	}

	records, err := loadReviews(cmd, filter)
	if err != nil {
		return err
	}

	format := reviewsFormat
	if jsonOutput {
		format = "json"
	}
	switch format {
	case "json":
		return jsonEncoder(os.Stdout).Encode(map[string]any{
			"reviews": records,
			"stats":   history.Summarize(records),
		})
	case "jsonl":
		enc := jsonEncoder(os.Stdout)
		for _, rec := range records {
			if err := enc.Encode(rec); err != nil {
				return err
			}
		}
		return nil
	case "text":
		return printReviews(records)
	default:
		return fmt.Errorf("unknown format %q: use text, json or jsonl", format)
	}
}

// loadReviews reads the log from disk, or from the running server when it
// keeps the log somewhere else.
func loadReviews(cmd *cobra.Command, filter history.Filter) ([]history.Record, error) {
	path, err := paths.HistoryFile()
	if err == nil {
		if records, err := history.Load(path, filter); err == nil && len(records) > 0 {
			return records, nil
		}
	}
	// Fall back to the server: it may be the one holding them.
	c := client.New(addr(), 5*time.Second)
	if _, err := c.Status(cmd.Context()); err != nil {
		return nil, nil
	}
	return c.Reviews(cmd.Context(), filter)
}

func printReviews(records []history.Record) error {
	if len(records) == 0 {
		fmt.Println("no review has been submitted yet")
		return nil
	}
	if !reviewsStats {
		for _, rec := range records {
			fmt.Printf("%s  %-14s %2d comment(s)%s  %d file(s), +%d -%d%s\n",
				rec.ReviewedAt.Local().Format("2006-01-02 15:04"),
				rec.Group,
				len(rec.Comments),
				suggestionCount(rec),
				rec.Files, rec.Additions, rec.Deletions,
				waited(rec))
			if note := strings.TrimSpace(rec.Note); note != "" {
				fmt.Printf("%s\n", indent(note, "      "))
			}
		}
		fmt.Println()
	}
	printStats(history.Summarize(records))
	return nil
}

func suggestionCount(rec history.Record) string {
	n := 0
	for _, c := range rec.Comments {
		n += len(c.Suggestions)
	}
	if n == 0 {
		return ""
	}
	return fmt.Sprintf(", %d suggestion(s)", n)
}

func waited(rec history.Record) string {
	if w := rec.Wait(); w > 0 {
		return "  waited " + shortDuration(w)
	}
	return ""
}

func printStats(s history.Stats) {
	fmt.Printf("%d review(s), %d comment(s) (%.1f per review), %d suggestion(s)\n",
		s.Reviews, s.Comments, s.CommentsPerReview, s.Suggestions)
	fmt.Printf("%d review(s) had nothing to say, %d comment(s) were resolved\n", s.Silent, s.Resolved)
	fmt.Printf("%d file(s) reviewed, +%d -%d\n", s.Files, s.Additions, s.Deletions)
	if s.MedianWait > 0 {
		fmt.Printf("median wait from diff to review: %s\n", shortDuration(s.MedianWait))
	}
	printTally("most commented", s.Paths)
	printTally("by kind of file", s.Extensions)
	printTally("by author", s.Authors)
}

func printTally(title string, counts []history.Count) {
	if len(counts) == 0 {
		return
	}
	if reviewsTop > 0 && len(counts) > reviewsTop {
		counts = counts[:reviewsTop]
	}
	fmt.Printf("%s:\n", title)
	for _, c := range counts {
		fmt.Printf("  %-40s %d\n", c.Key, c.Count)
	}
}

// shortDuration writes a duration the way someone would say it.
func shortDuration(d time.Duration) string {
	switch {
	case d >= 48*time.Hour:
		return fmt.Sprintf("%dd", int(d.Hours()/24))
	case d >= time.Hour:
		return fmt.Sprintf("%dh%02dm", int(d.Hours()), int(d.Minutes())%60)
	case d >= time.Minute:
		return fmt.Sprintf("%dm", int(d.Minutes()))
	default:
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
}

func indent(text, prefix string) string {
	lines := strings.Split(text, "\n")
	for i, line := range lines {
		lines[i] = prefix + line
	}
	return strings.Join(lines, "\n")
}
