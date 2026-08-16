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

### 1. Start from a clean page

A group keeps whatever was left in it: diffs from an earlier task, comments
nobody cleared, hooks from a session that ended. Mixing last week's change
into today's review costs the human the attention you asked for, so close the
old review before opening a new one:

```
sa --status --json                  # what is the group holding?
sa --clear --target <topic>         # close it: diffs, comments and hooks
```

Two exceptions, both of which mean "do not clear":

- You are sending the **next round** of a review you already started. Then
  the diffs belong together; clear the handled comments instead (step 7).
- The group holds **comments the human wrote that you have not addressed**.
  Say what is in there and ask before throwing it away.

### 2. Send the diff

Pipe the diff into `sa` and use `--target` to name the review, so several
reviews can be open at once without mixing their comments:

```
git diff | sa --target <topic>
```

Pick the name yourself and keep using it for every command of that review.
sa attaches no meaning to it, so make it stand for whatever separates this
review from your others — the task, the branch, the checkout you are working
in. If everything you do in this session belongs to one review, export
`SA_TARGET=<topic>` once instead of repeating the flag.

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

### 3. Hand the URL to the human, and decide how you come back

Tell the user the URL sa printed and say what you want reviewed. Then pick
one of these — never poll `sa comments` in a loop:

- **They are reviewing now and you can wait**: `sa wait --target <topic>`
  blocks until they press Submit review and then prints the comments. Give it
  a `--timeout` you can afford; status 2 means "not reviewed yet".
- **The review may land later** — they are in a meeting, it is late, your
  session will not live that long: register the follow-up before you go, so
  the sa server starts it when the review is submitted, and end your turn.

  ```
  sa hook --target <topic> --on-review '<command that resumes the work>'
  ```

  The command gets the review prompt on its stdin and `SA_GROUP`, `SA_URL`,
  `SA_COMMENTS` and `SA_REVIEW_NOTE` in its environment. Ask the user what
  that command should be for their setup rather than guessing; if they do not
  want one, tell them to run `sa comments` and paste the result to you when
  they are back.
- **Neither**: say you will pick the review up next time, and stop. Nothing
  is lost — the comments stay in the sa server until they are cleared.

### 4. Leave your own comments, if you have any

Before handing over, you can mark the places you are unsure about, so the
human reviews them first. Always pass `--author` with your own name so the
human can tell your notes from theirs:

```
sa comment <path>:<line> -m "<question or note>" --author <you> --target <topic>
sa comment <path>:<line>-<line> -m "..." --suggest "<replacement>" --author <you>
```

`--suggest` appends the replacement to the comment as a ` ```suggestion `
block, so the human sees it as a proposed change and can copy it:

```
```

Use it for what is genuinely worth a human's attention — a decision you had
to guess at, a trade-off, something you could not verify. A comment on every
change is noise, not a review.

### 5. Read the comments

```
sa comments --target <topic>
```

This prints the open comments as Markdown, each with the file, the line
range, the reviewed code and the comment body. For programmatic handling:

```
sa comments --target <topic> --format json
```

Every JSON entry has `id`, `path`, `author`, `side` (`new` or `old`),
`startLine`, `endLine`, `body`, `snippet`, `suggestions` and `resolved`. Line
numbers refer to the side named by `side`. `author` is empty for the comments
the human wrote in the browser and set for the ones posted from the command
line — including your own, so skip those when working through the list.

A comment may carry suggested replacements, written as fenced
` ```suggestion ` blocks inside the comment itself, the same convention
GitHub uses. The Markdown output prints the comment as it is and then names
the lines the block replaces; in JSON they are the `suggestions` array. Apply
each block verbatim to exactly those lines unless it is wrong, and say so if
you do not.

### 6. Act on every comment

Work through the comments one by one. Change the code where the comment asks
for a change, and replace the named lines exactly as written where a comment
carries a suggestion; when you disagree or a comment cannot be acted on, say
so explicitly in your reply to the user rather than silently skipping it.

### 7. Send the next round

Clear the handled comments and send the updated diff so the next round starts
clean. This is the one case where the diffs stay: the rounds of one review
belong together.

```
sa comments --target <topic> --clear
git diff | sa --target <topic>
```

When the work is done and the review has served its purpose, close it, so the
next one starts on an empty page (the human can also press Close in the
browser):

```
sa --clear --target <topic>
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

## Fitting sa into what you were already doing

sa is one command among the ones you already run, so let the shell do the
joining rather than looking for a flag:

```
git diff | sa                          # anything that writes a diff feeds it
sa comments | pbcopy                   # anything that reads text takes it
sa reviews --format jsonl | jq ...     # a line per review, for whatever asks
```

`sa comments` and `sa wait` say what they found in their exit status too — 0
when there is nothing to address, 1 when there is, and 2 from `sa wait` when
the review has not happened yet — so a review can gate what comes next
without anyone reading the output:

```
git diff | sa --target <topic>
sa wait --target <topic> -q && git commit -m "<message>"
```

That is the recommended way round committing, and it is worth being plain
with the user about why: send what you are about to commit, wait for the
review, and commit only once it comes back with nothing to address. sa has
no idea what a commit is and stays out of the way of one — it writes nothing
into the working tree, so `git status` says exactly what it said before you
started. When a review does have comments, address them and send the next
round before committing rather than committing over them.

If the change is already committed, review it the same way: `git show | sa`,
or `git diff <base>..HEAD | sa`. Use `--title` to say which is which, since
sa only sees the text.

## Learning from past reviews

Every submitted review is kept, which makes the reviewer's habits readable
rather than guessed at:

```
sa reviews --stats                # which files draw comments, how many per review
sa reviews --since 30d --format json   # every comment, to read properly
```

Worth doing before you hand over a change of the same shape: if the last ten
reviews of this repository were mostly about error messages and test names,
check yours before asking. Say what you found and what you changed because of
it — a pattern you read out of the log is a claim about the human, so let
them correct it.

## Command reference

| Command | What it does |
| --- | --- |
| `<diff producer> \| sa` | Add a diff to the default group and print its URL |
| `... \| sa -t <name>` | Add it to a named group (its own URL and comments) |
| `... \| sa --title "..."` | Give the diff a title shown in the UI |
| `... \| sa --no-open` | Do not open a browser (useful in headless runs) |
| `sa comment <path>:<line> -m "..."` | Leave a comment of your own (pass `--author`) |
| `sa comment --json` | Post many comments at once, read from stdin |
| `sa comments [-t <name>]` | Print open comments as Markdown |
| `sa comments --format json` | Print comments as JSON |
| `sa comments --include-resolved` | Include comments the human resolved |
| `sa comments --clear` | Remove the comments of the group |
| `sa --status [--json]` | Show the running server, its groups and comment counts |
| `sa --clear [-t <name>]` | Close a review: its diffs, comments and hooks |
| `sa --clear --all` | Close every review on the server |
| `sa wait [-t <name>]` | Block until the review is submitted, then print it |
| `sa hook --on-review '<cmd>'` | Have the server run something when the review lands |
| `sa hook [--clear]` | List or drop those hooks |
| `sa reviews [--stats] [--since 7d]` | The reviews that were submitted, and what they say together |
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
  waiting: it reports `comments`, `unresolved` and `reviewed` per group.
  `reviewed` is true once the human has submitted, and false again as soon as
  a newer diff arrives.
