# Sound in chess games: research for milestone 13

Research notes for milestone 13 (Sound). The open questions are in "Sound: the idea" in [the roadmap](../plans/2026-10-03-00-roadmap.md). Gathered 2026-10-07.

**How this was checked.** Claims come from primary sources only: source code read at a pinned commit, asset licence files, official help pages and specs, and the developers' own docs. Code links are permalinks with line numbers. Where a primary source could not be found, the note says so. The last section holds **recommendations**. They are opinions, not facts.

Pinned commits used below:

| Repo | Commit |
| --- | --- |
| [lichess-org/lila](https://github.com/lichess-org/lila) | `4a85308` (2026-10-07) |
| [lichess-org/mobile](https://github.com/lichess-org/mobile) | `3447391` |
| [lichess-org/chessground](https://github.com/lichess-org/chessground) | `66e33f0` |
| [gbtami/pychess-variants](https://github.com/gbtami/pychess-variants) | `bf1ab76` |
| [franciscoBSalgueiro/en-croissant](https://github.com/franciscoBSalgueiro/en-croissant) | `23f8314` |
| [lukasmonk/lucaschessR2](https://github.com/lukasmonk/lucaschessR2) | `b3ab767` |
| [nolenroyalty/one-million-chessboards](https://github.com/nolenroyalty/one-million-chessboards) | `219699b` |
| [goldfire/howler.js](https://github.com/goldfire/howler.js) | `1d30535` (v2.2.4) |

## 1. Lichess (web)

### Engine: plain Web Audio, no library

- One module, [`ui/site/src/sound.ts`](https://github.com/lichess-org/lila/blob/4a8530820c00cca1af80c5847fdcb381bc5c4a61/ui/site/src/sound.ts), plays every sound. It uses the Web Audio API directly, with no Howler.
- It creates one `AudioContext` with `latencyHint: 'interactive'` when the browser is idle ([L28-35](https://github.com/lichess-org/lila/blob/4a8530820c00cca1af80c5847fdcb381bc5c4a61/ui/site/src/sound.ts#L28-L35), [L286-292](https://github.com/lichess-org/lila/blob/4a8530820c00cca1af80c5847fdcb381bc5c4a61/ui/site/src/sound.ts#L286-L292)).
- Each sound is fetched, decoded once into an `AudioBuffer`, and cached by path. A play makes a new `AudioBufferSourceNode` through a gain node ([L45-63](https://github.com/lichess-org/lila/blob/4a8530820c00cca1af80c5847fdcb381bc5c4a61/ui/site/src/sound.ts#L45-L63), [L254-284](https://github.com/lichess-org/lila/blob/4a8530820c00cca1af80c5847fdcb381bc5c4a61/ui/site/src/sound.ts#L254-L284)).
- The game page preloads `move`, `capture`, `check`, `checkmate` and `genericNotify` at startup ([`sound.ts` L221-223](https://github.com/lichess-org/lila/blob/4a8530820c00cca1af80c5847fdcb381bc5c4a61/ui/site/src/sound.ts#L221-L223), called from [`round.ts` L45](https://github.com/lichess-org/lila/blob/4a8530820c00cca1af80c5847fdcb381bc5c4a61/ui/round/src/round.ts#L45)). Other sounds load on first use.
- Plays through `move()` are throttled to one per 100 ms ([L85](https://github.com/lichess-org/lila/blob/4a8530820c00cca1af80c5847fdcb381bc5c4a61/ui/site/src/sound.ts#L85)).

### Autoplay and iOS

- **Unlock on first gesture.** A "primer" listens in the capture phase for `touchend`, `pointerup`, `pointerdown`, `mousedown` and `keydown`. The first one calls `ctx.resume()` and removes the listeners ([L22](https://github.com/lichess-org/lila/blob/4a8530820c00cca1af80c5847fdcb381bc5c4a61/ui/site/src/sound.ts#L22), [L27](https://github.com/lichess-org/lila/blob/4a8530820c00cca1af80c5847fdcb381bc5c4a61/ui/site/src/sound.ts#L27), [L38-43](https://github.com/lichess-org/lila/blob/4a8530820c00cca1af80c5847fdcb381bc5c4a61/ui/site/src/sound.ts#L38-L43)).
- **Every play checks the context first** (`resumeWithTest`, [L225-251](https://github.com/lichess-org/lila/blob/4a8530820c00cca1af80c5847fdcb381bc5c4a61/ui/site/src/sound.ts#L225-L251)):
  - A state other than `running` or `suspended` gets a new context, and every cached sound is rewired to it. The code comment: "in addition to 'closed', iOS has 'interrupted'".
  - A suspended context gets `resume()` raced against a 400 ms timeout, "sometimes it never resolves".
  - If it still isn't running, the sound is dropped, and a header warning (`#warn-no-autoplay`) is shown until audio works.
- **iOS theme change.** On iOS, changing the sound set also calls `ctx.resume()` ([L216-219](https://github.com/lichess-org/lila/blob/4a8530820c00cca1af80c5847fdcb381bc5c4a61/ui/site/src/sound.ts#L216-L219)).
- Lila does not set `navigator.audioSession` anywhere in `ui/`, so it gets the iOS default (see section 5).

### Formats and size

- `public/sound` holds every clip as both `.mp3` and `.ogg` ([tree](https://github.com/lichess-org/lila/tree/4a8530820c00cca1af80c5847fdcb381bc5c4a61/public/sound)).
- The web client only ever builds `.mp3` URLs ([`resolvePath` L65-73](https://github.com/lichess-org/lila/blob/4a8530820c00cca1af80c5847fdcb381bc5c4a61/ui/site/src/sound.ts#L65-L73)). No TypeScript in `ui/` references `.ogg`.
- The standard clips are tiny. They are 128 kbps mono MP3 at 44.1 kHz (measured with `file` and `afinfo`):

| Clip | Bytes | Length |
| --- | --- | --- |
| `standard/Move.mp3` | 2,547 | 0.24 s |
| `standard/Capture.mp3` | 3,432 | 0.39 s |
| `standard/LowTime.mp3` | 3,122 | 0.84 s |
| `standard/GenericNotify.mp3` | 8,011 | 0.65 s |

  The whole `standard` folder, both formats, is 348 KB.
- In the standard set, `Check` and `Checkmate` are symlinks to `Silence.mp3`. `Victory`, `Defeat` and `Draw` are symlinks to `GenericNotify` ([tree](https://github.com/lichess-org/lila/tree/4a8530820c00cca1af80c5847fdcb381bc5c4a61/public/sound/standard)). So the default set says nothing for check, and one chime ends every game.

### Events

| Event | Sound name | Where it's played |
| --- | --- | --- |
| Move / capture | `move`, `capture` | Chessground's `move` event ([`round/ctrl.ts` L179-186](https://github.com/lichess-org/lila/blob/4a8530820c00cca1af80c5847fdcb381bc5c4a61/ui/round/src/ctrl.ts#L179-L186)) |
| Atomic capture | `explosion` | same place, atomic variant only |
| Check, mate | `check`, `checkmate` | a move from the server ([L458-462](https://github.com/lichess-org/lila/blob/4a8530820c00cca1af80c5847fdcb381bc5c4a61/ui/round/src/ctrl.ts#L458-L462)) |
| Game end | `victory` / `defeat` / `draw` | [L598-604](https://github.com/lichess-org/lila/blob/4a8530820c00cca1af80c5847fdcb381bc5c4a61/ui/round/src/ctrl.ts#L598-L604); delayed 600 ms after a mate unless the set is `standard` ([`sound.ts` L106-109](https://github.com/lichess-org/lila/blob/4a8530820c00cca1af80c5847fdcb381bc5c4a61/ui/site/src/sound.ts#L106-L109)) |
| Low clock | `lowTime` | [`clockCtrl.ts` L65-71, L173-186](https://github.com/lichess-org/lila/blob/4a8530820c00cca1af80c5847fdcb381bc5c4a61/ui/lib/src/game/clock/clockCtrl.ts#L173-L186) |
| First-move timer under 8 s | `lowTime` | [`expiration.ts` L16-20](https://github.com/lichess-org/lila/blob/4a8530820c00cca1af80c5847fdcb381bc5c4a61/ui/round/src/view/expiration.ts#L16-L20) |
| Berserk (arena) | `berserk` | [L759, L766](https://github.com/lichess-org/lila/blob/4a8530820c00cca1af80c5847fdcb381bc5c4a61/ui/round/src/ctrl.ts#L755-L767) |
| Move confirmed (confirm-move option) | `confirmation` | [L805](https://github.com/lichess-org/lila/blob/4a8530820c00cca1af80c5847fdcb381bc5c4a61/ui/round/src/ctrl.ts#L805) |
| Countdowns (racer, etc.) | `countDown10` … `countDown0`, then `genericNotify` | [`sound.ts` L111-124](https://github.com/lichess-org/lila/blob/4a8530820c00cca1af80c5847fdcb381bc5c4a61/ui/site/src/sound.ts#L111-L124) |
| New message, challenge | `newPM`, `newChallenge` | `playOnce`, which takes a localStorage lock (`just-played`, 2 s) so only one open tab plays it ([L126-136](https://github.com/lichess-org/lila/blob/4a8530820c00cca1af80c5847fdcb381bc5c4a61/ui/site/src/sound.ts#L126-L136)) |

Lichess plays the same move sound for your moves and your opponent's. There is no self/opponent split.

**Low-time rule.**
- The alarm threshold is `min(60 s, initial < 60 s ? max(2 s, 20% of initial) : max(10 s, 12.5% of initial))` ([L102-104](https://github.com/lichess-org/lila/blob/4a8530820c00cca1af80c5847fdcb381bc5c4a61/ui/lib/src/game/clock/clockCtrl.ts#L102-L104)). For a 10+5 game that is 60 s.
- It plays once. It can play again only after the clock climbs back above 1.5× the threshold, and then no sooner than 20 s after the last alarm.
- It sounds only for the player's own clock. It never sounds for spectators or in simuls, and the `clockSound` preference (default on) can turn it off ([`round/ctrl.ts` L668-672](https://github.com/lichess-org/lila/blob/4a8530820c00cca1af80c5847fdcb381bc5c4a61/ui/round/src/ctrl.ts#L668-L672), [`Pref.scala` L488](https://github.com/lichess-org/lila/blob/4a8530820c00cca1af80c5847fdcb381bc5c4a61/modules/pref/src/main/Pref.scala#L488)).

### Replays, reconnects and spectators

- **Reload or reconnect: silent.** It calls `ground.reload`, which is `chessground.set(config)` ([`round/ctrl.ts` L546-571](https://github.com/lichess-org/lila/blob/4a8530820c00cca1af80c5847fdcb381bc5c4a61/ui/round/src/ctrl.ts#L546-L571), [`ground.ts` L112](https://github.com/lichess-org/lila/blob/4a8530820c00cca1af80c5847fdcb381bc5c4a61/ui/round/src/ground.ts#L112)). Setting a position fires no `move` event, so nothing plays. Sounds come only from moves that arrive live (`apiMove`, [L412](https://github.com/lichess-org/lila/blob/4a8530820c00cca1af80c5847fdcb381bc5c4a61/ui/round/src/ctrl.ts#L412)), and only when the player isn't browsing history (`if (!this.replaying())`).
- **Browsing history:** a move sound plays only for a single step forward. A jump plays nothing ([`isForwardStep`, L256 and L277](https://github.com/lichess-org/lila/blob/4a8530820c00cca1af80c5847fdcb381bc5c4a61/ui/round/src/ctrl.ts#L254-L277)).
- **Spectators** hear moves, captures and check like players do. They do not hear victory, defeat or draw (`!d.player.spectator`, [L598](https://github.com/lichess-org/lila/blob/4a8530820c00cca1af80c5847fdcb381bc5c4a61/ui/round/src/ctrl.ts#L598)), and they get no low-time alarm (above). No reduced spectator volume was found in the round code.
- **Haptics:** the opponent's move can also vibrate (`navigator.vibrate(100)`) when that preference is on ([L506](https://github.com/lichess-org/lila/blob/4a8530820c00cca1af80c5847fdcb381bc5c4a61/ui/round/src/ctrl.ts#L506)).

### Sound sets, volume, storage

- **Sets:** the server-side list is Silent, Standard (the default), Piano, NES, SFX, Futuristic, Lisp, WoodLand, Robot, Pentatonic (key `music`) and Speech ([`SoundSet.scala` L11-27](https://github.com/lichess-org/lila/blob/4a8530820c00cca1af80c5847fdcb381bc5c4a61/modules/pref/src/main/SoundSet.scala#L11-L27)).
- **Where the set is kept:**
  - Signed-in users: a server preference, saved by `POST /pref/soundSet` ([`dasher/sound.ts` L156-162](https://github.com/lichess-org/lila/blob/4a8530820c00cca1af80c5847fdcb381bc5c4a61/ui/dasher/src/sound.ts#L156-L162)).
  - Anonymous users: the session ([`RequestPref.scala` L25](https://github.com/lichess-org/lila/blob/4a8530820c00cca1af80c5847fdcb381bc5c4a61/modules/pref/src/main/RequestPref.scala#L25)).
- **Volume:** a 0–1 slider stored in localStorage `sound-volume`, default 0.7 ([`sound.ts` L20, L138-144](https://github.com/lichess-org/lila/blob/4a8530820c00cca1af80c5847fdcb381bc5c4a61/ui/site/src/sound.ts#L138-L144)).
  - Moving the slider plays a move sound as a preview ([`dasher/sound.ts` L184-188](https://github.com/lichess-org/lila/blob/4a8530820c00cca1af80c5847fdcb381bc5c4a61/ui/dasher/src/sound.ts#L184-L188)), and choosing a set plays `genericNotify`.
  - Sound counts as off when the set is `silent` or the volume is 0 ([L175](https://github.com/lichess-org/lila/blob/4a8530820c00cca1af80c5847fdcb381bc5c4a61/ui/site/src/sound.ts#L175)). There is no separate mute toggle.
- **Speech:** the browser's `speechSynthesis` reads moves aloud, with a chosen voice and rate kept in localStorage ([L177-212](https://github.com/lichess-org/lila/blob/4a8530820c00cca1af80c5847fdcb381bc5c4a61/ui/site/src/sound.ts#L177-L212)). In Speech and Pentatonic modes, move, capture, check and checkmate don't play the sample sounds. Every other event falls back to the standard set ([L65-73](https://github.com/lichess-org/lila/blob/4a8530820c00cca1af80c5847fdcb381bc5c4a61/ui/site/src/sound.ts#L65-L73)).

### Pentatonic: notes by square

[`bits.soundMove.ts`](https://github.com/lichess-org/lila/blob/4a8530820c00cca1af80c5847fdcb381bc5c4a61/ui/bits/src/bits.soundMove.ts) is the one shipped "a game makes a tune" design, which is our "notes on the board" idea:

- **Pitch:** the destination square's index (`file*8 + rank`) is scaled onto 24 pitches ([L40-42](https://github.com/lichess-org/lila/blob/4a8530820c00cca1af80c5847fdcb381bc5c4a61/ui/bits/src/bits.soundMove.ts#L40-L42)).
- **Instruments:** pawns and kings play a clavinet. Other pieces play a celesta.
- **Accents:** a capture, check, castle or mate adds one of 3 "swells" ([L54-63](https://github.com/lichess-org/lila/blob/4a8530820c00cca1af80c5847fdcb381bc5c4a61/ui/bits/src/bits.soundMove.ts#L54-L63)).
- **Mix:** each instrument has a fixed volume (celesta 0.3, clav 0.2, swells 0.8), and at most 15 notes overlap ([L1-25](https://github.com/lichess-org/lila/blob/4a8530820c00cca1af80c5847fdcb381bc5c4a61/ui/bits/src/bits.soundMove.ts#L1-L25)).
- **Assets:** 51 note files, loaded lazily the first time the set is used.
- **Status:** it is an opt-in set. The default stays `standard`.

### Licensing (from [`COPYING.md`](https://github.com/lichess-org/lila/blob/4a8530820c00cca1af80c5847fdcb381bc5c4a61/COPYING.md))

| Sound set | Author | Licence | Source |
| --- | --- | --- | --- |
| futuristic, nes, piano, sfx | Enigmahack | **AGPLv3+** | [L71-74](https://github.com/lichess-org/lila/blob/4a8530820c00cca1af80c5847fdcb381bc5c4a61/COPYING.md#L71-L74) |
| lisp | EdinburghCollective | **CC BY-NC-SA 4.0** | [L75](https://github.com/lichess-org/lila/blob/4a8530820c00cca1af80c5847fdcb381bc5c4a61/COPYING.md#L75) |
| everything else in `public/sound` (**standard**, robot, woodland, instrument/Pentatonic, other) | not stated | listed under "Exceptions (non-free)" as "The other sounds in public/sound" | [L84-101](https://github.com/lichess-org/lila/blob/4a8530820c00cca1af80c5847fdcb381bc5c4a61/COPYING.md#L84-L101) |

Notes:
- COPYING writes the free paths as `public/sounds/…`, but the folder is `public/sound`. That is a typo in the file. The authors and licences are still clear.
- **Lichess's default sounds are not under a free licence.** Only the five sets above are licensed for reuse.

## 2. Lichess mobile (Flutter)

- **Engine:** [`sound_service.dart`](https://github.com/lichess-org/mobile/blob/3447391665bae6bc17501cb90be290780a2111ee/lib/src/model/common/service/sound_service.dart) plays through the `sound_effect` plugin, with at most 2 sounds at once ([L14-15](https://github.com/lichess-org/mobile/blob/3447391665bae6bc17501cb90be290780a2111ee/lib/src/model/common/service/sound_service.dart#L14-L15)).
- **Events:** `move`, `capture`, `explosion`, `lowTime`, `dong`, `error`, `confirmation`, `puzzleStormEnd`, `clock`, `berserk`, plus some lesson sounds ([L22-41](https://github.com/lichess-org/mobile/blob/3447391665bae6bc17501cb90be290780a2111ee/lib/src/model/common/service/sound_service.dart#L22-L41)). There is no check sound.
- **Formats:** iOS uses `.aifc` files and other platforms use `.mp3` ([L50](https://github.com/lichess-org/mobile/blob/3447391665bae6bc17501cb90be290780a2111ee/lib/src/model/common/service/sound_service.dart#L50)). A sound missing from a theme falls back to standard ([L128-145](https://github.com/lichess-org/mobile/blob/3447391665bae6bc17501cb90be290780a2111ee/lib/src/model/common/service/sound_service.dart#L128-L145)).
- **Settings:** sound is on by default, the theme is Standard, and the master volume is 0.8 ([`general_preferences.dart` L85-108](https://github.com/lichess-org/mobile/blob/3447391665bae6bc17501cb90be290780a2111ee/lib/src/model/settings/general_preferences.dart#L85-L108)). The themes are Standard, Piano, NES, SFX, Futuristic and Lisp ([L148-155](https://github.com/lichess-org/mobile/blob/3447391665bae6bc17501cb90be290780a2111ee/lib/src/model/settings/general_preferences.dart#L148-L155)). That is the five freely licensed web sets plus Standard.
- **Timing:** a move sound plays **half-way through the piece animation**, or at once after a drag ([`game_controller.dart` L610-623](https://github.com/lichess-org/mobile/blob/3447391665bae6bc17501cb90be290780a2111ee/lib/src/model/game/game_controller.dart#L610-L623)).
- **Haptics:** a light impact for a move and a medium one for check, when haptics are on ([`move_feedback.dart` L14-47](https://github.com/lichess-org/mobile/blob/3447391665bae6bc17501cb90be290780a2111ee/lib/src/model/common/service/move_feedback.dart#L14-L47)).
- **Game start and end:**
  - "dong" plays at the start only if it is your game and no move has been played ([L173-178](https://github.com/lichess-org/mobile/blob/3447391665bae6bc17501cb90be290780a2111ee/lib/src/model/game/game_controller.dart#L173-L178)).
  - It plays again 500 ms after the end ([L888-894](https://github.com/lichess-org/mobile/blob/3447391665bae6bc17501cb90be290780a2111ee/lib/src/model/game/game_controller.dart#L888-L894)).
- **Live moves vs replays:**
  - A live move plays only while the player isn't browsing history ([L786-790](https://github.com/lichess-org/mobile/blob/3447391665bae6bc17501cb90be290780a2111ee/lib/src/model/game/game_controller.dart#L786-L790)).
  - Stepping through history plays a plain move or capture sound for each step ([L360-405, L636-643](https://github.com/lichess-org/mobile/blob/3447391665bae6bc17501cb90be290780a2111ee/lib/src/model/game/game_controller.dart#L636-L643)).
- **Low time:** plays only for your own side and only if the account's clock-sound preference is on ([L459-466](https://github.com/lichess-org/mobile/blob/3447391665bae6bc17501cb90be290780a2111ee/lib/src/model/game/game_controller.dart#L459-L466)).
- **Licence conflict:** the app's [`COPYING.md`](https://github.com/lichess-org/mobile/blob/3447391665bae6bc17501cb90be290780a2111ee/COPYING.md#L24-L31) lists no exception for `assets/sounds`, so read literally it puts them under GPLv3+. That conflicts with lila's COPYING, which calls the standard set non-free. Treat lila's statement as the owner's. **Unresolved**: no primary source settles which is right.

## 3. Chess.com

**Settings (help center):**
- Web: Settings → Board & Pieces → "Sound" section. It has an on/off toggle and a sound-theme dropdown with a speaker icon to preview, then "Save Sound Theme" ([How do I turn off sound?](https://support.chess.com/en/articles/8704531-how-do-i-turn-off-sound)).
- Mobile apps: More → Settings → Board → "Sounds" toggle.
- Live Chess settings list "Play Sounds", "Sound Theme" ("the sound you want to hear when moves are made") and "Low-Time Warning" ("a warning sound when you're running low on time") ([Live Chess settings](https://support.chess.com/en/articles/8614003-how-do-i-manage-my-live-chess-settings)).
- The help pages mention no volume control and do not say whether sound is on by default. **Not confirmed.**

**Sound files.** The files are served at `https://images.chesscomfiles.com/chess-themes/sounds/_MP3_/<theme>/<event>.mp3`.
- The CDN refuses non-browser user agents. With a browser user agent, a missing name returns `403` with an XML body, so a `200 audio/mp3` reply means the file exists.
- These names answered in the `default` theme on 2026-10-07:

| Group | File names |
| --- | --- |
| Moves | `move-self`, `move-opponent`, `capture`, `castle`, `move-check`, `promote`, `premove`, `illegal` |
| Game | `game-start`, `game-end`, `game-win`, `game-lose`, `game-draw`, `game-win-long`, `game-lose-long` |
| Clock | `tenseconds` |
| UI and other | `notify`, `notification`, `click`, `decline`, `achievement`, `correct`, `incorrect`, `puzzle-correct`, `puzzle-wrong`, `event-start`, `event-end`, `event-warning`, `scatter`, `boom`, `shoutout` |

- **What the names show:** unlike Lichess, Chess.com separates your move from your opponent's. `move-self` and `move-opponent` are different files (different MD5) in both `default` and `nature`. It also has its own sounds for check, castle, promotion, premove and illegal moves, and a ten-second clock warning.
- **What they don't show:** which event plays which file. That is inferred from the names only.
- **Size:** clips are small, about 2–11 KB each (`move-self.mp3` is 3,433 bytes). `boom` is the outlier at 57 KB.
- **Formats:** the same clip is also served as `_OGG_/…ogg`, `_WEBM_/…webm` and `_WAV_/…wav`.
- **Themes:** `nature`, `marble`, `beat`, `lolz`, `newspaper`, `silly`, `space` and `metal` answered when probed. Guessed names like `wood`, `robot` and `8bit` did not. **The full official theme list was not confirmed.** The help pages don't list it.
- **Licence:** none published. The sounds are Chess.com's own, so they are not reusable.

## 4. Other open-source chess software

| Project | Sound system | What stands out |
| --- | --- | --- |
| **Chessground** (Lichess's board) | **None.** No "sound" or "audio" anywhere in `src/`. | It only emits `events.move(orig, dest, capturedPiece)` ([`config.ts` L76-80](https://github.com/lichess-org/chessground/blob/66e33f03082790ccbb284bcb462ddcbcbabf35fe/src/config.ts#L76-L80), [`board.ts` L116](https://github.com/lichess-org/chessground/blob/66e33f03082790ccbb284bcb462ddcbcbabf35fe/src/board.ts#L116)). The app plays the sound. |
| **PyChess** (pychess.org, variants) | Howler.js. Each clip is listed as `.ogg` then `.mp3` ([`client/sound.ts` L104-119](https://github.com/gbtami/pychess-variants/blob/bf1ab76b9817a1b54358c6bcdd0f58d24109106a/client/sound.ts#L104-L119)). | See the list below this table. |
| **En Croissant** (desktop, Tauri) | A pool of 5 `<audio>` elements ([`src/utils/sound.ts` L8-9](https://github.com/franciscoBSalgueiro/en-croissant/blob/23f83142fcbd4d3af60a524855defe8c9a5703fe/src/utils/sound.ts#L8-L9)). | At most one sound every 75 ms ([L37-43](https://github.com/franciscoBSalgueiro/en-croissant/blob/23f83142fcbd4d3af60a524855defe8c9a5703fe/src/utils/sound.ts#L37-L43)). Ships Lichess sets as Move, Capture and Check only ([`sound/`](https://github.com/franciscoBSalgueiro/en-croissant/tree/23f83142fcbd4d3af60a524855defe8c9a5703fe/sound)). |
| **Lucas Chess** (desktop) | Its own WAV store. | Each event has its own recordable sound: beep after move, error, time trouble, win and loss, each draw rule, draw offer. Moves can be **spoken from recorded clips**: one clip per coordinate a–h and 1–8, each piece, castling, capture, check, mate ([`Sound.py` L151-186](https://github.com/lukasmonk/lucaschessR2/blob/b3ab7678077d6433e45d85f55acac2102e13e82e/bin/Code/Sound/Sound.py#L151-L186)). The app can record them from the microphone ([L225-246](https://github.com/lukasmonk/lucaschessR2/blob/b3ab7678077d6433e45d85f55acac2102e13e82e/bin/Code/Sound/Sound.py#L225-L246)). |
| **Scid vs PC** (desktop) | Tcl Snack. | "Can speak moves in English, or play a tock sound with every move" ([Sound help](https://scidvspc.sourceforge.net/doc/Sound.htm)). |
| **One Million Chessboards** (our feel reference) | **None.** No audio files, `AudioContext`, `new Audio` or Howler in the repo. | |

What stands out in **PyChess**:
- **Sound by variant.** Shogi-family variants use wooden `ShogiMove` and `ShogiCapture` clips, and atomic uses `Explosion` ([L219-236](https://github.com/gbtami/pychess-variants/blob/bf1ab76b9817a1b54358c6bcdd0f58d24109106a/client/sound.ts#L219-L236)).
- **Byoyomi tick.** The last 10 s of byoyomi play a `Tick` every second, and `LowTime` plays at 10 s ([`clock.ts` L14, L101-122](https://github.com/gbtami/pychess-variants/blob/bf1ab76b9817a1b54358c6bcdd0f58d24109106a/client/clock.ts#L101-L122)).
- **Bughouse voice lines.** Partner messages are spoken clips made with Piper text-to-speech ([`bugchat.sh`](https://github.com/gbtami/pychess-variants/blob/bf1ab76b9817a1b54358c6bcdd0f58d24109106a/static/sound/bugchat/bugchat.sh)).
- **Unlock.** A blocked play waits for Howler's `unlock` event, then plays.
- **Settings.** Volume defaults to 1 and the theme to `standard` ([L271-301](https://github.com/gbtami/pychess-variants/blob/bf1ab76b9817a1b54358c6bcdd0f58d24109106a/client/sound.ts#L271-L301)).
- **Replays and spectators.**
  - Move sounds play only for live updates, not a full board load (`msg.steps.length <= 2`).
  - Check and game-end sounds are skipped for spectators ([`roundCtrl.ts` L1128-1129, L1323-1329](https://github.com/gbtami/pychess-variants/blob/bf1ab76b9817a1b54358c6bcdd0f58d24109106a/client/roundCtrl.ts#L1323-L1329)).
- **Assets.** It reuses Lichess's sets, and adds clips credited to freesound.org and soundbible in [`static/sound/standard/license.txt`](https://github.com/gbtami/pychess-variants/blob/bf1ab76b9817a1b54358c6bcdd0f58d24109106a/static/sound/standard/license.txt).

## 5. Background music and ambient sound

- **Lichess:** no background music or ambient loop. Every set is per-event clips, and "Pentatonic" plays notes per move, not a bed (section 1).
- **Chess.com:** the help pages describe only move and event sounds. **No primary source found** either way on background music.
- **One Million Chessboards:** silent.
- **Commercial chess games** (e.g. Chess Ultra): only third-party reviews mention a soundtrack. The developer's [Steam page](https://store.steampowered.com/app/518060/Chess_Ultra/) doesn't. **Not confirmed from a primary source.**
- **So:** no primary source shows a notable web chess site with an ambient soundtrack. A NYC street bed would be unusual for the genre.

How to play a long loop:
- [Howler's docs](https://github.com/goldfire/howler.js/blob/1d3053576a860e9854645493ad6c4a72c6cc6e45/README.md#L215-L216): `html5: true` "should be used for large audio files so that you don't have to wait for the full file to be downloaded and decoded before playing."
- Web Audio, by contrast, decodes the whole file into memory first (Lichess's `load`, above).

## 6. Web audio facts that matter on phones

- **Chrome:** sound may autoplay only after the user has interacted with the site, or after a Media Engagement Index score (desktop), or once the site is installed. An `AudioContext` created before a gesture "will be created in the 'suspended' state, and you will need to call `resume()` after the user gesture" ([Chrome autoplay policy](https://developer.chrome.com/blog/autoplay)).
- **MDN:**
  - Starting a Web Audio source "outside the context of handling a user input event is subject to autoplay rules" ([MDN autoplay guide](https://developer.mozilla.org/en-US/docs/Web/Media/Guides/Autoplay)).
  - `navigator.getAutoplayPolicy("audiocontext")` reports `allowed`, `allowed-muted` or `disallowed`. That page doesn't say which browsers support it. Check it before relying on it.
- **Safari:** "Websites should assume any use of `<video>` or `<audio>` requires a user gesture click to play" ([WebKit, 2017](https://webkit.org/blog/7734/auto-play-policy-changes-for-macos/)).
- **iOS silent switch:**
  - By default Web Audio is muted when the ringer switch is off. A WebKit engineer: "By default the type is `ambient` and so audio will be muted if the phone is muted. … Since iOS 17, you can set the audio session type to 'playback'" with `navigator.audioSession.type = 'playback'` ([WebKit bug 237322](https://bugs.webkit.org/show_bug.cgi?id=237322)).
  - `<audio>` elements default to `playback`, so they ignore the switch ([MDN Audio Session API](https://developer.mozilla.org/en-US/docs/Web/API/Audio_Session_API)).
  - The [Audio Session spec](https://w3c.github.io/audio-session/) is an Editor's Draft (13 Nov 2024) with types `auto`, `playback`, `transient`, `transient-solo`, `ambient` and `play-and-record`. MDN marks it experimental and not Baseline.
- **Background tabs on iOS:** WebKit stops the `AudioContext` when the page goes to the background ([bug 237878](https://bugs.webkit.org/show_bug.cgi?id=237878)). Lichess handles the extra `interrupted` state by rebuilding its context (section 1).
- **Formats:**
  - Safari 15 added "support for the Opus audio codec in WebM containers" ([WebKit, Safari 15](https://webkit.org/blog/11989/new-webkit-features-in-safari-15/)).
  - MDN's codec guide still says Safari plays Opus only in CAF ([MDN audio codecs](https://developer.mozilla.org/en-US/docs/Web/Media/Guides/Formats/Audio_codecs)). The two primary sources disagree, so test on a real iPhone before choosing Opus.
  - MP3 has no such caveat.
  - Howler's docs advise "default to `webm` and fallback to `mp3`" and list the order of sources as significant ([README L550-553](https://github.com/goldfire/howler.js/blob/1d3053576a860e9854645493ad6c4a72c6cc6e45/README.md#L550-L553)).
- **Howler unlock:** Howler unlocks by "playing an empty buffer on the first `touchend` event" (`autoUnlock`, default true). A sound that fails can wait for the `unlock` event ([README L381-382, L514-530](https://github.com/goldfire/howler.js/blob/1d3053576a860e9854645493ad6c4a72c6cc6e45/README.md#L514-L530)). Howler's latest release is v2.2.4, from 2023-09-19.

## 7. What this means for Mamdani Chess (recommendations, not facts)

Each item answers an open question from the roadmap's "What it must get right".

1. **Engine.** Use plain Web Audio, as Lichess does. Build one `AudioContext` and decode every effect into an `AudioBuffer` once. Each play gets a fresh source node through a gain node per channel: effects, soundtrack, master.
   - Copy Lichess's `resumeWithTest` checks: rebuild the context when its state isn't `running` or `suspended`, and give `resume()` a timeout.
   - Skip Howler. We need about 20 short clips and one loop, and Howler's last release was 2023.

2. **Off until asked, or on?**
   - **Effects:** on by default, as at Lichess (web and mobile). They can't sound before the first tap anyway, so unlock on the first `pointerdown` or `keydown` (Lichess's primer list).
   - **Soundtrack:** off until the player turns it on. No primary-source chess site ships one, and phones in public are the norm.
   - **The switch is the consent.** Turning the soundtrack on is itself a tap, so it can start right away.

3. **Mute controls and storage.**
   - **Where:** one speaker button on the game page and the phone layout. A long-press or small menu opens two switches (effects, soundtrack) and one volume slider.
   - **Storage:** localStorage, since we only have guests. Lichess keeps its volume in localStorage too.
   - **Keys:** something like `sound.effects`, `sound.music`, `sound.volume`. Default volume about 0.7, Lichess's value.
   - **Feedback:** play a sample when a switch or the slider changes, as Lichess does.
   - **When audio is blocked:** show a small hint when the context won't resume, like Lichess's `#warn-no-autoplay`. On iOS, point out the silent switch.

4. **iOS silent switch: respect it.**
   - Leave `navigator.audioSession` at its default, so Web Audio stays `ambient`. Effects then go quiet with the ringer off, which suits a game played in public.
   - Play the soundtrack through Web Audio as well, not an `<audio>` element. An `<audio>` element defaults to `playback`, ignores the switch, and becomes "media" on the lock screen.
   - Accept that a hidden tab stops the sound. Resume on `visibilitychange`, through the same checks as above.

5. **Event list.**
   - **Keep the set small and distinct.** Lichess's default set has no check sound and one chime for every ending. Chess.com is the richer model: separate self/opponent moves, check, castle, promote, illegal and ten seconds.
   - **Adapt it to our board:**
     - land (self), land (opponent), capture;
     - dice thrown, die stops;
     - pothole opens, cone tick, hole closes;
     - fall, save, repair;
     - check, mate;
     - low clock, "+5";
     - win, loss.
   - **Self vs opponent:** a quieter or different landing for the opponent's piece helps on a phone left on the table. That's why Chess.com splits them.
   - **Low clock:** play it once, only for your own clock, never for spectators. Lichess and PyChess both do this. Our idea is 10 s, as at PyChess and Chess.com's `tenseconds`. Lichess's formula would give 60 s for 10+5; 10 s fits a 10+5 game with dice pauses better.
   - **Timing:** start a landing sound when the piece lands, not when it is picked up. Lichess mobile plays the move sound half-way through the animation. Dice and pothole sounds follow `dice-timing.json`, as planned.
   - **Mate then result:** delay the win/loss sound about 600 ms after the mate sound, as Lichess does, so they don't overlap.
   - **Throttle:** at most one sound per kind per 75–100 ms (Lichess 100 ms, En Croissant 75 ms), so bursts don't stack.

6. **Notes on the board.** Lichess built this (Pentatonic: 24 pitches from the destination square, two instruments, a swell on captures) and kept it opt-in. Don't make it the default. If we try it, make it a later toggle once the core set exists.

7. **Sound sets.** Ship one set ("street") and no theme picker for now. Lichess and Chess.com offer many sets, but that is a lot of asset work for a friends-only game.

8. **Replays and reconnects.** Follow Lichess, PyChess and Lichess mobile: **sound only for events that arrive live after the page has settled. Never for a state shown at once** (load, reload, reconnect, catching up a hidden tab, opening the moves sheet).
   - That is the same rule the "Known, left for later" re-throw bug needs. Fixing the replays first (or a shared "shown at once" flag that both animation and sound read) solves both.
   - Instant mode plays nothing, as the roadmap says.

9. **Spectators.** Give spectators the same move, dice and pothole sounds. Skip the win/loss sting and the low-clock alarm, which belong to the players. Lichess and PyChess draw the line in the same place.

10. **Formats and size.**
    - **Effects:** a single MP3 per clip, mono, short, around 2–10 KB each like Lichess's and Chess.com's. The whole effect set should stay well under 200 KB (an estimate). Preload only the core clips (land, capture, dice, pothole) when the game page opens, as Lichess preloads five.
    - **Opus-in-WebM:** smaller, but the sources disagree on Safari support (section 6). Use it only after testing on a real iPhone.
    - **Soundtrack:** a mono loop of 30–60 s at a low bitrate, roughly 250–500 KB (an estimate). Fetch it only when switched on.

11. **Licensing.**
    - **Don't reuse Lichess's sounds:**
      - The default **standard** set (plus robot, woodland, Pentatonic and "other") is listed as **non-free** in lila's COPYING.
      - **futuristic, nes, piano, sfx** are **AGPLv3+** (Enigmahack). Using them would bring AGPL terms into a repo that has no LICENSE file.
      - **lisp** is **CC BY-NC-SA 4.0** (EdinburghCollective): non-commercial and share-alike.
      - None of them fit the road-works look anyway.
    - **Don't use Chess.com's files.** They are proprietary.
    - **Use instead:** clips we record ourselves (street sounds suit field recording) or CC0 sources:
      - [Kenney's assets](https://kenney.nl/support) are CC0: "You're free to use them, even in commercial projects".
      - [Freesound](https://freesound.org/help/faq/) mixes CC0, CC BY and CC BY-NC per sound. Filter to CC0 (or CC BY with credit) and avoid BY-NC.
    - **Credits:** list every clip's source and licence on the Rules page, as the roadmap plans.

12. **Reduced motion.** No primary source ties sound to `prefers-reduced-motion`. Lichess keeps them separate. Keep sound on its own switches, and don't turn it off when motion is reduced.

13. **Haptics (optional).** Lichess uses a short vibration for the opponent's move (web, behind a preference) and light/medium impacts (mobile). The web Vibration API was not checked on iOS Safari here. Verify before planning on it.
