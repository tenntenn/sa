---
name: sa
description: Show a diff to a human for review in the browser with sa, then read their line comments back. Use after producing or being handed changes that a human should look at, when the user asks to review a diff or open a diff/review UI, or when review comments are waiting in sa.
license: MIT
---

# Reviewing a diff with a human using sa

`sa` serves a unified diff as a review page in the browser. The human reads
the diff, attaches comments to lines, and you read those comments back from
the command line. sa never runs git itself: it reads the diff from stdin, so
it works with `git diff`, `jj diff`, `diff -u`, a `.patch` file, or a diff
you produced yourself.

## When to use this

- You changed code and want a human to look at it before continuing.
- The user asks to "review this diff", "show me the changes", "open the diff
  in a browser".
- The user says they left comments in sa (or you sent a diff earlier in this
  session and are picking the work back up).

Do not use it for changes the user asked you to apply without review, and do
not use it as a way to read a diff yourself: you can read the diff text
directly.

## Prerequisites

`sa` must be on PATH. Check with `sa --version`; if it is missing, tell the
user how to install it instead of guessing at a substitute:

```
go install github.com/tenntenn/sa@latest
```

Markdown preview inside sa is provided by `mo`, which is optional and only
needed when the diff touches Markdown:
`brew install k1LoW/tap/mo` (or a binary from
https://github.com/k1LoW/mo/releases).

## Workflow

### 1. Send the diff

Pipe the diff into `sa` and use `--target` to name the review, so several
reviews can be open at once without mixing their comments:

```
git diff | sa --target <topic>
```

Other ways to produce the same diff text:

```
git diff HEAD~1 | sa --target <topic>     # a specific range
git diff --cached | sa --target <topic>   # staged changes
diff -u old.txt new.txt | sa --target <topic>
cat change.patch | sa --target <topic>
```

sa prints the review URL and returns immediately; the server keeps running in
the background. Running `sa` again adds another diff to the same page rather
than starting a second server.

Use `--json` when you want to parse the result:

```
git diff | sa --target <topic> --json
```

### 2. Hand the URL to the human

Tell the user the URL sa printed and say what you want reviewed. Then stop
and wait for them. Do not poll `sa comments` in a loop and do not sleep
waiting for input; the user tells you when they are done.

### 3. Read the comments

```
sa comments --target <topic>
```

This prints the open comments as Markdown, each with the file, the line
range, the reviewed code and the comment body. For programmatic handling:

```
sa comments --target <topic> --format json
```

Every JSON entry has `id`, `path`, `side` (`new` or `old`), `startLine`,
`endLine`, `body`, `snippet`, `suggestion` and `resolved`. Line numbers refer
to the side named by `side`.

A comment may carry a suggested replacement. In the Markdown output it is a
fenced ` ```suggestion ` block under a line naming the file and the lines it
replaces; in JSON it is the `suggestion` field. Apply it verbatim to exactly
those lines unless it is wrong, and say so if you do not.

### 4. Act on every comment

Work through the comments one by one. Change the code where the comment asks
for a change, and replace the named lines exactly as written where a comment
carries a suggestion; when you disagree or a comment cannot be acted on, say
so explicitly in your reply to the user rather than silently skipping it.

### 5. Send the next round

Clear the handled comments and send the updated diff so the next round starts
clean:

```
sa comments --target <topic> --clear
git diff | sa --target <topic>
```

### Sharing a review without sa

When the human cannot run sa — a review that travels by mail, a page for
someone else, an artifact — write the review out as a single HTML file:

```
git diff | sa export --target <topic> review.html
```

The page carries the diff and the same UI, needs no server, and the comments
written on it stay in that browser. Use `--fragment` when the page is
embedded into something that brings its own `<html>` (for example an
artifact).

## Command reference

| Command | What it does |
| --- | --- |
| `<diff producer> \| sa` | Add a diff to the default group and print its URL |
| `... \| sa -t <name>` | Add it to a named group (its own URL and comments) |
| `... \| sa --title "..."` | Give the diff a title shown in the UI |
| `... \| sa --no-open` | Do not open a browser (useful in headless runs) |
| `sa comments [-t <name>]` | Print open comments as Markdown |
| `sa comments --format json` | Print comments as JSON |
| `sa comments --include-resolved` | Include comments the human resolved |
| `sa comments --clear` | Remove the comments of the group |
| `sa --status [--json]` | Show the running server, its groups and comment counts |
| `sa --clear -t <name>` | Drop the diffs and comments of a group |
| `sa --shutdown` | Stop the server |
| `... \| sa export <file>` | Write the review as one self-contained HTML page |
| `... \| sa export --fragment <file>` | The same, body only, for embedding |

`--port` (default 6280) selects the server; use it only if the user runs sa
on a non-default port.

## Notes

- New files are shown as a unified diff, because there is no old side to put
  next to them.
- Markdown files get a preview pane next to the diff. The preview shows the
  working tree file when it exists; otherwise sa rebuilds what it can from
  the diff, and unified diffs only carry the changed hunks, so such a preview
  is partial by nature.
- Comments are stored by the sa server, not in the browser, which is why they
  survive a reload and why you can read them from the command line.
- `sa --status --json` is the reliable way to check whether comments are
  waiting: it reports `comments` and `unresolved` per group.
