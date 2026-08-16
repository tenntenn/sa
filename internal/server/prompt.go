package server

import (
	"fmt"
	"strings"

	"github.com/tenntenn/sa/internal/model"
)

// PromptOptions tunes the prompt rendered from review comments.
type PromptOptions struct {
	// IncludeResolved keeps comments that were marked as resolved.
	IncludeResolved bool
	// NoInstruction drops the closing instruction, leaving only the
	// comments themselves.
	NoInstruction bool
}

// Prompt renders the review comments of a group as Markdown meant to be
// handed to a coding agent.
func Prompt(g *model.Group, opts PromptOptions) string {
	var b strings.Builder
	comments := make([]*model.Comment, 0, len(g.Comments))
	for _, c := range g.Comments {
		if c.Resolved && !opts.IncludeResolved {
			continue
		}
		comments = append(comments, c)
	}

	fmt.Fprintf(&b, "# Review comments (sa group %q)\n\n", g.Name)
	if len(comments) == 0 {
		b.WriteString("No open review comments.\n")
		return b.String()
	}
	fmt.Fprintf(&b, "%d comment(s) to address.\n", len(comments))

	titles := map[string]string{}
	for _, d := range g.Diffs {
		titles[d.ID] = d.Title
	}

	for i, c := range comments {
		fmt.Fprintf(&b, "\n## %d. %s%s\n", i+1, c.Path, lineRange(c))
		if title := titles[c.DiffID]; title != "" {
			fmt.Fprintf(&b, "\nDiff: %s\n", title)
		}
		if c.Author != "" {
			fmt.Fprintf(&b, "\nFrom: %s\n", c.Author)
		}
		if c.Resolved {
			b.WriteString("\nStatus: resolved\n")
		}
		if snippet := strings.TrimRight(c.Snippet, "\n"); snippet != "" {
			fence := fenceFor(snippet)
			fmt.Fprintf(&b, "\n%s\n%s\n%s\n", fence, snippet, fence)
		}
		// The body is Markdown and may carry suggestion blocks, so it goes
		// out as it is rather than quoted line by line.
		if body := strings.TrimRight(c.Body, "\n"); body != "" {
			fmt.Fprintf(&b, "\n%s\n", body)
		}
		if n := len(model.Suggestions(c.Body)); n > 0 {
			fmt.Fprintf(&b, "\nThe suggestion block above replaces %s.\n", lineRangeText(c))
			if n > 1 {
				fmt.Fprintf(&b, "(%d suggestion blocks: apply them in order.)\n", n)
			}
		}
	}

	if !opts.NoInstruction {
		b.WriteString("\n---\n\n")
		b.WriteString("Address every comment above. A suggestion block replaces the lines it " +
			"names, verbatim. When a comment is not worth acting on, say why instead of " +
			"changing the code.\n")
	}
	return b.String()
}

// lineRangeText names the lines a suggestion replaces, e.g. "path:12-18".
func lineRangeText(c *model.Comment) string {
	return c.Path + lineRange(c)
}

// lineRange formats the reviewed line range, e.g. ":12-18 (old)".
func lineRange(c *model.Comment) string {
	if c.StartLine <= 0 {
		return ""
	}
	side := ""
	if c.Side == "old" {
		side = " (old)"
	}
	if c.EndLine > c.StartLine {
		return fmt.Sprintf(":%d-%d%s", c.StartLine, c.EndLine, side)
	}
	return fmt.Sprintf(":%d%s", c.StartLine, side)
}

// fenceFor returns a code fence long enough to wrap content that may itself
// contain backticks.
func fenceFor(content string) string {
	longest, current := 0, 0
	for _, r := range content {
		if r == '`' {
			current++
			if current > longest {
				longest = current
			}
			continue
		}
		current = 0
	}
	if longest < 3 {
		return "```"
	}
	return strings.Repeat("`", longest+1)
}
