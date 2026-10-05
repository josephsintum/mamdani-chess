//! Finds live games by code.

use std::collections::HashMap;
use std::sync::{Arc, Mutex, MutexGuard, PoisonError, Weak};
use std::time::Duration;

use tokio::sync::{mpsc, oneshot, watch};

use super::actor::{self, Cmd, OnExit};
use super::{Code, GameError, GuestId, View};
use rules::{Dice, Move};

/// Makes the dice for each new game.
pub type DiceFactory = Arc<dyn Fn() -> Box<dyn Dice + Send> + Send + Sync>;

/// Maps game codes to live games. Cheap to clone.
#[derive(Clone)]
pub struct Hub(Arc<Inner>);

struct Inner {
    games: Mutex<HashMap<Code, GameHandle>>,
    dice: DiceFactory,
    idle: Duration,
}

impl Hub {
    /// A hub whose games roll with dice from `dice` and are evicted after
    /// `idle` without commands, once they are over or nobody is watching.
    #[must_use]
    pub fn new(dice: DiceFactory, idle: Duration) -> Hub {
        Hub(Arc::new(Inner {
            games: Mutex::new(HashMap::new()),
            dice,
            idle,
        }))
    }

    /// Starts a game with `creator` in White's seat.
    #[must_use]
    pub fn create(&self, creator: GuestId) -> GameHandle {
        let mut games = self.games();
        let code = std::iter::repeat_with(Code::random)
            .find(|c| !games.contains_key(c))
            .unwrap_or_else(Code::random);
        let hub = Arc::downgrade(&self.0);
        let tx = actor::spawn(
            code,
            creator,
            (self.0.dice)(),
            self.0.idle,
            OnExit::new(move || remove(&hub, code)),
        );
        let handle = GameHandle { code, tx };
        games.insert(code, handle.clone());
        handle
    }

    /// The live game with `code`, if any.
    #[must_use]
    pub fn get(&self, code: &Code) -> Option<GameHandle> {
        let mut games = self.games();
        match games.get(code) {
            // A game whose task has stopped is gone, even if its exit hook
            // hasn't run yet.
            Some(h) if h.tx.is_closed() => {
                games.remove(code);
                None
            }
            h => h.cloned(),
        }
    }

    /// How many games are live.
    #[must_use]
    pub fn len(&self) -> usize {
        self.games().len()
    }

    #[must_use]
    pub fn is_empty(&self) -> bool {
        self.len() == 0
    }

    fn games(&self) -> MutexGuard<'_, HashMap<Code, GameHandle>> {
        // The map is valid even if a holder panicked: every update is one
        // insert or remove.
        self.0.games.lock().unwrap_or_else(PoisonError::into_inner)
    }
}

fn remove(hub: &Weak<Inner>, code: Code) {
    if let Some(inner) = hub.upgrade() {
        inner
            .games
            .lock()
            .unwrap_or_else(PoisonError::into_inner)
            .remove(&code);
    }
}

/// Talks to one game's task.
#[derive(Clone)]
pub struct GameHandle {
    code: Code,
    tx: mpsc::Sender<Cmd>,
}

impl GameHandle {
    #[must_use]
    pub fn code(&self) -> Code {
        self.code
    }

    /// Opens a stream for `guest`: the creator plays White, the first other
    /// guest takes Black, everyone after that watches. A guest who comes
    /// back keeps their seat. The receiver already holds the current view.
    ///
    /// # Errors
    ///
    /// [`GameError::Gone`] if the game has stopped.
    pub async fn join(&self, guest: GuestId) -> Result<watch::Receiver<Arc<View>>, GameError> {
        self.call(|reply| Cmd::Join { guest, reply }).await
    }

    /// Plays `guest`'s move. `seq` must equal the number of turns played so
    /// far, which rejects a move made from an out-of-date board. The new
    /// state goes out on the streams.
    ///
    /// # Errors
    ///
    /// [`GameError`] saying why the move was refused.
    pub async fn make_move(&self, guest: GuestId, mv: Move, seq: usize) -> Result<(), GameError> {
        self.call(|reply| Cmd::Move {
            guest,
            mv,
            seq,
            reply,
        })
        .await?
    }

    /// Ends the game with `guest`'s opponent as the winner.
    ///
    /// # Errors
    ///
    /// [`GameError`] saying why the resignation was refused.
    pub async fn resign(&self, guest: GuestId) -> Result<(), GameError> {
        self.call(|reply| Cmd::Resign { guest, reply }).await?
    }

    async fn call<T>(&self, cmd: impl FnOnce(oneshot::Sender<T>) -> Cmd) -> Result<T, GameError> {
        let (reply, answer) = oneshot::channel();
        self.tx
            .send(cmd(reply))
            .await
            .map_err(|_| GameError::Gone)?;
        answer.await.map_err(|_| GameError::Gone)
    }
}
