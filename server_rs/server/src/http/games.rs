//! The game API: create, stream, move, resign.

use std::convert::Infallible;
use std::sync::Arc;

use axum::Json;
use axum::extract::{FromRequest, Path, State};
use axum::http::StatusCode;
use axum::http::header::HeaderName;
use axum::response::sse::{Event, KeepAlive, Sse};
use axum_extra::extract::CookieJar;
use futures_util::{Stream, StreamExt};
use serde::Deserialize;
use serde_json::json;
use tokio_stream::wrappers::WatchStream;

use super::{ApiError, AppState, Guest};
use crate::game::{Code, GameHandle, View};
use rules::{Move, Promo};

/// Starts a friend game with the caller as White.
pub(super) async fn create(
    State(app): State<AppState>,
    guest: Guest,
) -> (StatusCode, CookieJar, Json<serde_json::Value>) {
    let game = app.hub.create(guest.id);
    (
        StatusCode::CREATED,
        guest.jar,
        Json(json!({ "code": game.code() })),
    )
}

/// Sends the caller's view of the game on connect and after every change.
/// Opening it takes Black's seat if that is still free.
pub(super) async fn stream(
    State(app): State<AppState>,
    guest: Guest,
    Path(code): Path<String>,
) -> (
    CookieJar,
    Result<impl axum::response::IntoResponse, ApiError>,
) {
    let open = async {
        let views = find(&app, &code)?.join(guest.id).await?;
        let events = state_events(views).take_until(app.shutdown.clone().cancelled_owned());
        let sse =
            Sse::new(events).keep_alive(KeepAlive::new().interval(app.heartbeat).text("ping"));
        // Stop proxies from buffering the stream.
        Ok(([(HeaderName::from_static("x-accel-buffering"), "no")], sse))
    };
    (guest.jar, open.await)
}

fn state_events(
    views: tokio::sync::watch::Receiver<Arc<View>>,
) -> impl Stream<Item = Result<Event, Infallible>> {
    WatchStream::new(views).filter_map(|v| async move {
        match Event::default().event("state").json_data(&*v) {
            Ok(e) => Some(Ok(e)),
            Err(err) => {
                tracing::error!(%err, "could not encode a view");
                None
            }
        }
    })
}

/// A move as the browser sends it.
#[derive(Deserialize)]
pub(super) struct MoveBody {
    from: String,
    to: String,
    #[serde(default)]
    promo: Option<String>,
    seq: usize,
}

impl MoveBody {
    fn to_move(&self) -> Option<Move> {
        let promo = match self.promo.as_deref() {
            None | Some("") => None,
            Some(p) => Some(Promo::from_uci(p)?),
        };
        Some(Move {
            from: self.from.parse().ok()?,
            to: self.to.parse().ok()?,
            promo,
        })
    }
}

/// `Json`, but a body that doesn't parse is our usual `{"error": ...}`.
#[derive(FromRequest)]
#[from_request(via(Json), rejection(ApiError))]
pub(super) struct Body<T>(T);

/// Plays the caller's move. The new state arrives on the stream.
pub(super) async fn make_move(
    State(app): State<AppState>,
    guest: Guest,
    Path(code): Path<String>,
    Body(body): Body<MoveBody>,
) -> (CookieJar, Result<StatusCode, ApiError>) {
    let result = async {
        let game = find(&app, &code)?;
        let mv = body.to_move().ok_or(ApiError::BadRequest("bad move"))?;
        game.make_move(guest.id, mv, body.seq).await?;
        Ok(StatusCode::NO_CONTENT)
    };
    (guest.jar, result.await)
}

/// Ends the game; the caller's opponent wins.
pub(super) async fn resign(
    State(app): State<AppState>,
    guest: Guest,
    Path(code): Path<String>,
) -> (CookieJar, Result<StatusCode, ApiError>) {
    let result = async {
        find(&app, &code)?.resign(guest.id).await?;
        Ok(StatusCode::NO_CONTENT)
    };
    (guest.jar, result.await)
}

fn find(app: &AppState, code: &str) -> Result<GameHandle, ApiError> {
    code.parse::<Code>()
        .ok()
        .and_then(|c| app.hub.get(&c))
        .ok_or(ApiError::NotFound("game not found"))
}
