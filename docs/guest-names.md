# Guest names

Every guest who plays gets a generated name like `pizza-rat-astoria`: a thing you'd run into on a New York block, then a New York place. The code is the `names/` package (`names.go` builds a name, `words.go` holds the words). Milestone 06a starts using it ([spec](superpowers/specs/2026-10-05-06a-finding-games-design.md)).

## Why generated names

All games are public and there are no moderators, so guests can't type their own name. Players can re-roll (🎲) until they like one. Strangers then only ever see words from our own list. Lichess and chess.com do the same for players without an account: Lichess shows "Anonymous", and chess.com guests get a generated name.

## Where the words came from

The words were picked by hand: a first list on 2026-10-02 (commit `a608b64`), then widened on 2026-10-05.

- **Places.** The starting point was Wikipedia's [List of counties in New York](https://en.wikipedia.org/wiki/List_of_counties_in_New_York) (62 counties) and [List of towns in New York](https://en.wikipedia.org/wiki/List_of_towns_in_New_York) (933 towns). Only the popular ones were kept, so a name stays recognisable: nobody smiles at `bagel-wampsville`.
  - **The first list leaned on New York City:** the boroughs, the city's counties (Bronx, Kings, Queens, Richmond), and well-known neighborhoods in each borough. Neighborhoods aren't on either Wikipedia list (the city has no towns), but they're what makes a name read as New York.
  - **The 2026-10-05 additions came from the two lists:**
    - popular counties (Westchester, Nassau, Suffolk, Albany, Rockland, Saratoga, Niagara);
    - towns from Long Island, Westchester, the Hudson Valley and the Catskills (Hempstead, Oyster Bay, Southampton, East Hampton, Scarsdale, Rye, Poughkeepsie, Woodstock, New Paltz, Ithaca…);
    - each one checked against the raw Wikipedia list.
  - **Famous places on neither list.** These are villages, hamlets, areas or cities, not counties or towns: Montauk, Yonkers, Tarrytown, Sleepy Hollow, Beacon, Cooperstown, Lake Placid, the Hamptons, Fire Island, Buffalo, Rochester, Syracuse. So the rule for adding a place is now "a New York place most people would recognise", with the two lists as the first place to look.
- **Nouns.** Words that feel distinctly New York, grouped in `words.go`:
  - critters (pizza rat, bodega cat);
  - food (knish, chopped cheese, halal cart);
  - the street (stoop, hydrant, pothole, MetroCard);
  - people (cabbie, super, busker);
  - transit (turnstile, straphanger, dollar van);
  - public office, since the game is named after a mayor (mayor, comptroller, public advocate, borough president, night mayor, dog catcher);
  - Gen Z and Gen Alpha slang, as personas (rizzler, unc, goat, npc, main character, aura farmer, yapper, delulu, skibidi, six-seven, karen); see below;
  - talk (schlep, kvetch);
  - chess words from Washington Square Park's hustlers.

  Brand names (Sabrett, Mister Softee) and campaign slogans are left out.

### Slang (added 2026-10-05)

Slang works when it names a persona: `rizzler-astoria` reads as "the rizzler of Astoria", and `npc-midtown` and `main-character-soho` are jokes about the place itself. Sources: [Mental Floss, top Gen Alpha slang of 2026](https://www.mentalfloss.com/language/slang/top-gen-alpha-slang-2026), [Gabb's teen slang guide](https://gabb.com/blog/teen-slang/), [genppt's Gen Alpha list](https://genppt.com/blog/gen-alpha-slang).

- **Left out:**
  - offensive or sexual terms (`foid`, `bop`, `chud`, `huzz`, `gyatt`);
  - insults that become a stereotype when paired with a place (`menace`, `crashout`, `opp`, `chopped`, `mid`);
  - `mogger`, which comes from "looksmaxxing";
  - `fanum-tax`, which is named after a streamer;
  - `deadass`, which is real New York slang but profane;
  - `cheugy`, which is already dated.
- **Kept on purpose despite some baggage:** `clanker`, `based` and `karen`. Their risky pairs are blocked instead.
- **Slang dates fast.** `skibidi` and `six-seven` may feel embarrassing within a year. Dropping a word later only stops new draws; it never renames anyone.

### What was taken out, and why (2026-10-05)

A review pass dropped words that wouldn't do well:

- **Could read as a dig at people.**
  - `coyote` is also slang for a people smuggler, and it would land on immigrant neighborhoods like Corona and Jackson Heights.
  - `black-white` (the cookie) reads as a statement about race without the word "cookie".
  - `hustler` was kept for the Washington Square chess hustlers, with its pairs blocked (below).
- **Places people wouldn't recognise, or would read as something else.**
  - `kings` reads as a plural. Few people know it's Brooklyn's county.
  - `erie` reads as "eerie", and `ulster` as Northern Ireland.
  - `dutchess` looks like a misspelling of "duchess".
  - `onondaga`, `tompkins` and `putnam` are counties people know by their city, not their own name.
  - `hunter` reads as a noun: `pigeon-hunter`.
  - `kingsbridge`, `windham`, `eastchester`, `southold` aren't widely known.
  - `saugerties` and `mamaroneck` are hard to spell and say.
- **Abbreviations and jargon that look like typos to anyone who isn't local.**
  - `les` reads as French, and `lic` and `bec` are unclear (`bec-les` was possible).
  - `alt-side` is parking jargon.
  - `check` reads as an instruction: `check-harlem`.

Some were kept on purpose: `richmond`, `the-village`, `two-bridges`, `greenburgh`, `yorktown`. A few ambiguous ones stay because the mix-up is fun or harmless:
- `rye` (`pastrami-rye` is a sandwich);
- `jamaica` and `kingston`;
- `corona`;
- `fishkill`, `babylon`.

## The rules

These are enforced by `names_test.go`:

- **Shape.** Words are lowercase slugs: letters and single hyphens (`hells-kitchen`, not `hell's kitchen`). Abbreviations people actually say are fine (`les`, `uws`, `fidi`, `lic`).
- **At most 3 words** (`names.MaxWords`). A name is `noun-place`, so a two-word noun takes a one-word place and a two-word place takes a one-word noun: `chopped-cheese-manhattan`, never `chopped-cheese-jackson-heights`.
- **At most 24 characters** (`names.MaxLen`). Since the 3-word rule, this rules out only 160 pairs, such as `straphanger-jackson-heights`. The longest names are 24 characters, e.g. `fire-escape-williamsburg`.
  - Where 24 characters don't fit, the page cuts the name with an ellipsis. On a phone's live-game card that applies to names over about 20 characters (950 of them).
  - Until 2026-10-05 the cap was 20 characters. That came from the original spec's free-text rename limit, which no longer exists.
- Pairs that break a rule are skipped and both words redrawn, so every allowed name is equally likely.
- **No duplicates** within a list.
- **Enough names.** At least 2,000 must fit. Today it's 10,448:
  - 112 nouns (77 one-word, 35 two-word) and 104 places (75 one-word, 29 two-word) make 11,648 pairs;
  - the 3-word rule drops 1,015 of them, the length cap 160, and the blocked list 25.

  The first list gave 2,987.
- **Blocked pairs.** `blocked` holds pairs that read badly together, and they're redrawn.
  - Mostly these are stereotypes about who lives somewhere: `hustler-` with Harlem, Bed-Stuy, Mott Haven, Hunts Point, the Bronx, Canarsie or Jamaica; rats and pigeons with Chinatown or Flushing; `dog-catcher` and `npc` with Chinatown or Flushing; `clanker` with the same places as `hustler` (online skits have used it as a stand-in for a racial slur); `landlord-harlem`.
  - Every entry must be a real noun-place pair (`TestBlockedPairsExist`), so a typo can't quietly block nothing.
  - When you add a word, read its pairs with the sensitive places and block any that read as a dig at the people who live there.

Names are display-only and don't need to be unique. With about 10,500 possible names, two guests out of about 120 are likely to share one.

## Names are stored, not derived

A guest's name is saved in the `guests` table when they first play. It is never recomputed from their ID. So editing `words.go` (adding, removing or blocking words) never renames anyone who already has a name. It only changes what new guests and re-rolls can draw. Each game also keeps a copy of both players' names from when they sat down.
