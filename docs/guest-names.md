# Guest names

Every guest who plays gets a generated name like `pizza-rat-astoria`: a thing you'd run into on a New York block, then a New York place. The code is the `names/` package (`names.go` builds a name, `words.go` holds the words). Milestone 06a starts using it ([spec](superpowers/specs/2026-10-05-06a-finding-games-design.md)).

## Why generated names

All games are public and there are no moderators, so guests can't type their own name. Players can change it with the die: it offers 3 names to pick from, and they get 3 changes in any 24 hours. A name only works if it stays put long enough to be recognised, and offers that stay the same until one is chosen mean nobody can fish for a particular combination. Strangers then only ever see words from our own list. Lichess and chess.com do the same for players without an account: Lichess shows "Anonymous", and chess.com guests get a generated name.

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
  - people (cabbie, super, busker, dog walker);
  - transit (turnstile, straphanger, dollar van);
  - public office, since the game is named after a mayor (mayor, comptroller, public advocate, borough president, night mayor);
  - Gen Z and Gen Alpha slang, as personas (rizzler, unc, goat, npc, main character, aura farmer, yapper, delulu); see below;
  - talk (schlep, kvetch);
  - chess words from Washington Square Park's hustlers;
  - characters from GTA III and Vice City (Vercetti, Lance Vance); see below.

  Brand names (Sabrett, Mister Softee) and campaign slogans are left out. The GTA words are the one exception, below.

### Slang (added 2026-10-05)

Slang works when it names a persona: `rizzler-astoria` reads as "the rizzler of Astoria", and `npc-midtown` and `main-character-soho` are jokes about the place itself. Sources: [Mental Floss, top Gen Alpha slang of 2026](https://www.mentalfloss.com/language/slang/top-gen-alpha-slang-2026), [Gabb's teen slang guide](https://gabb.com/blog/teen-slang/), [genppt's Gen Alpha list](https://genppt.com/blog/gen-alpha-slang).

- **Left out:**
  - offensive or sexual terms (`foid`, `bop`, `chud`, `huzz`, `gyatt`);
  - insults that become a stereotype when paired with a place (`menace`, `crashout`, `opp`, `chopped`, `mid`);
  - `mogger`, which comes from "looksmaxxing";
  - `fanum-tax`, which is named after a streamer;
  - `deadass`, which is real New York slang but profane;
  - `cheugy`, which is already dated.
- **Slang dates fast.** Dropping a word later only stops new draws; it never renames anyone.

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

Some were kept on purpose: `richmond`, `the-village`, `greenburgh`, `yorktown`. A few ambiguous ones stay because the mix-up is fun or harmless:
- `rye` (`pastrami-rye` is a sandwich);
- `jamaica` and `kingston`;
- `corona`;
- `fishkill`, `babylon`.

### Second pass (2026-10-05)

The doubtful words were ranked worst first and the bottom was cut. The words that clearly work (`unc`, `manhattan`) weren't ranked.

- **Nouns removed.**
  - `karen` is a dig at the people of every place it lands on (`karen-park-slope`), so blocking pairs can't fix it.
  - `token` reads as "the token minority" next to Harlem or Chinatown.
  - `clanker` needed seven blocked pairs because of its use as a stand-in for a racial slur.
  - `based` has alt-right baggage and says nothing about a place.
  - `skibidi` and `six-seven` were dating fast, and `six-seven-bronx` reads as a number.
  - `express` reads as a train line, not a persona.
- **Places removed.**
  - `catskill` looks like a typo for "Catskills".
  - `brookhaven`, `islip`, `smithtown` and `riverhead` are Long Island towns that only Long Islanders know.
  - `two-bridges` is obscure even to New Yorkers.
- **Replaced.**
  - `dog-catcher` became `dog-walker`. The office no longer exists, and its pairs read as insults.
  - `staten` became `staten-island`, since nobody says "Staten" alone.
- **Blocked.** The Chinatown blocks (rats, pigeons, raccoons, `npc`) now also cover Sunset Park and Elmhurst. `pizza-rat-sunset-park` is four words, so it can't be drawn anyway.

### GTA throwback (2026-10-05)

A nod to GTA III (2001) and Vice City (2002). GTA III's Liberty City is a parody of New York, so its places sit with the real ones: `pothole-saint-marks`, `rizzler-staunton`.

- **Places:** Liberty City's three islands and their districts:
  - `liberty-city`;
  - `staunton` (Staunton Island, the game's Manhattan), `fort-staunton`, `bedford-point`, `belleville-park`, `torrington`;
  - `shoreside-vale`, `wichita-gardens`;
  - `hepburn-heights`, `saint-marks` (also a real East Village street), `callahan-point`.
- **Nouns:** four characters: `vercetti` (Tommy Vercetti, Vice City's hero), `lance-vance`, `kent-paul`, `phil-cassidy`.
- **Sources:** spellings checked against [gtabase's GTA III map locations](https://gtabase.com/gta-3/map-locations), and its [Lance Vance](https://gtabase.com/gta-vice-city/characters/lance-vance) page.
- **These are Rockstar's names.** They're kept as a deliberate homage, unlike the brand names left out above.
- **Blocked:** Vercetti is a drug kingpin and Lance Vance a cocaine dealer, so they get the same blocks as `hustler`, plus `vercetti-bensonhurst` (an Italian-American mob stereotype).
- **Left out:**
  - districts that read as real places elsewhere (`portland`, `trenton`, `rockford`);
  - `faggio` (a scooter whose name contains a slur);
  - `eight-ball` (drug and race baggage);
  - `busted` and `wasted` (arrest and drinking digs about a place);
  - `rampage` and `ammu-nation` (guns).
- **Offered but not picked:** cars (`banshee`, `infernus`), radio stations (`flashback-fm`), `mr-whoopee` and `borgnine`.

## The rules

These are enforced by `names_test.go`:

- **Shape.** Words are lowercase slugs: letters and single hyphens (`hells-kitchen`, not `hell's kitchen`). Abbreviations people actually say are fine (`les`, `uws`, `fidi`, `lic`).
- **At most 3 words** (`names.MaxWords`). A name is `noun-place`, so a two-word noun takes a one-word place and a two-word place takes a one-word noun: `chopped-cheese-manhattan`, never `chopped-cheese-jackson-heights`.
- **At most 24 characters** (`names.MaxLen`). Since the 3-word rule, this rules out only 193 pairs, such as `straphanger-jackson-heights`. The longest names are 24 characters, e.g. `fire-escape-williamsburg`.
  - Where 24 characters don't fit, the page cuts the name with an ellipsis. On a phone's live-game card that applies to names over about 20 characters (about 1,600 of them).
  - Until 2026-10-05 the cap was 20 characters. That came from the original spec's free-text rename limit, which no longer exists.
- Pairs that break a rule are skipped and both words redrawn, so every allowed name is equally likely.
- **No duplicates** within a list.
- **Enough names.** At least 2,000 must fit. Today it's 10,245:
  - 109 nouns (72 one-word, 37 two-word) and 109 places (71 one-word, 38 two-word) make 11,881 pairs;
  - the 3-word rule drops 1,406 of them, the length cap 193, and the blocked list 37.

  Before the second pass it was 10,448, and 9,128 after it (before the GTA words).

  The first list gave 2,987.
- **Blocked pairs.** `blocked` holds pairs that read badly together, and they're redrawn.
  - Mostly these are stereotypes about who lives somewhere: `hustler-` with Harlem, Bed-Stuy, Mott Haven, Hunts Point, the Bronx, Canarsie or Jamaica; rats, pigeons and `npc` with Chinatown, Flushing, Sunset Park or Elmhurst (the city's large Chinese neighborhoods); `landlord-harlem`; `vercetti` and `lance-vance` with the `hustler` places, and `vercetti-bensonhurst`.
  - Every entry must be a real noun-place pair (`TestBlockedPairsExist`), so a typo can't quietly block nothing.
  - When you add a word, read its pairs with the sensitive places and block any that read as a dig at the people who live there.

Names are display-only and don't need to be unique, but new names prefer ones nobody has: the server draws up to 5 candidates and keeps the first unused one, and falls back to the last draw if all are taken. That keeps names unique in practice until roughly 10,000 guests. Without that check, with about 10,200 possible names, two guests out of about 120 are likely to share one.

## Names are stored, not derived

A guest's name is saved in the `guests` table when they first play. It is never recomputed from their ID. So editing `words.go` (adding, removing or blocking words) never renames anyone who already has a name. It only changes what new guests and name offers can draw. Each game also keeps a copy of both players' names from when they sat down.
