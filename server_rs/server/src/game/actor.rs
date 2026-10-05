//! The task that owns one game. Commands arrive on a channel and run one at
//! a time, so the state needs no lock, and later the clock, dice-pause and
//! disconnect timers can be more arms of the same `select!`.

use std::sync::Arc;
use std::time::Duration;

use tokio::sync::{mpsc, oneshot, watch};
use tokio::time::{Instant, sleep_until};

use super::view::{
    EventView, LogEntry, Lost, MoveView, PotholeView, ResultView, Stats, Status, View, describe,
    fallen_piece,
};
use super::{Code, Conflict, GameError, GuestId, Role};
use rules::{Color, Dice, Event, Move, Occupant, Piece, Square};

/// What a [`super::GameHandle`] asks of the game.
pub(super) enum Cmd {
    /// Opens a stream: takes Black's seat if it is free.
    Join {
        guest: GuestId,
        reply: oneshot::Sender<watch::Receiver<Arc<View>>>,
    },
    Move {
        guest: GuestId,
        mv: Move,
        seq: usize,
        reply: oneshot::Sender<Result<(), GameError>>,
    },
    Resign {
        guest: GuestId,
        reply: oneshot::Sender<Result<(), GameError>>,
    },
}

/// Runs `f` when dropped: the actor uses it to leave the hub, whether it
/// stops normally or panics.
pub(super) struct OnExit(Option<Box<dyn FnOnce() + Send>>);

impl OnExit {
    pub(super) fn new(f: impl FnOnce() + Send + 'static) -> OnExit {
        OnExit(Some(Box::new(f)))
    }
}

impl Drop for OnExit {
    fn drop(&mut self) {
        if let Some(f) = self.0.take() {
            f();
        }
    }
}

/// Starts a game with `creator` in White's seat and returns its command
/// channel. The game stops after `idle` without commands once it is over or
/// nobody is watching.
pub(super) fn spawn(
    code: Code,
    creator: GuestId,
    dice: Box<dyn Dice + Send>,
    idle: Duration,
    on_exit: OnExit,
) -> mpsc::Sender<Cmd> {
    let (tx, rx) = mpsc::channel(32);
    let state = State {
        code,
        game: rules::Game::new(),
        dice,
        seats: [Some(creator), None],
        last: Vec::new(),
        log: Vec::new(),
        lost: [Vec::new(), Vec::new()],
        stats: Stats::default(),
    };
    let views = Role::ALL.map(|role| watch::Sender::new(Arc::new(state.view(role))));
    tokio::spawn(Actor { state, views }.run(rx, idle, on_exit));
    tx
}

struct Actor {
    state: State,
    /// The latest view for each role. `watch` keeps only the newest value,
    /// so a slow reader skips stale views and never holds the game up.
    views: [watch::Sender<Arc<View>>; 3],
}

impl Actor {
    async fn run(mut self, mut rx: mpsc::Receiver<Cmd>, idle: Duration, _on_exit: OnExit) {
        let mut deadline = Instant::now() + idle;
        loop {
            tokio::select! {
                cmd = rx.recv() => {
                    let Some(cmd) = cmd else { break };
                    self.handle(cmd);
                    deadline = Instant::now() + idle;
                }
                () = sleep_until(deadline) => {
                    if self.state.game.outcome().is_some() || self.watchers() == 0 {
                        tracing::info!(code = %self.state.code, "evicting idle game");
                        break;
                    }
                    deadline = Instant::now() + idle;
                }
            }
        }
    }

    fn watchers(&self) -> usize {
        self.views.iter().map(watch::Sender::receiver_count).sum()
    }

    fn handle(&mut self, cmd: Cmd) {
        match cmd {
            Cmd::Join { guest, reply } => {
                if self.state.join(guest) {
                    self.broadcast(); // White's view changes from waiting to playing
                }
                let role = self.state.role_of(guest);
                let _ = reply.send(self.views[role.index()].subscribe());
            }
            Cmd::Move {
                guest,
                mv,
                seq,
                reply,
            } => {
                let result = self.state.make_move(guest, mv, seq);
                self.finish(guest, result, reply);
            }
            Cmd::Resign { guest, reply } => {
                let result = self.state.resign(guest);
                self.finish(guest, result, reply);
            }
        }
    }

    /// Broadcasts after a change, or attaches the caller's current view to a
    /// conflict, then replies.
    fn finish(
        &self,
        guest: GuestId,
        result: Result<(), Refusal>,
        reply: oneshot::Sender<Result<(), GameError>>,
    ) {
        let result = match result {
            Ok(()) => {
                self.broadcast();
                Ok(())
            }
            Err(Refusal::NotPlayer) => Err(GameError::NotPlayer),
            Err(Refusal::Conflict(conflict)) => Err(GameError::Conflict {
                conflict,
                state: self.views[self.state.role_of(guest).index()]
                    .borrow()
                    .clone(),
            }),
        };
        let _ = reply.send(result);
    }

    fn broadcast(&self) {
        for role in Role::ALL {
            self.views[role.index()].send_replace(Arc::new(self.state.view(role)));
        }
    }
}

#[derive(Clone, Copy)]
enum Refusal {
    NotPlayer,
    Conflict(Conflict),
}

impl From<Conflict> for Refusal {
    fn from(c: Conflict) -> Refusal {
        Refusal::Conflict(c)
    }
}

/// Everything about one game except how it is delivered.
struct State {
    code: Code,
    game: rules::Game,
    dice: Box<dyn Dice + Send>,
    /// The guest in each colour's seat. Seats are never freed: a player who
    /// reloads the page gets theirs back.
    seats: [Option<GuestId>; 2],
    last: Vec<Event>,
    log: Vec<LogEntry>,
    lost: [Vec<&'static str>; 2],
    stats: Stats,
}

impl State {
    /// Seats `guest` as Black if that seat is free and they aren't White.
    /// Returns whether the seat was taken.
    fn join(&mut self, guest: GuestId) -> bool {
        let [white, black] = &mut self.seats;
        if black.is_none() && *white != Some(guest) {
            *black = Some(guest);
            return true;
        }
        false
    }

    fn seat_of(&self, guest: GuestId) -> Option<Color> {
        Color::ALL
            .into_iter()
            .find(|c| self.seats[c.index()] == Some(guest))
    }

    fn role_of(&self, guest: GuestId) -> Role {
        self.seat_of(guest).map_or(Role::Spectator, Role::seat)
    }

    fn status(&self) -> Status {
        if self.game.outcome().is_some() {
            Status::Over
        } else if self.seats[Color::Black.index()].is_none() {
            Status::Waiting
        } else {
            Status::Playing
        }
    }

    /// The checks every command makes, in order: seated, not over, not
    /// waiting.
    fn seated_and_playing(&self, guest: GuestId) -> Result<Color, Refusal> {
        let color = self.seat_of(guest).ok_or(Refusal::NotPlayer)?;
        match self.status() {
            Status::Over => Err(Conflict::GameOver.into()),
            Status::Waiting => Err(Conflict::Waiting.into()),
            Status::Playing => Ok(color),
        }
    }

    fn make_move(&mut self, guest: GuestId, mv: Move, seq: usize) -> Result<(), Refusal> {
        let color = self.seated_and_playing(guest)?;
        let pos = self.game.position();
        if color != pos.turn() {
            return Err(Conflict::NotYourTurn.into());
        }
        if seq != self.game.turns().len() {
            return Err(Conflict::Stale.into());
        }
        if !pos.is_legal(mv) {
            return Err(Conflict::Illegal.into());
        }
        let san = pos.san(mv); // before the move: SAN reads the old position
        let events = self
            .game
            .play(mv, &mut *self.dice)
            .map_err(|_| Conflict::Illegal)?;
        self.log.push(LogEntry {
            san,
            color: color.name(),
            dice: describe(&events),
        });
        self.tally(&events);
        self.last = events;
        Ok(())
    }

    fn resign(&mut self, guest: GuestId) -> Result<(), Refusal> {
        let color = self.seated_and_playing(guest)?;
        self.game.resign(color).map_err(|_| Conflict::GameOver)?;
        self.last.clear();
        Ok(())
    }

    /// Adds a turn's events to the stats and lost-piece lists.
    fn tally(&mut self, events: &[Event]) {
        for e in events {
            if let Some(p) = fallen_piece(*e) {
                self.lost[p.color().index()].push(p.code());
            }
            match *e {
                Event::Fell {
                    occupant: Occupant::Mamdani,
                    ..
                } => self.stats.mamdani_fell = true,
                Event::SavingRoll { saved, .. } => {
                    self.stats.saving_rolls += 1;
                    self.stats.saved += u32::from(saved);
                }
                Event::Repaired { .. } => self.stats.repaired += 1,
                _ => {}
            }
        }
    }

    fn view(&self, role: Role) -> View {
        let p = self.game.position();
        let status = self.status();
        let to_move = status == Status::Playing && role.color() == Some(p.turn());
        View {
            code: self.code,
            status,
            you: role.name(),
            board: Square::all()
                .map(|s| p.piece_at(s).map_or("", Piece::code))
                .collect(),
            mamdani: p.mamdani().map_or("", Square::name),
            potholes: Color::ALL
                .into_iter()
                .filter_map(|c| {
                    p.pothole(c).map(|s| PotholeView {
                        sq: s.name(),
                        by: c.name(),
                    })
                })
                .collect(),
            turn: p.turn().name(),
            check: p.in_check(p.turn()),
            legal: if to_move {
                p.legal_moves().into_iter().map(MoveView::from).collect()
            } else {
                Vec::new()
            },
            last: self.last.iter().map(EventView::from).collect(),
            log: self.log.clone(),
            lost: Lost {
                white: self.lost[0].clone(),
                black: self.lost[1].clone(),
            },
            stats: self.stats,
            result: self.game.outcome().map(|o| ResultView {
                winner: o.winner.map(Color::name),
                draw: o.winner.is_none(),
                reason: o.reason.as_str(),
            }),
            seq: self.game.turns().len(),
        }
    }
}
