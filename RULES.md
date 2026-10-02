# Pothole Chess: Mamdani Edition

Oct 1, 2026

Live doc: https://claude.ai/code/artifact/50584f76-084a-4ea2-ae02-8534639e9b76

This is ordinary chess with two additions. Potholes open at random and swallow pieces. A neutral piece called the Mamdani, which either player can move, blocks lines and repairs potholes. Rules marked *(proposed)* fill gaps in the original patch and are open for the group to change.

## Setup

Set up a normal chessboard, then put the Mamdani on a5. That square is empty at the start of a standard game.

| Item | Use |
| --- | --- |
| Chess set | Standard starting position |
| Mamdani token | Any distinct piece or coin, placed on a5 |
| 1 six-sided die (d6) | The pothole roll after each turn |
| 2 eight-sided dice (d8) | Pick the pothole's file (1 = a … 8 = h) and rank |
| 1 extra d8 | Saving rolls |
| A few checkers | Mark open potholes |

White moves first, as usual.

## Turn order

Every turn is one move followed by one pothole roll. Play these steps in order:

1. **Move.** Make one legal move: either one of your own pieces or the Mamdani.
2. **Close.** If you opened a pothole on your previous turn, it closes now.
3. **Repair.** Any open pothole next to the Mamdani (any of its 8 surrounding squares) is repaired and removed.
4. **Roll for a pothole.** Roll the d6. Odd: nothing happens and the turn ends. Even: a pothole opens.
5. **Place it.** Roll the two d8s for file and rank. Resolve that square as described in Potholes and Saving rolls.

## Potholes

A pothole destroys whatever stands on its square, unless a saving roll succeeds. It then blocks that square until the player who rolled it finishes their next move.

When the d8s land on a square:

- **King:** kings never fall. Re-roll both d8s.
- **Next to the Mamdani:** the pothole is repaired the moment it opens. Nothing falls. *(proposed)*
- **Already a pothole:** re-roll both d8s. *(proposed)*
- **Would expose the roller's king:** if the piece falling would leave the player who just moved in check, re-roll both d8s. *(proposed)*
- **Empty square:** the pothole opens. Mark it with a checker.
- **Any other piece:** the piece falls in and is lost, unless it is saved (see Saving rolls).
- **The Mamdani:** the Mamdani must make a saving roll (see Saving rolls).

While a pothole is open:

- No piece, and not the Mamdani, may move onto it.
- Bishops, rooks, queens and the Mamdani cannot slide across it. *(proposed)*
- Knights may jump over it but cannot land on it.
- Because sliders can't cross it, a pothole blocks attacks and checks along that line, like a piece would.
- Your move is illegal if your king would be in check after your own pothole closes and nearby potholes are repaired. *(proposed)*

Two potholes can be open at once, one from each player. Each closes on its own schedule.

## The Mamdani

The Mamdani belongs to neither player. It moves like a queen, never captures and fixes potholes around it.

- **Moving it:** on your turn you may move the Mamdani instead of one of your pieces. That uses your whole move.
- **Movement:** any distance in a straight line or diagonal, like a queen. It cannot jump over pieces or potholes, and it can only land on an empty square.
- **Captures:** it never captures, and nobody can capture it.
- **Blocking:** it blocks lines like any piece. It can block a check, and it can block your opponent's escape squares.
- **Your own king:** you may not move the Mamdani so that it leaves your own king in check. *(proposed)*
- **No ping-pong:** you may not move the Mamdani straight back to the square it just came from. *(proposed)* This stops two players stalling by moving it back and forth.
- **Repairs:** any pothole on one of the 8 squares next to the Mamdani is repaired at once. The repair happens after every move, and when a new pothole opens there.
- **Falling in:** the Mamdani can fall into a pothole like any other piece (see Saving rolls). Once it falls, it is gone for the rest of the game.

## Saving rolls

A saving roll is one d8. An odd number saves the piece, and the pothole never opens.

| Who is on the square | Gets a saving roll when | Odd (1, 3, 5, 7) | Even (2, 4, 6, 8) |
| --- | --- | --- | --- |
| A player's piece | The Mamdani could legally move onto that square next turn, along a clear queen line | Piece saved, no pothole | Piece lost, pothole opens |
| The Mamdani | Always | Mamdani saved, no pothole | Mamdani removed for the rest of the game, pothole opens |

- The owner of the piece rolls. For the Mamdani, the player who rolled the pothole rolls. *(proposed)*
- A piece with no clear Mamdani line gets no saving roll and is lost.
- If the Mamdani has already fallen, no more saving rolls are possible.

## Winning and draws

You win by checkmate, as in normal chess. Kings never fall into potholes, so a pothole can't win the game directly.

- **Checkmate:** check where no move of your pieces and no Mamdani move gets you out of check.
- **Stalemate:** a draw, but only if you also have no legal Mamdani move. *(proposed)*
- **50-move rule:** moving the Mamdani does not reset the count. Losing a piece to a pothole does reset it. *(proposed)*
- **Repetition:** a position counts as repeated only if the pieces, the Mamdani and the open potholes are all the same.
- Castling, en passant and promotion work as normal. You cannot castle through or onto a pothole.

## Open questions

The group should play a few test games, then decide these.

- [ ] **Pothole rate:** an even d6 opens a pothole about every other turn. Is that too chaotic? A rate of only on a 6 would be about one in six turns.
- [ ] **Longer potholes:** the original authors suggest potholes stay open for a d6 number of turns. That would give the Mamdani more to repair.
- [ ] **Stalling:** is the ping-pong rule enough, or should a player be barred from moving the Mamdani two turns in a row?
- [ ] **Stalemate:** should a Mamdani move count as a way out of stalemate?
- [ ] **Sliders and potholes:** may bishops, rooks and queens slide across a pothole?
- [ ] **Repair timing:** if a pothole opens next to the Mamdani, is the piece on it safe (current rule), or does it fall first?

## Sources

- [Pot-Hole Chess](https://www.chessvariants.com/boardrules.dir/potholechess.html) by Peter Spicer and Michael Chamberlain, The Chess Variant Pages (2001)
- [Mamdani patch video](https://www.instagram.com/reels/Dd8tV_MxEQL/) (Instagram reel)
