# Agent guide for this repository

## Reviewing changes with sa

This repository ships an agent skill for sa itself. Read
[`skills/sa/SKILL.md`](skills/sa/SKILL.md) before showing a diff to a human:
it describes the whole loop (send the diff, hand over the URL, read the
comments back, address them, start the next round).

To use the same skill in another repository, install it there with
`sa skill --install <dir>` (see the README for the directory each agent reads,
or link the installed file from that repository's AGENTS.md the way this
section does).

Short version:

```console
$ git diff | sa --target <topic>   # opens the review page, prints its URL
$ sa comments --target <topic>     # the comments the human left
$ sa comments --target <topic> --clear
$ git diff | sa export --target <topic> review.html   # a page that needs no server
```

## Working on sa

- Build: `task build` (runs `pnpm build` in `web/`, then `go build`).
- Test: `task test`.
- Tools are managed with [aqua](https://aquaproj.github.io/); run `aqua install` to get `task`.
- The built UI in `web/dist` is committed on purpose, so `go install` works
  without Node. Rebuild it whenever `web/src` changes.
- sa must not shell out to git: diffs only ever come from stdin.
