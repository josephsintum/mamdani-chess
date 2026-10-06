# Mamdani Chess

Live doc: https://claude.ai/artifact/AvSPCQS42ggQGpTWQGoQRB (source of truth; this file is a copy)

This is Pot-Hole Chess (Spicer and Chamberlain, 2001) with longer-lasting potholes and one addition. Potholes open at random and swallow pieces, as in the original, but each stays open for three rounds. The addition is the Mamdani: a neutral piece either player can move, which blocks lines, repairs potholes and saves pieces from falling. The rules are locked; see the decisions below.

## How it should feel

Chess comes first; the dice bring chaos, and the Mamdani is how you fight back. Every rule below is judged against these goals.

1. **The Mamdani is your answer to the dice.** The dice roll after every move and you can't control them. Where you put the Mamdani decides which pieces they can hurt.
2. **Luck finishes, skill sets up.** A roll can deliver mate, but kings never fall, so it only finishes a king that play has already cornered.
3. **Most bad rolls have an answer.** Block, capture, or send the Mamdani.
4. **Readable at a glance.** Players and spectators can see what's open, what's blocked and why. The portal game showed that hidden or moving hazards feel unfair.
5. **Fun to watch.** Every roll is shown big, so a pothole opening or a piece being saved is an event everyone reacts to.
6. **Simple to explain.** A new player learns the additions in one minute.

## Decisions

All sixteen are agreed. Rows 4, 10, 11 and 16 changed on 2026-10-05 (milestone 06c): potholes closed too fast in sandbox play, and a simulation of 2,000 games per variant found about 2.5 holes open on average, the cap closing about one hole a game, and mates by roll in about 1 game in 1,000.

| # | Decision | Rule |
| --- | --- | --- |
| 1 | When potholes happen | Automatic, after your move (original). Move first, then roll a d8. Even: a new pothole opens (#10). Odd: nothing. The dice never cost you a move. |
| 2 | How many rolls | Unlimited (original). One roll after every move, for both players. |
| 3 | Where it lands | Fully random (original). Two d8s pick the file and rank. |
| 4 | How long it lasts | Three rounds. A pothole closes when the player who rolled it finishes their third move after opening it. The Mamdani can repair it sooner. (Was one round, as in the original: holes closed before they mattered.) |
| 5 | How the Mamdani repairs | Next to it. Any pothole on the 8 squares around the Mamdani is fixed at once. Saving rolls stay as in #7. |
| 6 | Pothole lands on a king | Re-roll. Kings never fall, as in the original Pot-Hole Chess. Re-roll both d8s. |
| 7 | Saving rolls | Keep. It's the Mamdani's defensive job and the answer to a bad roll (goal 3). |
| 8 | Sliders and potholes | Can't cross, with blocked lines shown on the board (goal 4). Knights can jump over a pothole but can't land on it. |
| 9 | Stalling with the Mamdani | No rule. Either player may move it anywhere it can reach, including straight back. Threefold repetition ends any back-and-forth as a draw. |
| 10 | Number of potholes | New pothole on every even roll (original), at most 5 open. When a pothole opens with 5 already open, the oldest closes first. Potholes never move. |
| 11 | Pothole would checkmate | It stands: a roll can deliver checkmate. Kings still never fall (#6), so a roll can trap a king but never take it. (Was a re-roll; changed by choice, and it decides about 1 game in 1,000.) |
| 12 | Saving-roll reach | Clear queen line. Nothing between the Mamdani and the square; nothing else is checked. |
| 13 | Endless re-rolls | Cap at 64. After 64 re-rolls without a valid square, no pothole opens that turn. |
| 14 | Pawns, castling, en passant | Blocks like a piece. No castling through or onto a pothole (rook's path included), no pawn double-step across one, and a pothole on the en passant square cancels that capture. |
| 15 | Roll after a checkmating move | No roll. A move that checkmates ends the game at once, so the dice can't undo a mate made on the board (goal 2). |
| 16 | Check held off only by your own pothole | Checkmate, if that pothole is on its last round: your next move closes it, so if the king would then be in check and no move fixes it, it is checkmate. |

## Setup

Set up a normal chessboard, then put the Mamdani on a5. That square is empty at the start of a standard game.

| Item | Use |
| --- | --- |
| Chess set | Standard starting position |
| Mamdani token | Any distinct piece or coin, placed on a5 |
| 3 eight-sided dice (d8) | All rolls: the pothole roll, the pothole's file (1 = a … 8 = h) and rank, and saving rolls |
| Up to 5 checkers, and 3 small markers each (cones or coins) | Mark open potholes and the rounds each has left |

White moves first, as usual.

## Turn order

Every turn is one move followed by one pothole roll. Play these steps in order:

1. **Move.** Make one legal move: either one of your own pieces or the Mamdani.
2. **Count down.** Each pothole you rolled loses a round (take a marker off it). One with no rounds left closes now.
3. **Repair.** Any open pothole next to the Mamdani (any of its 8 surrounding squares) is repaired and removed.
4. **Roll for a pothole.** Roll a d8. Odd: nothing happens and the turn ends. Even: a pothole opens.
5. **Place it.** Roll two d8s for file and rank. Resolve that square as described in Potholes and Saving rolls. If it opens with 5 already open, the oldest closes first. A new pothole has 3 rounds.

## Potholes

A pothole destroys whatever stands on its square, unless a saving roll succeeds. It then blocks that square for three rounds: it closes when the player who rolled it finishes their third move after opening it. Every even roll opens a new pothole; potholes never move. Up to 5 can be open at once, each closing on its own schedule; when a pothole opens with 5 already open, the oldest closes first.

When the d8s land on a square:

- **King:** kings never fall. Re-roll both d8s.
- **Next to the Mamdani:** the pothole is repaired the moment it opens. Nothing falls.
- **Already a pothole:** re-roll both d8s.
- **Would expose the roller's king:** if the piece falling would leave the player who just moved in check once the hole closes, or if the cap closing the oldest pothole would leave them in check, re-roll both d8s.
- **Too many re-rolls:** after 64 re-rolls without a valid square, no pothole opens this turn.
- **Empty square:** the pothole opens. Mark it with a checker and 3 markers for its rounds.
- **Any other piece:** the piece falls in and is lost, unless it is saved (see Saving rolls).
- **The Mamdani:** the Mamdani must make a saving roll (see Saving rolls).

While a pothole is open:

- No piece, and not the Mamdani, may move onto it.
- Bishops, rooks, queens and the Mamdani cannot slide across it.
- Knights may jump over it but cannot land on it.
- Because sliders can't cross it, a pothole blocks attacks and checks along that line, like a piece would.
- Your move is illegal if your king would be in check after your potholes on their last round close and nearby potholes are repaired.

## The Mamdani

The Mamdani belongs to neither player. It moves like a queen, never captures and fixes potholes around it.

- **Moving it:** on your turn you may move the Mamdani instead of one of your pieces. That uses your whole move.
- **Movement:** any distance in a straight line or diagonal, like a queen. It cannot jump over pieces or potholes, and it can only land on an empty square.
- **Captures:** it never captures, and nobody can capture it.
- **Blocking:** it blocks lines like any piece. It can block a check, and it can block your opponent's escape squares.
- **Your own king:** you may not move the Mamdani so that it leaves your own king in check.
- **Repairs:** any pothole on one of the 8 squares next to the Mamdani is repaired at once. The repair happens after every move, and when a new pothole opens there.
- **Falling in:** the Mamdani can fall into a pothole like any other piece (see Saving rolls). Once it falls, it is gone for the rest of the game.

## Saving rolls

A saving roll is one d8. An odd number saves the piece, and the pothole never opens.

| Who is on the square | Gets a saving roll when | Odd (1, 3, 5, 7) | Even (2, 4, 6, 8) |
| --- | --- | --- | --- |
| A player's piece | The Mamdani has a clear queen line to that square (nothing between them; nothing else is checked) | Piece saved, no pothole | Piece lost, pothole opens |
| The Mamdani | Always | Mamdani saved, no pothole | Mamdani removed for the rest of the game, pothole opens |

- The owner of the piece rolls. For the Mamdani, the player who rolled the pothole rolls.
- A piece with no clear Mamdani line gets no saving roll and is lost.
- If the Mamdani has already fallen, no more saving rolls are possible.

## Winning and draws

You win by checkmate, as in normal chess. Kings never fall into potholes, but a roll can deliver checkmate: a hole on a king's last escape square, or a blocking piece falling in.

- **Checkmate:** check where no move of your pieces and no Mamdani move gets you out of check. A move that checkmates ends the game at once; no pothole roll follows. If your own pothole on its last round is the only thing blocking a check and no move fixes it, that is also checkmate, because your next move closes that pothole.
- **Stalemate:** a draw, but only if you also have no legal Mamdani move.
- **50-move rule:** moving the Mamdani does not reset the count. Losing a piece to a pothole does reset it; the Mamdani falling does not.
- **Repetition:** a position counts as repeated only if the pieces, the Mamdani, the open potholes and the rounds each has left are all the same.
- Castling, en passant and promotion work as normal. A pothole blocks like a piece: you cannot castle through or onto one (including the rook's path over b1 or b8), a pawn cannot double-step across one, and a pothole on the en passant square cancels that capture.

## Watch in test games

The rules are locked. These are the things to watch when the group plays, in case a house rule is needed later.

- [ ] **Stalling:** do players shuffle the Mamdani to waste time? If so, ban moving it two turns in a row.
- [ ] **Chaos:** an even d8 opens a pothole about every other move, as in the original. If it feels too swingy, the house rules can make it rarer.
- [ ] **Readability:** can everyone tell at a glance which lines a pothole blocks, and how long each has left?
- [ ] **Petering out:** about 9 pieces fall a game. Do games end with too little material to mate?

## Sources

- [Pot-Hole Chess](https://www.chessvariants.com/boardrules.dir/potholechess.html) by Peter Spicer and Michael Chamberlain, The Chess Variant Pages (2001)
- [Mamdani patch video](https://www.instagram.com/reels/Dd8tV_MxEQL/) (Instagram reel)
