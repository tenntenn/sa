# sa

diff viewer and reviewer.

`sa` serves a unified diff as a review page in your browser: GitHub-like diffs
on the left, a Markdown preview on the right, and review comments you can read
back from the command line.

It is inspired by [difit](https://github.com/yoshiko-pg/difit), with a few
deliberate differences:

- **sa never runs git.** It reads a diff from stdin, so anything that can
  produce one works: `git diff`, `jj diff`, `diff -u`, a `.patch` file, or a
  coding agent.
- **New files are shown unified**, because there is no old side to put next to
  them.
- **Markdown is previewed by [mo](https://github.com/k1LoW/mo)**, side by side
  with the diff in a split pane.
- **One server, growing session.** Like mo, the first invocation starts a
  background server and later ones add their diff to it.
- **Comments live in the server**, not in the browser, so an agent can read
  them with `sa comments`.

## Install

```console
$ go install github.com/tenntenn/sa@latest
```

The Markdown preview needs the `mo` binary on PATH:

```console
$ brew install k1LoW/tap/mo
```

or download it from the [mo releases page](https://github.com/k1LoW/mo/releases).
`go install` does not work for mo: its published module does not carry the
embedded frontend. Everything except the Markdown preview works without it.

## Usage

```console
$ git diff | sa                      # start the server and review the diff
$ git diff --cached | sa             # add another diff to the same page
$ git diff HEAD~3 | sa -t refactor   # a named group with its own URL
$ diff -u old.md new.md | sa
$ cat change.patch | sa
```

The first invocation starts the server on <http://localhost:6280>, opens a
browser and returns the shell. Later invocations add to the running server.

```console
$ sa --status          # what is being reviewed
$ sa --clear -t api    # close that review: its diffs, comments and hooks
$ sa --clear --all     # close every review the server holds
$ sa --restart         # restart, keeping the session
$ sa --shutdown        # stop the server
```

**Close** in the header does the same for the review you are looking at, once
it has served its purpose. Closing a review is not the same as stopping the
server: `--shutdown` leaves everything where it is and `--restart` brings it
back, while closing throws the review away.

### Groups

`--target` (`-t`) puts a diff in a named group with its own URL, its own
comments and its own review. What a group stands for is up to you: a task, a
branch, a checkout you keep open next to another one. sa does not try to
work any of that out — it only keeps apart what you asked it to keep apart.

`SA_TARGET` says it once for a whole shell, which is what a script or an
agent driving several reviews at a time wants:

```console
$ export SA_TARGET=hotfix
$ git diff | sa            # http://localhost:6280/hotfix
$ sa comments              # the comments of the same review
$ sa comments -t default   # the flag still wins
```

### Reviewing

- Select the lines to comment on by dragging over the line numbers, by
  shift-clicking, or - on a touch screen - by tapping one line and then
  another while the draft is open.
- **Suggest a change** drops a ` ```suggestion ` block into the comment,
  pre-filled with the lines as they read today — the same shape GitHub uses.
  It renders as a small diff (the lines now, the lines as proposed) and
  travels through `sa comments` as the fenced block itself, so an agent can
  apply it verbatim. A comment can carry prose, several suggestions, or both.
- Comments can be resolved, edited and deleted, and they survive a reload.
- **Copy prompt** puts every open comment on the clipboard, ready for a coding
  agent.
- Split and unified views can be switched per file; new, deleted and binary
  files stay unified.
- Every pane is resizable and can be minimised away: drag the edge between
  two panes, double click it to reset (or to put the file list away), or use
  the **Files**, **Diff** and **Preview** switches in the header. Dragging an
  edge all the way in minimises that pane, and dragging it back out or double
  clicking brings it back. The layout is remembered per browser.
- Markdown files get a preview pane. The preview shows the working tree file
  when it exists; otherwise sa rebuilds the new side from the diff, which is
  complete for new files and partial for modified ones (a unified diff only
  carries the changed hunks).
- The header switch cycles **Auto / Light / Dark**: Auto follows the system
  (or whatever host an exported page is read in), and a choice is remembered
  per browser. The Markdown preview keeps mo's own theme — mo has its own
  switch, in the toolbar inside the preview.
- On a phone the page shows one pane at a time behind a **Files / Diff /
  Preview** tab bar, and the diff drops to unified. The preview is rendered by
  the page itself there — mo keeps its own sidebar inside the frame, which
  would leave a column too narrow to read — and **Open in mo** opens the real
  mo page in its own tab.

### Comments from an agent

The loop goes both ways: an agent can point at the lines it is unsure about,
ask a question, or propose a change, and you see it in the browser next to
the diff, labelled with who wrote it.

```console
$ sa comment internal/server/server.go:120 -m "Should this be a 404?" --author claude
$ sa comment README.md:12-18 -m "Reworded" --suggest-file new.md --author claude
$ cat new.txt | sa comment main.go:42 -m "Simpler" --suggest -
```

`--suggest` appends the replacement to the comment as a ` ```suggestion `
block; the same block can be typed straight into `-m`.

The lines are the ones the diff shows (`--side old` for a removed line), the
file is looked up in the newest diff carrying that path, and sa fills in the
reviewed code itself. A whole self review can be posted at once:

```console
$ sa comment --json --author claude <<'EOF'
[
  {"path": "cmd/root.go", "line": "88", "body": "left over from the old flag"},
  {"path": "README.md", "line": "12-18", "body": "reworded", "suggestion": "..."}
]
EOF
```

### Finishing a review

**Submit review** in the header says "I am done looking". That is the moment
sa tells everything that is waiting — an agent, a script, another machine —
that the comments are worth reading. An optional note goes with it and shows
up at the top of the prompt. Sending another diff starts the next round, and
the group counts as unreviewed again.

### Waiting, and being woken up

A review lands when the human is ready: after the meeting, after lunch,
tomorrow. Two ways to be there when it does.

**Wait for it.** `sa wait` blocks on the server's event stream — no polling,
nothing missed, and a review that already happened returns straight away:

```console
$ git diff | sa --target api
$ sa wait --target api                 # returns when the review is submitted
$ sa wait --target api --timeout 30m   # exits with status 2 if it is not
```

**Or have the server start the work.** Register what to do and go away; sa
runs it when the human presses Submit review, whether or not anyone is still
around:

```console
$ git diff | sa --on-review 'claude -p "$(sa comments)"'
$ sa hook --on-review-url http://localhost:9000/reviews   # a POST instead
$ sa hook            # what is registered
$ sa hook --clear
```

The command runs through the shell with the review prompt on its stdin and
`SA_GROUP`, `SA_URL`, `SA_SERVER`, `SA_PORT`, `SA_COMMENTS` and
`SA_REVIEW_NOTE` in its environment. Hooks belong to a group and survive a
restart. They run a command of your own on your own machine, which is what
makes them useful and worth being deliberate about.

### Reading the comments back

```console
$ sa comments                    # Markdown, ready to paste into an agent
$ sa comments --format json      # machine readable
$ sa comments -t api             # comments of the "api" group
$ sa comments --clear            # start the next review round
```

## Looking back at reviews

Every submitted review is written down, so the rounds do not vanish with the
groups they belonged to:

```console
$ sa reviews                     # one line each, newest last
$ sa reviews --since 7d          # this week
$ sa reviews -t api --limit 5
$ sa reviews --stats             # what they say together
$ sa reviews --format json       # every comment, for your own analysis
```

```
2026-08-16 03:26  default    2 comment(s), 1 suggestion(s)  3 file(s), +11 -1  waited 42m
      だいたいOK
2026-08-16 11:02  api        1 comment(s)  3 file(s), +11 -1  waited 3h10m

2 review(s), 3 comment(s) (1.5 per review), 1 suggestion(s)
median wait from diff to review: 1h24m
most commented:
  internal/server/server.go                7
  README.md                                5
by kind of file:
  .go                                     22
  .md                                      9
by author:
  reviewer                                31
  claude                                   8
```

The log is one JSON object per line at `$XDG_STATE_HOME/sa/reviews.jsonl`,
so `jq` works on it directly and nothing about it is sa-shaped:

```console
$ jq -r '.comments[].path' ~/.local/state/sa/reviews.jsonl | sort | uniq -c | sort -rn
$ jq 'select(.group == "api") | .comments[].body' ~/.local/state/sa/reviews.jsonl
```

It stays on the machine that recorded it. Delete the file to forget the lot.

## Exporting a review

`sa export` writes the whole review as one self-contained HTML page: the same
UI, with the diff frozen into it and no server, no mo and no network behind
it. It is how a review reaches someone who does not run sa — attached to a
mail, dropped in a bucket, or published as an artifact.

```console
$ git diff | sa export review.html    # straight from stdin, no server needed
$ sa export -t api review.html        # the "api" group of a running server
$ git diff | sa export                # to stdout
$ git diff | sa export --fragment page.html   # body only, to embed elsewhere
```

On an exported page the diff is read-only, but reviewing still works:
comments can be written, resolved and edited (they are kept in that browser),
Markdown is rendered by the page itself instead of mo, and **Copy prompt**
produces exactly what `sa comments` would.

## Agent skill

sa ships a vendor neutral agent skill: one Markdown file with YAML front
matter that describes the review loop — send the diff, hand the URL to the
human, read the comments back, address them, start the next round — entirely
in terms of the sa command line. Nothing in it is specific to a vendor, so any
agent that can read an instruction file can use it.

The source is [`skills/sa/SKILL.md`](skills/sa/SKILL.md) and it is embedded in
the binary, so the copy you install always matches the sa you are running.

### Installing it

```console
$ sa skill                          # print SKILL.md to stdout
$ sa skill --list                   # list the files the skill consists of
$ sa skill --install <dir>          # write <dir>/sa/SKILL.md
$ sa skill --install <dir> --force  # overwrite an older copy
```

`--install` writes the skill as a `sa/` directory inside the directory you
name, and refuses to overwrite an existing file unless `--force` is given, so
re-running it after upgrading sa is a one-liner:

```console
$ sa skill --install ~/.claude/skills --force
```

Where `<dir>` should point depends on the agent. The common ones:

| Agent | Command |
| --- | --- |
| Claude Code, this project only | `sa skill --install .claude/skills` |
| Claude Code, all your projects | `sa skill --install ~/.claude/skills` |
| Anything that reads `AGENTS.md` (Codex, Jules, …) | `sa skill --install .agents/skills`, then link it (below) |
| An agent with its own rules directory (Cursor, Cline, …) | install into that directory — check the agent's docs for the path and the file naming it expects |
| No skill support at all | `sa skill >> AGENTS.md` (or the agent's instruction file) |

For agents that read `AGENTS.md` rather than a skills directory, install the
file and point at it from `AGENTS.md`:

```markdown
## Reviewing changes

Before showing a diff to a human, read [.agents/skills/sa/SKILL.md](.agents/skills/sa/SKILL.md).
```

That is exactly what this repository does — see [`AGENTS.md`](AGENTS.md).

### Checking that it took

Ask the agent to review something and watch what it runs: it should pipe a
diff into `sa --target <topic>`, give you the URL, wait, and then read your
comments with `sa comments`. If it does not, most agents need the skill
directory to be picked up at session start, so start a new session after
installing.

## How the Markdown preview works

sa runs `mo --json <file> --target sa-<group>` and embeds the page mo answers
with. mo sends `frame-ancestors 'none'` on every response, so a page cannot
frame it directly; sa therefore publishes mo through a loopback-only reverse
proxy that rewrites that one directive to allow sa's own origin, and forwards
everything else — the rest of mo's CSP included — unchanged. "Open in mo"
always links to mo itself.

Two places render Markdown without mo, because mo cannot be there: an
exported page (no server behind it) and a phone (no room for mo's own layout
inside the frame). Both ask sa for the Markdown and render it in the page,
and both keep a link to mo for the full thing.

Note that mo cannot be used as a Go library today: everything but its cobra
entry point lives under `internal/`, and the published module does not build
because the embedded frontend is not part of it.

## Files and ports

| What | Where |
| --- | --- |
| sa server | `localhost:6280` (`--port`) |
| mo server | `localhost:6275` (`--mo-port`) |
| Preview proxy | a loopback port picked at startup |
| Session state (diffs, comments, hooks) | `$XDG_STATE_HOME/sa/session-<port>.json` |
| Server log | `$XDG_STATE_HOME/sa/server-<port>.log` |
| Rebuilt previews | `$XDG_CACHE_HOME/sa/preview/…` |
| Exported pages | wherever you point `sa export` |

sa binds to loopback and has no authentication; `--dangerously-allow-remote-access`
is required to bind anywhere else.

## Development

The UI is React + Vite, embedded into the binary with `go:embed`. The built
assets are committed so that `go install` needs no Node.

```console
$ make build     # pnpm build in web/, then go build
$ make test      # go test ./...
$ make dev       # sa in the foreground plus the Vite dev server
```

## License

MIT
