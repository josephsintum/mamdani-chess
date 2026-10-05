//! The HTTP layer: routes, the SSE stream and the static frontend.

mod error;
mod games;
mod guest;

use std::path::Path;
use std::time::Duration;

use axum::extract::DefaultBodyLimit;
use axum::extract::Request;
use axum::http::HeaderValue;
use axum::http::header::CACHE_CONTROL;
use axum::middleware::{self, Next};
use axum::response::{IntoResponse, Response};
use axum::routing::{get, post};
use axum::{Json, Router};
use serde_json::json;
use tokio_util::sync::CancellationToken;
use tower_http::services::{ServeDir, ServeFile};

pub use error::ApiError;
pub use guest::Guest;

use crate::game::Hub;

/// What every handler can reach.
#[derive(Clone)]
pub struct AppState {
    pub hub: Hub,
    /// Cancelled when the server starts shutting down; open streams end.
    pub shutdown: CancellationToken,
    /// How often an idle stream sends a keep-alive comment.
    pub heartbeat: Duration,
}

/// Request bodies are tiny; anything bigger is a mistake or abuse.
const BODY_LIMIT: usize = 4 * 1024;

/// The whole app: the API under `/api`, a health check, and the built
/// frontend from `web_dir` for everything else.
pub fn router(state: AppState, web_dir: &Path) -> Router {
    let api = Router::new()
        .route("/games", post(games::create))
        .route("/games/{code}/stream", get(games::stream))
        .route("/games/{code}/move", post(games::make_move))
        .route("/games/{code}/resign", post(games::resign))
        .fallback(|| async { ApiError::NotFound("not found") })
        .layer(DefaultBodyLimit::max(BODY_LIMIT));
    Router::new()
        .route(
            "/healthz",
            get(|| async { Json(json!({ "status": "ok" })) }),
        )
        .nest("/api", api)
        .fallback_service(static_files(web_dir))
        .with_state(state)
}

/// Serves the built SvelteKit app. Paths that aren't files fall back to
/// `index.html`, so client-side routes like `/game/K7F3QZ` load the app.
/// Hashed assets under `_app/` don't fall back: a missing one is a 404.
fn static_files(web_dir: &Path) -> Router {
    let assets = ServeDir::new(web_dir.join("_app"))
        .precompressed_br()
        .precompressed_gzip();
    let site = ServeDir::new(web_dir)
        .precompressed_br()
        .precompressed_gzip()
        .fallback(ServeFile::new(web_dir.join("index.html")));
    Router::new()
        .nest_service("/_app", assets)
        .fallback_service(site)
        .layer(middleware::from_fn(cache_control))
}

/// Hashed build assets never change, so browsers may keep them for a year;
/// everything else must be revalidated so a deploy shows up at once.
async fn cache_control(req: Request, next: Next) -> Response {
    let immutable = req.uri().path().starts_with("/_app/immutable/");
    let mut res = next.run(req).await;
    let value = if immutable && res.status().is_success() {
        "public, max-age=31536000, immutable"
    } else {
        "no-cache"
    };
    res.headers_mut()
        .insert(CACHE_CONTROL, HeaderValue::from_static(value));
    res.into_response()
}
