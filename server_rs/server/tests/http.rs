//! The HTTP API end to end over a real socket: what the browser sees. Port
//! of `server/server_test.go`, plus the 409 state and stricter static files.

#![allow(clippy::unwrap_used, clippy::expect_used)]

mod common;

use std::path::Path;
use std::pin::Pin;
use std::time::Duration;

use bytes::Bytes;
use futures::{Stream, StreamExt};
use reqwest::StatusCode;
use serde_json::{Value, json};
use server::http::{AppState, router};
use tokio::net::TcpListener;
use tokio::time::timeout;
use tokio_util::sync::CancellationToken;

use common::{hub, odd};

struct TestServer {
    url: String,
    shutdown: CancellationToken,
    _web: tempfile::TempDir,
}

async fn start() -> TestServer {
    start_with_heartbeat(Duration::from_secs(15)).await
}

async fn start_with_heartbeat(heartbeat: Duration) -> TestServer {
    let web = tempfile::tempdir().unwrap();
    write(web.path(), "index.html", "<!doctype html>app shell");
    write(web.path(), "_app/immutable/app.js", "console.log(1)");
    write(web.path(), "favicon.svg", "<svg/>");
    let shutdown = CancellationToken::new();
    let state = AppState {
        hub: hub(odd()),
        shutdown: shutdown.clone(),
        heartbeat,
    };
    let app = router(state, web.path());
    let listener = TcpListener::bind("127.0.0.1:0").await.unwrap();
    let url = format!("http://{}", listener.local_addr().unwrap());
    tokio::spawn(async move { axum::serve(listener, app).await });
    TestServer {
        url,
        shutdown,
        _web: web,
    }
}

fn write(dir: &Path, name: &str, body: &str) {
    let path = dir.join(name);
    std::fs::create_dir_all(path.parent().unwrap()).unwrap();
    std::fs::write(path, body).unwrap();
}

/// One browser: its own cookie jar, so its own guest ID.
struct Player {
    c: reqwest::Client,
    url: String,
}

fn player(s: &TestServer) -> Player {
    Player {
        c: reqwest::Client::builder()
            .cookie_store(true)
            .build()
            .unwrap(),
        url: s.url.clone(),
    }
}

impl Player {
    async fn post(&self, path: &str, body: &str) -> (StatusCode, String) {
        let resp = self
            .c
            .post(format!("{}{path}", self.url))
            .header("content-type", "application/json")
            .body(body.to_owned())
            .send()
            .await
            .unwrap();
        (resp.status(), resp.text().await.unwrap())
    }

    async fn create(&self) -> String {
        let (status, body) = self.post("/api/games", "").await;
        assert_eq!(status, StatusCode::CREATED, "{body}");
        let v: Value = serde_json::from_str(&body).unwrap();
        v["code"].as_str().unwrap().to_owned()
    }

    async fn stream(&self, code: &str) -> Sse {
        let resp = self
            .c
            .get(format!("{}/api/games/{code}/stream", self.url))
            .send()
            .await
            .unwrap();
        assert_eq!(resp.status(), StatusCode::OK);
        assert_eq!(resp.headers()["content-type"], "text/event-stream");
        assert_eq!(resp.headers()["x-accel-buffering"], "no");
        Sse {
            body: Box::pin(resp.bytes_stream()),
            buf: String::new(),
        }
    }
}

/// Reads server-sent events frame by frame.
struct Sse {
    body: Pin<Box<dyn Stream<Item = reqwest::Result<Bytes>> + Send>>,
    buf: String,
}

#[derive(Debug, PartialEq)]
enum Frame {
    Event { name: String, data: String },
    Comment(String),
    End,
}

impl Sse {
    async fn next(&mut self) -> Frame {
        loop {
            if let Some(i) = self.buf.find("\n\n") {
                let frame: String = self.buf.drain(..i + 2).collect();
                return parse_frame(&frame);
            }
            match timeout(Duration::from_secs(2), self.body.next()).await {
                Ok(Some(chunk)) => self
                    .buf
                    .push_str(std::str::from_utf8(&chunk.unwrap()).unwrap()),
                Ok(None) => return Frame::End,
                Err(elapsed) => panic!("no frame within 2s: {elapsed}"),
            }
        }
    }

    /// The next `state` event's view.
    async fn state(&mut self) -> Value {
        match self.next().await {
            Frame::Event { name, data } if name == "state" => serde_json::from_str(&data).unwrap(),
            other => panic!("want a state event, got {other:?}"),
        }
    }
}

fn parse_frame(frame: &str) -> Frame {
    let (mut name, mut data) = (String::new(), String::new());
    for line in frame.lines() {
        if let Some(text) = line.strip_prefix(':') {
            return Frame::Comment(text.trim().to_owned());
        } else if let Some(v) = line.strip_prefix("event:") {
            v.trim_start().clone_into(&mut name);
        } else if let Some(v) = line.strip_prefix("data:") {
            v.trim_start().clone_into(&mut data);
        }
    }
    Frame::Event { name, data }
}

#[tokio::test]
async fn friend_game_over_http() {
    let s = start().await;
    let (alice, bob, carol) = (player(&s), player(&s), player(&s));
    let code = alice.create().await;

    let mut a = alice.stream(&code).await;
    let v = a.state().await;
    assert_eq!(
        (&v["you"], &v["status"]),
        (&json!("white"), &json!("waiting"))
    );

    let mut b = bob.stream(&code).await;
    let v = b.state().await;
    assert_eq!(
        (&v["you"], &v["status"]),
        (&json!("black"), &json!("playing"))
    );

    let v = a.state().await;
    assert_eq!(v["status"], "playing");
    assert_eq!(v["legal"].as_array().unwrap().len(), 33);

    assert_eq!(carol.stream(&code).await.state().await["you"], "spectator");

    let (status, body) = alice
        .post(
            &format!("/api/games/{code}/move"),
            r#"{"from":"e2","to":"e4","seq":0}"#,
        )
        .await;
    assert_eq!(status, StatusCode::NO_CONTENT, "{body}");

    let v = b.state().await;
    assert_eq!((&v["seq"], &v["board"][28]), (&json!(1), &json!("wP")));
    assert_eq!(v["legal"].as_array().unwrap().len(), 32);
    assert_eq!(
        v["log"],
        json!([{"san": "e4", "color": "white", "dice": "d8 1"}])
    );
    let v = a.state().await;
    assert_eq!((&v["seq"], &v["legal"]), (&json!(1), &json!([])));
}

#[tokio::test]
async fn move_errors_map_to_status_codes() {
    let s = start().await;
    let (alice, bob, carol) = (player(&s), player(&s), player(&s));
    let code = alice.create().await;
    alice.stream(&code).await.state().await;
    bob.stream(&code).await.state().await;
    let mv = format!("/api/games/{code}/move");
    let too_big = format!(r#"{{"from":"{}"}}"#, "x".repeat(5000));
    for (who, path, body, want) in [
        (
            &carol,
            mv.as_str(),
            r#"{"from":"e2","to":"e4","seq":0}"#,
            StatusCode::FORBIDDEN,
        ),
        (
            &bob,
            &mv,
            r#"{"from":"e7","to":"e5","seq":0}"#,
            StatusCode::CONFLICT,
        ),
        (
            &alice,
            &mv,
            r#"{"from":"e2","to":"e4","seq":5}"#,
            StatusCode::CONFLICT,
        ),
        (
            &alice,
            &mv,
            r#"{"from":"e2","to":"e5","seq":0}"#,
            StatusCode::CONFLICT,
        ),
        (
            &alice,
            &mv,
            r#"{"from":"e9","to":"e4","seq":0}"#,
            StatusCode::BAD_REQUEST,
        ),
        (&alice, &mv, "not json", StatusCode::BAD_REQUEST),
        (&alice, &mv, &too_big, StatusCode::BAD_REQUEST),
        (
            &alice,
            "/api/games/NOPE99/move",
            r#"{"from":"e2","to":"e4","seq":0}"#,
            StatusCode::NOT_FOUND,
        ),
    ] {
        let (status, text) = who.post(path, body).await;
        assert_eq!(status, want, "{path} {body:.40}: {text}");
        let v: Value = serde_json::from_str(&text).unwrap();
        assert!(v["error"].is_string(), "{text}");
    }
}

#[tokio::test]
async fn conflict_body_has_message_and_state() {
    let s = start().await;
    let (alice, bob) = (player(&s), player(&s));
    let code = alice.create().await;
    bob.stream(&code).await.state().await;
    let (status, text) = bob
        .post(
            &format!("/api/games/{code}/move"),
            r#"{"from":"e7","to":"e5","seq":0}"#,
        )
        .await;
    assert_eq!(status, StatusCode::CONFLICT);
    let v: Value = serde_json::from_str(&text).unwrap();
    assert_eq!(v["error"], "not your turn");
    assert_eq!(
        (&v["state"]["you"], &v["state"]["seq"]),
        (&json!("black"), &json!(0))
    );
}

#[tokio::test]
async fn unknown_game_stream_is_404() {
    let s = start().await;
    let resp = reqwest::get(format!("{}/api/games/NOPE99/stream", s.url))
        .await
        .unwrap();
    assert_eq!(resp.status(), StatusCode::NOT_FOUND);
}

async fn guest_cookie(s: &TestServer, forwarded_https: bool) -> String {
    let mut req = reqwest::Client::new().post(format!("{}/api/games", s.url));
    if forwarded_https {
        req = req.header("x-forwarded-proto", "https");
    }
    let resp = req.send().await.unwrap();
    resp.headers()["set-cookie"].to_str().unwrap().to_owned()
}

#[tokio::test]
async fn guest_cookie_is_a_year_long_http_only_lax_cookie() {
    let s = start().await;
    let c = guest_cookie(&s, false).await;
    let value = c
        .strip_prefix("guest=")
        .and_then(|v| v.split(';').next())
        .unwrap();
    assert_eq!(value.len(), 32, "{c}");
    for attr in ["HttpOnly", "SameSite=Lax", "Path=/", "Max-Age=31536000"] {
        assert!(c.contains(attr), "{c} lacks {attr}");
    }
    assert!(!c.contains("Secure"), "{c}");
}

#[tokio::test]
async fn guest_cookie_is_secure_behind_https_proxy() {
    let s = start().await;
    assert!(guest_cookie(&s, true).await.contains("Secure"));
}

#[tokio::test]
async fn idle_stream_gets_keep_alive_comments() {
    let s = start_with_heartbeat(Duration::from_millis(20)).await;
    let alice = player(&s);
    let mut r = alice.stream(&alice.create().await).await;
    r.state().await;
    assert_eq!(r.next().await, Frame::Comment("ping".into()));
}

#[tokio::test]
async fn static_files_spa_fallback_and_cache_headers() {
    let s = start().await;
    for (path, status, body, cache) in [
        ("/", 200, "app shell", Some("no-cache")),
        ("/game/K7F3QZ", 200, "app shell", Some("no-cache")),
        ("/favicon.svg", 200, "<svg/>", Some("no-cache")),
        (
            "/_app/immutable/app.js",
            200,
            "console.log(1)",
            Some("public, max-age=31536000, immutable"),
        ),
        ("/_app/immutable/missing.js", 404, "", Some("no-cache")),
        ("/api/nope", 404, r#"{"error":"not found"}"#, None),
        ("/healthz", 200, r#"{"status":"ok"}"#, None),
    ] {
        let resp = reqwest::get(format!("{}{path}", s.url)).await.unwrap();
        assert_eq!(resp.status().as_u16(), status, "{path}");
        if let Some(cache) = cache {
            assert_eq!(resp.headers()["cache-control"], cache, "{path}");
        }
        let text = resp.text().await.unwrap();
        assert!(text.contains(body), "{path}: {text:?}");
    }
}

#[tokio::test]
async fn posting_to_a_page_is_405() {
    let s = start().await;
    let resp = reqwest::Client::new()
        .post(format!("{}/", s.url))
        .send()
        .await
        .unwrap();
    assert_eq!(resp.status(), StatusCode::METHOD_NOT_ALLOWED);
}

#[tokio::test]
async fn shutdown_ends_streams_but_not_requests() {
    let s = start().await;
    let alice = player(&s);
    let mut r = alice.stream(&alice.create().await).await;
    r.state().await;
    s.shutdown.cancel();
    assert_eq!(r.next().await, Frame::End);
    alice.create().await; // other requests still work
}

#[tokio::test]
async fn resign_over_http() {
    let s = start().await;
    let (alice, bob, carol) = (player(&s), player(&s), player(&s));
    let code = alice.create().await;
    alice.stream(&code).await.state().await;
    let mut b = bob.stream(&code).await;
    b.state().await;
    let resign = format!("/api/games/{code}/resign");
    assert_eq!(carol.post(&resign, "").await.0, StatusCode::FORBIDDEN);
    assert_eq!(alice.post(&resign, "").await.0, StatusCode::NO_CONTENT);
    let v = b.state().await;
    assert_eq!(
        v["result"],
        json!({"winner": "black", "draw": false, "reason": "resignation"})
    );
    assert_eq!(bob.post(&resign, "").await.0, StatusCode::CONFLICT);
    assert_eq!(
        alice.post("/api/games/NOPE99/resign", "").await.0,
        StatusCode::NOT_FOUND
    );
}
