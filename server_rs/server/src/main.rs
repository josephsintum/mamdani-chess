//! Runs Pothole Chess: Mamdani Edition.
//!
//! `PORT` (default 8080) is where it listens; `WEB_DIR` (default
//! `../web/build`, which suits `cargo run` from `server_rs/`) is the built
//! frontend; `RUST_LOG` sets the log level.

use std::path::PathBuf;
use std::time::Duration;

use anyhow::Context;
use tokio::net::TcpListener;
use tokio_util::sync::CancellationToken;
use tower_http::trace::TraceLayer;
use tracing_subscriber::EnvFilter;

use server::game::{Hub, fair_dice};
use server::http::{AppState, router};

/// Games with no commands for this long are dropped once they are over or
/// nobody is watching.
const IDLE_GAME: Duration = Duration::from_secs(24 * 60 * 60);
const HEARTBEAT: Duration = Duration::from_secs(15);
/// How long requests other than streams get to finish on shutdown.
const DRAIN: Duration = Duration::from_secs(10);

#[tokio::main]
async fn main() -> anyhow::Result<()> {
    tracing_subscriber::fmt()
        .with_env_filter(
            EnvFilter::try_from_default_env().unwrap_or_else(|_| EnvFilter::new("info")),
        )
        .init();

    let port: u16 = env_or("PORT", "8080")
        .parse()
        .context("PORT must be a port number")?;
    let web_dir = PathBuf::from(env_or("WEB_DIR", "../web/build"));
    if !web_dir.join("index.html").is_file() {
        tracing::warn!(
            web_dir = %web_dir.display(),
            "no index.html: build the frontend with `pnpm build` in web/, or set WEB_DIR"
        );
    }

    let shutdown = CancellationToken::new();
    let state = AppState {
        hub: Hub::new(fair_dice(), IDLE_GAME),
        shutdown: shutdown.clone(),
        heartbeat: HEARTBEAT,
    };
    let app = router(state, &web_dir).layer(TraceLayer::new_for_http());

    let listener = TcpListener::bind(("0.0.0.0", port))
        .await
        .with_context(|| format!("listening on port {port}"))?;
    tracing::info!(addr = %listener.local_addr()?, "listening");

    let signalled = shutdown.clone();
    let server = axum::serve(listener, app).with_graceful_shutdown(async move {
        wait_for_signal().await;
        tracing::info!("shutting down");
        signalled.cancel(); // ends every open stream
    });
    tokio::select! {
        result = server.into_future() => result.context("server failed")?,
        () = async { shutdown.cancelled().await; tokio::time::sleep(DRAIN).await } => {
            tracing::warn!("requests still running after {DRAIN:?}; stopping anyway");
        }
    }
    Ok(())
}

fn env_or(key: &str, default: &str) -> String {
    std::env::var(key)
        .ok()
        .filter(|v| !v.is_empty())
        .unwrap_or_else(|| default.to_owned())
}

async fn wait_for_signal() {
    let ctrl_c = async {
        if let Err(err) = tokio::signal::ctrl_c().await {
            tracing::error!(%err, "cannot listen for Ctrl-C");
            std::future::pending::<()>().await;
        }
    };
    #[cfg(unix)]
    let term = async {
        match tokio::signal::unix::signal(tokio::signal::unix::SignalKind::terminate()) {
            Ok(mut s) => {
                s.recv().await;
            }
            Err(err) => {
                tracing::error!(%err, "cannot listen for SIGTERM");
                std::future::pending::<()>().await;
            }
        }
    };
    #[cfg(not(unix))]
    let term = std::future::pending::<()>();
    tokio::select! {
        () = ctrl_c => {}
        () = term => {}
    }
}
