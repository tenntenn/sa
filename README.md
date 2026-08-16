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
$ sa --clear -t api    # drop the diffs and comments of a group
$ sa --restart         # restart, keeping the session
$ sa --shutdown        # stop the server
```

### Reviewing

- Click a line number to select a line, shift-click another to select a range,
  then write the comment.
- Comments can be resolved, edited and deleted, and they survive a reload.
- **Copy prompt** puts every open comment on the clipboard, ready for a coding
  agent.
- Split and unified views can be switched per file; new, deleted and binary
  files stay unified.
- Markdown files get a preview pane. The preview shows the working tree file
  when it exists; otherwise sa rebuilds the new side from the diff, which is
  complete for new files and partial for modified ones (a unified diff only
  carries the changed hunks).

### Reading the comments back

```console
$ sa comments                    # Markdown, ready to paste into an agent
$ sa comments --format json      # machine readable
$ sa comments -t api             # comments of the "api" group
$ sa comments --clear            # start the next review round
```

## Agent skill

sa ships a vendor neutral agent skill: plain Markdown with YAML front matter
that explains the review loop in terms of the sa command line, with no
assumptions about which agent reads it.

```console
$ sa skill                              # print it
$ sa skill --install .agents/skills     # write .agents/skills/sa/SKILL.md
$ sa skill --install ~/.claude/skills   # or wherever your agent looks
```

The source lives in [`skills/sa/SKILL.md`](skills/sa/SKILL.md), and
[`AGENTS.md`](AGENTS.md) points at it for agents that read that file.

## How the Markdown preview works

sa runs `mo --json <file> --target sa-<group>` and embeds the page mo answers
with. mo sends `frame-ancestors 'none'` on every response, so a page cannot
frame it directly; sa therefore publishes mo through a loopback-only reverse
proxy that rewrites that one directive to allow sa's own origin, and forwards
everything else — the rest of mo's CSP included — unchanged. "Open in mo"
always links to mo itself.

Note that mo cannot be used as a Go library today: everything but its cobra
entry point lives under `internal/`, and the published module does not build
because the embedded frontend is not part of it.

## Files and ports

| What | Where |
| --- | --- |
| sa server | `localhost:6280` (`--port`) |
| mo server | `localhost:6275` (`--mo-port`) |
| Preview proxy | a loopback port picked at startup |
| Session state | `$XDG_STATE_HOME/sa/session-<port>.json` |
| Server log | `$XDG_STATE_HOME/sa/server-<port>.log` |
| Rebuilt previews | `$XDG_CACHE_HOME/sa/preview/…` |

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
