# Milestone 07: launch — brainstorm notes

**Status:** brainstorming, 2026-10-06. Not a spec yet. The decisions below are agreed; the rest of milestone 07's scope is in the roadmap (`docs/superpowers/plans/2026-10-03-00-roadmap.md`, row 07 and "Launch hardening"). One spec for all of 07, written from these notes.

## Agreed

- **Audience:** a few friends first.
- **URL:** launch on the Railway subdomain (https://mamdani-chess.up.railway.app).
- **Rules page:** the canvas rules page, plus a collapsible full-rules section written for 06c's rules (three-round potholes, at most 5 open, a roll can checkmate).
- **The Mamdani art** is already on the boards.
- **Player history** waits for milestone 08.

## The app is called Mamdani Chess (decided 2026-10-06)

The name was "Pothole Chess: Mamdani Edition". It becomes **Mamdani Chess**, so that the name matches the repo, the Railway project and subdomain (`mamdani-chess`), and a domain if one is bought later.

Why not the alternatives:
- **"Pothole Chess"** is Spicer and Chamberlain's 2001 variant. Leading with it claims their game; what is new here is the Mamdani. Their credit stays wherever it is now (home page footer, rules, README).
- **"Mamdani Patch"** (after the reel) doesn't say it's a game, and "patch" reads as a software update. "Patch" is kept as the word for the repair moment instead (e.g. "Mamdani patched it! 👍").

**Tagline:** not settled. Candidate: "Pothole chess. Roll the dice, dodge the holes, and let the Mamdani patch them." It takes the place of the "Mamdani Edition" line wherever a subtitle is needed.

**Domain:** mamdanichess.com was unregistered on 2026-10-06 (also potholechess.com and mamdanipatch.com). Buying it is the owner's call. Pointing it at Railway waits until the game is opened up beyond friends. The name belongs to a real public figure, so keep the site clearly unofficial and ad-free.

### What the rename touches

Done on branch `rename-mamdani-chess`: the header drops the edition tag, and the preview card reads "A POTHOLE CHESS VARIANT" over "MAMDANI CHESS". The game page (desktop logo, phone logo, tab title), the quick match tab title and the note in `web/static/pieces/LICENSE.txt` were renamed too. Left: the live rules doc and the canvas.

| Where | Now | Becomes |
| --- | --- | --- |
| `web/src/lib/Header.svelte` | "Pothole Chess" + "Mamdani Edition" | "Mamdani Chess" (same length, so the 20 px phone header still fits); drop or replace the edition line |
| `web/src/routes/+page.svelte` | `<title>Pothole Chess: Mamdani Edition</title>` | `Mamdani Chess` |
| `web/src/routes/dev/board/+page.svelte` | "Pothole Chess" title and logo | "Mamdani Chess" |
| `server/preview.go` | link previews: site name, titles ("… · Pothole Chess", "invites you to Pothole Chess"), image alt | "Mamdani Chess" throughout |
| `server/preview_test.go` | expects the old titles | the new ones |
| `web/scripts/og-image.js` | "MAMDANI EDITION" eyebrow | rework the preview image around the new name; regenerate it |
| `README.md`, `RULES.md`, `CLAUDE.md` | "# Pothole Chess: Mamdani Edition" | "# Mamdani Chess"; the first line still says it is Pot-Hole Chess plus the Mamdani |
| `cmd/server/main.go`, `rules/board.go` | package comments | "Mamdani Chess" |
| Live rules doc and canvas (artifacts) | old name in titles and artboards | update with the rules page work |

Unchanged: the footer credit "Based on Pot-Hole Chess by Peter Spicer and Michael Chamberlain (2001)", the cburnett credit, and older specs and plans (they record what was decided at the time).
