# Glossary

Words the code and docs use for the game's moving parts. Add a term when a module is named after it.

**Turn**: one move and everything its dice decide, as a view's `last` events. `seq` counts turns played; a resignation or a timeout changes the view without a new turn.

**Played out / shown at once**: a turn that arrives live, the next `seq` in a visible tab, plays out one dice step at a time (the Animator). Anything else (a load, a reconnect that skipped turns, a hidden tab, the rest of a roll skipped on coming back) is shown at once.

**One-off effect**: something drawn once for a moment of a turn, not for the position: a die's throw, the tray's lines fading in, the scan, the corner pill, a save's hop, a repair's 👍, a bubble, the mate burst. A pothole, its cones and the target are the position, not effects.

**Age**: how far in a one-off effect is, in ms since its event was revealed (`Animator.age`), or since the game ended (`age('end')`). An effect drawn late starts that far in; one shown at once has no age (Infinity) and shows how it ends.
