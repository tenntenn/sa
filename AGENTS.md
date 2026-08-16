# Agent guide for this repository

## Reviewing changes with sa

This repository ships an agent skill for sa itself. Read
[`skills/sa/SKILL.md`](skills/sa/SKILL.md) before showing a diff to a human:
it describes the whole loop (send the diff, hand over the URL, read the
comments back, address them, start the next round).

Short version:

```console
$ git diff | sa --target <topic>   # opens the review page, prints its URL
$ sa comments --target <topic>     # the comments the human left
$ sa comments --target <topic> --clear
```

## Working on sa

- Build: `make build` (runs `pnpm build` in `web/`, then `go build`).
- Test: `make test`.
- The built UI in `web/dist` is committed on purpose, so `go install` works
  without Node. Rebuild it whenever `web/src` changes.
- sa must not shell out to git: diffs only ever come from stdin.
