//! Differential test against the Go server. `server_rs/parity` plays seeded
//! random games through the Go `game` package and records every turn: who
//! acted, the move, the dice and what each role then saw. This test replays
//! each game through the Rust actor with the same dice and checks every view
//! matches, as JSON values (the code differs; legal moves may come in any
//! order).
//!
//! By default it runs `go run ./server_rs/parity -n 30` and skips if Go
//! isn't installed. `PARITY_FILE=path` uses a recorded file instead.

#![allow(clippy::unwrap_used, clippy::expect_used)]

mod common;

use std::sync::{Arc, Mutex};

use rules::{D8, Dice, ScriptedDice};
use serde::Deserialize;
use serde_json::Value;
use server::game::{DiceFactory, GameError, GameHandle, GuestId, Hub, View};
use tokio::sync::watch;

use common::{DAY, guest};

#[derive(Deserialize)]
struct Record {
    seed: u64,
    start: Views,
    turns: Vec<Turn>,
}

#[derive(Deserialize)]
struct Views {
    white: Value,
    black: Value,
    spectator: Value,
}

#[derive(Deserialize)]
struct Turn {
    guest: String,
    action: String,
    #[serde(rename = "move")]
    mv: Option<WireMove>,
    seq: usize,
    dice: Vec<u8>,
    error: Option<String>,
    views: Option<Views>,
}

#[derive(Deserialize)]
struct WireMove {
    from: String,
    to: String,
    #[serde(default)]
    promo: Option<String>,
}

impl WireMove {
    fn uci(&self) -> String {
        format!(
            "{}{}{}",
            self.from,
            self.to,
            self.promo.as_deref().unwrap_or("")
        )
    }
}

const WHITE: u8 = 1;
const BLACK: u8 = 2;
const SPECTATOR: u8 = 3;

fn seat(name: &str) -> GuestId {
    guest(if name == "white" { WHITE } else { BLACK })
}

/// Hands the game a script of every roll the Go game made, in order.
fn scripted(rolls: Vec<D8>) -> DiceFactory {
    let script = Arc::new(Mutex::new(Some(ScriptedDice::new(rolls))));
    Arc::new(move || {
        let s = script.lock().unwrap().take().expect("one game per hub");
        Box::new(s) as Box<dyn Dice + Send>
    })
}

/// A view as the parity test compares it: no code, the log cut to its last
/// entry (as the recorder does), legal moves sorted.
fn normalize(mut v: Value) -> Value {
    let o = v.as_object_mut().unwrap();
    o.remove("code");
    if let Some(Value::Array(log)) = o.get_mut("log") {
        let keep = log.len().saturating_sub(1);
        log.drain(..keep);
    }
    if let Some(Value::Array(legal)) = o.get_mut("legal") {
        legal.sort_by_key(ToString::to_string);
    }
    v
}

/// The first path where `go` and `rust` differ, for a readable failure.
fn first_diff(go: &Value, rust: &Value, path: &str) -> Option<String> {
    match (go, rust) {
        (Value::Object(go_map), Value::Object(rust_map)) => go_map
            .keys()
            .chain(rust_map.keys())
            .find_map(|key| match (go_map.get(key), rust_map.get(key)) {
                (Some(g), Some(r)) => first_diff(g, r, &format!("{path}.{key}")),
                (g, r) => Some(format!("{path}.{key}: go {g:?} rust {r:?}")),
            }),
        (Value::Array(go_items), Value::Array(rust_items))
            if go_items.len() == rust_items.len() =>
        {
            go_items
                .iter()
                .zip(rust_items)
                .enumerate()
                .find_map(|(i, (g, r))| first_diff(g, r, &format!("{path}[{i}]")))
        }
        _ if go == rust => None,
        _ => Some(format!("{path}: go {go} rust {rust}")),
    }
}

struct Streams([watch::Receiver<Arc<View>>; 3]);

impl Streams {
    fn check(&mut self, want: &Views, at: &str) {
        for (rx, (role, want)) in self.0.iter_mut().zip([
            ("white", &want.white),
            ("black", &want.black),
            ("spectator", &want.spectator),
        ]) {
            let got = normalize(serde_json::to_value(&**rx.borrow_and_update()).unwrap());
            let want = normalize(want.clone());
            if let Some(diff) = first_diff(&want, &got, role) {
                panic!("{at}: {diff}");
            }
        }
    }
}

async fn replay(rec: &Record) {
    let rolls: Vec<D8> = rec
        .turns
        .iter()
        .flat_map(|t| &t.dice)
        .map(|&r| D8::new(r).unwrap())
        .collect();
    let game: GameHandle = Hub::new(scripted(rolls), DAY).create(guest(WHITE));
    let mut streams = Streams([
        game.join(guest(WHITE)).await.unwrap(),
        game.join(guest(BLACK)).await.unwrap(),
        game.join(guest(SPECTATOR)).await.unwrap(),
    ]);
    streams.check(&rec.start, &format!("seed {} start", rec.seed));

    for (i, t) in rec.turns.iter().enumerate() {
        let who = seat(&t.guest);
        let result = match (t.action.as_str(), &t.mv) {
            ("move", Some(m)) => game.make_move(who, m.uci().parse().unwrap(), t.seq).await,
            ("resign", _) => game.resign(who).await,
            (action, _) => panic!("unknown action {action}"),
        };
        let at = format!(
            "seed {} turn {i} ({} {} {})",
            rec.seed,
            t.guest,
            t.action,
            t.mv.as_ref().map(WireMove::uci).unwrap_or_default()
        );
        match (&t.error, result) {
            (None, Ok(())) => streams.check(t.views.as_ref().unwrap(), &at),
            (Some(want), Err(err @ (GameError::Conflict { .. } | GameError::NotPlayer))) => {
                assert_eq!(&err.to_string(), want, "{at}");
            }
            (want, got) => panic!("{at}: go said {want:?}, rust said {got:?}"),
        }
    }
}

fn recorded_games() -> Option<String> {
    if let Ok(path) = std::env::var("PARITY_FILE") {
        return Some(std::fs::read_to_string(&path).expect("PARITY_FILE"));
    }
    let repo = concat!(env!("CARGO_MANIFEST_DIR"), "/../..");
    match std::process::Command::new("go")
        .args(["run", "./server_rs/parity", "-n", "30"])
        .current_dir(repo)
        .output()
    {
        Ok(out) if out.status.success() => Some(String::from_utf8(out.stdout).unwrap()),
        Ok(out) => panic!(
            "go run ./server_rs/parity failed:\n{}",
            String::from_utf8_lossy(&out.stderr)
        ),
        Err(err) => {
            eprintln!("skipping the parity test: cannot run go ({err})");
            None
        }
    }
}

#[tokio::test]
async fn rust_server_matches_go_server_turn_by_turn() {
    let Some(games) = recorded_games() else {
        return;
    };
    let mut played = 0;
    for line in games.lines().filter(|l| !l.is_empty()) {
        let rec: Record = serde_json::from_str(line).unwrap();
        replay(&rec).await;
        played += 1;
    }
    assert!(played > 0, "no games recorded");
    eprintln!("parity: {played} games match");
}
