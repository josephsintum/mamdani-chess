//! The game actor, driven through `Hub` and `GameHandle`. Port of
//! `game/game_test.go`, plus what the Rust design adds: conflicts carry the
//! state, crashed games are forgotten, idle games are evicted.

#![allow(clippy::unwrap_used)]

mod common;

use std::sync::Arc;

use common::{DAY, exploding, guest, hub, mv, odd, script};
use server::game::{Conflict, GameError, GameHandle, LogEntry, Stats, Status, View};
use tokio::sync::watch;

const ALICE: u8 = 1;
const BOB: u8 = 2;
const CAROL: u8 = 3;

async fn join(g: &GameHandle, who: u8) -> watch::Receiver<Arc<View>> {
    g.join(guest(who)).await.unwrap()
}

/// The newest view on a stream. Commands reply only after broadcasting, so
/// once one returns its view is already here.
fn now(rx: &mut watch::Receiver<Arc<View>>) -> Arc<View> {
    rx.borrow_and_update().clone()
}

async fn play(g: &GameHandle, moves: &[&str]) {
    for (i, uci) in moves.iter().enumerate() {
        let who = if i % 2 == 0 { ALICE } else { BOB };
        g.make_move(guest(who), mv(uci), i)
            .await
            .unwrap_or_else(|e| panic!("{uci}: {e}"));
    }
}

fn conflict(r: Result<(), GameError>) -> Conflict {
    match r {
        Err(GameError::Conflict { conflict, .. }) => conflict,
        other => panic!("want a conflict, got {other:?}"),
    }
}

#[tokio::test]
async fn creator_waits_as_white_with_no_moves() {
    let g = hub(odd()).create(guest(ALICE));
    let v = now(&mut join(&g, ALICE).await);
    assert_eq!(
        (v.you, v.status, v.legal.len()),
        ("white", Status::Waiting, 0)
    );
}

#[tokio::test]
async fn second_guest_takes_black_and_white_gets_33_moves() {
    let g = hub(odd()).create(guest(ALICE));
    let mut a = join(&g, ALICE).await;
    let b = now(&mut join(&g, BOB).await);
    assert_eq!(
        (b.you, b.status, b.legal.len()),
        ("black", Status::Playing, 0)
    );
    let a = now(&mut a);
    assert_eq!((a.status, a.legal.len()), (Status::Playing, 33));
}

#[tokio::test]
async fn third_guest_watches_and_reconnect_keeps_seat() {
    let g = hub(odd()).create(guest(ALICE));
    join(&g, BOB).await;
    let c = now(&mut join(&g, CAROL).await);
    assert_eq!((c.you, c.legal.len()), ("spectator", 0));
    assert_eq!(now(&mut join(&g, ALICE).await).you, "white");
}

#[tokio::test]
async fn move_reaches_every_role() {
    let g = hub(odd()).create(guest(ALICE));
    let mut streams = [
        (join(&g, ALICE).await, 0),
        // Black: 19 normal moves (a7a5 is blocked by the Mamdani) + 13 Mamdani moves.
        (join(&g, BOB).await, 32),
        (join(&g, CAROL).await, 0),
    ];
    g.make_move(guest(ALICE), mv("e2e4"), 0).await.unwrap();
    for (rx, legal) in &mut streams {
        let legal = *legal;
        let v = now(rx);
        assert_eq!(
            (v.seq, v.turn, v.board[28], v.board[12]),
            (1, "black", "wP", "")
        );
        assert_eq!(
            v.log,
            [LogEntry {
                san: "e4".into(),
                color: "white",
                dice: "d8 1".into()
            }]
        );
        assert_eq!(v.legal.len(), legal, "{}", v.you);
    }
}

#[tokio::test]
async fn moving_before_black_joins_is_waiting() {
    let g = hub(odd()).create(guest(ALICE));
    assert_eq!(
        conflict(g.make_move(guest(ALICE), mv("e2e4"), 0).await),
        Conflict::Waiting
    );
}

#[tokio::test]
async fn move_errors_in_check_order() {
    let g = hub(odd()).create(guest(ALICE));
    join(&g, BOB).await;
    assert!(matches!(
        g.make_move(guest(CAROL), mv("e2e4"), 0).await,
        Err(GameError::NotPlayer)
    ));
    for (who, uci, seq, want) in [
        (BOB, "e7e5", 0, Conflict::NotYourTurn),
        (ALICE, "e2e4", 3, Conflict::Stale),
        (ALICE, "e2e5", 0, Conflict::Illegal),
    ] {
        assert_eq!(
            conflict(g.make_move(guest(who), mv(uci), seq).await),
            want,
            "{uci}"
        );
    }
}

#[tokio::test]
async fn conflict_carries_the_callers_current_view() {
    let g = hub(odd()).create(guest(ALICE));
    join(&g, BOB).await;
    match g.make_move(guest(BOB), mv("e7e5"), 0).await {
        Err(GameError::Conflict { state, .. }) => {
            assert_eq!((state.you, state.seq, state.turn), ("black", 0, "white"));
        }
        other => panic!("want a conflict, got {other:?}"),
    }
}

#[tokio::test]
async fn pothole_shows_in_view_and_log() {
    // Even roll, then file 4 rank 4: a pothole opens on d4.
    let g = hub(script(&[2, 4, 4])).create(guest(ALICE));
    let mut a = join(&g, ALICE).await;
    join(&g, BOB).await;
    g.make_move(guest(ALICE), mv("e2e4"), 0).await.unwrap();
    let v = now(&mut a);
    assert_eq!(
        v.potholes.iter().map(|p| (p.sq, p.by)).collect::<Vec<_>>(),
        [("d4", "white")]
    );
    assert_eq!(
        v.last.iter().map(|e| e.kind).collect::<Vec<_>>(),
        ["moved", "rolled_pothole", "target", "pothole_opened"]
    );
    assert!(v.log[0].dice.starts_with("d8 2 → d4"), "{:?}", v.log[0]);
}

#[tokio::test]
async fn checkmate_ends_the_game() {
    let g = hub(odd()).create(guest(ALICE));
    let mut a = join(&g, ALICE).await;
    join(&g, BOB).await;
    play(&g, &["f2f3", "e7e5", "g2g4", "d8h4"]).await;
    let v = now(&mut a);
    let result = v.result.as_ref().unwrap();
    assert_eq!(
        (v.status, result.winner, result.reason),
        (Status::Over, Some("black"), "checkmate")
    );
    assert_eq!(
        conflict(g.make_move(guest(ALICE), mv("a2a3"), 4).await),
        Conflict::GameOver
    );
}

#[tokio::test]
async fn resign_before_black_joins_is_waiting() {
    let g = hub(odd()).create(guest(ALICE));
    assert_eq!(conflict(g.resign(guest(ALICE)).await), Conflict::Waiting);
}

#[tokio::test]
async fn spectator_cannot_resign() {
    let g = hub(odd()).create(guest(ALICE));
    join(&g, BOB).await;
    assert!(matches!(
        g.resign(guest(CAROL)).await,
        Err(GameError::NotPlayer)
    ));
}

#[tokio::test]
async fn resigning_ends_the_game_for_the_opponent() {
    let g = hub(odd()).create(guest(ALICE));
    let mut b = join(&g, BOB).await;
    g.resign(guest(ALICE)).await.unwrap();
    let v = now(&mut b);
    let result = v.result.as_ref().unwrap();
    assert_eq!(
        (v.status, result.winner, result.reason, v.legal.len()),
        (Status::Over, Some("black"), "resignation", 0)
    );
    assert_eq!(conflict(g.resign(guest(BOB)).await), Conflict::GameOver);
    assert_eq!(
        conflict(g.make_move(guest(BOB), mv("e7e5"), 0).await),
        Conflict::GameOver
    );
}

#[tokio::test]
async fn resign_after_mate_is_refused() {
    let g = hub(odd()).create(guest(ALICE));
    join(&g, BOB).await;
    play(&g, &["f2f3", "e7e5", "g2g4", "d8h4"]).await;
    assert_eq!(conflict(g.resign(guest(ALICE)).await), Conflict::GameOver);
}

#[tokio::test]
async fn stats_and_lost_pieces_add_up() {
    let g = hub(script(&[
        2, 7, 8, // e4: g8 is hit; the Mamdani on a5 has no line to it, so the knight falls
        2, 4, 2, 5, // e5: d2 is hit; a5-b4-c3-d2 is clear, so White rolls to save: 5 saves it
        2, 2, 4, // Nf3: b4 is next to the Mamdani, so the new pothole is repaired at once
    ]))
    .create(guest(ALICE));
    let mut a = join(&g, ALICE).await;
    join(&g, BOB).await;
    play(&g, &["e2e4", "e7e5", "g1f3"]).await;
    let v = now(&mut a);
    assert_eq!(
        (v.lost.white.as_slice(), v.lost.black.as_slice()),
        (&[][..], &["bN"][..])
    );
    assert_eq!(
        v.stats,
        Stats {
            saving_rolls: 1,
            saved: 1,
            repaired: 1,
            mamdani_fell: false
        }
    );
    assert_eq!(
        v.log[0],
        LogEntry {
            san: "e4".into(),
            color: "white",
            dice: "d8 2 → g8 · bN falls".into()
        }
    );
}

#[tokio::test]
async fn crashed_game_reports_gone_and_is_forgotten() {
    let h = hub(exploding());
    let g = h.create(guest(ALICE));
    join(&g, BOB).await;
    assert!(matches!(
        g.make_move(guest(ALICE), mv("e2e4"), 0).await,
        Err(GameError::Gone)
    ));
    assert!(h.get(&g.code()).is_none());
}

#[tokio::test(start_paused = true)]
async fn finished_game_is_evicted_after_a_quiet_day() {
    let h = hub(odd());
    let g = h.create(guest(ALICE));
    let _watching = join(&g, BOB).await;
    g.resign(guest(ALICE)).await.unwrap();
    tokio::time::sleep(DAY * 2).await;
    assert!(h.get(&g.code()).is_none());
}

#[tokio::test(start_paused = true)]
async fn unwatched_game_is_evicted_after_a_quiet_day() {
    let h = hub(odd());
    let g = h.create(guest(ALICE));
    tokio::time::sleep(DAY * 2).await;
    assert!(h.is_empty());
    assert!(matches!(g.join(guest(ALICE)).await, Err(GameError::Gone)));
}

#[tokio::test(start_paused = true)]
async fn watched_game_in_progress_is_kept() {
    let h = hub(odd());
    let g = h.create(guest(ALICE));
    let _a = join(&g, ALICE).await;
    let _b = join(&g, BOB).await;
    tokio::time::sleep(DAY * 3).await;
    assert!(h.get(&g.code()).is_some());
}
