//! How failures become responses: always `{"error": "<message>"}`, plus the
//! caller's current `state` for a conflict.

use axum::Json;
use axum::extract::rejection::JsonRejection;
use axum::http::StatusCode;
use axum::response::{IntoResponse, Response};
use serde::Serialize;

use crate::game::{GameError, View};

/// Everything an API handler can fail with.
#[derive(Debug, thiserror::Error)]
pub enum ApiError {
    #[error("{0}")]
    BadRequest(&'static str),
    #[error("{0}")]
    NotFound(&'static str),
    #[error(transparent)]
    Game(#[from] GameError),
}

impl From<JsonRejection> for ApiError {
    fn from(_: JsonRejection) -> ApiError {
        ApiError::BadRequest("bad request body")
    }
}

#[derive(Serialize)]
struct ErrorBody<'a> {
    error: &'a str,
    #[serde(skip_serializing_if = "Option::is_none")]
    state: Option<&'a View>,
}

impl IntoResponse for ApiError {
    fn into_response(self) -> Response {
        let (status, state) = match &self {
            ApiError::BadRequest(_) => (StatusCode::BAD_REQUEST, None),
            ApiError::NotFound(_) => (StatusCode::NOT_FOUND, None),
            ApiError::Game(GameError::NotPlayer) => (StatusCode::FORBIDDEN, None),
            ApiError::Game(GameError::Conflict { state, .. }) => {
                (StatusCode::CONFLICT, Some(state.as_ref()))
            }
            ApiError::Game(GameError::Gone) => {
                tracing::error!("request reached a game whose task has stopped");
                let body = ErrorBody {
                    error: "internal error",
                    state: None,
                };
                return (StatusCode::INTERNAL_SERVER_ERROR, Json(body)).into_response();
            }
        };
        let error = self.to_string();
        (
            status,
            Json(ErrorBody {
                error: &error,
                state,
            }),
        )
            .into_response()
    }
}
