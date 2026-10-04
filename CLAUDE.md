# Pothole Chess: Mamdani Edition

A browser chess variant to play with friends: the original Pot-Hole Chess (Spicer and Chamberlain, 2001) plus one neutral piece, the Mamdani.

## Sources of truth

| What | Where |
| --- | --- |
| Rules | Live doc: https://claude.ai/artifact/AvSPCQS42ggQGpTWQGoQRB. Its "Decisions to lock down" table records every rule choice and why. `RULES.md` is a copy; if they disagree, the doc wins and `RULES.md` should be re-synced. |
| Design spec | `docs/superpowers/specs/2026-10-01-mamdani-chess-design.md`: stack, API, SSE events, clocks, rules engine, testing, visual tokens. |
| Visual design | Canvas: https://claude.ai/artifact/XVJqVp283CoZEHpmHDrSih. Every artboard is the reference ("road works" look). |

## Decisions that are settled (don't reopen without asking)

- Rules are the original Pot-Hole Chess plus only the Mamdani. Roll a d8 after every move; even opens a new pothole; it closes after the roller's next move. Kings never fall (re-roll). All dice are d8.
- Stack: Go server (SSE out, JSON POST in, SQLite), SvelteKit static frontend with a custom board (no chessground). One Railway service.
- Guests only, all games public, one clock (10+5), quick match only, emoji reactions instead of chat.
- Visual design: "road works", dark only. Potholes are a dark hole with an orange ring.

## Working rules

- Do not add Claude attribution to commits or PRs: no `Co-Authored-By: Claude` trailer, no "Generated with Claude Code" line.
- Stage only the files you changed (`git add <paths>`), never `git add -A`. `go.mod` and `names/` (guest-name generator) were written separately; don't move or rewrite them without asking.
- The Mamdani photo on the canvas is a placeholder; the shipped game needs art we have rights to.

## Next step

Follow `docs/superpowers/plans/2026-10-03-00-roadmap.md`: a playable game first, the Railway launch last. Plans 01 (walking skeleton, minus the deploy) and 02 (rules engine) are merged; next is Plan 03, a rough playable game. Repo: https://github.com/josephsintum/mamdani-chess (public).
